package update

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"v1.2.3", "v1.2.4", -1},
		{"v1.2.4", "v1.2.3", 1},
		{"1.2.3", "1.2.3", 0},
		{"v1.2.3", "v2.0.0", -1},
		{"v2.0.0", "v10.0.0", -1},
		{"v1.2.3", "v1.2", 1},
		{"v1.2", "v1.2.0", 0},
		{"v1.2.3-rc.1", "v1.2.3", -1}, // pre-release is older
		{"v1.2.3", "v1.2.3-rc.1", 1},  // release beats pre-release
		{"v2.0.0-rc.1", "v2.0.0", -1},
		{"v1.2.3", "v1.2.3-beta", 1},
		{"dev", "v1.0.0", -1}, // unknown/"dev" is older than any tag
		{"", "v1.0.0", -1},
		{"dev", "dev", 0},
		{"v1.2.3", "dev", 1},
	}
	for _, tc := range tests {
		t.Run(fmt.Sprintf("%s vs %s", tc.a, tc.b), func(t *testing.T) {
			if got := compareVersion(tc.a, tc.b); got != tc.want {
				t.Errorf("compareVersion(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

const sampleFeed = `<?xml version="1.0" encoding="UTF-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
  <title>Release notes from obox-app</title>
  <entry>
    <title>1.0.5</title>
    <content type="html">&lt;p&gt;[FIX] &amp; network printing&lt;/p&gt;</content>
  </entry>
  <entry>
    <title>1.0.4</title>
    <content type="html">&lt;p&gt;older&lt;/p&gt;</content>
  </entry>
</feed>`

func TestLatestRelease(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ua := r.Header.Get("User-Agent"); !strings.HasPrefix(ua, requestUA) {
			t.Errorf("unexpected User-Agent %q", ua)
		}
		w.Header().Set("Content-Type", "application/atom+xml")
		fmt.Fprint(w, sampleFeed)
	}))
	defer server.Close()

	baseURL := feedBaseURL
	defer func() { feedBaseURL = baseURL }()
	feedBaseURL = server.URL

	rel, err := latestRelease(server.Client())
	if err != nil {
		t.Fatalf("latestRelease: %v", err)
	}
	if rel.TagName != "1.0.5" {
		t.Errorf("TagName = %q, want 1.0.5", rel.TagName)
	}
	if !strings.Contains(rel.Notes, "[FIX] & network printing") {
		t.Errorf("Notes = %q", rel.Notes)
	}
	if strings.Contains(rel.Notes, "<") {
		t.Errorf("Notes still contains HTML tags: %q", rel.Notes)
	}
}

func TestLatestReleaseEmptyFeed(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `<?xml version="1.0"?><feed xmlns="http://www.w3.org/2005/Atom"></feed>`)
	}))
	defer server.Close()

	baseURL := feedBaseURL
	defer func() { feedBaseURL = baseURL }()
	feedBaseURL = server.URL

	if _, err := latestRelease(server.Client()); !errors.Is(err, ErrNoReleases) {
		t.Errorf("got err %v, want ErrNoReleases", err)
	}
}

func TestLatestReleaseBadStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	baseURL := feedBaseURL
	defer func() { feedBaseURL = baseURL }()
	feedBaseURL = server.URL

	if _, err := latestRelease(server.Client()); err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestCheckGoOS(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodHead:
			w.Header().Set("Content-Length", "123")
		default:
			w.Header().Set("Content-Type", "application/atom+xml")
			fmt.Fprint(w, sampleFeed)
		}
	}))
	defer server.Close()

	baseURL := feedBaseURL
	defer func() { feedBaseURL = baseURL }()
	feedBaseURL = server.URL

	oldVersion := Version
	defer func() { Version = oldVersion }()

	Version = "dev"
	info := CheckGoOS("linux")
	if !info.CheckOK || !info.Available {
		t.Errorf("dev build should report update available: %+v", info)
	}
	if !strings.HasSuffix(info.DownloadURL, "/releases/download/1.0.5/obox-app-linux64-1.0.5") {
		t.Errorf("unexpected download URL: %q", info.DownloadURL)
	}
	if info.AssetSize != 123 {
		t.Errorf("AssetSize = %d, want 123", info.AssetSize)
	}

	Version = "1.0.5"
	info = CheckGoOS("linux")
	if !info.CheckOK || info.Available {
		t.Errorf("same version should not report update available: %+v", info)
	}

	info = CheckGoOS("darwin")
	if !info.CheckOK || info.Available {
		t.Errorf("darwin has no build, CheckOK should be true and Available false: %+v", info)
	}
}

func TestCheckGoOSNetworkError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	baseURL := feedBaseURL
	defer func() { feedBaseURL = baseURL }()
	feedBaseURL = server.URL

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
	path, err := Download(server.URL, "pkg.bin", dir, func(d, total int64) {
		last = d
		if d > total || total != int64(len(payload)) {
			t.Errorf("bad progress d=%d total=%d", d, total)
		}
	})
	if err != nil {
		t.Fatalf("Download: %v", err)
	}
	if got := readFile(t, path); string(got) != string(payload) {
		t.Errorf("downloaded content mismatch: %q", got)
	}
	if last != int64(len(payload)) {
		t.Errorf("progress never reached total: got %d", last)
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return b
}
