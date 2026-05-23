package ui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var operationalStatePattern = regexp.MustCompile(`(?i)\b(running|available|ready|online|active|succeeded|in-use|attached|completed|reachable|ssh-ok|ping-ok|starting|pending|provisioning|creating|checking|stopping|stopped|terminated|deleted|suspended|failed|unavailable|unreachable|deallocated|halted|shut|off|unknown)\b`)
var semanticTableTokenPattern = regexp.MustCompile(`(?i)\b(name|host|user|connection|provider|context|region|status|state|type|network|subnet|tags|public|private|ip|id|running|available|ready|online|active|succeeded|in-use|attached|completed|reachable|ssh-ok|ping-ok|starting|pending|provisioning|creating|checking|stopping|stopped|terminated|deleted|suspended|failed|unavailable|unreachable|deallocated|halted|shut|off|unknown|true|false|yes|no|aws|gcp|azure|manual|digitalocean|do|[a-z]{2}-[a-z]+-[0-9]+|(?:\d{1,3}\.){3}\d{1,3}|(?:i|vol|snap|sg|vpc|subnet|eni|rtb|db|cluster|aks|eks|gke)-[a-z0-9-]+|[tmcr][0-9][a-z]*\.[a-z0-9]+|n[0-9]-[a-z0-9-]+)\b`)

func ColorizeOperationalStates(rendered string) string {
	if rendered == "" {
		return rendered
	}
	lines := strings.Split(rendered, "\n")
	for i, line := range lines {
		if strings.Contains(line, "\x1b[") {
			continue
		}
		lines[i] = ColorizeStatusText(line)
	}
	return strings.Join(lines, "\n")
}

func ColorizeStatusText(text string) string {
	return operationalStatePattern.ReplaceAllStringFunc(text, func(match string) string {
		return RenderStatus(match)
	})
}

func ColorizeTableText(text string) string {
	return semanticTableTokenPattern.ReplaceAllStringFunc(text, func(match string) string {
		return RenderSemanticToken(match)
	})
}

func RenderSemanticToken(token string) string {
	if color, bold := StatusColor(token); color != nil {
		return lipgloss.NewStyle().Foreground(*color).Bold(bold).Render(token)
	}
	color, bold := SemanticTokenColor(token)
	if color == nil {
		return token
	}
	return lipgloss.NewStyle().Foreground(*color).Bold(bold).Render(token)
}

func SemanticTokenColor(token string) (*lipgloss.AdaptiveColor, bool) {
	normalized := strings.ToLower(strings.TrimSpace(token))
	switch {
	case normalized == "name" || normalized == "host" || normalized == "user" || normalized == "connection":
		return &ColumnName, true
	case normalized == "id":
		return &ColumnID, true
	case normalized == "public" || normalized == "private" || normalized == "ip":
		return &ColumnIP, true
	case normalized == "provider" || normalized == "context":
		return &ColumnProvider, true
	case normalized == "region":
		return &ColumnRegion, true
	case normalized == "type":
		return &ColumnType, true
	case normalized == "status" || normalized == "state" || normalized == "network" || normalized == "subnet" || normalized == "tags":
		return &ColumnMeta, true
	case normalized == "aws" || normalized == "gcp" || normalized == "azure" || normalized == "manual" || normalized == "digitalocean" || normalized == "do":
		return &ColumnProvider, true
	case isIPv4Token(normalized):
		return &ColumnIP, true
	case isRegionToken(normalized):
		return &ColumnRegion, true
	case isIDToken(normalized):
		return &ColumnID, true
	case isTypeToken(normalized):
		return &ColumnType, false
	case normalized == "true" || normalized == "yes":
		return &StatusReady, true
	case normalized == "false" || normalized == "no":
		return &StatusStopped, true
	default:
		return nil, false
	}
}

func RenderStatus(status string) string {
	color, bold := StatusColor(status)
	if color == nil {
		return status
	}
	return lipgloss.NewStyle().Foreground(*color).Bold(bold).Render(status)
}

func StatusColor(status string) (*lipgloss.AdaptiveColor, bool) {
	switch normalizedStatus(status) {
	case "running", "online", "active", "succeeded":
		return &StatusRunning, true
	case "available":
		return &StatusAvailable, true
	case "ready":
		return &StatusReady, true
	case "in-use", "attached", "completed":
		return &StatusInUse, true
	case "reachable", "ssh-ok", "ping-ok":
		return &StatusReachable, true
	case "starting", "pending", "provisioning", "creating", "checking":
		return &StatusStarting, true
	case "stopping", "suspended":
		return &StatusStopping, true
	case "stopped", "deleted", "failed", "unavailable", "halted", "shut", "off":
		return &StatusStopped, true
	case "terminated":
		return &StatusTerminated, true
	case "deallocated":
		return &StatusDeallocated, true
	case "unreachable":
		return &StatusUnreachable, true
	case "unknown", "", "-":
		return &StatusUnknown, false
	default:
		return nil, false
	}
}

func StatusTone(state string) string {
	normalized := normalizedStatus(state)
	for _, token := range []string{"running", "available", "ready", "online", "active", "succeeded", "in-use", "attached", "completed", "reachable", "ssh-ok", "ping-ok"} {
		if normalized == token {
			return "ok"
		}
	}
	for _, token := range []string{"starting", "pending", "provisioning", "creating", "checking"} {
		if normalized == token {
			return "progress"
		}
	}
	for _, token := range []string{"stopping", "stopped", "terminated", "deleted", "suspended", "failed", "unavailable", "unreachable", "deallocated", "halted", "shut", "off"} {
		if normalized == token {
			return "warn"
		}
	}
	if normalized == "unknown" || normalized == "" || normalized == "-" {
		return "unknown"
	}
	return ""
}

func normalizedStatus(state string) string {
	return strings.ToLower(strings.TrimSpace(state))
}

func isIPv4Token(token string) bool {
	return regexp.MustCompile(`^(?:\d{1,3}\.){3}\d{1,3}$`).MatchString(token)
}

func isRegionToken(token string) bool {
	return regexp.MustCompile(`^[a-z]{2}-[a-z]+-[0-9]+$`).MatchString(token)
}

func isIDToken(token string) bool {
	return regexp.MustCompile(`^(?:i|vol|snap|sg|vpc|subnet|eni|rtb|db|cluster|aks|eks|gke)-[a-z0-9-]+$`).MatchString(token)
}

func isTypeToken(token string) bool {
	return regexp.MustCompile(`^(?:[tmcr][0-9][a-z]*\.[a-z0-9]+|n[0-9]-[a-z0-9-]+)$`).MatchString(token)
}

func operationalStateTone(state string) string {
	switch StatusTone(state) {
	case "ok":
		return "running"
	case "warn":
		return "stopped"
	case "progress", "unknown":
		return StatusTone(state)
	default:
		return ""
	}
}

func StatusSortRank(state string) int {
	switch StatusTone(state) {
	case "ok":
		return 0
	case "progress":
		return 1
	case "warn":
		return 2
	case "unknown":
		return 3
	default:
		return 4
	}
}
