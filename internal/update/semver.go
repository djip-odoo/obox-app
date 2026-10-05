package update

import (
	"fmt"
	"strconv"
	"strings"
)

// SemVer represents a parsed Semantic Version (SemVer 2.0.0).
type SemVer struct {
	Major      int
	Minor      int
	Patch      int
	Prerelease []string
	Build      string
	Raw        string
}

// ParseSemVer parses a version string like "v1.2.3", "1.2.3-rc.1", "1.2.3-beta".
func ParseSemVer(s string) (SemVer, error) {
	raw := s
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "v")
	s = strings.TrimPrefix(s, "V")

	if s == "" {
		return SemVer{}, fmt.Errorf("%w: empty version", ErrInvalidVersion)
	}

	var build string
	if idx := strings.IndexByte(s, '+'); idx >= 0 {
		build = s[idx+1:]
		s = s[:idx]
	}

	var prerelease []string
	if idx := strings.IndexByte(s, '-'); idx >= 0 {
		preStr := s[idx+1:]
		s = s[:idx]
		if preStr == "" {
			return SemVer{}, fmt.Errorf("%w: empty prerelease in %q", ErrInvalidVersion, raw)
		}
		prerelease = strings.Split(preStr, ".")
		for _, part := range prerelease {
			if part == "" {
				return SemVer{}, fmt.Errorf("%w: empty prerelease segment in %q", ErrInvalidVersion, raw)
			}
		}
	}

	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return SemVer{}, fmt.Errorf("%w: version must have 3 numeric segments, got %q", ErrInvalidVersion, raw)
	}

	major, err := parseNonNegativeInt(parts[0])
	if err != nil {
		return SemVer{}, fmt.Errorf("%w: invalid major version in %q: %v", ErrInvalidVersion, raw, err)
	}
	minor, err := parseNonNegativeInt(parts[1])
	if err != nil {
		return SemVer{}, fmt.Errorf("%w: invalid minor version in %q: %v", ErrInvalidVersion, raw, err)
	}
	patch, err := parseNonNegativeInt(parts[2])
	if err != nil {
		return SemVer{}, fmt.Errorf("%w: invalid patch version in %q: %v", ErrInvalidVersion, raw, err)
	}

	return SemVer{
		Major:      major,
		Minor:      minor,
		Patch:      patch,
		Prerelease: prerelease,
		Build:      build,
		Raw:        raw,
	}, nil
}

func parseNonNegativeInt(s string) (int, error) {
	if s == "" {
		return 0, fmt.Errorf("empty integer")
	}
	// Reject leading zero if multiple digits
	if len(s) > 1 && s[0] == '0' {
		return 0, fmt.Errorf("numeric segments must not have leading zeroes")
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("invalid integer: %q", s)
	}
	return n, nil
}

// CompareSemVer compares two SemVer versions following SemVer 2.0.0 rules.
// Returns -1 if a < b, 0 if a == b, and 1 if a > b.
func CompareSemVer(a, b SemVer) int {
	if a.Major != b.Major {
		return compareInt(a.Major, b.Major)
	}
	if a.Minor != b.Minor {
		return compareInt(a.Minor, b.Minor)
	}
	if a.Patch != b.Patch {
		return compareInt(a.Patch, b.Patch)
	}

	// 1. A normal version has greater precedence than a pre-release version at the same major/minor/patch.
	hasPreA := len(a.Prerelease) > 0
	hasPreB := len(b.Prerelease) > 0
	if !hasPreA && hasPreB {
		return 1
	}
	if hasPreA && !hasPreB {
		return -1
	}
	if !hasPreA && !hasPreB {
		return 0
	}

	// 2. Both have pre-releases: compare each dot-separated identifier.
	limit := len(a.Prerelease)
	if len(b.Prerelease) < limit {
		limit = len(b.Prerelease)
	}

	for i := 0; i < limit; i++ {
		idA := a.Prerelease[i]
		idB := b.Prerelease[i]

		numA, isNumA := strconv.Atoi(idA)
		numB, isNumB := strconv.Atoi(idB)

		switch {
		case isNumA == nil && isNumB == nil:
			// Identifiers consisting of only digits are compared numerically.
			if numA != numB {
				return compareInt(numA, numB)
			}
		case isNumA == nil && isNumB != nil:
			// Numeric identifiers always have lower precedence than non-numeric identifiers.
			return -1
		case isNumA != nil && isNumB == nil:
			return 1
		default:
			// Identifiers with letters or hyphens are compared lexically in ASCII sort order.
			if idA != idB {
				if idA < idB {
					return -1
				}
				return 1
			}
		}
	}

	// A larger set of pre-release fields has a higher precedence than a smaller set.
	return compareInt(len(a.Prerelease), len(b.Prerelease))
}

// CompareVersions compares two version strings.
// Handles special development versions ("dev", "local", "") safely.
func CompareVersions(currentStr, candidateStr string) (int, error) {
	cand, err := ParseSemVer(candidateStr)
	if err != nil {
		return 0, fmt.Errorf("invalid candidate version: %w", err)
	}

	// Check if current is a development/local build
	if isDevVersion(currentStr) {
		// Any valid candidate version is considered newer than a dev build
		return -1, nil
	}

	curr, err := ParseSemVer(currentStr)
	if err != nil {
		return 0, fmt.Errorf("invalid current version: %w", err)
	}

	return CompareSemVer(curr, cand), nil
}

// IsUpgrade checks if candidateStr is a strictly newer valid version than currentStr.
// It explicitly rejects downgrades and invalid versions.
func IsUpgrade(currentStr, candidateStr string) (bool, error) {
	cmp, err := CompareVersions(currentStr, candidateStr)
	if err != nil {
		return false, err
	}
	if cmp < 0 {
		return true, nil
	}
	return false, nil
}

func isDevVersion(v string) bool {
	v = strings.TrimSpace(strings.ToLower(v))
	return v == "" || v == "dev" || v == "local" || v == "unknown" || v == "none"
}

func compareInt(a, b int) int {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}
