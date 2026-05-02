package core

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

var clusterExecCommand = exec.Command
var clusterExecCommandContext = exec.CommandContext

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

type KubeContextTarget struct {
	ExpectedContext string
	ClusterName     string
	ClusterID       string
	AccountID       string
	Location        string
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

func EnsureKubeContext(ctx context.Context, expectedContext string, fetchName string, fetchArgs ...string) (*exec.Cmd, error) {
	return EnsureKubeContextForTarget(ctx, KubeContextTarget{ExpectedContext: expectedContext}, fetchName, fetchArgs...)
}

func EnsureKubeContextForTarget(ctx context.Context, target KubeContextTarget, fetchName string, fetchArgs ...string) (*exec.Cmd, error) {
	target.ExpectedContext = strings.TrimSpace(target.ExpectedContext)
	out, err := clusterExecCommand("kubectl", "config", "get-contexts", "-o", "name").Output()
	if err == nil {
		if matched := bestKubeContextMatch(strings.Split(string(out), "\n"), target); matched != "" {
			if err := runClusterCommand(ctx, "kubectl", "config", "use-context", matched); err != nil {
				return nil, err
			}
			return clusterExecCommandContext(ctx, "k9s"), nil
		}
	}
	if fetchName == "" {
		return nil, fmt.Errorf("fetch command is required when kube context %q is missing", target.ExpectedContext)
	}
	if err := runClusterCommand(ctx, fetchName, fetchArgs...); err != nil {
		return nil, err
	}
	return clusterExecCommandContext(ctx, "k9s"), nil
}

func bestKubeContextMatch(contexts []string, target KubeContextTarget) string {
	best := ""
	bestScore := 0
	for _, contextName := range contexts {
		contextName = strings.TrimSpace(contextName)
		if contextName == "" {
			continue
		}
		score := kubeContextMatchScore(contextName, target)
		if score > bestScore {
			best = contextName
			bestScore = score
		}
	}
	if bestScore >= 80 {
		return best
	}
	return ""
}

func kubeContextMatchScore(contextName string, target KubeContextTarget) int {
	if stringEqualFold(contextName, target.ExpectedContext) {
		return 100
	}
	if stringEqualFold(contextName, target.ClusterName) {
		return 95
	}
	contextKey := normalizedKubeContextKey(contextName)
	expectedKey := normalizedKubeContextKey(target.ExpectedContext)
	clusterKey := normalizedKubeContextKey(target.ClusterName)
	idKey := normalizedKubeContextKey(target.ClusterID)
	accountKey := normalizedKubeContextKey(target.AccountID)
	locationKey := normalizedKubeContextKey(target.Location)
	switch {
	case expectedKey != "" && contextKey == expectedKey:
		return 94
	case clusterKey != "" && contextKey == clusterKey:
		return 92
	case idKey != "" && contextKey == idKey:
		return 90
	}
	if clusterKey == "" || !strings.Contains(contextKey, clusterKey) {
		return 0
	}
	score := 70
	if expectedKey != "" && strings.Contains(contextKey, expectedKey) {
		score += 15
	}
	if accountKey != "" && strings.Contains(contextKey, accountKey) {
		score += 10
	}
	if locationKey != "" && strings.Contains(contextKey, locationKey) {
		score += 10
	}
	if idKey != "" && strings.Contains(contextKey, idKey) {
		score += 10
	}
	return score
}

func stringEqualFold(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	return left != "" && right != "" && strings.EqualFold(left, right)
}

func normalizedKubeContextKey(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if strings.Contains(value, "/") {
		parts := strings.Split(value, "/")
		value = parts[len(parts)-1]
	}
	var b strings.Builder
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func runClusterCommand(ctx context.Context, name string, args ...string) error {
	cmd := clusterExecCommandContext(ctx, name, args...)
	output, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	message := strings.TrimSpace(string(output))
	if message != "" {
		return fmt.Errorf("%s %s failed: %w: %s", name, strings.Join(args, " "), err, message)
	}
	return fmt.Errorf("%s %s failed: %w", name, strings.Join(args, " "), err)
}
