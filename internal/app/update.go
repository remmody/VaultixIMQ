package app

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type GitHubRelease struct {
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
	HTMLURL string `json:"html_url"`
}

type UpdateInfo struct {
	HasUpdate     bool   `json:"has_update"`
	LatestVersion string `json:"latest_version"`
	CurrentVersion string `json:"current_version"`
	ReleaseNotes  string `json:"release_notes"`
	DownloadURL   string `json:"download_url"`
}

func (c *Core) CheckForUpdates() (*UpdateInfo, error) {
	owner := "remmody"
	repo := "VaultixIMQ"
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", owner, repo)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github api returned status: %d", resp.StatusCode)
	}

	var release GitHubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, err
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	current := strings.TrimPrefix(Version, "v")

	hasUpdate := isNewerVersion(current, latest)

	return &UpdateInfo{
		HasUpdate:      hasUpdate,
		LatestVersion:  latest,
		CurrentVersion: current,
		ReleaseNotes:   release.Body,
		DownloadURL:    release.HTMLURL,
	}, nil
}

func isNewerVersion(current, latest string) bool {
	// Simple string comparison for now, can be improved to semantic versioning
	return latest != "" && latest != current
}
