package service

import (
	"context"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
)

type settingBrandMigrationRepoStub struct {
	values map[string]string
	sets   map[string]string
}

func (s *settingBrandMigrationRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *settingBrandMigrationRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if value, ok := s.values[key]; ok {
		return value, nil
	}
	return "", ErrSettingNotFound
}

func (s *settingBrandMigrationRepoStub) Set(_ context.Context, key, value string) error {
	if s.sets == nil {
		s.sets = make(map[string]string)
	}
	s.sets[key] = value
	return nil
}

func (s *settingBrandMigrationRepoStub) GetMultiple(_ context.Context, keys []string) (map[string]string, error) {
	result := make(map[string]string, len(keys))
	for _, key := range keys {
		if value, ok := s.values[key]; ok {
			result[key] = value
		}
	}
	return result, nil
}

func (s *settingBrandMigrationRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (s *settingBrandMigrationRepoStub) GetAll(context.Context) (map[string]string, error) {
	result := make(map[string]string, len(s.values))
	for key, value := range s.values {
		result[key] = value
	}
	return result, nil
}

func (s *settingBrandMigrationRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

func TestInitializeDefaultSettingsBackfillsOnlyOldDefaultSiteName(t *testing.T) {
	ctx := context.Background()

	repo := &settingBrandMigrationRepoStub{values: map[string]string{
		SettingKeyRegistrationEnabled: "true",
		SettingKeySiteName:            "Sub2API",
	}}
	svc := NewSettingService(repo, &config.Config{})

	if err := svc.InitializeDefaultSettings(ctx); err != nil {
		t.Fatalf("InitializeDefaultSettings() error = %v", err)
	}
	if repo.sets[SettingKeySiteName] != defaultSiteName {
		t.Fatalf("site_name backfill=%q, want %q", repo.sets[SettingKeySiteName], defaultSiteName)
	}

	customRepo := &settingBrandMigrationRepoStub{values: map[string]string{
		SettingKeyRegistrationEnabled: "true",
		SettingKeySiteName:            "Custom API",
	}}
	customSvc := NewSettingService(customRepo, &config.Config{})

	if err := customSvc.InitializeDefaultSettings(ctx); err != nil {
		t.Fatalf("InitializeDefaultSettings() with custom name error = %v", err)
	}
	if _, changed := customRepo.sets[SettingKeySiteName]; changed {
		t.Fatalf("custom site_name was overwritten: %q", customRepo.sets[SettingKeySiteName])
	}
}

func TestGetPublicSettingsBackfillsOnlyOldDefaultSiteName(t *testing.T) {
	ctx := context.Background()

	repo := &settingBrandMigrationRepoStub{values: map[string]string{
		SettingKeySiteName: legacyDefaultSiteName,
	}}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetPublicSettings(ctx)
	if err != nil {
		t.Fatalf("GetPublicSettings() error = %v", err)
	}
	if settings.SiteName != defaultSiteName {
		t.Fatalf("public site_name=%q, want %q", settings.SiteName, defaultSiteName)
	}
	if repo.sets[SettingKeySiteName] != defaultSiteName {
		t.Fatalf("persisted site_name backfill=%q, want %q", repo.sets[SettingKeySiteName], defaultSiteName)
	}

	customRepo := &settingBrandMigrationRepoStub{values: map[string]string{
		SettingKeySiteName: "Custom API",
	}}
	customSvc := NewSettingService(customRepo, &config.Config{})

	customSettings, err := customSvc.GetPublicSettings(ctx)
	if err != nil {
		t.Fatalf("GetPublicSettings() with custom name error = %v", err)
	}
	if customSettings.SiteName != "Custom API" {
		t.Fatalf("custom public site_name=%q, want %q", customSettings.SiteName, "Custom API")
	}
	if _, changed := customRepo.sets[SettingKeySiteName]; changed {
		t.Fatalf("custom site_name was overwritten: %q", customRepo.sets[SettingKeySiteName])
	}
}

func TestGetAllSettingsBackfillsOnlyOldDefaultSiteName(t *testing.T) {
	ctx := context.Background()

	repo := &settingBrandMigrationRepoStub{values: map[string]string{
		SettingKeySiteName: legacyDefaultSiteName,
	}}
	svc := NewSettingService(repo, &config.Config{})

	settings, err := svc.GetAllSettings(ctx)
	if err != nil {
		t.Fatalf("GetAllSettings() error = %v", err)
	}
	if settings.SiteName != defaultSiteName {
		t.Fatalf("admin site_name=%q, want %q", settings.SiteName, defaultSiteName)
	}
	if repo.sets[SettingKeySiteName] != defaultSiteName {
		t.Fatalf("persisted site_name backfill=%q, want %q", repo.sets[SettingKeySiteName], defaultSiteName)
	}

	customRepo := &settingBrandMigrationRepoStub{values: map[string]string{
		SettingKeySiteName: "Custom API",
	}}
	customSvc := NewSettingService(customRepo, &config.Config{})

	customSettings, err := customSvc.GetAllSettings(ctx)
	if err != nil {
		t.Fatalf("GetAllSettings() with custom name error = %v", err)
	}
	if customSettings.SiteName != "Custom API" {
		t.Fatalf("custom admin site_name=%q, want %q", customSettings.SiteName, "Custom API")
	}
	if _, changed := customRepo.sets[SettingKeySiteName]; changed {
		t.Fatalf("custom site_name was overwritten: %q", customRepo.sets[SettingKeySiteName])
	}
}

func TestGetSiteNameBackfillsOnlyOldDefaultSiteName(t *testing.T) {
	ctx := context.Background()

	repo := &settingBrandMigrationRepoStub{values: map[string]string{
		SettingKeySiteName: legacyDefaultSiteName,
	}}
	svc := NewSettingService(repo, nil)

	if siteName := svc.GetSiteName(ctx); siteName != defaultSiteName {
		t.Fatalf("site_name=%q, want %q", siteName, defaultSiteName)
	}
	if repo.sets[SettingKeySiteName] != defaultSiteName {
		t.Fatalf("persisted site_name backfill=%q, want %q", repo.sets[SettingKeySiteName], defaultSiteName)
	}

	customRepo := &settingBrandMigrationRepoStub{values: map[string]string{
		SettingKeySiteName: "Custom API",
	}}
	customSvc := NewSettingService(customRepo, nil)

	if siteName := customSvc.GetSiteName(ctx); siteName != "Custom API" {
		t.Fatalf("custom site_name=%q, want %q", siteName, "Custom API")
	}
	if _, changed := customRepo.sets[SettingKeySiteName]; changed {
		t.Fatalf("custom site_name was overwritten: %q", customRepo.sets[SettingKeySiteName])
	}
}

func TestSiteNameFromSettingRepoBackfillsOnlyOldDefaultSiteName(t *testing.T) {
	ctx := context.Background()

	repo := &settingBrandMigrationRepoStub{values: map[string]string{
		SettingKeySiteName: legacyDefaultSiteName,
	}}
	if siteName := siteNameFromSettingRepo(ctx, repo); siteName != defaultSiteName {
		t.Fatalf("site_name=%q, want %q", siteName, defaultSiteName)
	}
	if repo.sets[SettingKeySiteName] != defaultSiteName {
		t.Fatalf("persisted site_name backfill=%q, want %q", repo.sets[SettingKeySiteName], defaultSiteName)
	}

	customRepo := &settingBrandMigrationRepoStub{values: map[string]string{
		SettingKeySiteName: "Custom API",
	}}
	if siteName := siteNameFromSettingRepo(ctx, customRepo); siteName != "Custom API" {
		t.Fatalf("custom site_name=%q, want %q", siteName, "Custom API")
	}
	if _, changed := customRepo.sets[SettingKeySiteName]; changed {
		t.Fatalf("custom site_name was overwritten: %q", customRepo.sets[SettingKeySiteName])
	}
}
