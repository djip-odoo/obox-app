package update

import (
	"strings"
	"testing"
)

func TestSelectAsset_Linux(t *testing.T) {
	assets := []Asset{
		{Name: "obox-app-win64-installer-v1.0.17.exe", DownloadURL: "https://github.com/test/win.exe"},
		{Name: "obox-app-linux64-v1.0.17", DownloadURL: "https://github.com/test/linux64"},
		{Name: "SHA256SUMS", DownloadURL: "https://github.com/test/sums"},
	}

	selected, err := SelectAsset(assets, "linux", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.Name != "obox-app-linux64-v1.0.17" {
		t.Errorf("got %q, want obox-app-linux64-v1.0.17", selected.Name)
	}
}

func TestSelectAsset_Windows(t *testing.T) {
	assets := []Asset{
		{Name: "obox-app-linux64-v1.0.17", DownloadURL: "https://github.com/test/linux64"},
		{Name: "obox-app-win64-installer-v1.0.17.exe", DownloadURL: "https://github.com/test/win.exe"},
	}

	selected, err := SelectAsset(assets, "windows", "amd64")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if selected.Name != "obox-app-win64-installer-v1.0.17.exe" {
		t.Errorf("got %q, want obox-app-win64-installer-v1.0.17.exe", selected.Name)
	}
}

func TestSelectAsset_UnsupportedPlatform(t *testing.T) {
	assets := []Asset{
		{Name: "obox-app-linux64-v1.0.17", DownloadURL: "https://github.com/test/linux64"},
	}

	_, err := SelectAsset(assets, "freebsd", "amd64")
	if err == nil {
		t.Fatal("expected error for unsupported OS, got nil")
	}
}

func TestSelectAsset_MissingPlatformAsset(t *testing.T) {
	assets := []Asset{
		{Name: "obox-app-win64-installer-v1.0.17.exe", DownloadURL: "https://github.com/test/win.exe"},
	}

	_, err := SelectAsset(assets, "linux", "amd64")
	if err == nil {
		t.Fatal("expected error when platform asset missing, got nil")
	}
}

func TestParseChecksums(t *testing.T) {
	raw := `
# Release checksums
e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855  obox-app-linux64-v1.0.17
a9f621fa06c701e541266dc9d81adcf267419301e41135c9ccbf8efc12e83eb8 *obox-app-win64-installer-v1.0.17.exe
`
	sums, err := ParseChecksums(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("ParseChecksums error: %v", err)
	}

	if got := sums["obox-app-linux64-v1.0.17"]; got != "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855" {
		t.Errorf("linux checksum mismatch, got %q", got)
	}
	if got := sums["obox-app-win64-installer-v1.0.17.exe"]; got != "a9f621fa06c701e541266dc9d81adcf267419301e41135c9ccbf8efc12e83eb8" {
		t.Errorf("win checksum mismatch, got %q", got)
	}
}

func TestValidateDownloadURL(t *testing.T) {
	valid := []string{
		"https://github.com/djip-odoo/obox-app/releases/download/v1.0.17/obox-app-linux64-v1.0.17",
		"https://objects.githubusercontent.com/github-production-release-asset-2e65be/123",
		"http://127.0.0.1:45455/pkg.bin",
		"http://localhost:8080/pkg.bin",
	}
	for _, u := range valid {
		if err := ValidateDownloadURL(u); err != nil {
			t.Errorf("ValidateDownloadURL(%q) unexpected error: %v", u, err)
		}
	}

	invalid := []string{
		"https://evil.com/malware.exe",
		"https://github.com.attacker.com/malware.exe",
		"ftp://github.com/test",
	}
	for _, u := range invalid {
		if err := ValidateDownloadURL(u); err == nil {
			t.Errorf("ValidateDownloadURL(%q) expected error, got nil", u)
		}
	}
}
