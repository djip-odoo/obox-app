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

// Apply stages the verified binary over the running executable and restarts it.
func Apply(stagedPath string) error {
	exe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("apply failed: cannot locate executable: %w", err)
	}

	// Resolve symlinks to target the real executable
	resolvedExe, err := filepath.EvalSymlinks(exe)
	if err == nil && resolvedExe != "" {
		exe = resolvedExe
	}

	return ApplyToExecutable(os.Getpid(), exe, stagedPath)
}

// ApplyToExecutable performs atomic replacement with parent PID wait, backup,
// rollback on failure, and relaunch. Exported to allow thorough testing.
func ApplyToExecutable(parentPID int, exePath, stagedPath string) error {
	if stagedPath == "" {
		return fmt.Errorf("%w: empty staged path", ErrUpdateNotReady)
	}

	stagedInfo, err := os.Stat(stagedPath)
	if err != nil {
		return fmt.Errorf("%w: staged update file not found at %s: %v", ErrUpdateNotReady, stagedPath, err)
	}
	if stagedInfo.Size() == 0 {
		return fmt.Errorf("%w: staged update file is empty", ErrUpdateNotReady)
	}

	// Ensure staged file has execution permissions
	if err := os.Chmod(stagedPath, 0o755); err != nil {
		return fmt.Errorf("apply failed: cannot set permissions on staged file: %w", err)
	}

	exeDir := filepath.Dir(exePath)
	// Verify target directory is writable
	testFile := filepath.Join(exeDir, fmt.Sprintf(".write_test_%d", parentPID))
	if err := os.WriteFile(testFile, []byte{1}, 0o600); err != nil {
		return fmt.Errorf("apply failed: installation directory %s is not writable: %w", exeDir, err)
	}
	_ = os.Remove(testFile)

	backupPath := exePath + ".old"
	scriptFile := filepath.Join(exeDir, fmt.Sprintf(".updater_%d.sh", parentPID))

	// Generate helper script that accepts args ($1: PID, $2: EXE, $3: STAGED, $4: BACKUP, $5: SCRIPT)
	// Passing variables as shell parameters eliminates shell injection and handles spaces properly.
	scriptContent := `#!/bin/sh
PID="$1"
EXE="$2"
STAGED="$3"
BACKUP="$4"
SCRIPT="$5"

# 1. Wait for parent process to exit (up to 15 seconds)
WAITED=0
while kill -0 "$PID" 2>/dev/null; do
    sleep 0.2
    WAITED=$((WAITED + 1))
    if [ "$WAITED" -ge 75 ]; then
        # Parent did not exit in time; abort without touching files
        rm -f "$SCRIPT"
        exit 1
    fi
done

# Small buffer to ensure all file locks/descriptors are closed
sleep 0.3

# 2. Backup current executable
rm -f "$BACKUP"
if ! mv -f "$EXE" "$BACKUP"; then
    rm -f "$SCRIPT"
    exit 1
fi

# 3. Move staged update into place
if ! mv -f "$STAGED" "$EXE"; then
    # ROLLBACK: restore backup
    mv -f "$BACKUP" "$EXE"
    rm -f "$SCRIPT"
    exit 1
fi

chmod 755 "$EXE"

# 4. Replacement succeeded: remove backup
rm -f "$BACKUP"

# 5. Relaunch the updated application detached
nohup "$EXE" >/dev/null 2>&1 &

# 6. Clean up self
rm -f "$SCRIPT"
exit 0
`

	if err := os.WriteFile(scriptFile, []byte(scriptContent), 0o755); err != nil {
		return fmt.Errorf("apply failed: cannot write updater script: %w", err)
	}

	logger.Infof("Launching detached updater script %s (PID: %d, target: %s)", scriptFile, parentPID, exePath)

	cmd := exec.Command("/bin/sh", scriptFile, fmt.Sprint(parentPID), exePath, stagedPath, backupPath, scriptFile)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if err := cmd.Start(); err != nil {
		_ = os.Remove(scriptFile)
		return fmt.Errorf("apply failed: cannot start updater process: %w", err)
	}

	// Release process so it runs independently in the background
	_ = cmd.Process.Release()
	return nil
}
