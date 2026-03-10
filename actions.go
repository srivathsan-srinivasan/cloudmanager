package main

import (
	"context"
	"os/exec"

	tea "github.com/charmbracelet/bubbletea"
)

type describeCompleteMsg struct {
	output string
	err    error
}

func executeActionCmd(action string, vm VM, ctx contextItem) tea.Cmd {
	return func() tea.Msg {
		provider := GetProvider()
		output, err := provider.ExecuteAction(context.Background(), action, vm, ctx)

		if action == "Describe" {
			return describeCompleteMsg{output: output, err: err}
		}

		if err != nil {
			return commandCompleteMsg{err: err}
		}

		return commandCompleteMsg{output: output, err: nil}
	}
}

func createSSHCmd(vm VM, ctx contextItem) *exec.Cmd {
	provider := GetProvider()
	cmd, _ := provider.GetSSHCmd(context.Background(), vm, ctx)
	return cmd
}
