package access

import (
	"bufio"
	"os"
	"strings"

	"github.com/srivathsan-srinivasan/cloudmanager/internal/core"
)

type SSHConfigEntry struct {
	Aliases      []string
	HostName     string
	User         string
	Port         string
	IdentityFile string
}

func (e SSHConfigEntry) PrimaryAlias() string {
	for _, alias := range e.Aliases {
		if alias == "" || strings.ContainsAny(alias, "*?!") {
			continue
		}
		return alias
	}
	return ""
}

func ParseSSHConfig(path string) ([]SSHConfigEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var entries []SSHConfigEntry
	var current *SSHConfigEntry
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(stripSSHConfigComment(scanner.Text()))
		if line == "" {
			continue
		}
		key, value, ok := splitConfigLine(line)
		if !ok {
			continue
		}
		switch strings.ToLower(key) {
		case "host":
			entry := SSHConfigEntry{Aliases: strings.Fields(value)}
			entries = append(entries, entry)
			current = &entries[len(entries)-1]
		case "hostname":
			if current != nil {
				current.HostName = value
			}
		case "user":
			if current != nil {
				current.User = value
			}
		case "port":
			if current != nil {
				current.Port = value
			}
		case "identityfile":
			if current != nil {
				current.IdentityFile = value
			}
		}
	}
	return entries, scanner.Err()
}

func entryMatchesVM(entry SSHConfigEntry, vm core.VM) bool {
	fields := []string{vm.Name, vm.ID, vm.PublicIP, vm.PrivateIP}
	for _, alias := range entry.Aliases {
		if strings.ContainsAny(alias, "*?!") {
			continue
		}
		for _, field := range fields {
			if sameToken(alias, field) {
				return true
			}
		}
	}
	for _, field := range fields {
		if sameToken(entry.HostName, field) {
			return true
		}
	}
	return false
}

func splitConfigLine(line string) (string, string, bool) {
	if idx := strings.Index(line, "="); idx >= 0 {
		key := strings.TrimSpace(line[:idx])
		value := strings.Trim(strings.TrimSpace(line[idx+1:]), `"`)
		return key, value, key != "" && value != ""
	}
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return "", "", false
	}
	return fields[0], strings.Trim(strings.Join(fields[1:], " "), `"`), true
}

func stripSSHConfigComment(line string) string {
	inQuote := false
	for i, r := range line {
		switch r {
		case '"':
			inQuote = !inQuote
		case '#':
			if !inQuote {
				return line[:i]
			}
		}
	}
	return line
}

func sameToken(left, right string) bool {
	left = strings.ToLower(strings.TrimSpace(left))
	right = strings.ToLower(strings.TrimSpace(right))
	return left != "" && right != "" && left == right
}
