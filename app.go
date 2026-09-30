package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"epos-proxy/buildinfo"
	"epos-proxy/internal/config"
	"epos-proxy/internal/logger"
	"epos-proxy/internal/printer"
	"epos-proxy/internal/server"
	"epos-proxy/internal/update"
	"epos-proxy/internal/util"

	autostart "github.com/emersion/go-autostart"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// dialoger abstracts the Wails runtime dialog calls. Production code uses
// runtimeDialogs; tests substitute a fake so the dialog-driven code paths can
// be exercised without a live Wails context.
type dialoger interface {
	Message(ctx context.Context, opts wailsruntime.MessageDialogOptions) (string, error)
	SaveFile(ctx context.Context, opts wailsruntime.SaveDialogOptions) (string, error)
}

// runtimeDialogs forwards to the real Wails runtime.
type runtimeDialogs struct{}

func (runtimeDialogs) Message(ctx context.Context, opts wailsruntime.MessageDialogOptions) (string, error) {
	return wailsruntime.MessageDialog(ctx, opts)
}

func (runtimeDialogs) SaveFile(ctx context.Context, opts wailsruntime.SaveDialogOptions) (string, error) {
	return wailsruntime.SaveFileDialog(ctx, opts)
}

// App struct
type App struct {
	ctx                 context.Context
	webserver           *server.Server
	config              *config.Manager
	printerManager      *printer.Manager
	autoStart           *autostart.App
	dialogs             dialoger
	updatePath          string
	restartingForUpdate bool
	debugTimer          *time.Timer
	debugMu             sync.Mutex
}

// dlg returns the dialog backend, defaulting to the Wails runtime so an App
// built as a bare struct literal still behaves correctly.
func (a *App) dlg() dialoger {
	if a.dialogs == nil {
		return runtimeDialogs{}
	}
	return a.dialogs
}

// showError surfaces an error to the user and logs any failure to do so.
func (a *App) showError(title, message string) {
	if _, err := a.dlg().Message(a.ctx, wailsruntime.MessageDialogOptions{
		Type:    wailsruntime.ErrorDialog,
		Title:   title,
		Message: message,
	}); err != nil {
		logger.Errorf("Failed to show error dialog %q: %v", title, err)
	}
}

type Printer struct {
	Name   string `json:"name"`
	Ip     string `json:"ip"`
	Id     string `json:"id"`
	IsLAN  bool   `json:"isLAN"`
	LANIp  string `json:"lanIp,omitempty"`
	Online bool   `json:"online"`
	Type   string `json:"type"`
}

type UnavailablePrinter struct {
	Name     string `json:"name"`
	ErrorMsg string `json:"errorMsg"`
	IsLAN    bool   `json:"isLAN"`
	LANIp    string `json:"lanIp,omitempty"`
}

type AppVariable struct {
	ServerRunning bool   `json:"serverRunning"`
	Os            string `json:"os"`
	Version       string `json:"version"`
	BuildTime     string `json:"buildTime"`
	Commit        string `json:"commit"`
}

type Printers struct {
	ErrorMsg            string               `json:"errorMsg"`
	Printers            []Printer            `json:"printers"`
	UnavailablePrinters []UnavailablePrinter `json:"unavailablePrinters"`
}

// UpdateInfo summarises the outcome of a release check for the frontend.
type UpdateInfo struct {
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

var currentVersion = update.Version

// CheckForUpdate asks GitHub whether a newer build exists for this OS. It
// never fails: network errors come back inside the result so the UI can show
// them inline.
func (a *App) CheckForUpdate() UpdateInfo {
	logger.Debugf("Checking for updates (current version %s)", currentVersion)
	info := update.Check()
	ui := UpdateInfo{
		CheckOK:        info.CheckOK,
		Available:      info.Available,
		CurrentVersion: currentVersion,
		LatestVersion:  info.LatestVersion,
		DownloadURL:    info.DownloadURL,
		AssetName:      info.AssetName,
		AssetSize:      info.AssetSize,
		Notes:          info.Notes,
		Error:          info.Error,
	}
	if info.Available {
		logger.Infof("Update available: %s -> %s", currentVersion, info.LatestVersion)
	}
	return ui
}

// DownloadUpdate downloads the latest release asset and returns the local
// path. Progress is emitted to the frontend as the "update-progress" event.
func (a *App) DownloadUpdate() (string, error) {
	info := a.CheckForUpdate()
	if !info.CheckOK || !info.Available {
		return "", errors.New("no update available for this operating system")
	}

	dir, err := os.MkdirTemp("", "epos-proxy-update")
	if err != nil {
		return "", fmt.Errorf("failed to create update directory: %w", err)
	}

	path, err := update.Download(info.DownloadURL, info.AssetName, dir, func(downloaded, total int64) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "update-progress", map[string]int64{
				"downloaded": downloaded,
				"total":      total,
			})
		}
	})
	if err != nil {
		return "", err
	}

	a.updatePath = path
	return path, nil
}

// ApplyUpdate installs the previously downloaded asset, then quits so the new
// version (or the running installer) can take over.
func (a *App) ApplyUpdate() error {
	if a.updatePath == "" {
		return errors.New("no downloaded update to apply")
	}

	logger.Infof("Applying update %s", a.updatePath)
	if err := update.Apply(a.updatePath); err != nil {
		logger.Errorf("Failed to apply update %s: %v", a.updatePath, err)
		return err
	}
	a.updatePath = ""
	a.restartingForUpdate = true

	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
	return nil
}

// LastSeenUpdate exposes the release tag whose banner was already shown, so the
// frontend can decide whether to offer the banner again.
func (a *App) LastSeenUpdate() string {
	if a.config == nil {
		return ""
	}
	return a.config.LastSeenUpdate()
}

// MarkUpdateSeen persists the release tag shown to the user. The banner for
// that version is then suppressed until a newer release appears.
func (a *App) MarkUpdateSeen(tag string) error {
	if a.config == nil {
		return nil
	}
	if err := a.config.SetLastSeenUpdate(tag); err != nil {
		logger.Warnf("Failed to persist seen update %q: %v", tag, err)
		return err
	}
	return nil
}

func NewApp() *App {
	a := &App{}

	a.autoStart = &autostart.App{
		Name:        "epos-proxy",
		DisplayName: "ePOS Proxy",
		Exec:        []string{os.Args[0]},
	}
	a.printerManager = printer.NewManager()
	a.dialogs = runtimeDialogs{}

	cfg, err := config.NewManager()
	if err != nil {
		logger.Fatalf("Config initialization failed: %v", err)
	}

	if err := cfg.Load(); err != nil {
		logger.Warnf("Config load warning: %v", err)
	}

	a.config = cfg

	if a.config.IsDebugMode() {
		logger.SetSupportMode(true)
		a.scheduleDebugModeExpiry()
	}

	return a
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	logger.Debugf("Application startup")
	logger.Debugf("Config loaded from %s", a.config.Path())

	port, err := a.config.ResolvePort()
	if err != nil {
		logger.Warn("Unable to resolve port, using default")
	}

	a.webserver = server.New(port, a.printerManager)
}

func (a *App) shutdown(ctx context.Context) {
	a.debugMu.Lock()
	if a.debugTimer != nil {
		a.debugTimer.Stop()
		a.debugTimer = nil
	}
	a.debugMu.Unlock()

	logger.Infof("Stopping proxy server")

	if err := a.webserver.Stop(); err != nil {
		logger.Errorf("Server stop error: %v", err)
	}
}

func (a *App) AppVariable() AppVariable {
	return AppVariable{
		Os:            runtime.GOOS,
		ServerRunning: a.webserver.Running(),
		Version:       buildinfo.Version,
		BuildTime:     buildinfo.BuildTime,
		Commit:        buildinfo.Commit,
	}
}

func (a *App) GetPrinterUrl(id string) string {
	url := fmt.Sprintf("%s:%d/p/%s", util.GetLocalIP(a.config.IsNetworkPrintingEnabled()), a.webserver.Port, id)
	logger.Debugf("Generated printer endpoint: %s", url)
	return url
}

func (a *App) Printers() Printers {

	logger.Debug("Collecting printer status")

	printers := make([]Printer, 0)
	unavailablePrinters := make([]UnavailablePrinter, 0)

	printerInfos, err := printer.ListUSBPrinters()
	errorMsg := ""
	if err == nil {

		logger.Debugf("Detected %d available USB printers", len(printerInfos.Available))

		for _, info := range printerInfos.Available {
			printers = append(printers, Printer{
				Id:     info.Id,
				Name:   info.Name,
				Ip:     a.GetPrinterUrl(info.Id),
				Online: true,
				Type:   string(info.Type),
			})
		}

		for _, info := range printerInfos.Unavailable {
			unavailablePrinters = append(unavailablePrinters, UnavailablePrinter{
				Name:     info.Name,
				ErrorMsg: info.Error,
			})

			logger.Warnf("USB printer unavailable: %s (%s)", info.Name, info.Error)
		}
	} else {
		errorMsg = err.Error()
		logger.Errorf("USB printer detection failed: %v", err)
	}

	lanPrinters := printer.ListLANPrinters(a.config)

	for _, info := range lanPrinters {
		printers = append(printers, Printer{
			Id:    info.Id,
			Name:  fmt.Sprintf("Network - %s", info.IP),
			Ip:    a.GetPrinterUrl(info.Id),
			IsLAN: true,
			LANIp: info.IP,
			Type:  string(printer.TypeReceipt),
		})
	}

	return Printers{
		Printers:            printers,
		UnavailablePrinters: unavailablePrinters,
		ErrorMsg:            errorMsg,
	}
}

func (a *App) AddLANPrinter(ip string) error {
	logger.Debugf("Adding LAN printer: %s", ip)

	ip, err := printer.ValidateIPAddress(ip)
	if err != nil {
		return fmt.Errorf("invalid IP address: %s, error: %v", ip, err)
	}

	if err := printer.CheckLANPrinter(ip); err != nil {
		return fmt.Errorf("LAN printer unreachable: %s, error: %v", ip, err)
	}

	if err := a.config.AddLanEposPrinter(ip); err != nil {
		return fmt.Errorf("failed to save LAN printer: %s, error: %v", ip, err)
	}

	logger.Debugf("LAN printer added successfully: %s", ip)
	return nil
}

func (a *App) ConfirmRemoveLANPrinter(ip string) (bool, error) {
	logger.Debugf("Remove LAN printer requested: %s", ip)

	result, err := a.dlg().Message(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Remove Printer",
		Message:       fmt.Sprintf("Are you sure you want to remove the printer at %s?", ip),
		Buttons:       []string{"Cancel", "Confirm"},
		DefaultButton: "Cancel",
		CancelButton:  "Cancel",
	})
	if err != nil {
		return false, fmt.Errorf("failed to show confirmation dialog: %w", err)
	}
	if result == "Confirm" || result == "Yes" {
		if err := a.config.RemoveLANPrinter(ip); err != nil {
			return false, fmt.Errorf("failed to remove LAN printer: %w", err)
		}
		return true, nil
	}
	logger.Infof("Remove LAN printer cancelled, Remove printer dialog result: %s", result)
	return false, nil
}

func (a *App) CheckLANPrinterStatus(ip string) bool {
	logger.Debugf("Checking LAN printer status: %s", ip)
	return printer.CheckLANPrinter(ip) == nil
}

func (a *App) DownloadLogs() {
	logger.Debugf("Download logs requested")
	logDir := logger.LogDirectory()
	zipName := fmt.Sprintf("epos-proxy-logs-%s.zip",
		time.Now().Format("2006-01-02"))
	logger.Debugf("Creating logs archive: %s", zipName)
	savePath, err := a.dlg().SaveFile(a.ctx, wailsruntime.SaveDialogOptions{
		Title:           "Save Archive",
		DefaultFilename: zipName,
		Filters: []wailsruntime.FileFilter{
			{
				DisplayName: "Zip Archives (*.zip)",
				Pattern:     "*.zip",
			},
		},
	})
	if err != nil {
		logger.Errorf("Save dialog failed: %v", err)
		a.showError("Download Logs Failed", err.Error())
		return
	}

	// An empty path means the user dismissed the save dialog.
	if savePath == "" {
		logger.Infof("Download logs cancelled by user")
		return
	}

	if err := util.ZipLogs(logDir, savePath); err != nil {
		logger.Errorf("Log export failed: %v", err)
		a.showError("Download Logs Failed", err.Error())
		return
	}
	logger.Infof("Logs successfully exported to: %s", savePath)
}

func (a *App) IsAutostartEnabled() bool {
	return a.autoStart.IsEnabled()
}

func (a *App) EnableAutostart() error {
	logger.Info("Enabling autostart")

	if runtime.GOOS == "linux" {
		return util.EnableLinuxAutostart()
	}

	if !a.autoStart.IsEnabled() {
		return a.autoStart.Enable()
	}

	return nil
}

func (a *App) DisableAutostart() error {
	logger.Info("Disabling autostart")

	if a.autoStart.IsEnabled() {
		return a.autoStart.Disable()
	}

	return nil
}

func (a *App) SetNetworkPrintingEnabled(enabled bool) error {
	logger.Infof("Setting network printing enabled: %v", enabled)
	return a.config.SetNetworkPrintingEnabled(enabled)
}

func (a *App) IsNetworkPrintingEnabled() bool {
	if a.config == nil {
		return false
	}
	return a.config.IsNetworkPrintingEnabled()
}

func (a *App) SetSupportModeEnabled(enabled bool) error {
	a.debugMu.Lock()
	if a.debugTimer != nil {
		a.debugTimer.Stop()
		a.debugTimer = nil
	}
	a.debugMu.Unlock()

	logger.SetSupportMode(enabled)
	if a.config != nil {
		if err := a.config.SetDebugMode(enabled); err != nil {
			return err
		}
		if enabled {
			a.scheduleDebugModeExpiry()
		}
	}
	return nil
}

func (a *App) scheduleDebugModeExpiry() {
	if a.config == nil {
		return
	}
	expiresAt := a.config.DebugModeExpiresAt()
	if expiresAt == nil {
		return
	}
	remaining := time.Until(*expiresAt)
	if remaining <= 0 {
		_ = a.SetSupportModeEnabled(false)
		return
	}

	a.debugMu.Lock()
	if a.debugTimer != nil {
		a.debugTimer.Stop()
	}
	a.debugTimer = time.AfterFunc(remaining, func() {
		logger.Infof("Support mode expired after 24 hours; auto-disabling")
		_ = a.SetSupportModeEnabled(false)
	})
	a.debugMu.Unlock()
}

func (a *App) IsSupportModeEnabled() bool {
	if a.config == nil {
		return logger.IsSupportMode()
	}
	return a.config.IsDebugMode()
}

type TroubleshootInfo struct {
	ActiveFirewall string `json:"activeFirewall"`
	FirewallZone   string `json:"firewallZone"`
	Port           int    `json:"port"`
	Subnet         string `json:"subnet"`
	LocalIP        string `json:"localIp"`
	ExecPath       string `json:"execPath"`
}

func (a *App) GetTroubleshootInfo() TroubleshootInfo {
	netInfo := util.GetNetworkInfo()
	execPath, _ := os.Executable()
	return TroubleshootInfo{
		ActiveFirewall: netInfo.ActiveFirewall,
		FirewallZone:   netInfo.Zone,
		Port:           a.config.GetPort(),
		Subnet:         netInfo.Subnet,
		LocalIP:        netInfo.IP,
		ExecPath:       execPath,
	}
}
