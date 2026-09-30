//go:build windows

package update

import (
	"strings"
	"testing"
)

func TestBuildUpdateScript(t *testing.T) {
	installer := `C:\Users\test\AppData\Local\Temp\epos-proxy-update\epos-proxy-win64-installer.exe`
	targetExe := `C:\Program Files\ePOS proxy\epos-proxy.exe`
	logFile := `C:\Users\test\AppData\Roaming\EposProxy\logs\update.log`
	pid := 12345

	script := buildUpdateScript(installer, targetExe, logFile, pid)

	// Verify CRLF line endings
	if !strings.Contains(script, "\r\n") {
		t.Errorf("expected script to have CRLF line endings")
	}

	// Verify installer and target paths
	if !strings.Contains(script, `set "INSTALLER=`+installer+`"`) {
		t.Errorf("script missing installer assignment")
	}
	if !strings.Contains(script, `set "TARGET_EXE=`+targetExe+`"`) {
		t.Errorf("script missing targetExe assignment")
	}
	if !strings.Contains(script, `set "TARGET_DIR=C:\Program Files\ePOS proxy"`) {
		t.Errorf("script missing targetDir assignment")
	}
	if !strings.Contains(script, `set "LOGFILE=`+logFile+`"`) {
		t.Errorf("script missing logFile assignment")
	}
	if !strings.Contains(script, `set "TARGET_PID=12345"`) {
		t.Errorf("script missing targetPID assignment")
	}

	// Verify start "" /wait "%INSTALLER%" /S
	if !strings.Contains(script, `start "" /wait "%INSTALLER%" /S`) {
		t.Errorf("script must invoke installer via start \"\" /wait to support UAC elevation and waiting")
	}

	// Verify PID waiting loop and taskkill
	if !strings.Contains(script, `tasklist /FI "PID eq %TARGET_PID%"`) {
		t.Errorf("script must check for target PID via tasklist")
	}
	if !strings.Contains(script, `taskkill /F /PID %TARGET_PID%`) {
		t.Errorf("script must have force termination fallback")
	}

	// Verify relaunching
	if !strings.Contains(script, `start "" /D "%TARGET_DIR%" "%TARGET_EXE%"`) {
		t.Errorf("script must relaunch target executable in target directory")
	}

	// Verify logging redirection
	if !strings.Contains(script, `>> "%LOGFILE%" 2>&1`) {
		t.Errorf("script must redirect output to log file")
	}
}

func TestBuildUpdateScriptEscaping(t *testing.T) {
	installer := `C:\Users\foo%20bar\installer.exe`
	targetExe := `C:\Program Files (x86)\App & More\app.exe`
	logFile := `C:\Logs%date%\update.log`
	pid := 9999

	script := buildUpdateScript(installer, targetExe, logFile, pid)

	// Percent signs should be escaped as %% for batch safety
	if strings.Contains(script, `foo%20bar`) {
		t.Errorf("expected percent sign to be escaped as %%%%")
	}
	if !strings.Contains(script, `foo%%20bar`) {
		t.Errorf("expected script to contain foo%%%%20bar")
	}
	if !strings.Contains(script, `Logs%%%%date%%%%`) {
		t.Errorf("expected script to contain Logs%%%%%%%%date%%%%%%%%")
	}
}
