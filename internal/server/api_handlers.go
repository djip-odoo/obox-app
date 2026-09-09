package server

import (
	"fmt"
	"os"
	"runtime"
	"time"

	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"
	"epos-proxy/internal/util"

	"github.com/gofiber/fiber/v3"
)

// ── Response types (mirror the Wails main-package structs for identical JSON) ─

type apiAppVariable struct {
	ServerRunning bool   `json:"serverRunning"`
	Os            string `json:"os"`
	KioskMode     bool   `json:"kioskMode"`
}

type apiPrinter struct {
	Name   string `json:"name"`
	Ip     string `json:"ip"`
	Id     string `json:"id"`
	IsLAN  bool   `json:"isLAN"`
	LANIp  string `json:"lanIp,omitempty"`
	Online bool   `json:"online"`
	Type   string `json:"type"`
}

type apiUnavailablePrinter struct {
	Name     string `json:"name"`
	ErrorMsg string `json:"errorMsg"`
	IsLAN    bool   `json:"isLAN"`
	LANIp    string `json:"lanIp,omitempty"`
}

type apiPrintersResponse struct {
	ErrorMsg            string                  `json:"errorMsg"`
	Printers            []apiPrinter            `json:"printers"`
	UnavailablePrinters []apiUnavailablePrinter `json:"unavailablePrinters"`
}

type apiWebViewConfig struct {
	URL         string   `json:"url"`
	Enabled     bool     `json:"enabled"`
	HasPIN      bool     `json:"hasPIN"`
	ExitCorner  string   `json:"exitCorner"`
	ExitCorners []string `json:"exitCorners"`
	ReloadCount int64    `json:"reloadCount"`
}

type apiTroubleshootInfo struct {
	ActiveFirewall string `json:"activeFirewall"`
	FirewallZone   string `json:"firewallZone"`
	Port           int    `json:"port"`
	Subnet         string `json:"subnet"`
	LocalIP        string `json:"localIp"`
	ExecPath       string `json:"execPath"`
}

// ── Read-only handlers ─────────────────────────────────────────────────────────

func (s *Server) handleGetApp(c fiber.Ctx) error {
	kioskMode := false
	if s.cfg != nil {
		kioskMode = s.cfg.IsKioskEnabled()
	}
	return c.JSON(apiAppVariable{
		ServerRunning: true, // server is running — we received the request
		Os:            runtime.GOOS,
		KioskMode:     kioskMode,
	})
}

func (s *Server) handleGetPrinters(c fiber.Ctx) error {
	printers := make([]apiPrinter, 0)
	unavailable := make([]apiUnavailablePrinter, 0)
	errorMsg := ""

	printerURL := func(id string) string {
		if s.cfg == nil {
			return fmt.Sprintf("localhost:%d/p/%s", s.Port, id)
		}
		return fmt.Sprintf("%s:%d/p/%s", util.GetLocalIP(s.cfg.IsNetworkPrintingEnabled()), s.Port, id)
	}

	infos, err := printer.ListUSBPrinters()
	if err == nil {
		logger.Debugf("API: detected %d available USB printers", len(infos.Available))
		for _, info := range infos.Available {
			printers = append(printers, apiPrinter{
				Id:     info.Id,
				Name:   info.Name,
				Ip:     printerURL(info.Id),
				Online: true,
				Type:   string(info.Type),
			})
		}
		for _, info := range infos.Unavailable {
			unavailable = append(unavailable, apiUnavailablePrinter{
				Name:     info.Name,
				ErrorMsg: info.Error,
			})
		}
	} else {
		errorMsg = err.Error()
		logger.Errorf("API: USB printer detection failed: %v", err)
	}

	if s.cfg != nil {
		for _, info := range printer.ListLANPrinters(s.cfg) {
			printers = append(printers, apiPrinter{
				Id:    info.Id,
				Name:  fmt.Sprintf("Network - %s", info.IP),
				Ip:    printerURL(info.Id),
				IsLAN: true,
				LANIp: info.IP,
				Type:  string(printer.TypeReceipt),
			})
		}
	}

	return c.JSON(apiPrintersResponse{
		Printers:            printers,
		UnavailablePrinters: unavailable,
		ErrorMsg:            errorMsg,
	})
}

func (s *Server) handleGetLANPrinterStatus(c fiber.Ctx) error {
	ip := c.Params("ip")
	online := printer.CheckLANPrinter(ip) == nil
	return c.JSON(fiber.Map{"online": online})
}

func (s *Server) handleGetWebView(c fiber.Ctx) error {
	if s.cfg == nil {
		return c.JSON(apiWebViewConfig{})
	}
	return c.JSON(apiWebViewConfig{
		URL:         s.cfg.GetWebViewURL(),
		Enabled:     s.cfg.GetWebViewEnabled(),
		HasPIN:      s.cfg.HasWebViewPIN(),
		ExitCorner:  s.cfg.GetWebViewExitCorner(),
		ExitCorners: s.cfg.GetWebViewExitCorners(),
		ReloadCount: s.reloadCount.Load(),
	})
}

func (s *Server) handleGetTroubleshoot(c fiber.Ctx) error {
	netInfo := util.GetNetworkInfo()
	execPath, _ := os.Executable()
	port := 0
	if s.cfg != nil {
		port = s.cfg.GetPort()
	}
	return c.JSON(apiTroubleshootInfo{
		ActiveFirewall: netInfo.ActiveFirewall,
		FirewallZone:   netInfo.Zone,
		Port:           port,
		Subnet:         netInfo.Subnet,
		LocalIP:        netInfo.IP,
		ExecPath:       execPath,
	})
}

// ── Auth handler ───────────────────────────────────────────────────────────────

type authSessionReq struct {
	PIN string `json:"pin"`
}

// handleAuthSession validates the supplied PIN and, on success, returns an
// opaque session token the remote client stores in sessionStorage.
func (s *Server) handleAuthSession(c fiber.Ctx) error {
	if s.cfg == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{"error": "service unavailable"})
	}

	var req authSessionReq
	if err := bindJSON(c, &req); err != nil || req.PIN == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	token, ok := s.CreatePINSession(req.PIN)
	if !ok {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid PIN"})
	}

	return c.JSON(fiber.Map{"token": token})
}

// ── Privileged handlers ────────────────────────────────────────────────────────

type addLANPrinterReq struct {
	IP string `json:"ip"`
}

func (s *Server) handleAddLANPrinter(c fiber.Ctx) error {
	var req addLANPrinterReq
	if err := bindJSON(c, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	ip, err := printer.ValidateIPAddress(req.IP)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid IP address: " + err.Error()})
	}

	if err := printer.CheckLANPrinter(ip); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "LAN printer unreachable: " + err.Error()})
	}

	if err := s.cfg.AddLanEposPrinter(ip); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to save: " + err.Error()})
	}

	logger.Debugf("API: LAN printer added: %s", ip)
	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"ok": true})
}

type removeLANPrinterReq struct {
	IP string `json:"ip"`
}

func (s *Server) handleRemoveLANPrinter(c fiber.Ctx) error {
	var req removeLANPrinterReq
	if err := bindJSON(c, &req); err != nil || req.IP == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}

	if err := s.cfg.RemoveLANPrinter(req.IP); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	logger.Debugf("API: LAN printer removed: %s", req.IP)
	return c.JSON(fiber.Map{"ok": true})
}

type setWebViewURLReq struct {
	URL string `json:"url"`
}

func (s *Server) handleSetWebViewURL(c fiber.Ctx) error {
	var req setWebViewURLReq
	if err := bindJSON(c, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := s.cfg.SetWebViewURL(req.URL); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	s.mu.RLock()
	cb := s.onConfigChanged
	s.mu.RUnlock()
	if cb != nil {
		cb()
	}
	return c.JSON(fiber.Map{"ok": true})
}

type setWebViewEnabledReq struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleSetWebViewEnabled(c fiber.Ctx) error {
	var req setWebViewEnabledReq
	if err := bindJSON(c, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	if err := s.cfg.SetWebViewEnabled(req.Enabled); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	s.mu.RLock()
	cb := s.onKioskChanged
	s.mu.RUnlock()
	if cb != nil {
		cb(req.Enabled)
	}
	return c.JSON(fiber.Map{"ok": true})
}

type setWebViewExitCornerReq struct {
	Corner  string   `json:"corner"`
	Corners []string `json:"corners"`
}

func (s *Server) handleSetWebViewExitCorner(c fiber.Ctx) error {
	var req setWebViewExitCornerReq
	if err := bindJSON(c, &req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
	}
	var err error
	if len(req.Corners) > 0 {
		err = s.cfg.SetWebViewExitCorners(req.Corners)
	} else if req.Corner != "" {
		err = s.cfg.SetWebViewExitCorner(req.Corner)
	} else {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "corner or corners is required"})
	}
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	s.mu.RLock()
	cb := s.onConfigChanged
	s.mu.RUnlock()
	if cb != nil {
		cb()
	}
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) handleReloadWebView(c fiber.Ctx) error {
	s.reloadCount.Add(1)
	s.mu.RLock()
	cb := s.onKioskReload
	s.mu.RUnlock()
	if cb != nil {
		cb()
	}
	return c.JSON(fiber.Map{"ok": true})
}

// ── Privileged print-test / cash-drawer routes ────────────────────────────────

func (s *Server) handleTestPrint(c fiber.Ctx) error {
	printerID := c.Params("printerId")
	logger.Debugf("API: test print for printer: %s", printerID)
	content := `<feed line="1" /><text font="font_e" em="true"/><text align="center">Test Print</text><feed line="10" /><cut type="feed" />`
	return s.executePrint(c, printerID, content)
}

func (s *Server) handleCashDrawer(c fiber.Ctx) error {
	printerID := c.Params("printerId")
	logger.Debugf("API: cash drawer open for printer: %s", printerID)
	return s.executePrint(c, printerID, "<pulse />")
}

func (s *Server) handleQuitApp(c fiber.Ctx) error {
	logger.Infof("Quit application requested via API")
	go func() {
		time.Sleep(200 * time.Millisecond)
		_ = s.Stop()
		os.Exit(0)
	}()
	return c.JSON(fiber.Map{"ok": true})
}

