package update

import "time"

// State represents the current lifecycle state of the updater.
type State string

const (
	StateIdle            State = "idle"
	StateChecking        State = "checking"
	StateUpdateAvailable State = "update_available"
	StateDownloading     State = "downloading"
	StateReadyToInstall  State = "ready_to_install"
	StateInstalling      State = "installing"
	StateRestarting      State = "restarting"
	StateFailed          State = "failed"
)

// Asset represents a downloadable binary or installer in a release.
type Asset struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	DownloadURL string `json:"browser_download_url"`
	Digest      string `json:"digest,omitempty"` // e.g. "sha256:abcd..."
	ExpectedSHA string `json:"-"`                // SHA-256 hex string without prefix
}

// Release represents a GitHub release.
type Release struct {
	ID          int64     `json:"id"`
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	Draft       bool      `json:"draft"`
	Prerelease  bool      `json:"prerelease"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
}

// Info summarizes the current updater state and findings for the UI.
type Info struct {
	CheckOK        bool   `json:"checkOk"`
	Available      bool   `json:"available"`
	State          State  `json:"state"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	DownloadURL    string `json:"downloadUrl"`
	AssetName      string `json:"assetName"`
	AssetSize      int64  `json:"assetSize"`
	Notes          string `json:"notes"`
	Error          string `json:"error,omitempty"`
}

// ProgressFunc reports progress during download.
type ProgressFunc func(downloaded, total int64, percent int)
