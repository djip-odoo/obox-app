package update

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

const sampleReleaseResponse = `{
  "id": 12345,
  "tag_name": "v1.2.3",
  "name": "v1.2.3 Stable",
  "body": "Fixed printing and networking issues.",
  "draft": false,
  "prerelease": false,
  "assets": [
    {
      "name": "obox-app-linux64-v1.2.3",
      "size": 16000000,
      "browser_download_url": "https://github.com/djip-odoo/obox-app/releases/download/v1.2.3/obox-app-linux64-v1.2.3",
      "digest": "sha256:d5435a220dfde5650d9bc406313d4ab67c8b57424e79caa19fa1f1ac0c9d752e"
    }
  ]
}`

func TestFetchLatestRelease_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Accept") != "application/vnd.github+json" {
			t.Errorf("unexpected Accept header: %q", r.Header.Get("Accept"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleReleaseResponse))
	}))
	defer server.Close()

	rel, err := FetchLatestRelease(context.Background(), server.Client(), server.URL, "testowner", "testrepo", "test-ua")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if rel.TagName != "v1.2.3" {
		t.Errorf("got TagName %q, want v1.2.3", rel.TagName)
	}
	if len(rel.Assets) != 1 {
		t.Fatalf("expected 1 asset, got %d", len(rel.Assets))
	}
	if rel.Assets[0].Name != "obox-app-linux64-v1.2.3" {
		t.Errorf("unexpected asset name: %q", rel.Assets[0].Name)
	}
}

func TestFetchLatestRelease_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	_, err := FetchLatestRelease(context.Background(), server.Client(), server.URL, "testowner", "testrepo", "test-ua")
	if !errors.Is(err, ErrNoReleases) {
		t.Errorf("got error %v, want ErrNoReleases", err)
	}
}

func TestFetchLatestRelease_RateLimited(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-RateLimit-Remaining", "0")
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	_, err := FetchLatestRelease(context.Background(), server.Client(), server.URL, "testowner", "testrepo", "test-ua")
	if err == nil || !errors.Is(err, errors.New("github api rate limit exceeded")) && err.Error() != "github api rate limit exceeded" {
		t.Errorf("expected rate limit error, got %v", err)
	}
}

func TestFetchLatestRelease_DraftRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name": "v1.2.3", "draft": true, "prerelease": false}`))
	}))
	defer server.Close()

	_, err := FetchLatestRelease(context.Background(), server.Client(), server.URL, "testowner", "testrepo", "test-ua")
	if !errors.Is(err, ErrNoReleases) {
		t.Errorf("expected ErrNoReleases for draft, got %v", err)
	}
}

func TestFetchLatestRelease_PrereleaseRejected(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name": "v1.2.3-rc.1", "draft": false, "prerelease": true}`))
	}))
	defer server.Close()

	_, err := FetchLatestRelease(context.Background(), server.Client(), server.URL, "testowner", "testrepo", "test-ua")
	if !errors.Is(err, ErrNoReleases) {
		t.Errorf("expected ErrNoReleases for prerelease, got %v", err)
	}
}

func TestFetchLatestRelease_MalformedJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not-json`))
	}))
	defer server.Close()

	_, err := FetchLatestRelease(context.Background(), server.Client(), server.URL, "testowner", "testrepo", "test-ua")
	if err == nil {
		t.Error("expected error for malformed json, got nil")
	}
}
