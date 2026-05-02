package aws

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"unicode"

	"cloudmanager/internal/core"
	"gopkg.in/ini.v1"
)

var defaultRegions = []string{
	"us-east-1", "us-east-2", "us-west-1", "us-west-2",
	"ca-central-1", "eu-central-1", "eu-west-1", "eu-west-2",
	"eu-west-3", "eu-north-1", "ap-northeast-1", "ap-northeast-2",
	"ap-northeast-3", "ap-southeast-1", "ap-southeast-2", "ap-south-1",
	"sa-east-1",
}

type awsProfile struct {
	name          string
	sourceProfile string
	region        string
}

func LoadContexts() ([]core.CloudContext, []string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, []string{"AWS: could not determine home directory"}
	}

	configPath := filepath.Join(homeDir, ".aws", "config")
	credsPath := filepath.Join(homeDir, ".aws", "credentials")

	var files []interface{}
	if _, err := os.Stat(configPath); err == nil {
		files = append(files, configPath)
	}
	if _, err := os.Stat(credsPath); err == nil {
		files = append(files, credsPath)
	}

	if len(files) == 0 {
		return nil, []string{"AWS: no config or credentials found"}
	}

	cfg, err := ini.Load(files[0], files[1:]...)
	if err != nil {
		return nil, []string{fmt.Sprintf("AWS: failed to parse config/credentials: %v", err)}
	}

	return loadContextsFromConfig(cfg)
}

func loadContextsFromConfig(cfg *ini.File) ([]core.CloudContext, []string) {
	var contexts []core.CloudContext
	var warnings []string

	defaultRegion := strings.TrimSpace(cfg.Section("default").Key("region").String())
	if defaultRegion == "" {
		defaultRegion = strings.TrimSpace(cfg.Section("DEFAULT").Key("region").String())
	}

	profiles := collectAWSProfiles(cfg)
	seen := make(map[string]bool)

	for _, profile := range profiles {
		region := profile.region
		if region == "" {
			if profile.sourceProfile == "" {
				continue
			}
			region = defaultRegion
		}
		if region == "" {
			region = "us-east-1"
			warnings = append(warnings, fmt.Sprintf("AWS: profile %s has no region, defaulting to us-east-1", profile.name))
		}

		accountID, accountName := inferAWSAccountIdentity(profile)
		if accountID == "" {
			accountID = fallbackAccountKey(profile)
		}
		if accountName == "" {
			accountName = accountID
		}

		dedupeKey := accountID + "|" + region
		if seen[dedupeKey] {
			continue
		}
		seen[dedupeKey] = true

		contexts = append(contexts, core.CloudContext{
			Provider:          "AWS",
			ContextName:       authProfile(profile),
			AccountID:         accountID,
			AccountName:       accountName,
			Region:            region,
			CredentialProfile: authProfile(profile),
		})
	}

	sort.Slice(contexts, func(i, j int) bool {
		if contexts[i].AccountID != contexts[j].AccountID {
			return contexts[i].AccountID < contexts[j].AccountID
		}
		if contexts[i].AccountName != contexts[j].AccountName {
			return contexts[i].AccountName < contexts[j].AccountName
		}
		return contexts[i].Region < contexts[j].Region
	})

	if len(contexts) == 0 {
		warnings = append(warnings, "AWS: no region-aware profiles found")
	}

	return contexts, warnings
}

func collectAWSProfiles(cfg *ini.File) []awsProfile {
	profilesByName := make(map[string]awsProfile)

	for _, section := range cfg.Sections() {
		name := canonicalProfileName(section.Name())
		if name == "" || name == "default" {
			continue
		}

		profile := profilesByName[name]
		profile.name = name

		if sourceProfile := strings.TrimSpace(section.Key("source_profile").String()); sourceProfile != "" {
			profile.sourceProfile = sourceProfile
		}
		if region := strings.TrimSpace(section.Key("region").String()); region != "" {
			profile.region = region
		}

		profilesByName[name] = profile
	}

	names := make([]string, 0, len(profilesByName))
	for name := range profilesByName {
		names = append(names, name)
	}
	sort.Strings(names)

	profiles := make([]awsProfile, 0, len(names))
	for _, name := range names {
		profiles = append(profiles, profilesByName[name])
	}
	return profiles
}

func canonicalProfileName(name string) string {
	name = strings.TrimSpace(name)
	if strings.HasPrefix(name, "profile ") {
		return strings.TrimSpace(strings.TrimPrefix(name, "profile "))
	}
	if name == "DEFAULT" || name == "default" {
		return "default"
	}
	return name
}

func inferAWSAccountIdentity(profile awsProfile) (string, string) {
	for _, candidate := range []string{profile.sourceProfile, profile.name} {
		if candidate == "" {
			continue
		}
		accountID, accountName := parseAWSIdentity(candidate)
		if accountID != "" || accountName != "" {
			return accountID, accountName
		}
	}
	return "", ""
}

func fallbackAccountKey(profile awsProfile) string {
	if profile.sourceProfile != "" {
		return profile.sourceProfile
	}
	return profile.name
}

func authProfile(profile awsProfile) string {
	if profile.sourceProfile != "" {
		return profile.sourceProfile
	}
	return profile.name
}

func parseAWSIdentity(profileName string) (string, string) {
	parts := strings.Split(profileName, "-")
	if len(parts) == 0 {
		return "", ""
	}

	if parts[0] == "aws" && len(parts) > 1 {
		parts = parts[1:]
	}

	if len(parts) >= 2 && looksLikeRegionAlias(parts[len(parts)-1]) {
		accountID := ""
		accountName := strings.Join(parts[:len(parts)-1], "-")
		if isDigits(parts[0]) {
			accountID = parts[0]
			accountName = strings.Join(parts[1:len(parts)-1], "-")
		}
		return accountID, strings.Trim(accountName, "-")
	}

	for idx, part := range parts {
		if !isDigits(part) {
			continue
		}
		accountName := strings.Join(append(parts[:idx], parts[idx+1:]...), "-")
		return part, strings.Trim(accountName, "-")
	}

	if strings.HasPrefix(profileName, "aws-") {
		return "", strings.Join(parts, "-")
	}

	return "", ""
}

func isDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func looksLikeRegionAlias(value string) bool {
	if len(value) < 3 || len(value) > 5 {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		default:
			return false
		}
	}
	return hasLetter && hasDigit
}
