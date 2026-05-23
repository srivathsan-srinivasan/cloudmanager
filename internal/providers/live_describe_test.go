package providers

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/core"
)

func TestLiveVMDescribeAllConfiguredContexts(t *testing.T) {
	if os.Getenv("CLOUDMANAGER_LIVE_DESCRIBE") != "1" {
		t.Skip("set CLOUDMANAGER_LIVE_DESCRIBE=1 to test live VM describe across configured contexts")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	contexts := configuredCloudContexts()
	if len(contexts) == 0 {
		t.Fatal("no configured cloud contexts found")
	}

	for _, cloudCtx := range contexts {
		if strings.EqualFold(cloudCtx.Provider, "Manual") {
			continue
		}
		for _, backend := range liveDescribeBackends(cloudCtx.Provider) {
			cloudCtx, backend := cloudCtx, backend
			t.Run(strings.Join([]string{cloudCtx.Provider, cloudCtx.AccountID, cloudCtx.Region, backend}, "/"), func(t *testing.T) {
				provider := GetProvider(config.AppConfig{Backend: backend})
				vms, err := provider.FetchVMs(ctx, cloudCtx)
				if err != nil {
					t.Fatalf("list VMs failed: %v", err)
				}
				if len(vms) == 0 {
					t.Skip("no VMs in context")
				}

				vm := vms[0]
				output, err := provider.ExecuteAction(ctx, "Describe", vm, cloudCtx)
				if err != nil {
					t.Fatalf("describe failed for %s/%s zone=%q rg=%q: %v", cloudCtx.AccountID, vm.Name, vm.Zone, vm.ResourceGroup, err)
				}
				assertLiveDescribeOutput(t, output, vm)
				t.Logf("describe ok provider=%s backend=%s context=%s region=%s vm=%s bytes=%d", cloudCtx.Provider, backend, cloudCtx.AccountID, cloudCtx.Region, vm.Name, len(output))
			})
		}
	}
}

func configuredCloudContexts() []core.CloudContext {
	cfg := config.Load()
	contexts := config.ManagedContexts(cfg)
	if len(contexts) > 0 {
		return dedupeContexts(contexts)
	}

	var discovered []core.CloudContext
	for _, registered := range RegisteredProviders() {
		if registered.LoadContexts == nil {
			continue
		}
		contexts, _ := registered.LoadContexts()
		discovered = append(discovered, contexts...)
	}
	return dedupeContexts(discovered)
}

func dedupeContexts(contexts []core.CloudContext) []core.CloudContext {
	seen := map[string]bool{}
	var out []core.CloudContext
	for _, cloudCtx := range contexts {
		key := strings.Join([]string{
			strings.ToLower(strings.TrimSpace(cloudCtx.Provider)),
			strings.TrimSpace(cloudCtx.ContextName),
			strings.TrimSpace(cloudCtx.AccountID),
			strings.TrimSpace(cloudCtx.Region),
			strings.TrimSpace(cloudCtx.CredentialProfile),
		}, "\x00")
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, cloudCtx)
	}
	return out
}

func liveDescribeBackends(provider string) []string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "digitalocean", "manual":
		return []string{"cli"}
	default:
		return []string{"cli", "sdk"}
	}
}

func assertLiveDescribeOutput(t *testing.T, output string, vm core.VM) {
	t.Helper()
	output = strings.TrimSpace(output)
	if output == "" {
		t.Fatal("describe returned empty output")
	}
	if vm.Name != "" && strings.Contains(output, vm.Name) {
		return
	}
	if vm.ID != "" && strings.Contains(output, vm.ID) {
		return
	}
	t.Fatalf("describe output does not mention VM name or ID; vm=%q id=%q output_prefix=%q", vm.Name, vm.ID, liveDescribePrefix(output, 300))
}

func liveDescribePrefix(value string, limit int) string {
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}
