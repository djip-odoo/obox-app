// Package update checks the app's GitHub releases for a newer build and allows
// the user to download and apply it in place.
package update

import (
	"encoding/xml"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"

	"obox-app/internal/logger"
)

// GitHub repository hosting the release assets checked by the app.
const (
	RepoOwner = "djip-odoo"
	RepoName  = "obox-app"

	assetNameLinux   = "obox-app-linux64"
	assetNameWindows = "obox-app-win64-installer"

	requestUA   = "obox-app-updater"
	apiTimeout  = 15 * time.Second
	downloadBuf = 64 * 1024
)

// Version is the version of the running binary. It is injected at build time
// from the release tag via:
//
//	go build -ldflags "-X obox-app/internal/update.Version=v1.2.3"
//
// The "dev" fallback is always treated as older than any published release.
var Version = "dev"

var ErrNoReleases = errors.New("no published releases found")

// feedBaseURL is a variable so tests can point the check at a fake server.
var feedBaseURL = "https://github.com"

var htmlTagRe = regexp.MustCompile(`<[^>]*>`)

// Release is the newest published release described by the releases feed.
type Release struct {
	TagName string
	Notes   string
}

type atomFeed struct {
	XMLName xml.Name `xml:"feed"`
	Entries []struct {
		Title   string `xml:"title"`
		Content struct {
			Inner string `xml:",innerxml"`
		} `xml:"content"`
	} `xml:"entry"`
}

// Info is the result of checking for updates, intended for the UI.
type Info struct {
	CheckOK        bool   `json:"checkOk"`
	Available      bool   `json:"available"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	DownloadURL    string `json:"downloadUrl"`
	AssetName      string `json:"assetName"`
	AssetSize      int64  `json:"assetSize"`
	Notes          string `json:"notes"`
	Error          string `json:"error,omitempty"`
}

// Check queries the release feed and reports whether an upgrade applies to the
// current OS. It never returns an error: failures are folded into the returned
// Info so the caller can show them inline.
func Check() Info {
	return CheckGoOS(runtime.GOOS)
}

// CheckGoOS is Check for an explicit target OS so the logic is testable.
func CheckGoOS(goos string) Info {
	info := Info{
		CurrentVersion: Version,
	}

	client := &http.Client{Timeout: apiTimeout}
	rel, err := latestRelease(client)
	switch {
	case errors.Is(err, ErrNoReleases):
		// Nothing published yet: the current build is the latest there is.
		info.CheckOK = true
		return info
	case err != nil:
		info.Error = err.Error()
		return info
	}

	info.LatestVersion = rel.TagName
	info.Notes = trimNotes(rel.Notes)

	name, ok := assetNameForOS(goos, rel.TagName)
	if !ok {
		// No build is published for this OS (for example darwin), so there is
		// nothing to offer.
		info.CheckOK = true
		return info
	}
	info.AssetName = name
	info.DownloadURL = downloadURL(rel.TagName, name)
	info.AssetSize = probeSize(client, info.DownloadURL)
	info.CheckOK = true

	info.Available = compareVersion(Version, rel.TagName) < 0
	return info
}

// latestRelease fetches the newest published release from the GitHub Atom
// releases feed. Unlike the api.github.com REST endpoint this page is served
// to anonymous clients without rate-limiting.
func latestRelease(client *http.Client) (*Release, error) {
	url := feedBaseURL + "/" + RepoOwner + "/" + RepoName + "/releases.atom"

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("release check failed: %w", err)
	}
	req.Header.Set("User-Agent", requestUA+"/"+Version)

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("release check failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release check failed: github returned %d", resp.StatusCode)
	}

	var feed atomFeed
	if err := xml.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&feed); err != nil {
		return nil, fmt.Errorf("release check failed: cannot parse release feed: %w", err)
	}
	if len(feed.Entries) == 0 {
		return nil, ErrNoReleases
	}

	entry := feed.Entries[0]

	notes := stripHTML(entry.Content.Inner)
	// GitHub renders an empty release description as "No content.".
	if strings.EqualFold(strings.TrimSpace(notes), "no content.") {
		notes = ""
	}

	return &Release{
		TagName: strings.TrimSpace(entry.Title),
		Notes:   notes,
	}, nil
}

// assetNameForOS maps the running OS and optional release tag to its published release asset.
func assetNameForOS(goos string, tag string) (string, bool) {
	switch goos {
	case "windows":
		if tag != "" {
			return fmt.Sprintf("%s-%s.exe", assetNameWindows, tag), true
		}
		return assetNameWindows + ".exe", true
	case "linux":
		if tag != "" {
			return fmt.Sprintf("%s-%s", assetNameLinux, tag), true
		}
		return assetNameLinux, true
	default:
		return "", false
	}
}

// downloadURL of a release asset is deterministic from the tag and asset name.
func downloadURL(tag, name string) string {
	return feedBaseURL + "/" + RepoOwner + "/" + RepoName + "/releases/download/" + tag + "/" + name
}

// probeSize best-effort reads the asset's Content-Length via HEAD. Any failure
// leaves the size at zero; progress is still reported in bytes downloaded.
func probeSize(client *http.Client, url string) int64 {
	req, err := http.NewRequest(http.MethodHead, url, nil)
	if err != nil {
		return 0
	}
	req.Header.Set("User-Agent", requestUA+"/"+Version)

	resp, err := client.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	return resp.ContentLength
}

// stripHTML turns a feed entry's HTML content into plain text.
func stripHTML(s string) string {
	s = html.UnescapeString(s)
	s = htmlTagRe.ReplaceAllString(s, " ")
	return strings.Join(strings.Fields(s), " ")
}

// trimNotes keeps the release notes short for the update banner.
func trimNotes(body string) string {
	body = strings.TrimSpace(body)
	if len(body) > 500 {
		return body[:500] + "…"
	}
	return body
}

// ProgressFunc reports the downloaded and total byte counts during a download.
type ProgressFunc func(downloaded, total int64)

// Download streams the asset at url into dir under filename and returns the
// local path. A non-nil progress callback is invoked as bytes arrive.
func Download(url, filename, dir string, progress ProgressFunc) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	client := &http.Client{Timeout: 0} // large binaries; no global deadline
	logger.Infof("Downloading update from %s", url)
	resp, err := client.Get(url)
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
		return "", fmt.Errorf("download failed: github returned %d", resp.StatusCode)
	}

	dest, err := os.Create(filepath.Join(dir, filename))
	if err != nil {
		return "", fmt.Errorf("download failed: %w", err)
	}

	total := resp.ContentLength
	written, copyErr := copyWithProgress(dest, resp.Body, total, progress)
	closeErr := dest.Close()
	if copyErr != nil {
		_ = os.Remove(dest.Name())
		return "", fmt.Errorf("download failed: %w", copyErr)
	}
	if closeErr != nil {
		_ = os.Remove(dest.Name())
		return "", fmt.Errorf("download failed: %w", closeErr)
	}
	if total > 0 && written != total {
		_ = os.Remove(dest.Name())
		return "", fmt.Errorf("download incomplete: got %d of %d bytes", written, total)
	}
	if total > 0 && progress != nil {
		progress(total, total)
	}
	logger.Infof("Update downloaded (%d bytes) to %s", written, dest.Name())
	return dest.Name(), nil
}

func copyWithProgress(dst io.Writer, src io.Reader, total int64, progress ProgressFunc) (int64, error) {
	if progress == nil {
		return io.Copy(dst, src)
	}
	buf := make([]byte, downloadBuf)
	var written int64
	for {
		n, readErr := src.Read(buf)
		if n > 0 {
			if _, err := dst.Write(buf[:n]); err != nil {
				return written, err
			}
			written += int64(n)
			progress(written, total)
		}
		switch readErr {
		case nil:
		case io.EOF:
			return written, nil
		default:
			return written, readErr
		}
	}
}

// compareVersion compares two version tags such as "v1.2.3" or "1.2.3-rc.1"
// and returns -1, 0 or 1. Strings that are not version tags (for example the
// "dev" default) sort before any tag with -1/1 semantics.
func compareVersion(a, b string) int {
	va, errA := parseVersion(a)
	vb, errB := parseVersion(b)
	if errA != nil || errB != nil {
		switch {
		case errA != nil && errB != nil:
			return strings.Compare(a, b)
		case errA != nil:
			return -1 // unknown/"dev" is always older than a tag
		default:
			return 1
		}
	}

	if va.maj != vb.maj {
		return compareInt(va.maj, vb.maj)
	}
	if va.min != vb.min {
		return compareInt(va.min, vb.min)
	}
	if va.pat != vb.pat {
		return compareInt(va.pat, vb.pat)
	}

	switch {
	case va.pre == "" && vb.pre != "":
		return 1 // release > pre-release on the same version
	case va.pre != "" && vb.pre == "":
		return -1
	default:
		return strings.Compare(va.pre, vb.pre)
	}
}

func compareInt(a, b int) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	default:
		return 0
	}
}

type parsedVersion struct {
	maj, min, pat int
	pre           string
}

// parseVersion splits "v1.2.3-rc.1" into numeric core parts and a pre-release
// component. It errors when the core is not numeric.
func parseVersion(s string) (parsedVersion, error) {
	s = strings.TrimSpace(strings.TrimPrefix(s, "v"))
	if s == "" {
		return parsedVersion{}, errors.New("empty version")
	}

	var pre string
	if i := strings.IndexByte(s, '-'); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
	}

	var nums []int
	for _, part := range strings.Split(s, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			return parsedVersion{}, fmt.Errorf("not a version tag: %q", s)
		}
		nums = append(nums, n)
	}

	v := parsedVersion{pre: pre}
	if len(nums) > 0 {
		v.maj = nums[0]
	}
	if len(nums) > 1 {
		v.min = nums[1]
	}
	if len(nums) > 2 {
		v.pat = nums[2]
	}
	return v, nil
}
