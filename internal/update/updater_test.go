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
	"sync"
	"testing"
	"time"
)

func TestUpdater_Lifecycle(t *testing.T) {
	binaryPayload := []byte("new-version-payload")
	hasher := sha256.New()
	hasher.Write(binaryPayload)
	binaryHash := hex.EncodeToString(hasher.Sum(nil))

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/djip-odoo/obox-app/releases/latest":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(fmt.Sprintf(`{
				"tag_name": "v1.2.0",
				"body": "Bug fixes and improvements",
				"draft": false,
				"prerelease": false,
				"assets": [
					{
						"name": "obox-app-linux64-v1.2.0",
						"size": %d,
						"browser_download_url": "%s/download/linux64",
						"digest": "sha256:%s"
					}
				]
			}`, len(binaryPayload), "http://"+r.Host, binaryHash)))
		case "/download/linux64":
			w.Header().Set("Content-Length", fmt.Sprint(len(binaryPayload)))
			_, _ = w.Write(binaryPayload)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	u := NewUpdater(Config{
		CurrentVersion: "v1.1.0",
		APIBaseURL:     server.URL,
		TargetOS:       "linux",
		TargetArch:     "amd64",
		HTTPClient:     server.Client(),
		CheckCooldown:  100 * time.Millisecond,
	})

	// 1. Initial State
	if u.Status().State != StateIdle {
		t.Errorf("initial state = %q, want %q", u.Status().State, StateIdle)
	}

	// 2. Check for update
	info, err := u.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("Check failed: %v", err)
	}
	if !info.Available || !info.CheckOK {
		t.Errorf("expected update available, got: %+v", info)
	}
	if info.LatestVersion != "v1.2.0" {
		t.Errorf("got latest version %q, want v1.2.0", info.LatestVersion)
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
	staged, err := u.Download(context.Background(), func(downloaded, total int64, percent int) {
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
	if u.Status().State != StateReadyToInstall {
		t.Errorf("state after download = %q, want %q", u.Status().State, StateReadyToInstall)
	}

	// 5. Duplicate download while ready
	// Can re-download or check status
	status := u.Status()
	if status.State != StateReadyToInstall {
		t.Errorf("expected ReadyToInstall status, got %s", status.State)
	}
}

func TestUpdater_ConcurrencyProtection(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Slow response
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"tag_name": "v1.2.0", "draft": false, "prerelease": false}`))
	}))
	defer server.Close()

	u := NewUpdater(Config{
		CurrentVersion: "v1.1.0",
		APIBaseURL:     server.URL,
		HTTPClient:     server.Client(),
	})

	var wg sync.WaitGroup
	var err1, err2 error

	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err1 = u.Check(context.Background(), true)
	}()
	go func() {
		defer wg.Done()
		time.Sleep(10 * time.Millisecond) // ensure first check has locked
		_, err2 = u.Check(context.Background(), true)
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

func TestUpdater_Dismissal(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"tag_name": "v1.2.0",
			"draft": false,
			"prerelease": false,
			"assets": [{"name": "obox-app-linux64-v1.2.0", "browser_download_url": "http://example.com/bin"}]
		}`))
	}))
	defer server.Close()

	u := NewUpdater(Config{
		CurrentVersion: "v1.1.0",
		APIBaseURL:     server.URL,
		TargetOS:       "linux",
		TargetArch:     "amd64",
		HTTPClient:     server.Client(),
		CheckCooldown:  0,
	})

	// First check: available
	info, err := u.Check(context.Background(), true)
	if err != nil || !info.Available {
		t.Fatalf("first check should be available, got err=%v info=%+v", err, info)
	}

	// User dismisses this version
	u.Dismiss("v1.2.0")

	// Next automatic check: suppressed
	info, err = u.Check(context.Background(), false)
	if err != nil {
		t.Fatalf("check failed: %v", err)
	}
	if info.Available {
		t.Error("expected dismissed update to NOT be offered in automatic check")
	}

	// Forced/manual check: still shows it so user can update manually!
	info, err = u.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("manual check failed: %v", err)
	}
	if !info.Available {
		t.Error("expected manual check to show update even if previously dismissed")
	}
}
