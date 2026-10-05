package update

import "errors"

var (
	// ErrNoReleases is returned when the repository has no published releases.
	ErrNoReleases = errors.New("no published releases found")

	// ErrNoUpdate is returned when the application is already running the latest version.
	ErrNoUpdate = errors.New("already running the latest version")

	// ErrNoCompatibleAsset is returned when a release has no build for the current platform.
	ErrNoCompatibleAsset = errors.New("no compatible asset found for this platform")

	// ErrInvalidVersion is returned when a version string cannot be parsed as semver.
	ErrInvalidVersion = errors.New("invalid or malformed semantic version")

	// ErrDowngradeRejected is returned when a candidate version is older than the current version.
	ErrDowngradeRejected = errors.New("cannot downgrade to an older version")

	// ErrChecksumMismatch is returned when the downloaded file's SHA-256 does not match.
	ErrChecksumMismatch = errors.New("update file checksum verification failed")

	// ErrUpdateInProgress is returned when a check, download, or install is already running.
	ErrUpdateInProgress = errors.New("an update operation is already in progress")

	// ErrUpdateNotReady is returned when Apply is called before a verified update is staged.
	ErrUpdateNotReady = errors.New("no verified update is staged and ready to install")

	// ErrInstallFailed is returned when the installer or replacement process fails.
	ErrInstallFailed = errors.New("failed to install the update")

	// ErrUntrustedSource is returned when a download URL does not match trusted domains.
	ErrUntrustedSource = errors.New("download URL is from an untrusted source")
)
