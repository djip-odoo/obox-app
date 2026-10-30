package update

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"obox-app/buildinfo"
	"obox-app/internal/logger"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

const (
	checkCooldown        = 30 * time.Second
	defaultMaxNotesRunes = 500
)

// Updater defines the contract for update lifecycle operations.
type Updater interface {
	Check(ctx context.Context) (Info, error)
	Download(ctx context.Context, progress func(updater.Progress)) (string, error)
	Apply() error
	Reset()
	Info() Info
	SetStatusListener(listener func(Info))
}

type updaterImpl struct {
	mu             sync.Mutex
	lastCheckTime  time.Time
	lastestInfo    Info
	statusListener func(Info)

	pending *updater.Release

	appAsset       *Asset
	stagedPath     string
	stagedDir      string
	cancelDownload context.CancelFunc
}

var client = &http.Client{
	Timeout: 0, // No hard body cutoff for streaming downloads
	Transport: &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   15 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 15 * time.Second,
		IdleConnTimeout:       90 * time.Second,
	},
}

func newGitHubProvider() (*github.Provider, error) {
	return github.New(github.Config{
		Repository:   RepoOwner + "/" + RepoName,
		HTTPClient:   client,
		AssetMatcher: customAssetMatcher,
	})
}

// New creates a new Updater instance.
func New() Updater {
	return &updaterImpl{
		lastestInfo: Info{
			CurrentVersion: buildinfo.Version,
			State:          updater.StateIdle,
		},
	}
}

func (u *updaterImpl) SetStatusListener(listener func(Info)) {
	u.mu.Lock()
	u.statusListener = listener
	u.mu.Unlock()
}

func (u *updaterImpl) setState(s updater.State, errStr string) Info {
	u.mu.Lock()
	u.lastestInfo.State = s
	u.lastestInfo.Error = errStr
	info := u.lastestInfo
	listener := u.statusListener
	u.mu.Unlock()

	if listener != nil {
		listener(info)
	}
	return info
}

func (u *updaterImpl) Info() Info {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.lastestInfo
}

func (u *updaterImpl) Check(ctx context.Context) (Info, error) {
	u.mu.Lock()

	if isInProgress(u.lastestInfo.State) {
		defer u.mu.Unlock()
		return u.lastestInfo, ErrUpdateInProgress
	}

	// Use cached check if within cooldown
	if !u.lastCheckTime.IsZero() && time.Since(u.lastCheckTime) < checkCooldown {
		defer u.mu.Unlock()
		return u.lastestInfo, nil
	}

	currentVer := u.lastestInfo.CurrentVersion
	u.mu.Unlock()

	u.setState(updater.StateChecking, "")

	logger.Infof("Checking for updates (current version: %s)", buildinfo.Version)

	provider, err := newGitHubProvider()
	if err != nil {
		logger.Warnf("Update check provider failed: %v", err)
		return u.setState(updater.StateError, "github provider not configured"), errors.New("github provider not configured")
	}

	checkCtx, checkCancel := context.WithTimeout(ctx, 30*time.Second)
	defer checkCancel()

	rel, err := provider.Check(checkCtx, updater.CheckRequest{
		CurrentVersion: currentVer,
		Platform:       runtime.GOOS,
		Arch:           runtime.GOARCH,
	})

	if err != nil {
		u.mu.Lock()
		u.lastestInfo.LatestVersion = ""
		u.mu.Unlock()
		logger.Warnf("Update check failed: %v", err)
		return u.setState(updater.StateError, err.Error()), err
	}

	if rel == nil {
		u.mu.Lock()
		u.lastCheckTime = time.Now()
		u.mu.Unlock()
		logger.Debugf("Application is up to date (%s)", currentVer)
		return u.setState(updater.StateUpToDate, ""), nil
	}

	downloadURL, _ := rel.Metadata["github.asset.url"].(string)
	asset := &Asset{
		Name:        rel.Artifact.Filename,
		Size:        rel.Artifact.Size,
		DownloadURL: downloadURL,
	}
	if rel.Verification != nil && len(rel.Verification.Digest) > 0 {
		asset.Digest = "sha256:" + hex.EncodeToString(rel.Verification.Digest)
	}

	u.mu.Lock()
	u.pending = rel
	u.appAsset = asset
	u.lastestInfo.LatestVersion = rel.Version
	u.lastestInfo.Notes = sanitizeNotes(rel.Notes, defaultMaxNotesRunes)
	u.lastCheckTime = time.Now()
	u.mu.Unlock()

	logger.Debugf("New update available: %s -> %s (asset: %s)", currentVer, rel.Version, asset.Name)
	return u.setState(updater.StateAvailable, ""), nil
}

func (u *updaterImpl) Download(ctx context.Context, progress func(updater.Progress)) (string, error) {
	u.mu.Lock()

	if runtime.GOOS == "darwin" {
		u.mu.Unlock()
		return "", errors.New("update from app is not supported on macOS; please download the new version manually")
	}

	if isInProgress(u.lastestInfo.State) {
		u.mu.Unlock()
		return "", ErrUpdateInProgress
	}

	// If appAsset or pending release is missing, attempt to check for updates
	if u.appAsset == nil || u.pending == nil {
		u.mu.Unlock()
		info, err := u.Check(ctx)
		if err != nil {
			return "", err
		}

		u.mu.Lock()
		if info.State != updater.StateAvailable || u.appAsset == nil || u.pending == nil {
			u.mu.Unlock()
			return "", ErrUpdateNotReady
		}
	}

	if u.lastestInfo.State != updater.StateAvailable && u.lastestInfo.State != updater.StateError {
		u.mu.Unlock()
		return "", ErrUpdateNotReady
	}

	provider, err := newGitHubProvider()
	if err != nil {
		u.mu.Unlock()
		return "", errors.New("github provider not configured")
	}

	// Clean up any stale staged directory before downloading fresh
	if u.stagedDir != "" {
		_ = os.RemoveAll(u.stagedDir)
		u.stagedDir = ""
		u.stagedPath = ""
	}

	asset := u.appAsset
	pending := u.pending

	downloadCtx, cancel := context.WithCancel(ctx)
	u.cancelDownload = cancel
	u.mu.Unlock()
	u.setState(updater.StateDownloading, "")

	logger.Debugf("Starting update download workflow for %s (url: %s)", asset.Name, asset.DownloadURL)

	defer func() {
		cancel()
		u.mu.Lock()
		u.cancelDownload = nil
		u.mu.Unlock()
	}()

	result, err := downloadViaProvider(downloadCtx, provider, pending, asset, progress, func() {
		u.setState(updater.StateVerifying, "")
	})

	if err != nil {
		if errors.Is(err, context.Canceled) {
			u.setState(updater.StateAvailable, "")
			return "", err
		}
		u.setState(updater.StateError, err.Error())
		return "", err
	}

	// Check if context was cancelled while DownloadAsset was completing
	if downloadCtx.Err() != nil {
		if result != nil && result.TempDir != "" {
			_ = os.RemoveAll(result.TempDir)
		}
		u.setState(updater.StateAvailable, "")
		return "", downloadCtx.Err()
	}

	u.mu.Lock()
	if u.stagedDir != "" && u.stagedDir != result.TempDir {
		logger.Debugf("Cleaning up previous staged directory: %s", u.stagedDir)
		_ = os.RemoveAll(u.stagedDir)
	}

	u.stagedPath = result.StagedPath
	u.stagedDir = result.TempDir
	u.mu.Unlock()
	logger.Debugf("Update successfully downloaded and staged at: %s (size: %d bytes)", result.StagedPath, result.Size)
	return result.StagedPath, nil
}

// Apply installs the downloaded and verified update.
func (u *updaterImpl) Apply() error {
	u.mu.Lock()
	if u.lastestInfo.State == updater.StateInstalling {
		u.mu.Unlock()
		logger.Debugf("Apply aborted: install already in progress")
		return ErrUpdateInProgress
	}
	if u.stagedPath == "" {
		u.mu.Unlock()
		logger.Debugf("Apply aborted: no staged update path ready")
		return ErrUpdateNotReady
	}

	staged := u.stagedPath
	stagedDir := u.stagedDir
	u.mu.Unlock()
	u.setState(updater.StateInstalling, "")

	logger.Infof("Installing update from staged file %s", staged)
	if err := applyStaged(staged); err != nil {
		logger.Errorf("Apply failed: %v", err)
		if stagedDir != "" {
			logger.Debugf("Removing staged directory after apply failure: %s", stagedDir)
			_ = os.RemoveAll(stagedDir)
			u.mu.Lock()
			u.stagedDir = ""
			u.stagedPath = ""
			u.mu.Unlock()
		}
		u.setState(updater.StateError, err.Error())
		return fmt.Errorf("%w: %v", ErrInstallFailed, err)
	}

	u.mu.Lock()
	u.stagedDir = ""
	u.stagedPath = ""
	u.mu.Unlock()
	u.setState(updater.StateReady, "")

	logger.Debugf("Apply successfully completed for staged file: %s", staged)
	return nil
}

// Reset returns the updater to updater.StateIdle or updater.StateAvailable if an update is available.
func (u *updaterImpl) Reset() {
	u.mu.Lock()
	logger.Debugf("Resetting updater state (current state: %s, hasStagedPath: %v)", u.lastestInfo.State, u.stagedPath != "")
	if u.cancelDownload != nil {
		logger.Debugf("Cancelling active download context")
		u.cancelDownload()
		u.cancelDownload = nil
	}

	// Never delete the staged update directory or disrupt state while installation is in progress.
	if u.lastestInfo.State == updater.StateInstalling {
		u.mu.Unlock()
		return
	}

	nextState := updater.StateIdle
	if u.appAsset != nil {
		nextState = updater.StateAvailable
	}

	u.pending = nil
	u.lastCheckTime = time.Time{}
	u.mu.Unlock()
	u.setState(nextState, "")
}

func sanitizeNotes(notes string, maxRunes int) string {
	notes = strings.TrimSpace(strings.ReplaceAll(notes, "\r\n", "\n"))
	var b strings.Builder
	count := 0
	for _, r := range notes {
		if r != '\n' && r != '\t' && r < 32 {
			continue
		}
		if count == maxRunes {
			return b.String() + "…"
		}
		b.WriteRune(r)
		count++
	}
	return b.String()
}
