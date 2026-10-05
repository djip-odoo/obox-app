package update

import (
	"testing"
)

func TestParseSemVer(t *testing.T) {
	tests := []struct {
		input      string
		wantErr    bool
		wantMajor  int
		wantMinor  int
		wantPatch  int
		wantPreLen int
		wantBuild  string
	}{
		{"1.2.3", false, 1, 2, 3, 0, ""},
		{"v1.2.3", false, 1, 2, 3, 0, ""},
		{"V1.2.3", false, 1, 2, 3, 0, ""},
		{"1.10.0", false, 1, 10, 0, 0, ""},
		{"1.2.3-rc.1", false, 1, 2, 3, 2, ""},
		{"v1.2.3-beta.2+build.42", false, 1, 2, 3, 2, "build.42"},
		{"", true, 0, 0, 0, 0, ""},
		{"invalid", true, 0, 0, 0, 0, ""},
		{"1.2", true, 0, 0, 0, 0, ""},
		{"1.2.3.4", true, 0, 0, 0, 0, ""},
		{"1.02.3", true, 0, 0, 0, 0, ""}, // leading zero rejected
		{"v", true, 0, 0, 0, 0, ""},
		{"1.2.3-", true, 0, 0, 0, 0, ""},
		{"1.2.3-..", true, 0, 0, 0, 0, ""},
	}

	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			v, err := ParseSemVer(tc.input)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseSemVer(%q) expected error, got nil", tc.input)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseSemVer(%q) unexpected error: %v", tc.input, err)
			}
			if v.Major != tc.wantMajor || v.Minor != tc.wantMinor || v.Patch != tc.wantPatch {
				t.Errorf("got %d.%d.%d, want %d.%d.%d", v.Major, v.Minor, v.Patch, tc.wantMajor, tc.wantMinor, tc.wantPatch)
			}
			if len(v.Prerelease) != tc.wantPreLen {
				t.Errorf("got prerelease len %d, want %d", len(v.Prerelease), tc.wantPreLen)
			}
			if v.Build != tc.wantBuild {
				t.Errorf("got build %q, want %q", v.Build, tc.wantBuild)
			}
		})
	}
}

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want int
	}{
		// Basic ordering
		{"1.2.3", "1.2.4", -1},
		{"1.2.4", "1.2.3", 1},
		{"1.9.9", "1.10.0", -1},
		{"1.10.0", "1.9.9", 1},
		{"v1.2.3", "1.2.3", 0},
		{"1.2.3", "v1.2.3", 0},

		// Prereleases vs releases
		{"1.2.3-rc.1", "1.2.3", -1},
		{"1.2.3", "1.2.3-rc.1", 1},
		{"1.2.3-beta", "1.2.3", -1},
		{"1.2.3", "1.2.3-beta", 1},
		{"1.2.3-alpha", "1.2.3-beta", -1},
		{"1.2.3-beta.2", "1.2.3-beta.10", -1}, // numeric comparison within prerelease
		{"1.2.3-rc.1", "1.2.3-rc.2", -1},
		{"1.2.3-rc.1", "1.2.3-rc.1.1", -1}, // longer prerelease has higher precedence

		// Development versions
		{"dev", "1.0.0", -1},
		{"local", "v1.0.0", -1},
		{"", "1.0.0", -1},
	}

	for _, tc := range tests {
		t.Run(tc.a+" vs "+tc.b, func(t *testing.T) {
			got, err := CompareVersions(tc.a, tc.b)
			if err != nil {
				t.Fatalf("CompareVersions(%q, %q) error: %v", tc.a, tc.b, err)
			}
			if got != tc.want {
				t.Errorf("CompareVersions(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestCompareVersions_Invalid(t *testing.T) {
	// Candidate must be valid semver
	if _, err := CompareVersions("1.0.0", "invalid"); err == nil {
		t.Error("expected error for invalid candidate version")
	}
	if _, err := CompareVersions("1.0.0", "dev"); err == nil {
		t.Error("expected error for dev candidate version")
	}
	if _, err := CompareVersions("invalid", "1.0.0"); err == nil {
		// "invalid" is not dev, so it should error
		t.Error("expected error for invalid current version")
	}
}

func TestIsUpgrade(t *testing.T) {
	tests := []struct {
		current   string
		candidate string
		want      bool
		wantErr   bool
	}{
		{"1.2.3", "1.2.4", true, false},
		{"1.9.9", "1.10.0", true, false},
		{"1.2.3-rc.1", "1.2.3", true, false},
		{"dev", "1.0.0", true, false},
		{"local", "v1.0.0", true, false},

		// Same version -> not an upgrade
		{"1.2.3", "1.2.3", false, false},
		{"v1.2.3", "1.2.3", false, false},

		// Downgrade -> not an upgrade
		{"1.2.4", "1.2.3", false, false},
		{"1.10.0", "1.9.9", false, false},
		{"1.2.3", "1.2.3-rc.1", false, false},

		// Malformed -> error
		{"1.2.3", "invalid", false, true},
		{"1.2.3", "", false, true},
	}

	for _, tc := range tests {
		t.Run(tc.current+"->"+tc.candidate, func(t *testing.T) {
			got, err := IsUpgrade(tc.current, tc.candidate)
			if (err != nil) != tc.wantErr {
				t.Fatalf("IsUpgrade(%q, %q) error = %v, wantErr = %v", tc.current, tc.candidate, err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("IsUpgrade(%q, %q) = %v, want %v", tc.current, tc.candidate, got, tc.want)
			}
		})
	}
}
