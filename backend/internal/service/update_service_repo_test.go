package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

type updateRepoCaptureCache struct{}

func (updateRepoCaptureCache) GetUpdateInfo(context.Context) (string, error) {
	return "", fmt.Errorf("cache miss")
}

func (updateRepoCaptureCache) SetUpdateInfo(context.Context, string, time.Duration) error {
	return nil
}

type updateRepoCaptureClient struct {
	repo string
}

type updateFixedReleaseClient struct {
	release *GitHubRelease
}

func (c *updateRepoCaptureClient) FetchLatestRelease(_ context.Context, repo string) (*GitHubRelease, error) {
	c.repo = repo
	return &GitHubRelease{
		TagName: "v999.0.0",
		Name:    "test release",
		HTMLURL: "https://github.com/desalahy/Sub2api_laffey/releases/tag/v999.0.0",
	}, nil
}

func (*updateRepoCaptureClient) DownloadFile(context.Context, string, string, int64) error {
	return fmt.Errorf("DownloadFile should not be called")
}

func (*updateRepoCaptureClient) FetchChecksumFile(context.Context, string) ([]byte, error) {
	return nil, fmt.Errorf("FetchChecksumFile should not be called")
}

func (c *updateFixedReleaseClient) FetchLatestRelease(context.Context, string) (*GitHubRelease, error) {
	return c.release, nil
}

func (*updateFixedReleaseClient) DownloadFile(context.Context, string, string, int64) error {
	return fmt.Errorf("DownloadFile should not be called")
}

func (*updateFixedReleaseClient) FetchChecksumFile(context.Context, string) ([]byte, error) {
	return nil, fmt.Errorf("FetchChecksumFile should not be called")
}

func TestUpdateServiceChecksLaffeyReleaseRepo(t *testing.T) {
	client := &updateRepoCaptureClient{}
	svc := NewUpdateService(updateRepoCaptureCache{}, client, "0.0.1", "release")

	_, err := svc.CheckUpdate(context.Background(), true)
	if err != nil {
		t.Fatalf("CheckUpdate() error = %v", err)
	}

	if client.repo != "desalahy/Sub2api_laffey" {
		t.Fatalf("FetchLatestRelease repo = %q, want %q", client.repo, "desalahy/Sub2api_laffey")
	}
}

func TestValidateSelfUpdateSupportedRejectsWindows(t *testing.T) {
	err := validateSelfUpdateSupported("windows")
	if err == nil {
		t.Fatal("validateSelfUpdateSupported(windows) error = nil, want unsupported error")
	}
	if !strings.Contains(err.Error(), "not supported on Windows") {
		t.Fatalf("validateSelfUpdateSupported(windows) error = %q, want Windows unsupported message", err)
	}
}

func TestValidateSelfUpdateSupportedAllowsLinux(t *testing.T) {
	if err := validateSelfUpdateSupported("linux"); err != nil {
		t.Fatalf("validateSelfUpdateSupported(linux) error = %v", err)
	}
}

func TestUpdateServiceDetectsLaffeyPatchRelease(t *testing.T) {
	client := &updateFixedReleaseClient{
		release: &GitHubRelease{
			TagName: "v0.1.126-laffey.2",
			Name:    "Laffey API 0.1.126-laffey.2",
			HTMLURL: "https://github.com/desalahy/Sub2api_laffey/releases/tag/v0.1.126-laffey.2",
		},
	}
	svc := NewUpdateService(updateRepoCaptureCache{}, client, "0.1.126-laffey.1", "release")

	info, err := svc.CheckUpdate(context.Background(), true)
	if err != nil {
		t.Fatalf("CheckUpdate() error = %v", err)
	}

	if !info.HasUpdate {
		t.Fatalf("HasUpdate = false, want true for newer Laffey patch release")
	}
}
