package service

import (
	"context"
	"strings"
)

func siteNameFromSettingRepo(ctx context.Context, repo SettingRepository) string {
	if repo == nil {
		return defaultSiteName
	}
	value, err := repo.GetValue(ctx, SettingKeySiteName)
	if err != nil || strings.TrimSpace(value) == "" {
		return defaultSiteName
	}
	value = strings.TrimSpace(value)
	if value == legacyDefaultSiteName {
		if err := repo.Set(ctx, SettingKeySiteName, defaultSiteName); err != nil {
			return defaultSiteName
		}
		return defaultSiteName
	}
	return value
}

func siteNameFromSettingsMap(ctx context.Context, repo SettingRepository, settings map[string]string) string {
	value := strings.TrimSpace(settings[SettingKeySiteName])
	if value == "" {
		return defaultSiteName
	}
	if value == legacyDefaultSiteName {
		if repo != nil {
			_ = repo.Set(ctx, SettingKeySiteName, defaultSiteName)
		}
		settings[SettingKeySiteName] = defaultSiteName
		return defaultSiteName
	}
	return value
}
