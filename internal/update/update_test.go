package update

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

const sampleReleaseFeedJSON = `{
  "id": 9999,
  "tag_name": "v1.0.5",
  "name": "Release 1.0.5",
  "body": "[FIX] & network printing",
  "draft": false,
  "prerelease": false,
  "assets": [
    {
      "name": "obox-app-linux64-1.0.5",
      "size": 123,
      "browser_download_url": "http://127.0.0.1/download/linux64"
    }
  ]
}`

func TestCheckGoOS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(sampleReleaseFeedJSON))
	}))
	defer server.Close()

	oldAPI := apiBaseURL
	defer func() { apiBaseURL = oldAPI }()
	apiBaseURL = server.URL

	oldVersion := Version
	defer func() { Version = oldVersion }()

	Version = "dev"
	info := CheckGoOS("linux")
	if !info.CheckOK || !info.Available {
		t.Errorf("dev build should report update available: %+v", info)
	}
	if info.LatestVersion != "v1.0.5" {
		t.Errorf("got latest version %q, want v1.0.5", info.LatestVersion)
	}
	if info.AssetName != "obox-app-linux64-1.0.5" {
		t.Errorf("got asset name %q, want obox-app-linux64-1.0.5", info.AssetName)
	}

	// Same version -> not available
	Version = "v1.0.5"
	info = CheckGoOS("linux")
	if !info.CheckOK || info.Available {
		t.Errorf("same version should not report update available: %+v", info)
	}

	// Unsupported OS
	info = CheckGoOS("freebsd")
	if !info.CheckOK || info.Available {
		t.Errorf("unsupported OS should report CheckOK=true, Available=false: %+v", info)
	}
}

func TestCheckGoOSNetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	oldAPI := apiBaseURL
	defer func() { apiBaseURL = oldAPI }()
	apiBaseURL = server.URL

	info := CheckGoOS("linux")
	if info.CheckOK {
		t.Errorf("network failure should leave CheckOK false: %+v", info)
	}
	if info.Error == "" {
		t.Error("expected an error message on network failure")
	}
}

func TestDownload(t *testing.T) {
	payload := []byte("update-binary-payload")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", fmt.Sprint(len(payload)))
		_, _ = w.Write(payload)
	}))
	defer server.Close()

	dir := t.TempDir()
	var last int64
	path, err := Download(server.URL, "pkg.bin", dir, func(d, total int64, percent int) {
		last = d
		if d > total || total != int64(len(payload)) {
			t.Errorf("bad progress d=%d total=%d", d, total)
		}
	})
	if err != nil {
		t.Fatalf("Download error: %v", err)
	}
	defer os.Remove(path)

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed reading file: %v", err)
	}
	if string(got) != string(payload) {
		t.Errorf("downloaded content mismatch: %q", got)
	}
	if last != int64(len(payload)) {
		t.Errorf("progress never reached total: got %d", last)
	}
}
