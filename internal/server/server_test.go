package server

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"epos-proxy/internal/config"
	"epos-proxy/internal/printer"
	"epos-proxy/internal/testutil"
)

func TestServer_Lifecycle(t *testing.T) {
	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	s := New(port, mgr, nil, nil)
	defer s.Stop()

	testutil.ExpectedTrue(t, s.Running(), "Expected server to be running after New()")
	testutil.ExpectedEqual(t, s.Port, port)

	err := s.Stop()
	testutil.ExpectedNoError(t, err)
}

func TestPrintData_ValidXML_Success(t *testing.T) {
	// Start mock TCP listener on port 9100 for LAN printer
	_, _, err := testutil.StartMockTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		buf := make([]byte, 1024)
		_, _ = conn.Read(buf)
	})
	testutil.ExpectedNoError(t, err)

	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	s := New(port, mgr, nil, nil)
	defer s.Stop()

	printerID := printer.EncodeLANPrinterID("127.0.0.1")
	xmlPayload := `<epos-print><text align="center">ORDER #123</text><cut /></epos-print>`

	url := fmt.Sprintf("/p/%s/cgi-bin/epos/service.cgi", printerID)
	req := httptest.NewRequest("POST", url, bytes.NewReader([]byte(xmlPayload)))
	req.Header.Set("Content-Type", "text/xml")

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusOK)

	body, err := io.ReadAll(resp.Body)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedContains(t, string(body), `success="true"`)
}

func TestPrintData_SchemaError(t *testing.T) {
	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	s := New(port, mgr, nil, nil)
	defer s.Stop()

	invalidPayload := `<invalid>not-an-epos-print</invalid>`
	req := httptest.NewRequest("POST", "/p/any-printer/cgi-bin/epos/service.cgi", bytes.NewReader([]byte(invalidPayload)))
	req.Header.Set("Content-Type", "text/xml")

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	testutil.ExpectedContains(t, bodyStr, `success="false"`)
	testutil.ExpectedContains(t, bodyStr, `code="SchemaError"`)
}

func TestPrintData_UnreachablePrinter_EX_BADPORT(t *testing.T) {
	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	s := New(port, mgr, nil, nil)
	defer s.Stop()

	// Use a non-existent USB printer serial that cannot be found
	printerID := "czpOT05fRVhJU1RFTlRfU0VSSUFMCg"
	xmlPayload := `<epos-print><text>Hello</text></epos-print>`

	url := fmt.Sprintf("/p/%s/cgi-bin/epos/service.cgi", printerID)
	req := httptest.NewRequest("POST", url, bytes.NewReader([]byte(xmlPayload)))
	req.Header.Set("Content-Type", "text/xml")

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	testutil.ExpectedContains(t, bodyStr, `success="false"`)
	testutil.ExpectedContains(t, bodyStr, `code="EX_BADPORT"`)
}

func TestPrintLabel_Success(t *testing.T) {
	// Start mock TCP listener on port 9100
	_, _, err := testutil.StartMockTCPServer(t, func(conn net.Conn) {
		defer conn.Close()
		buf := make([]byte, 1024)
		_, _ = conn.Read(buf)
	})
	testutil.ExpectedNoError(t, err)

	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	s := New(port, mgr, nil, nil)
	defer s.Stop()

	printerID := printer.EncodeLANPrinterID("127.0.0.1")
	labelData := []byte("^XA^FDBarcode123^FS^XZ")

	url := fmt.Sprintf("/p/%s/pstprnt", printerID)
	req := httptest.NewRequest("POST", url, bytes.NewReader(labelData))

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusOK)
}

func TestPrintLabel_EmptyBody_BadRequest(t *testing.T) {
	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	s := New(port, mgr, nil, nil)
	defer s.Stop()

	req := httptest.NewRequest("POST", "/p/any-printer/pstprnt", bytes.NewReader([]byte{}))
	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusBadRequest)
}

func TestPrintLabel_UnreachablePrinter_ServerError(t *testing.T) {
	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	s := New(port, mgr, nil, nil)
	defer s.Stop()

	printerID := "czpOT05fRVhJU1RFTlRfU0VSSUFMCg"
	labelData := []byte("^XA^FDTest^FS^XZ")

	url := fmt.Sprintf("/p/%s/pstprnt", printerID)
	req := httptest.NewRequest("POST", url, bytes.NewReader(labelData))

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, resp.StatusCode, http.StatusInternalServerError)
}

func TestCORSHeaders(t *testing.T) {
	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	s := New(port, mgr, nil, nil)
	defer s.Stop()

	req := httptest.NewRequest("OPTIONS", "/cgi-bin/epos/service.cgi", nil)
	req.Header.Set("Origin", "http://example.com")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := s.app.Test(req)
	testutil.ExpectedNoError(t, err)

	allowOrigin := resp.Header.Get("Access-Control-Allow-Origin")
	testutil.ExpectedEqual(t, allowOrigin, "*")
}

func TestPrintData_AutoSelectRoute(t *testing.T) {
	tests := []struct {
		name     string
		payload  string
		expected []string
	}{
		{
			name:    "schema error",
			payload: `<bad></bad>`,
			expected: []string{
				`code="SchemaError"`,
			},
		},
		{
			name:    "no USB printer",
			payload: `<epos-print><text align="center">RECEIPT</text><cut /></epos-print>`,
			expected: []string{
				`success="false"`,
				`code="EX_BADPORT"`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			port := testutil.GetFreePort(t)
			mgr := printer.NewManager()
			s := New(port, mgr, nil, nil)
			defer s.Stop()

			req := httptest.NewRequest("POST", "/cgi-bin/epos/service.cgi", bytes.NewReader([]byte(tc.payload)))
			req.Header.Set("Content-Type", "text/xml")

			resp, err := s.app.Test(req)
			testutil.ExpectedNoError(t, err)

			body, err := io.ReadAll(resp.Body)
			testutil.ExpectedNoError(t, err)

			for _, expected := range tc.expected {
				testutil.ExpectedContains(t, string(body), expected)
			}
		})
	}
}

func TestServer_APIRoutes_ReadOnly(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	s := New(port, mgr, cfg, nil)
	defer s.Stop()

	// 1. GET /api/app
	reqApp := httptest.NewRequest("GET", "/api/app", nil)
	respApp, err := s.app.Test(reqApp)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respApp.StatusCode, http.StatusOK)

	// 2. GET /api/printers
	reqPrinters := httptest.NewRequest("GET", "/api/printers", nil)
	respPrinters, err := s.app.Test(reqPrinters)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respPrinters.StatusCode, http.StatusOK)

	// 3. GET /api/webview
	reqWebView := httptest.NewRequest("GET", "/api/webview", nil)
	respWebView, err := s.app.Test(reqWebView)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respWebView.StatusCode, http.StatusOK)

	// 4. GET /api/troubleshoot
	reqTrouble := httptest.NewRequest("GET", "/api/troubleshoot", nil)
	respTrouble, err := s.app.Test(reqTrouble)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respTrouble.StatusCode, http.StatusOK)
}

func TestServer_AuthAndPrivilegedRoutes(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)
	err = cfg.SetWebViewPIN("1234")
	testutil.ExpectedNoError(t, err)

	port := testutil.GetFreePort(t)
	mgr := printer.NewManager()
	s := New(port, mgr, cfg, nil)
	defer s.Stop()

	wailsToken := "trusted-wails-token-xyz"
	s.SetSessionToken(wailsToken)

	// 1. Privileged call without auth -> 401 Unauthorized
	reqUnauth := httptest.NewRequest("POST", "/api/webview/url", bytes.NewReader([]byte(`{"url":"https://example.com"}`)))
	reqUnauth.Header.Set("Content-Type", "application/json")
	respUnauth, err := s.app.Test(reqUnauth)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respUnauth.StatusCode, http.StatusUnauthorized)

	// 2. Privileged call with Wails Token -> 200 OK
	reqWails := httptest.NewRequest("POST", "/api/webview/url", bytes.NewReader([]byte(`{"url":"https://example.com"}`)))
	reqWails.Header.Set("Content-Type", "application/json")
	reqWails.Header.Set("X-Wails-Token", wailsToken)
	respWails, err := s.app.Test(reqWails)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respWails.StatusCode, http.StatusOK)
	testutil.ExpectedEqual(t, cfg.GetWebViewURL(), "https://example.com")

	// 3. Auth session: invalid PIN -> 401
	reqAuthBad := httptest.NewRequest("POST", "/api/auth/session", bytes.NewReader([]byte(`{"pin":"0000"}`)))
	reqAuthBad.Header.Set("Content-Type", "application/json")
	respAuthBad, err := s.app.Test(reqAuthBad)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respAuthBad.StatusCode, http.StatusUnauthorized)

	// 4. Auth session: valid PIN -> 200 and return token
	reqAuthGood := httptest.NewRequest("POST", "/api/auth/session", bytes.NewReader([]byte(`{"pin":"1234"}`)))
	reqAuthGood.Header.Set("Content-Type", "application/json")
	respAuthGood, err := s.app.Test(reqAuthGood)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respAuthGood.StatusCode, http.StatusOK)

	bodyAuth, _ := io.ReadAll(respAuthGood.Body)
	testutil.ExpectedContains(t, string(bodyAuth), `"token":`)

	// 5. Privileged call with Bearer token from session
	token, ok := s.CreatePINSession("1234")
	testutil.ExpectedTrue(t, ok)

	reqBearer := httptest.NewRequest("POST", "/api/webview/enabled", bytes.NewReader([]byte(`{"enabled":true}`)))
	reqBearer.Header.Set("Content-Type", "application/json")
	reqBearer.Header.Set("Authorization", "Bearer "+token)
	respBearer, err := s.app.Test(reqBearer)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respBearer.StatusCode, http.StatusOK)
	testutil.ExpectedTrue(t, cfg.GetWebViewEnabled())

	// 5b. Set exit corners with single corner array
	reqCorner := httptest.NewRequest("POST", "/api/webview/corners", bytes.NewReader([]byte(`{"corners":["bottom-left"]}`)))
	reqCorner.Header.Set("Content-Type", "application/json")
	reqCorner.Header.Set("Authorization", "Bearer "+token)
	respCorner, err := s.app.Test(reqCorner)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respCorner.StatusCode, http.StatusOK)
	testutil.ExpectedEqual(t, cfg.GetWebViewExitCorner(), "bottom-left")

	// 5c. Set exit corners with array
	reqCorners := httptest.NewRequest("POST", "/api/webview/corners", bytes.NewReader([]byte(`{"corners":["top-left","bottom-right"]}`)))
	reqCorners.Header.Set("Content-Type", "application/json")
	reqCorners.Header.Set("Authorization", "Bearer "+token)
	respCorners, err := s.app.Test(reqCorners)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respCorners.StatusCode, http.StatusOK)
	testutil.ExpectedLen(t, cfg.GetWebViewExitCorners(), 2)
	testutil.ExpectedEqual(t, cfg.GetWebViewExitCorners()[0], "top-left")
	testutil.ExpectedEqual(t, cfg.GetWebViewExitCorners()[1], "bottom-right")

	// 6. Reload kiosk callback with Bearer token
	reloadCalled := false
	s.SetKioskReloadCallback(func() {
		reloadCalled = true
	})

	reqReload := httptest.NewRequest("POST", "/api/webview/reload", nil)
	reqReload.Header.Set("Authorization", "Bearer "+token)
	respReload, err := s.app.Test(reqReload)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respReload.StatusCode, http.StatusOK)
	testutil.ExpectedTrue(t, reloadCalled)

	// 7. Kiosk exit callback
	exitCalled := false
	s.SetKioskExitCallback(func() {
		exitCalled = true
	})

	reqExit := httptest.NewRequest("POST", "/api/kiosk/exit", nil)
	respExit, err := s.app.Test(reqExit)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedEqual(t, respExit.StatusCode, http.StatusOK)
	time.Sleep(50 * time.Millisecond)
	testutil.ExpectedTrue(t, exitCalled)
}

