package tagging

import (
	"fmt"
	"strings"

	"cloudmanager/internal/config"
	"cloudmanager/internal/core"
)

func SplitInput(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == ';' || r == '\n' || r == '\t'
	})
	tags := make([]string, 0, len(fields))
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		tag := strings.TrimSpace(field)
		if tag == "" {
			continue
		}
		key := strings.ToLower(tag)
		if seen[key] {
			continue
		}
		seen[key] = true
		tags = append(tags, tag)
	}
	return tags
}

func Save(cfg *config.AppConfig, ctx core.CloudContext, resource core.Resource, tags []string) error {
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}
	if len(tags) == 0 {
		return fmt.Errorf("no tags entered")
	}
	if _, err := config.BackupConfig(); err != nil {
		return fmt.Errorf("backup failed: %w", err)
	}
	*cfg = config.UpsertResourceTags(*cfg, ctx, config.ResourceTagTargetFromResource(resource), tags)
	if err := config.Save(*cfg); err != nil {
		return fmt.Errorf("save failed: %w", err)
	}
	return nil
}
