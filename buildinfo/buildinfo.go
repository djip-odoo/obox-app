package buildinfo

import (
	"fmt"
	"runtime"
)

var (
	Version   = "local"
	BuildTime = "unknown"
	Commit    = "none"
	BuildName = ""
)

func GetBuildName() string {
	if BuildName != "" {
		return BuildName
	}
	return fmt.Sprintf("obox-app-%s-%s", runtime.GOOS, runtime.GOARCH)
}

func GetVersionInfo() string {
	return "Version: " + Version + "\n" + "Build Time: " + BuildTime + "\n" + "Commit: " + Commit
}
