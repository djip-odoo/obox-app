//go:build windows

package update

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/sys/windows"

	"obox-app/internal/logger"
)

// Apply launches the downloaded installer with elevation and normal window
// visibility. The caller must quit the running application so that its
// executable file lock is released before the installer attempts to overwrite it.
func Apply(downloaded string) error {
	if _, err := os.Stat(downloaded); err != nil {
		err = fmt.Errorf("apply failed: downloaded file not found at %s: %w", downloaded, err)
		logger.Errorf("%v", err)
		return err
	}

	logger.Infof("Launching Windows installer: %s", downloaded)

	verb := windows.StringToUTF16Ptr("open")
	file := windows.StringToUTF16Ptr(downloaded)
	dir := windows.StringToUTF16Ptr(filepath.Dir(downloaded))

	if err := windows.ShellExecute(0, verb, file, nil, dir, windows.SW_SHOWNORMAL); err != nil {
		err = fmt.Errorf("apply failed: cannot launch installer %s: %w", downloaded, err)
		logger.Errorf("%v", err)
		return err
	}

	logger.Infof("Installer launched successfully: %s", downloaded)
	return nil
}
