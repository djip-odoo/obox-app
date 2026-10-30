//go:build linux

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"obox-app/internal/logger"
)

var (
	linuxApplyMu         sync.Mutex
	installerStarted     bool
	launchDebInstallerFn = launchDebInstaller
)

// applyStaged launches the downloaded .deb package via the desktop package installer
// (e.g. xdg-open, gdebi-gtk, gnome-software). The application quits after this call
// so that the package manager can update the installed files.
func applyStaged(downloaded string) error {
	linuxApplyMu.Lock()
	defer linuxApplyMu.Unlock()

	if installerStarted {
		return fmt.Errorf("%w: installer already started", ErrUpdateInProgress)
	}

	if downloaded == "" {
		return fmt.Errorf("%w: empty installer path", ErrUpdateNotReady)
	}

	info, err := os.Stat(downloaded)
	if err != nil {
		err = fmt.Errorf("%w: downloaded file not found at %s: %v", ErrUpdateNotReady, downloaded, err)
		logger.Errorf("%v", err)
		return err
	}
	if info.Size() == 0 {
		return fmt.Errorf("%w: installer file is empty", ErrUpdateNotReady)
	}

	if !strings.HasSuffix(strings.ToLower(downloaded), ".deb") {
		return fmt.Errorf("%w: staged file is not a .deb package: %s", ErrUpdateNotReady, downloaded)
	}

	// Ensure the parent directory and .deb package are readable by other users
	// (e.g. when apt/dpkg drops privileges to the _apt user during installation).
	if err := os.Chmod(filepath.Dir(downloaded), 0o755); err != nil {
		logger.Debugf("Could not chmod update directory %s: %v", filepath.Dir(downloaded), err)
	}
	if err := os.Chmod(downloaded, 0o644); err != nil {
		logger.Debugf("Could not chmod deb package %s: %v", downloaded, err)
	}

	logger.Infof("Launching Linux .deb installer: %s", downloaded)
	if err := launchDebInstallerFn(downloaded); err != nil {
		err = fmt.Errorf("apply failed: %w", err)
		logger.Errorf("%v", err)
		return err
	}

	installerStarted = true
	logger.Infof("Linux installer launched successfully: %s", downloaded)
	return nil
}

func launchDebInstaller(debPath string) error {
	candidates := [][]string{
		{"xdg-open", debPath},
		{"gdebi-gtk", debPath},
		{"gnome-software", "--local-filename=" + debPath},
		{"plasma-discover", debPath},
	}

	var lastErr error
	for _, args := range candidates {
		bin, err := exec.LookPath(args[0])
		if err != nil {
			continue
		}

		cmd := exec.Command(bin, args[1:]...)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Setsid: true,
		}
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil

		if err := cmd.Start(); err == nil {
			_ = cmd.Process.Release()
			logger.Infof("Successfully started installer process (%s %s)", bin, strings.Join(args[1:], " "))
			return nil
		}
		lastErr = err
		logger.Warnf("Failed to start installer via %s: %v", bin, err)
	}

	if lastErr != nil {
		return fmt.Errorf("cannot launch installer for %s: %w", debPath, lastErr)
	}
	return fmt.Errorf("no suitable package installer found to open %s (checked xdg-open, gdebi-gtk, gnome-software, plasma-discover)", debPath)
}

// PlatformNeedsQuitAfterApply reports whether the application must quit
// immediately after Apply() returns so the update can take effect.
//
// On Linux, launching the .deb package installer requires the existing
// application to quit so the package manager can replace its files.
func PlatformNeedsQuitAfterApply() bool { return true }

func resetLinuxApplyState() {
	linuxApplyMu.Lock()
	installerStarted = false
	launchDebInstallerFn = launchDebInstaller
	linuxApplyMu.Unlock()
}
