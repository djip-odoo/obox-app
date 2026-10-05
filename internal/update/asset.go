package update

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// SelectAsset inspects the release's published assets and chooses the binary
// or installer matching the target operating system and CPU architecture.
func SelectAsset(assets []Asset, goos, goarch string) (*Asset, error) {
	if len(assets) == 0 {
		return nil, ErrNoCompatibleAsset
	}

	for i := range assets {
		a := &assets[i]
		if isAssetMatch(a.Name, goos, goarch) {
			cleanDigest(a)
			return a, nil
		}
	}

	return nil, fmt.Errorf("%w: for %s/%s", ErrNoCompatibleAsset, goos, goarch)
}

func isAssetMatch(filename, goos, goarch string) bool {
	lower := strings.ToLower(filename)

	switch goos {
	case "linux":
		// Must be a linux binary, not a Windows exe or macOS pkg/zip
		if strings.HasSuffix(lower, ".exe") || strings.HasSuffix(lower, ".pkg") || strings.HasSuffix(lower, ".zip") {
			return false
		}
		if strings.Contains(lower, "windows") || strings.Contains(lower, "win64") || strings.Contains(lower, "darwin") || strings.Contains(lower, "macos") {
			return false
		}
		if strings.Contains(lower, "arm64") && goarch != "arm64" {
			return false
		}
		if (goarch == "amd64" || goarch == "") && (strings.Contains(lower, "linux64") || strings.Contains(lower, "linux-amd64") || strings.Contains(lower, "linux")) {
			return true
		}
		return false

	case "windows":
		// Must be an installer executable
		if !strings.HasSuffix(lower, ".exe") {
			return false
		}
		if strings.Contains(lower, "arm64") && goarch != "arm64" {
			return false
		}
		// Match windows installer (e.g. obox-app-win64-installer-v1.0.17.exe or epos-proxy-win64-installer...)
		if strings.Contains(lower, "installer") || strings.Contains(lower, "win64") || strings.Contains(lower, "windows") {
			return true
		}
		return false

	case "darwin":
		// macOS package or zip
		if strings.HasSuffix(lower, ".pkg") || strings.HasSuffix(lower, ".app.zip") || strings.HasSuffix(lower, ".zip") {
			if goarch == "arm64" && strings.Contains(lower, "arm64") {
				return true
			}
			if (goarch == "amd64" || goarch == "") && (strings.Contains(lower, "amd64") || strings.Contains(lower, "x64") || !strings.Contains(lower, "arm64")) {
				return true
			}
		}
		return false

	default:
		return false
	}
}

// FindChecksumAsset searches the assets for a checksum file like "SHA256SUMS" or "checksums.txt".
func FindChecksumAsset(assets []Asset) *Asset {
	for i := range assets {
		lower := strings.ToLower(assets[i].Name)
		if lower == "sha256sums" || lower == "checksums.txt" || strings.HasSuffix(lower, ".sha256") {
			return &assets[i]
		}
	}
	return nil
}

// ParseChecksums parses a standard sha256sum file content (`<hash>  <filename>`)
// and returns a map from filename to expected lowercase sha256 hex string.
func ParseChecksums(r io.Reader) (map[string]string, error) {
	result := make(map[string]string)
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			hash := strings.ToLower(fields[0])
			name := strings.TrimPrefix(fields[1], "*") // binary mode prefix
			name = strings.TrimSpace(name)
			result[name] = hash
		}
	}
	return result, scanner.Err()
}

// FetchExpectedChecksum tries to get the expected SHA-256 for targetAsset:
// 1. Checks if asset has a "sha256:" digest directly.
// 2. Otherwise downloads and parses any checksum asset in the release.
func FetchExpectedChecksum(client *http.Client, targetAsset *Asset, allAssets []Asset) (string, error) {
	if targetAsset.ExpectedSHA != "" {
		return targetAsset.ExpectedSHA, nil
	}
	if targetAsset.Digest != "" {
		if sha := extractSHA256(targetAsset.Digest); sha != "" {
			targetAsset.ExpectedSHA = sha
			return sha, nil
		}
	}

	csAsset := FindChecksumAsset(allAssets)
	if csAsset == nil || csAsset.DownloadURL == "" {
		return "", nil // No checksum file published
	}

	req, err := http.NewRequest(http.MethodGet, csAsset.DownloadURL, nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to fetch checksums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to fetch checksums: HTTP %d", resp.StatusCode)
	}

	sums, err := ParseChecksums(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("failed to parse checksums: %w", err)
	}

	if sha, ok := sums[targetAsset.Name]; ok {
		targetAsset.ExpectedSHA = sha
		return sha, nil
	}

	return "", nil
}

func cleanDigest(a *Asset) {
	if a.Digest != "" {
		a.ExpectedSHA = extractSHA256(a.Digest)
	}
}

func extractSHA256(digest string) string {
	parts := strings.SplitN(digest, ":", 2)
	if len(parts) == 2 && strings.EqualFold(parts[0], "sha256") {
		return strings.ToLower(strings.TrimSpace(parts[1]))
	}
	return ""
}

// ValidateDownloadURL verifies that the asset download URL points to a trusted GitHub domain.
func ValidateDownloadURL(rawURL string, trustedHosts ...string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrUntrustedSource, err)
	}

	if u.Scheme != "https" && u.Scheme != "http" {
		return fmt.Errorf("%w: unsupported scheme %q", ErrUntrustedSource, u.Scheme)
	}

	host := strings.ToLower(u.Hostname())
	allowed := []string{
		"github.com",
		"api.github.com",
		"objects.githubusercontent.com",
		"github-production-release-asset-2e65be.s3.amazonaws.com",
	}
	for _, th := range trustedHosts {
		if th != "" {
			allowed = append(allowed, strings.ToLower(th))
		}
	}

	for _, a := range allowed {
		if host == a || strings.HasSuffix(host, "."+a) || host == "127.0.0.1" || host == "localhost" {
			return nil
		}
	}

	return fmt.Errorf("%w: untrusted host %q", ErrUntrustedSource, host)
}
