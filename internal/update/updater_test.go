package update

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"sync"
	"testing"
	"time"
)

func getTestPlatformAssetName() string {
	switch runtime.GOOS {
	case "windows":
		return assetNameWindows + "-1.2.0.exe"
	case "darwin":
		return assetNameMacos + "-1.2.0"
	default:
		return assetNameLinux + "-1.2.0"
	}
}

func TestUpdater_Lifecycle(t *testing.T) {
	binaryPayload := []byte("new-version-payload")
	hasher := sha256.New()
	hasher.Write(binaryPayload)
	binaryHash := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/djip-odoo/obox-app/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"tag_name": "1.2.0",
				"body": "Bug fixes and improvements",
				"draft": false,
				"prerelease": false,
				"assets": [
					{
						"name": "%s",
						"size": %d,
						"browser_download_url": "%s/download/pkg",
						"digest": "sha256:%s"
					}
				]
			}`, getTestPlatformAssetName(), len(binaryPayload), "https://"+r.Host, binaryHash)
		case "/download/pkg":
			w.Header().Set("Content-Length", fmt.Sprint(len(binaryPayload)))
			_, _ = w.Write(binaryPayload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	oldHosts := allowedHosts
	allowedHosts = append(allowedHosts, "127.0.0.1")
	defer func() { allowedHosts = oldHosts }()

	u := GetUpdaterInstance()
	u.mu.Lock()
	u.selectedAsset = nil
	u.latestRelease = nil
	u.stagedPath = ""
	u.lastCheckTime = time.Time{}
	u.lastestInfo.CurrentVersion = "1.1.0"
	u.lastestInfo.State = StateIdle
	u.mu.Unlock()
	u.Reset()

	oldAPI := getAPIBaseURL
	getAPIBaseURL = func() string { return server.URL }
	defer func() { getAPIBaseURL = oldAPI }()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	// 1. Initial State
	if u.Status().State != StateIdle {
		t.Errorf("initial state = %q, want %q", u.Status().State, StateIdle)
	}

	// 2. Check for update
	info, err := Check(context.Background())
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info.State != StateUpdateAvailable {
		t.Errorf("expected update available, got: %+v", info)
	}
	if info.LatestVersion != "1.2.0" {
		t.Errorf("got latest version %q, want 1.2.0", info.LatestVersion)
	}
	if u.Status().State != StateUpdateAvailable {
		t.Errorf("state after check = %q, want %q", u.Status().State, StateUpdateAvailable)
	}

	// 3. Apply before download should fail
	if err := u.Apply(); !errors.Is(err, ErrUpdateNotReady) {
		t.Errorf("Apply before download should fail with ErrUpdateNotReady, got %v", err)
	}

	// 4. Download update
	var progressReported bool
	staged, err := Download(context.Background(), func(p Progress) {
		progressReported = true
	})
	if err != nil {
		t.Fatalf("Download failed: %v", err)
	}
	defer os.RemoveAll(u.stagedPath)

	if !progressReported {
		t.Error("progress was not reported during download")
	}
	if staged == "" {
		t.Fatal("empty staged path returned")
	}
	if u.Status().State != StateUpdateAvailable {
		t.Errorf("state after download = %q, want %q", u.Status().State, StateUpdateAvailable)
	}

	// 5. Duplicate download while staged
	// Can re-download or check status
	status := u.Status()
	if status.State != StateUpdateAvailable {
		t.Errorf("expected UpdateAvailable status, got %s", status.State)
	}
}

func TestUpdater_ConcurrencyProtection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"tag_name": "1.2.0", "draft": false, "prerelease": false, "assets": [{"name": "%s", "browser_download_url": "https://github.com/djip-odoo/obox-app/releases/download/v1.2.0/download"}]}`, getTestPlatformAssetName())
	}))
	defer server.Close()

	oldAPI := getAPIBaseURL
	getAPIBaseURL = func() string { return server.URL }
	defer func() { getAPIBaseURL = oldAPI }()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	u := GetUpdaterInstance()
	u.Reset()
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "1.1.0"
	u.selectedAsset = nil
	u.latestRelease = nil
	u.stagedPath = ""
	u.lastCheckTime = time.Time{}
	u.mu.Unlock()

	var wg sync.WaitGroup
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err1 = Check(context.Background())
	}()
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // ensure first check has locked
		_, err2 = Check(context.Background())
	}()
	wg.Wait()

	// At least one should succeed, or if a check was in flight the second gets ErrUpdateInProgress
	if err1 != nil && !errors.Is(err1, ErrUpdateInProgress) {
		t.Errorf("unexpected error on concurrent check 1: %v", err1)
	}
	if err2 != nil && !errors.Is(err2, ErrUpdateInProgress) {
		t.Errorf("unexpected error on concurrent check 2: %v", err2)
	}
}

func TestUpdater_RetryAfterFailureAndCancel(t *testing.T) {
	u := GetUpdaterInstance()
	u.Reset()
	u.mu.Lock()
	u.lastestInfo.CurrentVersion = "1.0.0"
	u.selectedAsset = &Asset{
		Name:        getTestPlatformAssetName(),
		DownloadURL: "https://invalid-host-untrusted.example.com/asset",
	}
	u.latestRelease = &Release{
		TagName: "1.1.0",
	}
	u.lastestInfo.State = StateUpdateAvailable
	u.mu.Unlock()

	// Download should fail because of untrusted host
	_, err := Download(context.Background(), nil)
	if err == nil {
		t.Fatal("expected download failure, got nil")
	}

	// State should remain StateUpdateAvailable with error set, enabling user to retry
	status := u.Status()
	if status.State != StateUpdateAvailable {
		t.Fatalf("expected state %s after failure, got %s", StateUpdateAvailable, status.State)
	}
	if status.Error == "" {
		t.Fatal("expected non-empty error after download failure")
	}

	// Test Reset / Cancel behavior
	u.Reset()
	statusAfterReset := u.Status()
	if statusAfterReset.State != StateUpdateAvailable {
		t.Fatalf("expected state %s after reset with selected asset, got %s", StateUpdateAvailable, statusAfterReset.State)
	}
	if statusAfterReset.Error != "" {
		t.Fatalf("expected error cleared after reset, got %q", statusAfterReset.Error)
	}
	u.mu.Lock()
	if !u.lastCheckTime.IsZero() {
		t.Errorf("expected lastCheckTime to be reset, got %v", u.lastCheckTime)
	}
	u.mu.Unlock()
}

func TestUpdater_CancelMidDownload(t *testing.T) {
	downloadStarted := make(chan struct{})
	allowExit := make(chan struct{})

	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/djip-odoo/obox-app/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{
				"tag_name": "1.3.0",
				"draft": false,
				"prerelease": false,
				"assets": [
					{
						"name": "%s",
						"size": 100000,
						"browser_download_url": "%s/download/pkg",
						"digest": "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
					}
				]
			}`, getTestPlatformAssetName(), "https://"+r.Host)
		case "/download/pkg":
			w.Header().Set("Content-Length", "100000")
			w.WriteHeader(http.StatusOK)
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
			close(downloadStarted)
			// Wait until test signals to exit or client disconnects
			select {
			case <-allowExit:
			case <-r.Context().Done():
			}
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer func() {
		close(allowExit)
		server.Close()
	}()

	oldHosts := allowedHosts
	allowedHosts = append(allowedHosts, "127.0.0.1")
	defer func() { allowedHosts = oldHosts }()

	oldClient := client
	client = server.Client()
	defer func() { client = oldClient }()

	u := GetUpdaterInstance()
	u.Reset()
	u.mu.Lock()
	u.selectedAsset = nil
	u.latestRelease = nil
	u.stagedPath = ""
	u.lastCheckTime = time.Time{}
	u.lastestInfo.CurrentVersion = "1.0.0"
	u.lastestInfo.State = StateIdle
	u.mu.Unlock()
	u.Reset()

	oldAPI := getAPIBaseURL
	getAPIBaseURL = func() string { return server.URL }
	defer func() { getAPIBaseURL = oldAPI }()

	// 1. Check for update
	info, err := Check(context.Background())
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if info.State != StateUpdateAvailable {
		t.Fatalf("expected update available, got: %+v", info)
	}

	// 2. Start download in a goroutine
	done := make(chan error, 1)
	go func() {
		_, dlErr := Download(context.Background(), nil)
		done <- dlErr
	}()

	// Wait for the server to receive the download request
	select {
	case <-downloadStarted:
	case <-time.After(3 * time.Second):
		t.Fatal("download did not start in time")
	}

	// Wait briefly for state to become StateDownloading
	deadline := time.Now().Add(1 * time.Second)
	for time.Now().Before(deadline) {
		if u.Status().State == StateDownloading {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 3. Cancel mid-download via Reset()
	u.Reset()

	// 4. Download goroutine must terminate promptly with error
	select {
	case dlErr := <-done:
		if dlErr == nil {
			t.Fatal("expected download to fail upon cancellation, got nil")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("download did not abort in time after Reset()")
	}

	// 5. State after Reset() should be StateUpdateAvailable, no error, and staged files cleared
	finalStatus := u.Status()
	if finalStatus.State != StateUpdateAvailable {
		t.Errorf("expected state %s after Reset(), got %s", StateUpdateAvailable, finalStatus.State)
	}
	if finalStatus.Error != "" {
		t.Errorf("expected error cleared after Reset(), got %q", finalStatus.Error)
	}
	u.mu.Lock()
	stagedPath := u.stagedPath
	stagedDir := u.stagedDir
	u.mu.Unlock()
	if stagedPath != "" || stagedDir != "" {
		t.Errorf("staged files not cleared: path=%q dir=%q", stagedPath, stagedDir)
	}
}
