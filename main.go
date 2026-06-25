package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"text/tabwriter"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/vyoogam/cloudmanager/internal/config"
	"github.com/vyoogam/cloudmanager/internal/logging"
	"github.com/vyoogam/cloudmanager/internal/providers"
	"github.com/vyoogam/cloudmanager/internal/smoke"
	"github.com/vyoogam/cloudmanager/internal/ui"
	"github.com/vyoogam/cloudmanager/internal/views/clusters"
	"github.com/vyoogam/cloudmanager/internal/views/databases"
	"github.com/vyoogam/cloudmanager/internal/views/disks"
	"github.com/vyoogam/cloudmanager/internal/views/firewalls"
	hostview "github.com/vyoogam/cloudmanager/internal/views/hosts"
	"github.com/vyoogam/cloudmanager/internal/views/networks"
	"github.com/vyoogam/cloudmanager/internal/views/snapshots"
	"github.com/vyoogam/cloudmanager/internal/views/storage"
	"github.com/vyoogam/cloudmanager/internal/views/vms"
)

// Version and BuildTime are injected at build time via ldflags.
var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	configureFlag := flag.Bool("configure", false, "Open configuration TUI to select backend and config options")
	backendFlag := flag.String("backend", "", "Force backend type ('cli' or 'sdk'). Overrides config file.")
	smokeTestFlag := flag.String("smoke-test", "", "Run non-destructive provider smoke tests (all|aws|gcp|azure|digitalocean) and exit.")
	debugFlag := flag.Bool("debug", false, "Show developer debug overlay in the TUI")
	versionFlag := flag.Bool("version", false, "Print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Printf("cloudmanager v%s (built %s)\n", Version, BuildTime)
		os.Exit(0)
	}

	cfg := config.Load()

	if *configureFlag {
		config.RunConfigureTUI()
		return
	}

	if *backendFlag != "" {
		if *backendFlag == "cli" || *backendFlag == "sdk" {
			cfg.Backend = *backendFlag
		} else {
			fmt.Printf("Invalid backend: %s. Use 'cli' or 'sdk'.\n", *backendFlag)
			os.Exit(1)
		}
	}

	if err := logging.Init(Version, BuildTime, cfg.Backend); err != nil {
		fmt.Printf("Warning: failed to initialize logging: %v\n", err)
	} else {
		logging.Infof("component=main event=start args=%s backend=%s", strings.Join(flag.Args(), ","), cfg.Backend)
	}

	if *smokeTestFlag != "" {
		summary, err := smoke.Run(cfg, *smokeTestFlag, os.Stdout)
		if err != nil {
			logging.Errorf("component=smoke event=failed target=%s checked=%d passed=%d failed=%d err=%v", *smokeTestFlag, summary.Checked, summary.Passed, summary.Failed, err)
			fmt.Printf("\nSmoke test summary: checked=%d passed=%d failed=%d\n", summary.Checked, summary.Passed, summary.Failed)
			fmt.Printf("Smoke test failed: %v\n", err)
			os.Exit(1)
		}
		logging.Infof("component=smoke event=completed target=%s checked=%d passed=%d failed=%d", *smokeTestFlag, summary.Checked, summary.Passed, summary.Failed)
		fmt.Printf("\nSmoke test summary: checked=%d passed=%d failed=%d\n", summary.Checked, summary.Passed, summary.Failed)
		return
	}

	if handled := handleProfileCLI(flag.Args(), cfg); handled {
		logging.Infof("component=main event=cli_exit args=%s", strings.Join(flag.Args(), ","))
		return
	}

	logging.Infof("component=main event=ui_start backend=%s", cfg.Backend)

	ui.InitTheme(cfg.Theme)

	app := ui.NewApp(cfg, Version, BuildTime)
	app.SetDebugOverlay(*debugFlag)

	// Register views to tabs
	vmView := vms.New(&cfg)
	diskView := disks.New(&cfg)
	snapView := snapshots.New(&cfg)
	firewallView := firewalls.New(&cfg)
	clusterView := clusters.New(&cfg)
	dbView := databases.New(&cfg)
	networkView := networks.New(&cfg)
	storageView := storage.New(&cfg)
	hostsView := hostview.New(&cfg)
	app.RegisterView("1", providers.CapabilityVMs, vmView)
	app.RegisterView("2", providers.CapabilityDisks, diskView)
	app.RegisterView("3", providers.CapabilitySnapshots, snapView)
	app.RegisterView("4", providers.CapabilityFirewalls, firewallView)
	app.RegisterView("5", providers.CapabilityClusters, clusterView)
	app.RegisterView("6", providers.CapabilityDatabases, dbView)
	app.RegisterView("7", providers.CapabilityNetworks, networkView)
	app.RegisterView("8", providers.CapabilityStorage, storageView)
	app.RegisterView("9", providers.CapabilityHosts, hostsView)

	app.Init() // prime the app

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		logging.Errorf("component=main event=ui_error err=%v", err)
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
	logging.Infof("component=main event=ui_exit")
}

func handleProfileCLI(args []string, cfg config.AppConfig) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case "tui":
		return false
	case "profile", "context":
		if len(args) < 2 {
			printProfileUsage()
			os.Exit(2)
		}
		switch args[1] {
		case "list", "ls":
			printProfiles(cfg)
		case "current":
			printCurrentProfile(cfg)
		case "use":
			if len(args) < 3 {
				fmt.Println("usage: cloudmanager profile use <name>")
				os.Exit(2)
			}
			useProfile(cfg, args[2])
		case "add":
			addProfile(cfg, args[2:])
		default:
			printProfileUsage()
			os.Exit(2)
		}
		return true
	case "login":
		if len(args) < 2 {
			fmt.Println("usage: cloudmanager login <profile>")
			os.Exit(2)
		}
		loginProfile(cfg, args[1])
		return true
	default:
		return false
	}
}

func printProfileUsage() {
	fmt.Println("usage:")
	fmt.Println("  cloudmanager profile list")
	fmt.Println("  cloudmanager profile add <name> --provider azure --tenant <tenant> --subscription-id <id> --subscription-name <name>")
	fmt.Println("  cloudmanager profile use <name>")
	fmt.Println("  cloudmanager profile current")
	fmt.Println("  cloudmanager login <name>")
	fmt.Println("  cloudmanager tui")
}

func printProfiles(cfg config.AppConfig) {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "CURRENT\tNAME\tPROVIDER\tTARGET\tTENANT\tAUTH")
	for _, ctx := range cfg.CloudContexts {
		ctx = config.SanitizeManagedCloudContext(ctx)
		current := ""
		if strings.EqualFold(cfg.CurrentContext, ctx.ContextName) {
			current = "*"
		}
		target := ctx.AccountID
		if ctx.AccountName != "" && ctx.AccountName != ctx.AccountID {
			target = fmt.Sprintf("%s (%s)", ctx.AccountID, ctx.AccountName)
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", current, ctx.ContextName, ctx.Provider, target, ctx.Tenant, ctx.AuthMode)
	}
	_ = w.Flush()
}

func printCurrentProfile(cfg config.AppConfig) {
	if cfg.CurrentContext == "" {
		fmt.Println("no current profile set")
		return
	}
	ctx, _, ok := config.ResolveManagedContext(cfg, cfg.CurrentContext)
	if !ok {
		fmt.Printf("current profile %q is not configured\n", cfg.CurrentContext)
		os.Exit(1)
	}
	fmt.Printf("%s\t%s\t%s\t%s\n", ctx.ContextName, ctx.Provider, ctx.AccountID, ctx.AccountName)
}

func useProfile(cfg config.AppConfig, name string) {
	ctx, _, ok := config.ResolveManagedContext(cfg, name)
	if !ok {
		fmt.Printf("profile not found: %s\n", name)
		os.Exit(1)
	}
	cfg.CurrentContext = ctx.ContextName
	if err := config.Save(cfg); err != nil {
		fmt.Printf("failed to save current profile: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("current profile: %s\n", ctx.ContextName)
}

func addProfile(cfg config.AppConfig, args []string) {
	if len(args) < 1 {
		fmt.Println("usage: cloudmanager profile add <name> [flags]")
		os.Exit(2)
	}
	name := args[0]
	fs := flag.NewFlagSet("profile add", flag.ExitOnError)
	provider := fs.String("provider", "azure", "provider name")
	tenant := fs.String("tenant", "", "tenant id or domain")
	accountID := fs.String("account-id", "", "account/project/subscription id")
	accountName := fs.String("account-name", "", "account/project/subscription display name")
	subID := fs.String("subscription-id", "", "Azure subscription id")
	subName := fs.String("subscription-name", "", "Azure subscription name")
	regions := fs.String("regions", "", "comma-separated regions")
	authMode := fs.String("auth-mode", config.AuthModeNativeCLI, "auth mode")
	persistence := fs.String("persistence", config.CredentialPersistenceNativeCLI, "credential persistence")
	credentialRef := fs.String("credential-ref", "", "provider-native credential profile/ref")
	_ = fs.Parse(args[1:])
	if *accountID == "" {
		*accountID = *subID
	}
	if *accountName == "" {
		*accountName = *subName
	}
	regionList := splitCSVMain(*regions)
	if len(regionList) == 0 {
		regionList = []string{"global"}
	}
	ctx := config.SanitizeManagedCloudContext(config.ManagedCloudContext{
		ContextName:           name,
		Provider:              *provider,
		AccountID:             *accountID,
		AccountName:           *accountName,
		Tenant:                *tenant,
		AuthMode:              *authMode,
		CredentialPersistence: *persistence,
		CredentialProfile:     *credentialRef,
		Regions:               regionList,
	})
	if ctx.Provider == "" || ctx.AccountID == "" {
		fmt.Println("provider and account/subscription id are required")
		os.Exit(2)
	}
	if _, idx, ok := config.ResolveManagedContext(cfg, ctx.ContextName); ok {
		cfg.CloudContexts[idx] = ctx
	} else {
		cfg.CloudContexts = append(cfg.CloudContexts, ctx)
	}
	if cfg.CurrentContext == "" {
		cfg.CurrentContext = ctx.ContextName
	}
	if err := config.Save(cfg); err != nil {
		fmt.Printf("failed to save profile: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("saved profile: %s\n", ctx.ContextName)
}

func loginProfile(cfg config.AppConfig, name string) {
	ctx, _, ok := config.ResolveManagedContext(cfg, name)
	if !ok {
		fmt.Printf("profile not found: %s\n", name)
		os.Exit(1)
	}
	var cmd *exec.Cmd
	switch ctx.Provider {
	case "Azure":
		args := []string{"login", "--use-device-code"}
		if strings.TrimSpace(ctx.Tenant) != "" {
			args = append(args, "--tenant", ctx.Tenant)
		}
		cmd = exec.Command("az", args...)
	case "AWS":
		args := []string{"sso", "login"}
		if strings.TrimSpace(ctx.CredentialProfile) != "" {
			args = append(args, "--profile", ctx.CredentialProfile)
		}
		cmd = exec.Command("aws", args...)
	case "GCP":
		cmd = exec.Command("gcloud", "auth", "login", "--no-browser")
	default:
		fmt.Printf("login is not implemented for provider %s\n", ctx.Provider)
		os.Exit(1)
	}
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		fmt.Printf("login failed for %s: %v\n", ctx.ContextName, err)
		os.Exit(1)
	}
	cfg.CurrentContext = ctx.ContextName
	if err := config.Save(cfg); err != nil {
		fmt.Printf("login succeeded, but failed to save current profile: %v\n", err)
		os.Exit(1)
	}
}

func splitCSVMain(value string) []string {
	var out []string
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}
