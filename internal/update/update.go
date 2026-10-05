// Package update checks the app's GitHub releases for a newer build and allows
// the user to download, verify, and apply it safely in place.
package update

import (
	"context"
	"net/http"
	"runtime"
	"sync"
)

const (
	RepoOwner = "djip-odoo"
	RepoName  = "obox-app"

	assetNameLinux   = "obox-app-linux64"
	assetNameWindows = "obox-app-win64-installer"

	requestUA = "obox-app-updater"
)

// Version is injected at build time via -ldflags.
var Version = "dev"

var (
	defaultMu      sync.Mutex
	defaultUpdater *Updater

	// Test/mock overrides for tests
	apiBaseURL  = "https://api.github.com"
	feedBaseURL = "https://github.com"
)

func getDefaultUpdater() *Updater {
	defaultMu.Lock()
	defer defaultMu.Unlock()
	if defaultUpdater == nil {
		defaultUpdater = NewUpdater(Config{
			RepoOwner:      RepoOwner,
			RepoName:       RepoName,
			CurrentVersion: Version,
			APIBaseURL:     apiBaseURL,
			TargetOS:       runtime.GOOS,
			TargetArch:     runtime.GOARCH,
			CheckCooldown:  0, // immediate for direct calls
		})
	}
	defaultUpdater.cfg.APIBaseURL = apiBaseURL
	defaultUpdater.cfg.CurrentVersion = Version
	return defaultUpdater
}

// Check queries GitHub releases and reports whether an upgrade applies to the current OS.
func Check() Info {
	u := getDefaultUpdater()
	info, _ := u.Check(context.Background(), true)
	return info
}

// CheckGoOS checks updates for an explicit target OS and architecture (for testability).
func CheckGoOS(goos string) Info {
	u := NewUpdater(Config{
		RepoOwner:      RepoOwner,
		RepoName:       RepoName,
		CurrentVersion: Version,
		APIBaseURL:     apiBaseURL,
		TargetOS:       goos,
		TargetArch:     "amd64",
		CheckCooldown:  0,
	})
	info, _ := u.Check(context.Background(), true)
	return info
}

// Download downloads an asset into dir and verifies it.
func Download(url, filename, dir string, progress ProgressFunc) (string, error) {
	asset := &Asset{
		Name:        filename,
		DownloadURL: url,
	}
	result, err := DownloadAsset(context.Background(), &http.Client{Timeout: 0}, asset, "", progress)
	if err != nil {
		return "", err
	}
	return result.StagedPath, nil
}
