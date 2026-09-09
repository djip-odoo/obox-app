package server

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"strings"
	"sync"
	"sync/atomic"

	"epos-proxy/internal/config"
	"epos-proxy/internal/escpos"
	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"

	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/google/uuid"
)

// EPOSResponse is the XML envelope returned by print proxy routes.
type EPOSResponse struct {
	XMLName xml.Name `xml:"response"`
	Success bool     `xml:"success,attr"`
	Code    string   `xml:"code,attr"`
	Status  string   `xml:"status,attr"`
}

// Server wraps the Fiber HTTP server with auth state.
type Server struct {
	app             *fiber.App
	Port            int
	Host            string
	ready           chan struct{}
	readyOnce       sync.Once
	running         atomic.Bool
	reloadCount     atomic.Int64
	cfg             *config.Manager
	mgr             *printer.Manager
	mu              sync.RWMutex
	wailsToken      string          // trusted Wails session token (empty = none set yet)
	sessions        map[string]bool // active PIN-auth session tokens (remote clients)
	onKioskChanged  func(enabled bool)
	onConfigChanged func()
	onKioskReload   func()
	onKioskExit     func()
	wailsAppURL     string
}

// SetKioskCallback registers a callback invoked when kiosk enabled status changes via HTTP API.
func (s *Server) SetKioskCallback(cb func(bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onKioskChanged = cb
}

// SetConfigCallback registers a callback invoked when webview configuration changes via HTTP API.
func (s *Server) SetConfigCallback(cb func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onConfigChanged = cb
}

// SetKioskReloadCallback registers a callback invoked when kiosk reload is requested via HTTP API.
func (s *Server) SetKioskReloadCallback(cb func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onKioskReload = cb
}

// SetKioskExitCallback registers a callback invoked when exiting kiosk/webapp to return to the Wails app.
func (s *Server) SetKioskExitCallback(cb func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.onKioskExit = cb
}

// SetWailsAppURL stores the local Wails UI URL used to redirect upon kiosk exit.
func (s *Server) SetWailsAppURL(url string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wailsAppURL = url
}

// SetSessionToken registers the trusted Wails session token.
// Called once from App.startup() after the token is generated.
func (s *Server) SetSessionToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.wailsToken = token
}

// CreatePINSession validates pin against the stored PIN. On success it issues
// an opaque session token the remote client can use for privileged calls.
// Returns ("", false) when pin is wrong or no PIN is configured.
func (s *Server) CreatePINSession(pin string) (string, bool) {
	if s.cfg == nil || !s.cfg.CheckWebViewPIN(pin) {
		return "", false
	}
	token := uuid.New().String()
	s.mu.Lock()
	s.sessions[token] = true
	s.mu.Unlock()
	return token, true
}

// isAuthenticated returns true when c carries either the Wails token or a
// valid PIN-session token.
func (s *Server) isAuthenticated(c fiber.Ctx) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Trusted Wails request
	if wt := c.Get("X-Wails-Token"); wt != "" && s.wailsToken != "" && wt == s.wailsToken {
		return true
	}

	// Remote PIN-session
	auth := c.Get("Authorization")
	if strings.HasPrefix(auth, "Bearer ") {
		return s.sessions[strings.TrimPrefix(auth, "Bearer ")]
	}
	return false
}

// requireAuth is a Fiber middleware that blocks unauthenticated remote requests.
func (s *Server) requireAuth(c fiber.Ctx) error {
	if s.isAuthenticated(c) {
		return c.Next()
	}
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "authentication required"})
}

// New creates and starts the HTTP server listening on 0.0.0.0.
//
// mgr   — printer manager (required)
// cfg   — config manager; may be nil in tests that don't exercise APIs
// distFS — embedded frontend/dist; may be nil (falls back to hello-world at /)
func New(port int, mgr *printer.Manager, cfg *config.Manager, distFS fs.FS) *Server {
	return NewWithHost("0.0.0.0", port, mgr, cfg, distFS)
}

// NewWithHost creates and starts the HTTP server on the specified bind host.
func NewWithHost(host string, port int, mgr *printer.Manager, cfg *config.Manager, distFS fs.FS) *Server {
	if host == "" {
		host = "0.0.0.0"
	}
	srv := &Server{
		Port:     port,
		Host:     host,
		ready:    make(chan struct{}),
		cfg:      cfg,
		mgr:      mgr,
		sessions: make(map[string]bool),
	}

	app := fiber.New(fiber.Config{
		AppName: "ePOS proxy",
	})
	srv.app = app

	app.Use(func(c fiber.Ctx) error {
		// Chrome / Edge Local Network Access (LNA) and Private Network Access (PNA)
		c.Set("Access-Control-Allow-Private-Network", "true")
		c.Set("Access-Control-Allow-Local-Network", "true")
		c.Set("Permissions-Policy", "local-network=*, local-network-access=*, private-network-access=*, loopback-network=*")
		return c.Next()
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:        []string{"*"},
		AllowPrivateNetwork: true,
	}))

	// ── Read-only APIs ────────────────────────────────────────────────────────

	app.Get("/api/app", srv.handleGetApp)
	app.Get("/api/printers", srv.handleGetPrinters)
	app.Get("/api/printers/lan/:ip/status", srv.handleGetLANPrinterStatus)
	app.Get("/api/webview", srv.handleGetWebView)
	app.Get("/api/troubleshoot", srv.handleGetTroubleshoot)

	// PIN session creation — validates PIN and issues a session token
	app.Post("/api/auth/session", srv.handleAuthSession)

	// ── Privileged APIs (require Wails token or PIN session) ─────────────────

	app.Post("/api/app/quit", srv.requireAuth, srv.handleQuitApp)
	app.Post("/api/printers/lan", srv.requireAuth, srv.handleAddLANPrinter)
	app.Delete("/api/printers/lan", srv.requireAuth, srv.handleRemoveLANPrinter)
	app.Post("/api/webview/url", srv.requireAuth, srv.handleSetWebViewURL)
	app.Post("/api/webview/enabled", srv.requireAuth, srv.handleSetWebViewEnabled)
	app.Post("/api/webview/corners", srv.requireAuth, srv.handleSetWebViewExitCorner)
	app.Post("/api/webview/reload", srv.requireAuth, srv.handleReloadWebView)
	app.All("/api/kiosk/exit", func(c fiber.Ctx) error {
		srv.mu.RLock()
		cb := srv.onKioskExit
		wailsURL := srv.wailsAppURL
		srv.mu.RUnlock()
		if cb != nil {
			go cb()
		}
		if c.Method() == fiber.MethodGet {
			if wailsURL == "" {
				wailsURL = "/"
			}
			return c.Redirect().Status(fiber.StatusTemporaryRedirect).To(wailsURL)
		}
		return c.JSON(fiber.Map{"status": "ok"})
	})

	// Test-print / cash-drawer via proxy — privileged so random remote callers
	// can't trigger prints, while Odoo POS continues to use the open /p/…
	// routes below.
	app.Post("/api/printers/:printerId/test-print", srv.requireAuth, srv.handleTestPrint)
	app.Post("/api/printers/:printerId/cash-drawer", srv.requireAuth, srv.handleCashDrawer)

	// ── ePOS print-proxy routes (open — also used by Odoo POS) ───────────────

	app.Post("/p/:printerId/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		printerId := ctx.Params("printerId")
		logger.Debugf("Print request received for printer: %s", printerId)
		return printData(mgr, ctx, printerId)
	})

	app.Post("/cgi-bin/epos/service.cgi", func(ctx fiber.Ctx) error {
		logger.Debugf("Print request received (auto printer selection)")
		return printData(mgr, ctx, "")
	})

	app.Post("/p/:printerId/pstprnt", func(ctx fiber.Ctx) error {
		printerId := ctx.Params("printerId")
		logger.Debugf("Label print request received for printer: %s", printerId)
		return printLabel(mgr, ctx, printerId)
	})

	// ── Embedded frontend (catch-all) ─────────────────────────────────────────

	if distFS != nil {
		app.Use("/", func(c fiber.Ctx) error {
			if c.Method() != "GET" {
				return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "not found"})
			}
			return serveFrontend(c, distFS)
		})
	} else {
		app.Get("/", func(ctx fiber.Ctx) error {
			return ctx.SendString(fmt.Sprintf("Hello from %s", app.Config().AppName))
		})
	}

	srv.running.Store(true)
	go func() {
		logger.Infof("HTTP server listening on %s:%d", host, port)
		listenCfg := fiber.ListenConfig{
			DisableStartupMessage: true,
			ListenerAddrFunc: func(addr net.Addr) {
				srv.signalReady()
			},
		}
		if err := app.Listen(fmt.Sprintf("%s:%d", host, port), listenCfg); err != nil {
			logger.Error("EPOS Server Error: ", err)
			srv.signalReady()
		}
		srv.running.Store(false)
		logger.Warn("HTTP server stopped")
	}()
	return srv
}

func (s *Server) signalReady() {
	s.readyOnce.Do(func() {
		close(s.ready)
	})
}

// WaitReady blocks until the HTTP server is listening or the context is cancelled.
func (s *Server) WaitReady(ctx context.Context) error {
	select {
	case <-s.ready:
		if !s.running.Load() {
			return errors.New("HTTP server failed to start")
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// serveFrontend serves files from the embedded dist FS. Unrecognised paths
// fall back to index.html so the React SPA handles them.
func serveFrontend(c fiber.Ctx, distFS fs.FS) error {
	path := strings.TrimPrefix(c.Path(), "/")
	if path == "" {
		path = "index.html"
	}

	data, err := fs.ReadFile(distFS, path)
	if err != nil {
		// SPA fallback: unknown paths serve index.html
		data, err = fs.ReadFile(distFS, "index.html")
		if err != nil {
			return c.Status(fiber.StatusNotFound).SendString("Not found")
		}
		c.Type("html")
		return c.Send(data)
	}

	// Set Content-Type from extension
	ext := ""
	if dot := strings.LastIndex(path, "."); dot >= 0 {
		ext = path[dot:]
	}
	switch ext {
	case ".html":
		c.Type("html")
	case ".js", ".mjs":
		c.Set("Content-Type", "application/javascript; charset=utf-8")
	case ".css":
		c.Type("css")
	case ".ico":
		c.Type("ico")
	case ".png":
		c.Type("png")
	case ".svg":
		c.Set("Content-Type", "image/svg+xml")
	case ".json":
		c.Type("json")
	case ".woff":
		c.Set("Content-Type", "font/woff")
	case ".woff2":
		c.Set("Content-Type", "font/woff2")
	default:
		c.Set("Content-Type", "application/octet-stream")
	}

	return c.Send(data)
}

// ── ePOS print helpers (unchanged) ────────────────────────────────────────────

func printData(mgr *printer.Manager, ctx fiber.Ctx, printerID string) error {
	logger.Debugf("Processing print job for printer: %s", printerID)
	jobData, err := escpos.ParseXML(ctx.Body())
	if err != nil {
		logger.Errorf("XML parsing error: %v", err)
		return ctx.XML(EPOSResponse{Success: false, Code: "SchemaError", Status: ""})
	}
	logger.Debug("XML parsed successfully")

	reply, err := mgr.WriteAsync(printerID, jobData)
	if err == nil {
		logger.Debug("Print job queued")
		result := <-reply
		if !result.OK {
			err = result.Err
		}
	}
	if err != nil {
		retCode := ""
		if errors.Is(err, printer.ErrQueueFull) {
			retCode = "TooManyRequests"
			logger.Warn("Printer queue full")
		} else {
			retCode = "EX_BADPORT"
		}
		logger.Errorf("Print error [%s]: %v, Printer ID: %s", retCode, err, printerID)
		return ctx.XML(EPOSResponse{Success: false, Code: retCode, Status: ""})
	}
	logger.Debugf("Print job completed successfully for printer: %s", printerID)
	return ctx.XML(EPOSResponse{Success: true, Code: "", Status: ""})
}

func printLabel(mgr *printer.Manager, ctx fiber.Ctx, printerID string) error {
	jobData := ctx.Body()

	if len(jobData) == 0 {
		logger.Warn("Empty label data received")
		return ctx.SendStatus(fiber.StatusBadRequest)
	}

	logger.Debugf("Processing label print job for printer: %s", printerID)

	reply, err := mgr.WriteAsync(printerID, jobData)
	if err == nil {
		logger.Debug("Label print job queued")
		result := <-reply
		if !result.OK {
			err = result.Err
		}
	}

	if err != nil {
		if errors.Is(err, printer.ErrQueueFull) {
			logger.Warnf("Printer queue full, Printer ID: %s", printerID)
			return ctx.SendStatus(fiber.StatusTooManyRequests)
		}
		logger.Errorf("Print error: %v, Printer ID: %s", err, printerID)
		return ctx.SendStatus(fiber.StatusInternalServerError)
	}

	logger.Debugf("Print job completed successfully for printer: %s", printerID)
	return ctx.SendStatus(fiber.StatusOK)
}

// executePrint builds and dispatches an ePOS-XML print job through the
// printer manager. content is the inner ePOS XML (without the SOAP wrapper).
func (s *Server) executePrint(c fiber.Ctx, printerID, content string) error {
	body := fmt.Sprintf(
		`<s:Envelope xmlns:s="http://schemas.xmlsoap.org/soap/envelope/"><s:Body>`+
			`<epos-print xmlns="http://www.epson-pos.com/schemas/2011/03/epos-print">`+
			`%s</epos-print></s:Body></s:Envelope>`,
		content,
	)

	jobData, err := escpos.ParseXML([]byte(body))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid print data"})
	}

	reply, err := s.mgr.WriteAsync(printerID, jobData)
	if err == nil {
		result := <-reply
		if !result.OK {
			err = result.Err
		}
	}
	if err != nil {
		if errors.Is(err, printer.ErrQueueFull) {
			return c.Status(fiber.StatusTooManyRequests).JSON(fiber.Map{"error": "printer queue full"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return c.JSON(fiber.Map{"ok": true})
}

// bindJSON is a tiny helper that unmarshals the request body into v.
func bindJSON(c fiber.Ctx, v any) error {
	return json.Unmarshal(c.Body(), v)
}

func (s *Server) Stop() error {
	logger.Infof("Stopping HTTP server")
	s.running.Store(false)
	return s.app.Shutdown()
}

func (s *Server) Running() bool {
	return s.running.Load()
}
