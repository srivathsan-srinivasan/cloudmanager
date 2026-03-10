package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

func fetchContextsCmd() tea.Cmd {
	return func() tea.Msg {
		var allCtx []contextItem

		awsCtx := loadAWSContexts()
		allCtx = append(allCtx, awsCtx...)

		gcpCtx := loadGCPContexts()
		allCtx = append(allCtx, gcpCtx...)

		azCtx := loadAzureContexts()
		allCtx = append(allCtx, azCtx...)

		if len(allCtx) == 0 {
			allCtx = []contextItem{
				{"AWS", "123456789012", "production", "us-east-1"},
				{"AWS", "123456789012", "production", "us-west-2"},
				{"AWS", "987654321098", "staging", "eu-central-1"},
				{"GCP", "my-gcp-project-1", "backend-services", "us-central1"},
				{"GCP", "my-gcp-project-2", "data-pipeline", "europe-west1"},
				{"Azure", "sub-abc-123", "core-infra", "eastus"},
			}
		}

		tree := buildContextTree(allCtx)
		return contextLoadMsg{tree: tree}
	}
}

func fetchAllGCPProjectsCmd() tea.Cmd {
	return func() tea.Msg {
		cmd := exec.Command("gcloud", "projects", "list", "--format=json(projectId)")
		output, err := cmd.Output()
		if err != nil {
			return gcpProjectFetchMsg{items: nil}
		}

		var projects []struct {
			ProjectId string `json:"projectId"`
		}
		if err := json.Unmarshal(output, &projects); err != nil {
			return gcpProjectFetchMsg{items: nil}
		}

		cfg := loadAppConfig()

		var items []list.Item
		for _, p := range projects {
			selected := false
			if !cfg.GCPConfigured {
				selected = true
			} else {
				selected = isGCPProjectSelected(p.ProjectId, cfg)
			}
			items = append(items, gcpProjectItem{
				projectId: p.ProjectId,
				selected:  selected,
			})
		}
		return gcpProjectFetchMsg{items: items}
	}
}

func fetchVMsCmd(ctx contextItem, force bool) tea.Cmd {
	return func() tea.Msg {
		cacheKey := fmt.Sprintf("%s-%s-%s", ctx.provider, ctx.accountID, ctx.region)
		cfg := loadAppConfig()
		cacheTTL := time.Duration(cfg.CacheTTL) * time.Minute

		if !force {
			if entry, ok := vmCache[cacheKey]; ok {
				if time.Since(entry.timestamp) < cacheTTL {
					return vmFetchMsg{vms: entry.vms, err: nil}
				}
			}
		}

		provider := GetProvider()
		rows, err := provider.FetchVMs(context.Background(), ctx)

		if err != nil {
			return vmFetchMsg{vms: nil, err: err}
		}

		vmCache[cacheKey] = cacheEntry{
			vms:       rows,
			timestamp: time.Now(),
		}

		return vmFetchMsg{vms: rows, err: nil}
	}
}
func executeCommandMock(action string, instanceID string) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1 * time.Second) // simulate API call
		return commandCompleteMsg{
			output: fmt.Sprintf("Successfully executed '%s' on %s", action, instanceID),
			err:    nil,
		}
	}
}
