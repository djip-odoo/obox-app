package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync"
	"time"

	"obox-app/buildinfo"
	"obox-app/internal/config"
	"obox-app/internal/logger"
	"obox-app/internal/printer"
	"obox-app/internal/server"
	"obox-app/internal/update"
	"obox-app/internal/util"

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
	updater             *update.Updater
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
type UpdateInfo = update.Info

func (a *App) getUpdater() *update.Updater {
	if a.updater == nil {
		currentVer := buildinfo.Version
		if currentVer == "" || currentVer == "local" {
			currentVer = update.Version
		}
		a.updater = update.NewUpdater(update.Config{
			RepoOwner:      update.RepoOwner,
			RepoName:       update.RepoName,
			CurrentVersion: currentVer,
			TargetOS:       runtime.GOOS,
			TargetArch:     runtime.GOARCH,
		})
	}
	if a.config != nil && a.config.LastSeenUpdate() != "" {
		a.updater.Dismiss(a.config.LastSeenUpdate())
	}
	return a.updater
}

// CheckForUpdate asks GitHub whether a newer build exists for this OS.
func (a *App) CheckForUpdate() UpdateInfo {
	u := a.getUpdater()
	info, _ := u.Check(context.Background(), true)
	return info
}

// DownloadUpdate downloads the latest release asset and returns the local
// path. Progress is emitted to the frontend as the "update-progress" event.
func (a *App) DownloadUpdate() (string, error) {
	u := a.getUpdater()
	return u.Download(context.Background(), func(downloaded, total int64, percent int) {
		if a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "update-progress", map[string]interface{}{
				"downloaded": downloaded,
				"total":      total,
				"percent":    percent,
			})
		}
	})
}

// ApplyUpdate installs the previously downloaded asset, then quits so the new
// version (or the running installer) can take over.
func (a *App) ApplyUpdate() error {
	u := a.getUpdater()
	a.restartingForUpdate = true

	// Stop background services and release locks before applying
	if a.webserver != nil && a.webserver.Running() {
		_ = a.webserver.Stop()
	}

	if err := u.Apply(); err != nil {
		a.restartingForUpdate = false
		logger.Errorf("Failed to apply update: %v", err)
		return err
	}

	if a.ctx != nil {
		wailsruntime.Quit(a.ctx)
	}
	go func() {
		time.Sleep(1 * time.Second)
		os.Exit(0)
	}()
	return nil
}

// DismissUpdate records that the user dismissed this update tag.
func (a *App) DismissUpdate(tag string) error {
	a.getUpdater().Dismiss(tag)
	if a.config != nil {
		return a.config.SetLastSeenUpdate(tag)
	}
	return nil
}

// LastSeenUpdate exposes the release tag whose banner was already shown/dismissed.
func (a *App) LastSeenUpdate() string {
	if a.config == nil {
		return ""
	}
	return a.config.LastSeenUpdate()
}

// MarkUpdateSeen persists the release tag shown to the user.
func (a *App) MarkUpdateSeen(tag string) error {
	return a.DismissUpdate(tag)
}

// GetUpdateStatus returns the current status of the updater.
func (a *App) GetUpdateStatus() UpdateInfo {
	return a.getUpdater().Status()
}

func NewApp() *App {
	a := &App{}

	a.autoStart = &autostart.App{
		Name:        "obox-app",
		DisplayName: "Obox App",
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
		logger.SetDebugMode(true)
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

	// Non-blocking automatic update check in the background after startup
	go func() {
		time.Sleep(2 * time.Second)
		u := a.getUpdater()
		info, err := u.Check(context.Background(), false)
		if err == nil && info.Available && a.ctx != nil {
			wailsruntime.EventsEmit(a.ctx, "update-available", info)
		}
	}()
}

func (a *App) shutdown(ctx context.Context) {
	a.debugMu.Lock()
	if a.debugTimer != nil {
		a.debugTimer.Stop()
		a.debugTimer = nil
	}
	a.debugMu.Unlock()

	logger.Infof("Stopping Obox App server")

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
	zipName := fmt.Sprintf("obox-app-logs-%s.zip",
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

func (a *App) SetDebugModeEnabled(enabled bool) error {
	a.debugMu.Lock()
	if a.debugTimer != nil {
		a.debugTimer.Stop()
		a.debugTimer = nil
	}
	a.debugMu.Unlock()

	logger.SetDebugMode(enabled)
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

// SetSupportModeEnabled is an alias for SetDebugModeEnabled.
func (a *App) SetSupportModeEnabled(enabled bool) error {
	return a.SetDebugModeEnabled(enabled)
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
		_ = a.SetDebugModeEnabled(false)
		return
	}

	a.debugMu.Lock()
	if a.debugTimer != nil {
		a.debugTimer.Stop()
	}
	a.debugTimer = time.AfterFunc(remaining, func() {
		logger.Infof("Debug mode expired after 24 hours; auto-disabling")
		_ = a.SetDebugModeEnabled(false)
	})
	a.debugMu.Unlock()
}

func (a *App) IsDebugModeEnabled() bool {
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
