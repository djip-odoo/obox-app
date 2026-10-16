package update

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"obox-app/buildinfo"
	"obox-app/internal/logger"
)

const (
	checkCooldown        = 30 * time.Second
	defaultMaxNotesRunes = 500
)

type Updater struct {
	mu            sync.Mutex
	lastCheckTime time.Time
	lastestInfo   Info

	latestRelease  *Release
	selectedAsset  *Asset
	stagedPath     string
	stagedDir      string
	cancelDownload context.CancelFunc
}

var (
	instance *Updater
	once     sync.Once
)

// GetUpdaterInstance returns the singleton Updater instance.
func GetUpdaterInstance() *Updater {
	once.Do(func() {
		instance = &Updater{
			lastestInfo: Info{
				CurrentVersion: buildinfo.Version,
				State:          StateIdle,
			},
		}
	})
	return instance
}

func Check(ctx context.Context) (Info, error) {
	u := GetUpdaterInstance()
	u.mu.Lock()

	if u.lastestInfo.State.IsInProgress() {
		info := u.lastestInfo
		u.mu.Unlock()
		return info, ErrUpdateInProgress
	}

	// Use cached check if within cooldown and not forced
	if !u.lastCheckTime.IsZero() && time.Since(u.lastCheckTime) < checkCooldown {
		info := u.lastestInfo
		u.mu.Unlock()
		return info, nil
	}

	u.lastestInfo.State = StateChecking
	u.mu.Unlock()

	logger.Infof("Checking for updates (current version: %s)", buildinfo.Version)

	release, err := FetchLatestRelease(ctx)

	u.mu.Lock()
	defer u.mu.Unlock()

	u.lastCheckTime = time.Now()

	if err != nil {
		u.lastestInfo.State = StateFailed
		u.lastestInfo.Error = err.Error()
		logger.Warnf("Update check failed: %v", err)
		return u.lastestInfo, err
	}

	isNewer, err := IsUpgrade(u.lastestInfo.CurrentVersion, release.TagName)
	if err != nil {
		u.lastestInfo.State = StateFailed
		u.lastestInfo.LatestVersion = release.TagName
		u.lastestInfo.Error = fmt.Sprintf("Invalid release version %q: %v", release.TagName, err)
		logger.Errorf("Failed to compare versions: %v", err)
		return u.lastestInfo, err
	}

	if !isNewer {
		u.lastestInfo.State = StateIdle
		u.lastestInfo.LatestVersion = release.TagName
		u.lastestInfo.Notes = sanitizeNotes(release.Body, defaultMaxNotesRunes)
		logger.Debugf("Application is up to date (%s)", u.lastestInfo.CurrentVersion)
		return u.lastestInfo, nil
	}

	asset, err := selectAsset(release.Assets)
	if err != nil {
		u.lastestInfo.State = StateFailed
		u.lastestInfo.LatestVersion = release.TagName
		u.lastestInfo.Notes = sanitizeNotes(release.Body, defaultMaxNotesRunes)
		u.lastestInfo.Error = err.Error()
		logger.Errorf("Update %s available, but no asset: %v", release.TagName, err)
		return u.lastestInfo, err
	}

	u.latestRelease = release
	u.selectedAsset = asset

	u.lastestInfo.State = StateUpdateAvailable
	u.lastestInfo.LatestVersion = release.TagName
	u.lastestInfo.Notes = sanitizeNotes(release.Body, defaultMaxNotesRunes)
	u.lastestInfo.Error = ""

	logger.Debugf("New update available: %s -> %s (asset: %s)", u.lastestInfo.CurrentVersion, release.TagName, asset.Name)
	return u.lastestInfo, nil
}

func Download(ctx context.Context, progress ProgressFunc) (string, error) {
	u := GetUpdaterInstance()
	u.mu.Lock()

	if u.lastestInfo.State.IsInProgress() {
		u.mu.Unlock()
		return "", ErrUpdateInProgress
	}

	if u.selectedAsset == nil || u.latestRelease == nil || u.lastestInfo.State != StateUpdateAvailable {
		u.mu.Unlock()
		return "", ErrUpdateNotReady
	}

	asset := u.selectedAsset
	rel := u.latestRelease

	// Setup cancellable context
	downloadCtx, cancel := context.WithCancel(ctx)
	u.cancelDownload = cancel
	u.lastestInfo.State = StateDownloading
	u.mu.Unlock()

	defer func() {
		cancel()
		u.mu.Lock()
		u.cancelDownload = nil
		u.mu.Unlock()
	}()

	// Validate download URL
	if err := ValidateDownloadURL(asset.DownloadURL); err != nil {
		u.mu.Lock()
		u.lastestInfo.State = StateUpdateAvailable
		u.lastestInfo.Error = err.Error()
		u.mu.Unlock()
		return "", err
	}

	// Fetch expected checksum before downloading - fail early if missing
	expectedSHA, err := FetchExpectedChecksum(downloadCtx, asset, rel.Assets)
	if err != nil || expectedSHA == "" {
		u.mu.Lock()
		u.lastestInfo.State = StateUpdateAvailable
		if err != nil {
			u.lastestInfo.Error = err.Error()
		} else {
			u.lastestInfo.Error = fmt.Sprintf("no checksum available for asset %s", asset.Name)
		}
		u.mu.Unlock()
		if err != nil {
			logger.Warnf("Could not fetch release checksums: %v", err)
			return "", fmt.Errorf("failed to fetch release checksums: %v", err)
		}
		return "", fmt.Errorf("no checksum available for asset %s", asset.Name)
	}

	result, err := DownloadAsset(downloadCtx, asset, expectedSHA, progress)

	u.mu.Lock()
	defer u.mu.Unlock()

	if err != nil {
		u.lastestInfo.State = StateUpdateAvailable
		if errors.Is(err, context.Canceled) {
			u.lastestInfo.Error = ""
			return "", err
		}
		u.lastestInfo.Error = err.Error()
		return "", err
	}

	// Check if context was cancelled while DownloadAsset was completing
	if downloadCtx.Err() != nil {
		if result != nil && result.TempDir != "" {
			_ = os.RemoveAll(result.TempDir)
		}
		u.lastestInfo.State = StateUpdateAvailable
		u.lastestInfo.Error = ""
		return "", downloadCtx.Err()
	}

	u.stagedPath = result.StagedPath
	u.stagedDir = result.TempDir
	u.lastestInfo.State = StateUpdateAvailable
	u.lastestInfo.Error = ""
	return result.StagedPath, nil
}

// Apply installs the downloaded and verified update.
func (u *Updater) Apply() error {
	u.mu.Lock()
	if u.lastestInfo.State == StateInstalling {
		u.mu.Unlock()
		return ErrUpdateInProgress
	}
	if u.stagedPath == "" {
		u.mu.Unlock()
		return ErrUpdateNotReady
	}

	staged := u.stagedPath
	stagedDir := u.stagedDir
	u.lastestInfo.State = StateInstalling
	u.mu.Unlock()

	logger.Infof("Installing update from staged file %s", staged)
	if err := Apply(staged); err != nil {
		u.mu.Lock()
		u.lastestInfo.State = StateFailed
		u.lastestInfo.Error = err.Error()
		// Only remove staged files if the staged file no longer exists or is corrupted.
		// On Windows, if user cancels the UAC prompt or an installer launch fails temporarily,
		// keep stagedDir so the update doesn't have to be re-downloaded.
		if _, statErr := os.Stat(staged); statErr != nil && stagedDir != "" {
			_ = os.RemoveAll(stagedDir)
			u.stagedDir = ""
			u.stagedPath = ""
		}
		u.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrInstallFailed, err)
	}

	return nil
}

// Status returns the current Info summary.
func (u *Updater) Status() Info {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastestInfo
}

// Reset returns the updater to StateIdle or StateUpdateAvailable if an update is available.
func (u *Updater) Reset() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.cancelDownload != nil {
		u.cancelDownload()
		u.cancelDownload = nil
	}
	// Never delete the staged update directory or disrupt state while installation is in progress.
	if u.lastestInfo.State == StateInstalling {
		return
	}
	if u.stagedDir != "" {
		_ = os.RemoveAll(u.stagedDir)
		u.stagedDir = ""
		u.stagedPath = ""
	}
	if u.selectedAsset != nil {
		u.lastestInfo.State = StateUpdateAvailable
	} else {
		u.lastestInfo.State = StateIdle
	}
	u.lastestInfo.Error = ""
	u.lastCheckTime = time.Time{}
}

const defaultStaleUpdateAge = 15 * time.Minute

// CleanupStaleUpdates removes any leftover temporary directories from previous update downloads
// that are older than defaultStaleUpdateAge (avoiding active downloads from other instances).
func CleanupStaleUpdates() {
	pattern := filepath.Join(os.TempDir(), "obox-app-update-*")
	matches, err := filepath.Glob(pattern)
	if err == nil {
		cutoff := time.Now().Add(-defaultStaleUpdateAge)
		for _, m := range matches {
			info, err := os.Stat(m)
			if err == nil && info.IsDir() && info.ModTime().Before(cutoff) {
				_ = os.RemoveAll(m)
			}
		}
	}
}

func sanitizeNotes(notes string, maxRunes int) string {
	notes = strings.TrimSpace(notes)
	// Normalize CRLF to LF
	notes = strings.ReplaceAll(notes, "\r\n", "\n")
	// Strip control characters except newline and tab
	var b strings.Builder
	for _, r := range notes {
		if r == '\n' || r == '\t' || r >= 32 {
			b.WriteRune(r)
		}
	}
	s := b.String()
	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return s
}
