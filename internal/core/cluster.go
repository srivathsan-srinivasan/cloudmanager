package core

import (
	"context"
	"os/exec"
	"strings"
)

var DefaultClusterColumns = []string{"Name", "Location", "Status", "Version", "Nodes"}

type Cluster struct {
	Name      string
	ID        string
	Location  string
	Version   string
	Status    string
	NodeCount string
	Labels    string
}

func (c Cluster) GetID() string   { return c.ID }
func (c Cluster) GetName() string { return c.Name }
func (c Cluster) GetKind() string { return "Cluster" }
func (c Cluster) GetField(col string) string {
	switch col {
	case "Name":
		return c.Name
	case "ID":
		return c.ID
	case "Location":
		return c.Location
	case "Version":
		return c.Version
	case "Status":
		return c.Status
	case "Nodes":
		return c.NodeCount
	case "Labels":
		return c.Labels
	default:
		return "-"
	}
}

func EnsureKubeContext(ctx context.Context, expectedContext string, fetchCmd string) (*exec.Cmd, error) {
	out, err := exec.Command("kubectl", "config", "get-contexts", "-o", "name").Output()
	if err == nil {
		for _, line := range strings.Split(string(out), "\n") {
			if strings.TrimSpace(line) == expectedContext {
				return exec.CommandContext(ctx, "bash", "-c", "kubectl config use-context "+expectedContext+" && k9s"), nil
			}
		}
	}
	return exec.CommandContext(ctx, "bash", "-c", fetchCmd), nil
}
