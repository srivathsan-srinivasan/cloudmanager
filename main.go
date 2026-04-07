package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"

	"cloudmanager/internal/config"
	"cloudmanager/internal/logging"
	"cloudmanager/internal/providers"
	"cloudmanager/internal/smoke"
	"cloudmanager/internal/ui"
	"cloudmanager/internal/views/clusters"
	"cloudmanager/internal/views/databases"
	"cloudmanager/internal/views/disks"
	"cloudmanager/internal/views/firewalls"
	"cloudmanager/internal/views/networks"
	"cloudmanager/internal/views/snapshots"
	"cloudmanager/internal/views/vms"
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

	if *smokeTestFlag != "" {
		_ = logging.Init(Version, BuildTime, cfg.Backend)
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

	if err := logging.Init(Version, BuildTime, cfg.Backend); err != nil {
		fmt.Printf("Warning: failed to initialize logging: %v\n", err)
	} else {
		logging.Infof("component=main event=ui_start backend=%s", cfg.Backend)
	}

	ui.InitStyles(cfg.Theme.Subtle, cfg.Theme.Highlight, cfg.Theme.Special, cfg.Theme.Alert)

	app := ui.NewApp(cfg, Version, BuildTime)

	// Register views to tabs
	vmView := vms.New(&cfg)
	diskView := disks.New(&cfg)
	snapView := snapshots.New(&cfg)
	firewallView := firewalls.New(&cfg)
	clusterView := clusters.New(&cfg)
	dbView := databases.New(&cfg)
	networkView := networks.New(&cfg)
	app.RegisterView("1", providers.CapabilityVMs, vmView)
	app.RegisterView("2", providers.CapabilityDisks, diskView)
	app.RegisterView("3", providers.CapabilitySnapshots, snapView)
	app.RegisterView("4", providers.CapabilityFirewalls, firewallView)
	app.RegisterView("5", providers.CapabilityClusters, clusterView)
	app.RegisterView("6", providers.CapabilityDatabases, dbView)
	app.RegisterView("7", providers.CapabilityNetworks, networkView)

	app.Init() // prime the app

	p := tea.NewProgram(app, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		logging.Errorf("component=main event=ui_error err=%v", err)
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
	logging.Infof("component=main event=ui_exit")
}
