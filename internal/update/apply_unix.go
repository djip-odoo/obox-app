//go:build !windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"

	"obox-app/internal/logger"
)

// Apply stages the downloaded binary over the running executable and relaunches
// it. The swap happens inside a detached shell process because the current
// binary cannot replace itself while it is running.
func Apply(downloaded string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("apply failed: cannot locate executable: %w", err)
	}
	exeDir := filepath.Dir(exe)

	// Keep a copy next to the target so the final move stays on one filesystem.
	staged := filepath.Join(exeDir, ".obox-app-update")
	if err := copyFile(downloaded, staged); err != nil {
		return fmt.Errorf("apply failed: cannot stage update: %w", err)
	}
	if err := os.Chmod(staged, 0o755); err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("apply failed: cannot make update executable: %w", err)
	}
	logger.Infof("Applying update from %s", staged)

	script := fmt.Sprintf(`#!/bin/sh
sleep 1
rm -f "%[1]s.old"
mv -f "%[1]s" "%[1]s.old" 2>/dev/null || exit 1
mv -f "%[2]s" "%[1]s" || exit 1
chmod +x "%[1]s"
rm -f "%[1]s.old"
rm -f "%[2]s"
nohup "%[1]s" >/dev/null 2>&1 &
exit 0
`, exe, staged)

	cmd := exec.Command("/bin/sh", "-c", script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = os.Remove(staged)
		return fmt.Errorf("apply failed: cannot launch updater: %w", err)
	}
	_ = cmd.Process.Release()
	return nil
}

// copyFile copies src to dst, replacing an existing dst, and reports whether
// the copy matches the source byte count.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}

	n, copyErr := out.ReadFrom(in)
	closeErr := out.Close()
	if copyErr != nil {
		_ = os.Remove(dst)
		return copyErr
	}
	if closeErr != nil {
		_ = os.Remove(dst)
		return closeErr
	}
	if fi, err := in.Stat(); err == nil && n != fi.Size() {
		_ = os.Remove(dst)
		return fmt.Errorf("short copy: %d of %d bytes", n, fi.Size())
	}
	return nil
}
