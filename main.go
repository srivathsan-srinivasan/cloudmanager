package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	configureFlag := flag.Bool("configure", false, "Open configuration TUI to select backend and config options")
	backendFlag := flag.String("backend", "", "Force backend type ('cli' or 'sdk'). Overrides config file.")
	flag.Parse()

	cfg := loadAppConfig()

	if *configureFlag {
		runConfigureTUI()
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

	initStyles(cfg)
	p := tea.NewProgram(initialModel(), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Printf("Error running program: %v\n", err)
		os.Exit(1)
	}
}
