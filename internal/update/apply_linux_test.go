package update

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"obox-app/internal/testutil"
)

func TestApply_MissingFile(t *testing.T) {
	resetLinuxApplyState()
	defer resetLinuxApplyState()

	nonExistent := filepath.Join(t.TempDir(), "nonexistent.deb")
	err := applyStaged(nonExistent)
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrUpdateNotReady))
}

func TestApply_EmptyPath(t *testing.T) {
	resetLinuxApplyState()
	defer resetLinuxApplyState()

	err := applyStaged("")
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrUpdateNotReady))
}

func TestApply_EmptyFile(t *testing.T) {
	resetLinuxApplyState()
	defer resetLinuxApplyState()

	emptyDeb := filepath.Join(t.TempDir(), "empty.deb")
	_ = os.WriteFile(emptyDeb, []byte{}, 0o644)

	err := applyStaged(emptyDeb)
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrUpdateNotReady))
}

func TestApply_NotDebFile(t *testing.T) {
	resetLinuxApplyState()
	defer resetLinuxApplyState()

	rawBin := filepath.Join(t.TempDir(), "app-binary")
	_ = os.WriteFile(rawBin, []byte("binary data"), 0o755)

	err := applyStaged(rawBin)
	testutil.ExpectedError(t, err)
	testutil.ExpectedTrue(t, errors.Is(err, ErrUpdateNotReady))
}

func TestApply_Success(t *testing.T) {
	resetLinuxApplyState()
	defer resetLinuxApplyState()

	debFile := filepath.Join(t.TempDir(), "test-app.deb")
	_ = os.WriteFile(debFile, []byte("fake deb package content"), 0o644)

	var launchedPath string
	launchDebInstallerFn = func(debPath string) error {
		launchedPath = debPath
		return nil
	}

	err := applyStaged(debFile)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, launchedPath, debFile)

	// Second apply should return ErrUpdateInProgress
	err2 := applyStaged(debFile)
	testutil.ExpectedError(t, err2)
	testutil.ExpectedTrue(t, errors.Is(err2, ErrUpdateInProgress))
}

func TestPlatformNeedsQuitAfterApply(t *testing.T) {
	testutil.ExpectedTrue(t, PlatformNeedsQuitAfterApply())
}
