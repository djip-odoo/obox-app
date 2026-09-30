//go:build windows

package update

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"epos-proxy/internal/logger"
)

// noWindowFlag = CREATE_NO_WINDOW: the helper console is never shown.
const noWindowFlag = 0x08000000

// buildUpdateScript generates a robust Windows batch script that:
// 1. Logs every operation and timestamp to logFile.
// 2. Waits for the parent application (pid) to terminate (up to 30s) so file locks are released.
// 3. Runs the NSIS installer silently with elevated privileges (via start "" /wait ... /S).
// 4. Logs the installer exit code and any errors.
// 5. Relaunches the updated application in its install directory.
func buildUpdateScript(installer, targetExe, logFile string, pid int) string {
	targetDir := filepath.Dir(targetExe)

	escapeBatch := func(s string) string {
		return strings.ReplaceAll(s, "%", "%%")
	}

	content := fmt.Sprintf(`@echo off
setlocal
set "PATH=%%SystemRoot%%\System32;%%SystemRoot%%;%%PATH%%"

set "LOGFILE=%s"
set "INSTALLER=%s"
set "TARGET_EXE=%s"
set "TARGET_DIR=%s"
set "TARGET_PID=%d"

echo ========================================================>> "%%LOGFILE%%" 2>&1
echo [%%DATE%% %%TIME%%] Windows update script started>> "%%LOGFILE%%" 2>&1
echo [%%DATE%% %%TIME%%] Script file: %%~f0>> "%%LOGFILE%%" 2>&1
echo [%%DATE%% %%TIME%%] Installer: "%%INSTALLER%%">> "%%LOGFILE%%" 2>&1
echo [%%DATE%% %%TIME%%] Target EXE: "%%TARGET_EXE%%">> "%%LOGFILE%%" 2>&1
echo [%%DATE%% %%TIME%%] Target Dir: "%%TARGET_DIR%%">> "%%LOGFILE%%" 2>&1
echo [%%DATE%% %%TIME%%] Target PID: %%TARGET_PID%%>> "%%LOGFILE%%" 2>&1

:: 1. Wait for the parent epos-proxy process to terminate
echo [%%DATE%% %%TIME%%] Waiting for process %%TARGET_PID%% to exit...>> "%%LOGFILE%%" 2>&1
set WAIT_COUNT=0

:WAIT_PID
tasklist /FI "PID eq %%TARGET_PID%%" 2>nul | findstr /i "%%TARGET_PID%%" >nul
if errorlevel 1 goto PID_EXITED

set /a WAIT_COUNT+=1
if %%WAIT_COUNT%% geq 30 (
    echo [%%DATE%% %%TIME%%] Process %%TARGET_PID%% still running after 30s. Force terminating...>> "%%LOGFILE%%" 2>&1
    taskkill /F /PID %%TARGET_PID%%>> "%%LOGFILE%%" 2>&1
    goto PID_EXITED
)
ping -n 2 127.0.0.1 >nul
goto WAIT_PID

:PID_EXITED
echo [%%DATE%% %%TIME%%] Target process has exited.>> "%%LOGFILE%%" 2>&1

:: Sleep 3 seconds so the OS releases all file locks, mutexes, and socket handles
ping -n 4 127.0.0.1 >nul

:: 2. Verify installer exists
if not exist "%%INSTALLER%%" (
    echo [%%DATE%% %%TIME%%] ERROR: Installer file not found at "%%INSTALLER%%"!>> "%%LOGFILE%%" 2>&1
    echo [%%DATE%% %%TIME%%] Attempting to relaunch existing application executable...>> "%%LOGFILE%%" 2>&1
    if exist "%%TARGET_EXE%%" (
        start "" /D "%%TARGET_DIR%%" "%%TARGET_EXE%%">> "%%LOGFILE%%" 2>&1
    )
    exit /b 1
)

:: 3. Run the installer silently with elevation
echo [%%DATE%% %%TIME%%] Running installer: "%%INSTALLER%%" /S>> "%%LOGFILE%%" 2>&1
start "" /wait "%%INSTALLER%%" /S
set INSTALL_ERR=%%ERRORLEVEL%%
echo [%%DATE%% %%TIME%%] Installer completed with exit code: %%INSTALL_ERR%%>> "%%LOGFILE%%" 2>&1

if %%INSTALL_ERR%% equ 0 (
    echo [%%DATE%% %%TIME%%] Update installer completed successfully!>> "%%LOGFILE%%" 2>&1
    del /f /q "%%INSTALLER%%">> "%%LOGFILE%%" 2>&1
) else (
    echo [%%DATE%% %%TIME%%] WARNING: Installer exited with non-zero error code %%INSTALL_ERR%%.>> "%%LOGFILE%%" 2>&1
)

:: Wait for installer file operations to settle
ping -n 3 127.0.0.1 >nul

:: 4. Relaunch the application
:: Check if the executable is at the target location or in Program Files
if not exist "%%TARGET_EXE%%" (
    if exist "%%ProgramFiles%%\ePOS proxy\epos-proxy.exe" (
        set "TARGET_EXE=%%ProgramFiles%%\ePOS proxy\epos-proxy.exe"
        set "TARGET_DIR=%%ProgramFiles%%\ePOS proxy"
    )
)

echo [%%DATE%% %%TIME%%] Launching updated application: "%%TARGET_EXE%%">> "%%LOGFILE%%" 2>&1
if exist "%%TARGET_EXE%%" (
    start "" /D "%%TARGET_DIR%%" "%%TARGET_EXE%%">> "%%LOGFILE%%" 2>&1
    if errorlevel 1 (
        echo [%%DATE%% %%TIME%%] ERROR: Failed to launch application (errorlevel %%ERRORLEVEL%%)>> "%%LOGFILE%%" 2>&1
    ) else (
        echo [%%DATE%% %%TIME%%] Application launched successfully.>> "%%LOGFILE%%" 2>&1
    )
) else (
    echo [%%DATE%% %%TIME%%] ERROR: Application executable not found at "%%TARGET_EXE%%"!>> "%%LOGFILE%%" 2>&1
)

echo [%%DATE%% %%TIME%%] Update script finished with code %%INSTALL_ERR%%>> "%%LOGFILE%%" 2>&1
echo ========================================================>> "%%LOGFILE%%" 2>&1
exit /b %%INSTALL_ERR%%
`,
		escapeBatch(logFile),
		escapeBatch(installer),
		escapeBatch(targetExe),
		escapeBatch(targetDir),
		pid,
	)

	// Ensure Windows CRLF line endings
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\n", "\r\n")
	return content
}

// Apply schedules a detached updater script that waits for the running app
// process to exit, runs the downloaded installer silently with elevation,
// and then relaunches the updated app. All actions, output, and errors are
// logged to update.log.
func Apply(downloaded string) error {
	exe, err := os.Executable()
	if err != nil {
		err = fmt.Errorf("apply failed: cannot locate executable: %w", err)
		logger.Errorf("%v", err)
		return err
	}
	if resolved, errEval := filepath.EvalSymlinks(exe); errEval == nil && resolved != "" {
		exe = resolved
	}

	if _, errStat := os.Stat(downloaded); errStat != nil {
		err = fmt.Errorf("apply failed: downloaded file not found at %s: %w", downloaded, errStat)
		logger.Errorf("%v", err)
		return err
	}

	if strings.Contains(downloaded, "\"") || strings.Contains(exe, "\"") {
		err = fmt.Errorf("apply failed: path contains double quotes: installer=%q, exe=%q", downloaded, exe)
		logger.Errorf("%v", err)
		return err
	}

	logDir := logger.LogDirectory()
	if logDir == "" {
		if userConfig, errConfig := os.UserConfigDir(); errConfig == nil {
			logDir = filepath.Join(userConfig, "EposProxy", "logs")
		} else {
			logDir = filepath.Dir(downloaded)
		}
	}
	_ = os.MkdirAll(logDir, 0o755)
	logFile := filepath.Join(logDir, "update.log")

	// Append initial record to update.log directly from Go
	if f, errOpen := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); errOpen == nil {
		ts := time.Now().Format("2006-01-02 15:04:05")
		_, _ = fmt.Fprintf(f, "\n[%s] Apply requested: installer=%s, target=%s\n", ts, downloaded, exe)
		_ = f.Close()
	}

	logger.Infof("Scheduling Windows update installer:")
	logger.Infof("  Installer:  %s", downloaded)
	logger.Infof("  Target exe: %s", exe)
	logger.Infof("  Log file:   %s", logFile)

	scriptDir := filepath.Dir(downloaded)
	scriptPath := filepath.Join(scriptDir, "apply_update.bat")
	pid := os.Getpid()

	batchContent := buildUpdateScript(downloaded, exe, logFile, pid)
	if errWrite := os.WriteFile(scriptPath, []byte(batchContent), 0o755); errWrite != nil {
		err = fmt.Errorf("apply failed: cannot write updater script: %w", errWrite)
		logger.Errorf("%v", err)
		if f, errOpen := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); errOpen == nil {
			ts := time.Now().Format("2006-01-02 15:04:05")
			_, _ = fmt.Fprintf(f, "[%s] ERROR: %v\n", ts, err)
			_ = f.Close()
		}
		return err
	}

	cmd := exec.Command("cmd.exe", "/C", scriptPath)
	cmd.Dir = scriptDir
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: noWindowFlag}
	if errStart := cmd.Start(); errStart != nil {
		err = fmt.Errorf("apply failed: cannot launch updater script: %w", errStart)
		logger.Errorf("%v", err)
		if f, errOpen := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); errOpen == nil {
			ts := time.Now().Format("2006-01-02 15:04:05")
			_, _ = fmt.Fprintf(f, "[%s] ERROR: %v\n", ts, err)
			_ = f.Close()
		}
		return err
	}

	logger.Infof("Updater script launched successfully (PID: %d)", cmd.Process.Pid)
	_ = cmd.Process.Release()
	return nil
}