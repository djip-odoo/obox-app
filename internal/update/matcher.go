package update

import (
	"strings"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

// customAssetMatcher selects the appropriate release asset for the target platform.
// - On Linux: only Debian (.deb) packages are matched; raw binary updates are disallowed.
// - On Windows: executable installer files (.exe, preferring -installer.exe) are matched.
// - On Darwin: installer packages (.pkg, .dmg, .zip) are matched.
func customAssetMatcher(req updater.CheckRequest, assets []github.ReleaseAsset) int {
	plat := strings.ToLower(req.Platform)
	arch := strings.ToLower(req.Arch)

	switch plat {
	case "linux":
		for i, a := range assets {
			name := strings.ToLower(a.Name)
			if isChecksumOrSignature(name) {
				continue
			}
			// Only .deb packages are allowed for Linux updates
			if !strings.HasSuffix(name, ".deb") {
				continue
			}
			if !matchArchitecture(name, arch) {
				continue
			}
			return i
		}
		return -1

	case "windows":
		bestIdx := -1
		for i, a := range assets {
			name := strings.ToLower(a.Name)
			if isChecksumOrSignature(name) {
				continue
			}
			if !strings.HasSuffix(name, ".exe") {
				continue
			}
			if !matchPlatform(name, plat) {
				continue
			}
			if !matchArchitecture(name, arch) {
				continue
			}
			if strings.Contains(name, "installer") {
				return i
			}
			if bestIdx == -1 {
				bestIdx = i
			}
		}
		return bestIdx

	case "darwin":
		for i, a := range assets {
			name := strings.ToLower(a.Name)
			if isChecksumOrSignature(name) {
				continue
			}
			if !strings.HasSuffix(name, ".pkg") && !strings.HasSuffix(name, ".dmg") && !strings.HasSuffix(name, ".zip") {
				continue
			}
			if !matchPlatform(name, plat) {
				continue
			}
			if strings.Contains(name, "universal") || strings.Contains(name, "all") || matchArchitecture(name, arch) {
				return i
			}
		}
		return -1

	default:
		for i, a := range assets {
			name := strings.ToLower(a.Name)
			if isChecksumOrSignature(name) {
				continue
			}
			if matchPlatform(name, plat) && matchArchitecture(name, arch) {
				return i
			}
		}
		return -1
	}
}

func isChecksumOrSignature(name string) bool {
	lower := strings.ToLower(name)
	if strings.HasSuffix(lower, ".sig") || strings.HasSuffix(lower, ".asc") {
		return true
	}
	return strings.Contains(lower, "checksum") ||
		strings.Contains(lower, "sha256") ||
		strings.Contains(lower, "sha1") ||
		strings.Contains(lower, "md5")
}

func matchArchitecture(filename, arch string) bool {
	if arch == "" {
		return true
	}
	f := strings.ToLower(filename)
	a := strings.ToLower(arch)

	switch a {
	case "amd64", "x86_64", "x64":
		return strings.Contains(f, "amd64") || strings.Contains(f, "x86_64") || strings.Contains(f, "x64")
	case "arm64", "aarch64":
		return strings.Contains(f, "arm64") || strings.Contains(f, "aarch64")
	case "386", "i386", "x86":
		return strings.Contains(f, "386") || strings.Contains(f, "i386") || strings.Contains(f, "x86")
	default:
		return strings.Contains(f, a)
	}
}

func matchPlatform(filename, platform string) bool {
	if platform == "" {
		return true
	}
	f := strings.ToLower(filename)
	p := strings.ToLower(platform)

	switch p {
	case "linux":
		return strings.Contains(f, "linux")
	case "windows":
		return strings.Contains(f, "windows") || strings.Contains(f, "win")
	case "darwin":
		return strings.Contains(f, "darwin") || strings.Contains(f, "mac") || strings.Contains(f, "macos") || strings.Contains(f, "osx")
	default:
		return strings.Contains(f, p)
	}
}
