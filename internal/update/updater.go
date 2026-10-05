package update

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"strings"
	"sync"
	"time"

	"obox-app/internal/logger"
)

const (
	defaultCheckCooldown = 30 * time.Second
	defaultMaxNotesRunes = 500
)

// Config configures an Updater instance.
type Config struct {
	RepoOwner      string
	RepoName       string
	CurrentVersion string
	APIBaseURL     string
	TargetOS       string
	TargetArch     string
	HTTPClient     *http.Client
	CheckCooldown  time.Duration
}

// Updater manages the update lifecycle, state transitions, and concurrency protection.
type Updater struct {
	mu sync.Mutex

	cfg Config

	state          State
	latestRelease  *Release
	selectedAsset  *Asset
	stagedPath     string
	lastCheckTime  time.Time
	lastInfo       Info
	dismissedTag   string
	cancelDownload context.CancelFunc
}

// NewUpdater creates a new Updater instance.
func NewUpdater(cfg Config) *Updater {
	if cfg.RepoOwner == "" {
		cfg.RepoOwner = RepoOwner
	}
	if cfg.RepoName == "" {
		cfg.RepoName = RepoName
	}
	if cfg.CurrentVersion == "" {
		cfg.CurrentVersion = Version
	}
	if cfg.APIBaseURL == "" {
		cfg.APIBaseURL = "https://api.github.com"
	}
	if cfg.TargetOS == "" {
		cfg.TargetOS = runtime.GOOS
	}
	if cfg.TargetArch == "" {
		cfg.TargetArch = runtime.GOARCH
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 15 * time.Second}
	}
	if cfg.CheckCooldown == 0 {
		cfg.CheckCooldown = defaultCheckCooldown
	}

	return &Updater{
		cfg:   cfg,
		state: StateIdle,
		lastInfo: Info{
			CurrentVersion: cfg.CurrentVersion,
			State:          StateIdle,
		},
	}
}

// Check queries GitHub for a newer release.
// If force is false and a check was performed recently, the cached result is returned.
func (u *Updater) Check(ctx context.Context, force bool) (Info, error) {
	u.mu.Lock()

	// Prevent running check during active download or installation
	if u.state == StateDownloading || u.state == StateInstalling || u.state == StateRestarting {
		info := u.lastInfo
		u.mu.Unlock()
		return info, ErrUpdateInProgress
	}
	if u.state == StateChecking {
		info := u.lastInfo
		u.mu.Unlock()
		return info, ErrUpdateInProgress
	}

	// Use cached check if within cooldown and not forced
	if !force && !u.lastCheckTime.IsZero() && time.Since(u.lastCheckTime) < u.cfg.CheckCooldown && u.lastInfo.CheckOK {
		info := u.lastInfo
		if u.dismissedTag != "" && u.dismissedTag == info.LatestVersion {
			info.Available = false
		}
		u.mu.Unlock()
		return info, nil
	}

	u.state = StateChecking
	u.mu.Unlock()

	logger.Infof("Checking for updates (current version: %s)", u.cfg.CurrentVersion)

	rel, err := FetchLatestRelease(ctx, u.cfg.HTTPClient, u.cfg.APIBaseURL, u.cfg.RepoOwner, u.cfg.RepoName, requestUA+"/"+u.cfg.CurrentVersion)

	u.mu.Lock()
	defer u.mu.Unlock()

	u.lastCheckTime = time.Now()

	if err != nil {
		u.state = StateFailed
		info := Info{
			CheckOK:        false,
			Available:      false,
			State:          StateFailed,
			CurrentVersion: u.cfg.CurrentVersion,
			Error:          userFriendlyError(err),
		}
		u.lastInfo = info
		logger.Warnf("Update check failed: %v", err)
		return info, err
	}

	// Compare versions
	isNewer, err := IsUpgrade(u.cfg.CurrentVersion, rel.TagName)
	if err != nil {
		u.state = StateFailed
		info := Info{
			CheckOK:        false,
			Available:      false,
			State:          StateFailed,
			CurrentVersion: u.cfg.CurrentVersion,
			LatestVersion:  rel.TagName,
			Error:          fmt.Sprintf("Invalid release version %q: %v", rel.TagName, err),
		}
		u.lastInfo = info
		return info, err
	}

	if !isNewer {
		// Already up to date
		u.state = StateIdle
		info := Info{
			CheckOK:        true,
			Available:      false,
			State:          StateIdle,
			CurrentVersion: u.cfg.CurrentVersion,
			LatestVersion:  rel.TagName,
			Notes:          sanitizeNotes(rel.Body, defaultMaxNotesRunes),
		}
		u.lastInfo = info
		logger.Infof("Application is up to date (%s)", u.cfg.CurrentVersion)
		return info, nil
	}

	// Match asset for the current platform
	asset, err := SelectAsset(rel.Assets, u.cfg.TargetOS, u.cfg.TargetArch)
	if err != nil {
		// No compatible build for this platform
		u.state = StateIdle
		info := Info{
			CheckOK:        true,
			Available:      false,
			State:          StateIdle,
			CurrentVersion: u.cfg.CurrentVersion,
			LatestVersion:  rel.TagName,
			Notes:          sanitizeNotes(rel.Body, defaultMaxNotesRunes),
		}
		u.lastInfo = info
		logger.Infof("Update %s available, but no asset for %s/%s", rel.TagName, u.cfg.TargetOS, u.cfg.TargetArch)
		return info, nil
	}

	// Check if this release was dismissed by the user
	if !force && u.dismissedTag == rel.TagName {
		u.state = StateUpdateAvailable
		info := Info{
			CheckOK:        true,
			Available:      false, // suppressed because dismissed
			State:          StateUpdateAvailable,
			CurrentVersion: u.cfg.CurrentVersion,
			LatestVersion:  rel.TagName,
		}
		u.lastInfo = info
		return info, nil
	}

	u.state = StateUpdateAvailable
	u.latestRelease = rel
	u.selectedAsset = asset

	info := Info{
		CheckOK:        true,
		Available:      true,
		State:          StateUpdateAvailable,
		CurrentVersion: u.cfg.CurrentVersion,
		LatestVersion:  rel.TagName,
		DownloadURL:    asset.DownloadURL,
		AssetName:      asset.Name,
		AssetSize:      asset.Size,
		Notes:          sanitizeNotes(rel.Body, defaultMaxNotesRunes),
	}
	u.lastInfo = info
	logger.Infof("New update available: %s -> %s (asset: %s)", u.cfg.CurrentVersion, rel.TagName, asset.Name)
	return info, nil
}

// Download downloads and verifies the update asset.
func (u *Updater) Download(ctx context.Context, progress ProgressFunc) (string, error) {
	u.mu.Lock()
	if u.state == StateDownloading {
		u.mu.Unlock()
		return "", ErrUpdateInProgress
	}
	if u.state == StateInstalling || u.state == StateRestarting {
		u.mu.Unlock()
		return "", ErrUpdateInProgress
	}
	if u.selectedAsset == nil || u.latestRelease == nil {
		u.mu.Unlock()
		return "", ErrUpdateNotReady
	}

	asset := u.selectedAsset
	rel := u.latestRelease

	// Setup cancellable context
	downloadCtx, cancel := context.WithCancel(ctx)
	u.cancelDownload = cancel
	u.state = StateDownloading
	u.lastInfo.State = StateDownloading
	u.mu.Unlock()

	// Validate download URL
	if err := ValidateDownloadURL(asset.DownloadURL); err != nil {
		u.mu.Lock()
		u.state = StateFailed
		u.lastInfo.State = StateFailed
		u.lastInfo.Error = err.Error()
		u.mu.Unlock()
		return "", err
	}

	// Fetch expected checksum if available
	expectedSHA, err := FetchExpectedChecksum(u.cfg.HTTPClient, asset, rel.Assets)
	if err != nil {
		logger.Warnf("Could not fetch release checksums: %v", err)
	}

	result, err := DownloadAsset(downloadCtx, u.cfg.HTTPClient, asset, expectedSHA, progress)

	u.mu.Lock()
	defer u.mu.Unlock()

	u.cancelDownload = nil

	if err != nil {
		u.state = StateFailed
		u.lastInfo.State = StateFailed
		u.lastInfo.Error = userFriendlyError(err)
		return "", err
	}

	u.state = StateReadyToInstall
	u.stagedPath = result.StagedPath
	u.lastInfo.State = StateReadyToInstall
	u.lastInfo.Error = ""
	return result.StagedPath, nil
}

// Apply installs the downloaded and verified update.
func (u *Updater) Apply() error {
	u.mu.Lock()
	if u.state == StateInstalling || u.state == StateRestarting {
		u.mu.Unlock()
		return ErrUpdateInProgress
	}
	if u.state != StateReadyToInstall || u.stagedPath == "" {
		u.mu.Unlock()
		return ErrUpdateNotReady
	}

	staged := u.stagedPath
	u.state = StateInstalling
	u.lastInfo.State = StateInstalling
	u.mu.Unlock()

	logger.Infof("Installing update from staged file %s", staged)
	if err := Apply(staged); err != nil {
		u.mu.Lock()
		u.state = StateFailed
		u.lastInfo.State = StateFailed
		u.lastInfo.Error = userFriendlyError(err)
		u.mu.Unlock()
		return fmt.Errorf("%w: %v", ErrInstallFailed, err)
	}

	u.mu.Lock()
	u.state = StateRestarting
	u.lastInfo.State = StateRestarting
	u.mu.Unlock()
	return nil
}

// Dismiss suppresses update banners for the specified version.
func (u *Updater) Dismiss(version string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.dismissedTag = version
	if u.lastInfo.LatestVersion == version {
		u.lastInfo.Available = false
	}
}

// Status returns the current Info summary.
func (u *Updater) Status() Info {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastInfo
}

// Reset returns the updater to StateIdle.
func (u *Updater) Reset() {
	u.mu.Lock()
	defer u.mu.Unlock()
	if u.cancelDownload != nil {
		u.cancelDownload()
		u.cancelDownload = nil
	}
	u.state = StateIdle
	u.lastInfo.State = StateIdle
	u.lastInfo.Error = ""
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

func userFriendlyError(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "rate limit"):
		return "GitHub rate limit reached. Please try again in a few minutes."
	case strings.Contains(msg, "connection refused") || strings.Contains(msg, "no such host") || strings.Contains(msg, "network"):
		return "Cannot reach update server. Please check your internet connection."
	case strings.Contains(msg, "timeout") || strings.Contains(msg, "deadline"):
		return "Update request timed out. Please try again."
	case strings.Contains(msg, "checksum"):
		return "Update file verification failed. Please try again."
	default:
		return msg
	}
}
