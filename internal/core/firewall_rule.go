package core

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	Inbound  = "Inbound"
	Outbound = "Outbound"
)

// FirewallRule represents a normalized firewall rule row across providers.
type FirewallRule struct {
	Name         string
	ID           string
	Direction    string
	Protocol     string
	PortRange    string
	Source       string
	Destination  string
	Action       string
	Priority     int
	Description  string
	ResourceID   string
	ResourceName string
	NetworkID    string
	Provider     string

	OriginalRule *FirewallRule
}

var DefaultFirewallRuleColumns = []string{
	"Direction", "Protocol", "Ports", "Source", "Destination", "Action", "Description",
}

func (r FirewallRule) GetID() string { return r.ID }
func (r FirewallRule) GetName() string {
	if r.Name != "" {
		return r.Name
	}
	return r.ID
}
func (r FirewallRule) GetKind() string { return "Firewall Rule" }

func (r FirewallRule) GetField(col string) string {
	switch col {
	case "Direction":
		return r.Direction
	case "Protocol":
		return r.Protocol
	case "Ports":
		return r.PortRange
	case "Source":
		return r.Source
	case "Destination":
		return r.Destination
	case "Action":
		return r.Action
	case "Priority":
		return fmt.Sprintf("%d", r.Priority)
	case "Description":
		return r.Description
	default:
		return "-"
	}
}

func FirewallRuleActions() []Action {
	return FirewallRuleActionsForProvider("")
}

func FirewallRuleActionsForProvider(provider string) []Action {
	actions := []Action{
		{"Describe", "Show full rule details", false},
		{"Edit", "Edit this firewall rule", false},
		{"Delete", "Delete this firewall rule (Destructive)", true},
	}

	if strings.EqualFold(strings.TrimSpace(provider), "gcp") {
		actions = append(actions,
			Action{"Enable", "Enable the firewall rule (GCP only)", false},
			Action{"Disable", "Disable the firewall rule (GCP only)", false},
		)
	}
	return actions
}

func IsRuleRisky(r FirewallRule) bool {
	if !isInternetSource(r.Source) {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(r.Protocol), "all") {
		return true
	}
	return PortRangeIncludes(r.PortRange, 22) || PortRangeIncludes(r.PortRange, 3389)
}

func isInternetSource(source string) bool {
	normalized := strings.ToLower(strings.TrimSpace(source))
	return strings.Contains(normalized, "0.0.0.0/0") || strings.Contains(normalized, "::/0") || normalized == "internet"
}

func PortRangeIncludes(portRange string, target int) bool {
	normalized := strings.TrimSpace(strings.ToLower(portRange))
	if normalized == "" || normalized == "all" || normalized == "*" {
		return true
	}
	for _, segment := range strings.Split(normalized, ",") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}
		if strings.Contains(segment, "-") {
			parts := strings.SplitN(segment, "-", 2)
			start, err1 := strconv.Atoi(strings.TrimSpace(parts[0]))
			end, err2 := strconv.Atoi(strings.TrimSpace(parts[1]))
			if err1 == nil && err2 == nil && target >= start && target <= end {
				return true
			}
			continue
		}
		value, err := strconv.Atoi(segment)
		if err == nil && value == target {
			return true
		}
	}
	return false
}

func DescribeFirewallRule(r FirewallRule) string {
	var b strings.Builder
	fmt.Fprintf(&b, "ID:           %s\n", r.ID)
	fmt.Fprintf(&b, "Direction:    %s\n", r.Direction)
	fmt.Fprintf(&b, "Protocol:     %s\n", r.Protocol)
	fmt.Fprintf(&b, "Ports:        %s\n", r.PortRange)
	fmt.Fprintf(&b, "Source:       %s\n", r.Source)
	fmt.Fprintf(&b, "Destination:  %s\n", r.Destination)
	fmt.Fprintf(&b, "Action:       %s\n", r.Action)
	fmt.Fprintf(&b, "Priority:     %d\n", r.Priority)
	fmt.Fprintf(&b, "Description:  %s\n", r.Description)
	fmt.Fprintf(&b, "Resource:     %s (%s)\n", r.ResourceName, r.ResourceID)
	fmt.Fprintf(&b, "Network:      %s\n", r.NetworkID)
	fmt.Fprintf(&b, "Provider:     %s\n", r.Provider)
	return b.String()
}
