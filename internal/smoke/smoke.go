package smoke

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
	"cloudmanager/internal/providers"
)

type Summary struct {
	Checked int
	Passed  int
	Failed  int
}

func Run(cfg config.AppConfig, filter string, out io.Writer) (Summary, error) {
	summary := Summary{}
	contexts, warnings, err := loadContexts(cfg, filter)
	if err != nil {
		return summary, err
	}

	if len(warnings) > 0 {
		for _, warning := range warnings {
			fmt.Fprintf(out, "warning: %s\n", warning)
		}
	}
	if len(contexts) == 0 {
		return summary, fmt.Errorf("no cloud contexts found for smoke test")
	}

	provider := providers.GetProvider(cfg)
	var failures []string

	fmt.Fprintf(out, "backend: %s\n", cfg.Backend)
	for _, cloudCtx := range contexts {
		summary.Checked++
		fmt.Fprintf(out, "\n[%s] %s / %s\n", cloudCtx.Provider, cloudCtx.DisplayName(), cloudCtx.Region)

		vms, err := provider.FetchVMs(context.Background(), cloudCtx)
		if err != nil {
			summary.Failed++
			failures = append(failures, fmt.Sprintf("%s %s/%s fetch failed: %v", cloudCtx.Provider, cloudCtx.DisplayName(), cloudCtx.Region, err))
			fmt.Fprintf(out, "  fetch_vms: FAIL (%v)\n", err)
			continue
		}

		fmt.Fprintf(out, "  fetch_vms: ok (%d VMs)\n", len(vms))
		if len(vms) == 0 {
			summary.Passed++
			fmt.Fprintf(out, "  sample_vm: skipped (no VMs)\n")
			continue
		}

		sample := vms[0]
		if _, err := provider.ExecuteAction(context.Background(), "Describe", sample, cloudCtx); err != nil {
			summary.Failed++
			failures = append(failures, fmt.Sprintf("%s %s/%s describe failed: %v", cloudCtx.Provider, cloudCtx.DisplayName(), cloudCtx.Region, err))
			fmt.Fprintf(out, "  describe: FAIL (%v)\n", err)
			continue
		}
		fmt.Fprintf(out, "  describe: ok (%s)\n", sample.Name)

		sshCmd, err := provider.GetSSHCmd(context.Background(), sample, cloudCtx)
		if err != nil || sshCmd == nil {
			summary.Failed++
			failures = append(failures, fmt.Sprintf("%s %s/%s ssh build failed: %v", cloudCtx.Provider, cloudCtx.DisplayName(), cloudCtx.Region, err))
			fmt.Fprintf(out, "  ssh: FAIL (%v)\n", err)
			continue
		}
		fmt.Fprintf(out, "  ssh: ok (%s)\n", strings.Join(sshCmd.Args, " "))

		previewMap := previewCommands(sample, cloudCtx)
		actions := make([]string, 0, len(previewMap))
		for action := range previewMap {
			actions = append(actions, action)
		}
		sort.Strings(actions)
		for _, action := range actions {
			preview := previewMap[action]
			fmt.Fprintf(out, "  %s_preview: %s\n", strings.ToLower(action), strings.Join(preview, " "))
		}

		summary.Passed++
	}

	if len(failures) > 0 {
		return summary, errors.New(strings.Join(failures, " | "))
	}
	return summary, nil
}

func loadContexts(cfg config.AppConfig, filter string) ([]core.CloudContext, []string, error) {
	var contexts []core.CloudContext
	var warnings []string

	targets, err := targetProviders(filter)
	if err != nil {
		return nil, nil, err
	}

	managed := config.ManagedContexts(cfg)
	for _, provider := range targets {
		for _, ctx := range managed {
			if ctx.Provider != provider {
				continue
			}
			contexts = append(contexts, ctx)
		}
		if config.HasManagedContextsForProvider(cfg, provider) {
			continue
		}
		discovered, providerWarnings := discoverProviderContexts(provider)
		contexts = append(contexts, discovered...)
		warnings = append(warnings, providerWarnings...)
	}

	sort.Slice(contexts, func(i, j int) bool {
		if contexts[i].Provider != contexts[j].Provider {
			return contexts[i].Provider < contexts[j].Provider
		}
		if contexts[i].AccountID != contexts[j].AccountID {
			return contexts[i].AccountID < contexts[j].AccountID
		}
		if contexts[i].AccountName != contexts[j].AccountName {
			return contexts[i].AccountName < contexts[j].AccountName
		}
		return contexts[i].Region < contexts[j].Region
	})

	return contexts, warnings, nil
}

func previewCommands(vm core.VM, cloudCtx core.CloudContext) map[string][]string {
	switch cloudCtx.Provider {
	case "AWS":
		profile := cloudCtx.CredentialProfile
		region := cloudCtx.Region
		
		buildCmd := func(base ...string) []string {
			cmd := append([]string{"aws"}, base...)
			if profile != "" {
				cmd = append(cmd, "--profile", profile)
			}
			if region != "" {
				cmd = append(cmd, "--region", region)
			}
			return cmd
		}

		sshArgs := []string{"ssm", "start-session", "--target", vm.ID}
		if vm.PublicIP != "" && vm.PublicIP != "-" {
			sshArgs = []string{"ec2-instance-connect", "ssh", "--instance-id", vm.ID}
		}

		return map[string][]string{
			"Restart": buildCmd("ec2", "reboot-instances", "--instance-ids", vm.ID, "--dry-run"),
			"SSH":     buildCmd(sshArgs...),
			"Start":   buildCmd("ec2", "start-instances", "--instance-ids", vm.ID, "--dry-run"),
			"Stop":    buildCmd("ec2", "stop-instances", "--instance-ids", vm.ID, "--dry-run"),
		}
	case "GCP":
		return map[string][]string{
			"Restart": {"gcloud", "compute", "instances", "reset", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone},
			"SSH":     {"gcloud", "compute", "ssh", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone},
			"Start":   {"gcloud", "compute", "instances", "start", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone},
			"Stop":    {"gcloud", "compute", "instances", "stop", vm.Name, "--project", cloudCtx.AccountID, "--zone", vm.Zone},
		}
	case "Azure":
		return map[string][]string{
			"Restart": {"az", "vm", "restart", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID},
			"SSH":     {"az", "ssh", "vm", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID},
			"Start":   {"az", "vm", "start", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID},
			"Stop":    {"az", "vm", "stop", "--name", vm.Name, "--resource-group", vm.ResourceGroup, "--subscription", cloudCtx.AccountID},
		}
	case "DigitalOcean":
		return map[string][]string{
			"Restart": {"doctl", "compute", "droplet-action", "reboot", vm.ID},
			"SSH":     {"ssh", vm.PublicIP},
			"Start":   {"doctl", "compute", "droplet-action", "power-on", vm.ID},
			"Stop":    {"doctl", "compute", "droplet-action", "shutdown", vm.ID},
		}
	default:
		return map[string][]string{}
	}
}

func discoverProviderContexts(provider string) ([]core.CloudContext, []string) {
	return providers.DiscoverContexts(provider)
}

func targetProviders(value string) ([]string, error) {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "ALL":
		names := providers.RegisteredProviderNames()
		sort.Strings(names)
		return names, nil
	case "AWS":
		return []string{"AWS"}, nil
	case "GCP":
		return []string{"GCP"}, nil
	case "AZURE":
		return []string{"Azure"}, nil
	case "DIGITALOCEAN", "DIGITAL_OCEAN", "DO":
		return []string{"DigitalOcean"}, nil
	default:
		return nil, fmt.Errorf("unsupported smoke-test target %q; use all, aws, gcp, azure, or digitalocean", value)
	}
}

func normalizeProvider(value string) string {
	switch strings.ToUpper(strings.TrimSpace(value)) {
	case "", "ALL":
		return ""
	case "AWS":
		return "AWS"
	case "GCP":
		return "GCP"
	case "AZURE":
		return "Azure"
	default:
		return ""
	}
}
