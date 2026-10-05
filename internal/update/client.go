package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultAPITimeout = 15 * time.Second
	maxJSONResponse   = 2 * 1024 * 1024 // 2 MB limit for API responses
)

// HTTPClient defines the client interface used for fetching releases.
type HTTPClient interface {
	Do(req *http.Request) (*http.Response, error)
}

// FetchLatestRelease queries the GitHub Releases API for the repository's latest stable release.
func FetchLatestRelease(ctx context.Context, client HTTPClient, apiBaseURL, owner, repo, userAgent string) (*Release, error) {
	if client == nil {
		client = &http.Client{Timeout: defaultAPITimeout}
	}

	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", strings.TrimRight(apiBaseURL, "/"), owner, repo)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create update request: %w", err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("network request failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success, decode below
	case http.StatusNotFound:
		return nil, ErrNoReleases
	case http.StatusForbidden:
		// Check for rate limiting
		if resp.Header.Get("X-RateLimit-Remaining") == "0" {
			return nil, fmt.Errorf("github api rate limit exceeded")
		}
		return nil, fmt.Errorf("github api access forbidden (status 403)")
	default:
		if resp.StatusCode >= 500 {
			return nil, fmt.Errorf("github server error (status %d)", resp.StatusCode)
		}
		return nil, fmt.Errorf("unexpected github response (status %d)", resp.StatusCode)
	}

	var rel Release
	limitedReader := io.LimitReader(resp.Body, maxJSONResponse)
	if err := json.NewDecoder(limitedReader).Decode(&rel); err != nil {
		return nil, fmt.Errorf("failed to parse release json: %w", err)
	}

	// Validation: only consider published, non-draft, non-prerelease stable releases
	if rel.Draft {
		return nil, fmt.Errorf("%w: latest release is a draft", ErrNoReleases)
	}
	if rel.Prerelease {
		return nil, fmt.Errorf("%w: latest release is marked as prerelease", ErrNoReleases)
	}

	rel.TagName = strings.TrimSpace(rel.TagName)
	if rel.TagName == "" {
		return nil, fmt.Errorf("%w: empty tag name in release", ErrInvalidVersion)
	}

	// Validate semantic version
	if _, err := ParseSemVer(rel.TagName); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidVersion, err)
	}

	return &rel, nil
}
