package update

import (
	"testing"

	"obox-app/internal/testutil"

	"github.com/wailsapp/wails/v3/pkg/updater"
	"github.com/wailsapp/wails/v3/pkg/updater/providers/github"
)

func TestCustomAssetMatcher_Linux_PrefersDebOnly(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "obox-app-linux-amd64"},
		{Name: "obox-app-linux-amd64.deb"},
		{Name: "obox-app-windows-amd64-installer.exe"},
		{Name: "SHA256SUMS"},
	}

	req := updater.CheckRequest{
		Platform: "linux",
		Arch:     "amd64",
	}

	idx := customAssetMatcher(req, assets)
	testutil.ExpectedEqual(t, idx, 1)
	testutil.ExpectedEqual(t, assets[idx].Name, "obox-app-linux-amd64.deb")
}

func TestCustomAssetMatcher_Linux_RejectsRawBinary(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "obox-app-linux-amd64"},
		{Name: "SHA256SUMS"},
	}

	req := updater.CheckRequest{
		Platform: "linux",
		Arch:     "amd64",
	}

	idx := customAssetMatcher(req, assets)
	testutil.ExpectedEqual(t, idx, -1)
}

func TestCustomAssetMatcher_Linux_ArchMismatch(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "obox-app-linux-arm64.deb"},
	}

	req := updater.CheckRequest{
		Platform: "linux",
		Arch:     "amd64",
	}

	idx := customAssetMatcher(req, assets)
	testutil.ExpectedEqual(t, idx, -1)
}

func TestCustomAssetMatcher_Linux_MatchesStandardDebName(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "obox-app_1.0.17_amd64.deb"},
	}

	req := updater.CheckRequest{
		Platform: "linux",
		Arch:     "amd64",
	}

	idx := customAssetMatcher(req, assets)
	testutil.ExpectedEqual(t, idx, 0)
}

func TestCustomAssetMatcher_Windows_PrefersInstaller(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "obox-app-windows-amd64.exe"},
		{Name: "obox-app-windows-amd64-installer-1.0.17.exe"},
	}

	req := updater.CheckRequest{
		Platform: "windows",
		Arch:     "amd64",
	}

	idx := customAssetMatcher(req, assets)
	testutil.ExpectedEqual(t, idx, 1)
	testutil.ExpectedEqual(t, assets[idx].Name, "obox-app-windows-amd64-installer-1.0.17.exe")
}

func TestCustomAssetMatcher_Darwin_MatchesPkg(t *testing.T) {
	assets := []github.ReleaseAsset{
		{Name: "obox-app-darwin-arm64.pkg"},
		{Name: "obox-app-darwin-amd64.pkg"},
	}

	req := updater.CheckRequest{
		Platform: "darwin",
		Arch:     "arm64",
	}

	idx := customAssetMatcher(req, assets)
	testutil.ExpectedEqual(t, idx, 0)
}
