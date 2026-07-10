package service

import (
	"context"
	"strings"
)

func normalizeLaffeySiteName(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || value == legacyDefaultSiteName {
		return defaultSiteName
	}
	return value
}

func resolveLaffeySiteName(ctx context.Context, repo SettingRepository, value string) string {
	normalized := normalizeLaffeySiteName(value)
	if repo != nil && strings.TrimSpace(value) == legacyDefaultSiteName {
		_ = repo.Set(ctx, SettingKeySiteName, defaultSiteName)
	}
	return normalized
}

