package main

import (
	"os"
	"runtime"

	"epos-proxy/internal/logger"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

func createMenu(app *App) *menu.Menu {
	mainMenu := menu.NewMenu()
	appMenu := mainMenu.AddSubmenu("App")

	if runtime.GOOS == "darwin" {
		// Without an Edit menu, copy/paste do nothing on the webview on macO
		mainMenu.Append(menu.EditMenu())
	}

	appMenu.AddText("Open WebApp", nil, func(_ *menu.CallbackData) {
		app.NavigateToWebApp()
	})

	appMenu.AddText("Return to Wails Management", keys.Combo("s", keys.CmdOrCtrlKey, keys.OptionOrAltKey), func(_ *menu.CallbackData) {
		app.ReturnToWailsApp()
	})

	appMenu.AddCheckbox("Auto Start", app.IsAutostartEnabled(), nil, func(cb *menu.CallbackData) {
		handleAutoStartToggle(app, cb)
	})

	appMenu.AddCheckbox("Allow Network Printing", app.IsNetworkPrintingEnabled(), nil, func(cb *menu.CallbackData) {
		handleNetworkPrintingToggle(app, cb)
	})

	appMenu.AddCheckbox("Fullscreen WebApp", app.config.GetWebViewFullscreen(), nil, func(cb *menu.CallbackData) {
		_ = app.SetWebViewFullscreen(cb.MenuItem.Checked)
	})

	appMenu.AddText("Set PIN", nil, func(_ *menu.CallbackData) {
		wailsruntime.EventsEmit(app.ctx, "open-set-pin-dialog")
	})

	appMenu.AddText("Download Logs", nil, func(_ *menu.CallbackData) {
		app.DownloadLogs()
	})

	appMenu.AddText("Quit", nil, func(_ *menu.CallbackData) {
		logger.Infof("Quit requested by user from menu")
		if app.webserver != nil {
			_ = app.webserver.Stop()
		}
		os.Exit(0)
	})

	return mainMenu
}

func handleAutoStartToggle(app *App, cb *menu.CallbackData) {
	checked := cb.MenuItem.Checked

	logger.Debugf("Auto Start toggled: %v", checked)

	if checked {
		if err := app.EnableAutostart(); err != nil {
			logger.Errorf("Failed to enable autostart: %v", err)
		}
		return
	}

	if err := app.DisableAutostart(); err != nil {
		logger.Errorf("Failed to disable autostart: %v", err)
	}
}

func handleNetworkPrintingToggle(app *App, cb *menu.CallbackData) {
	checked := cb.MenuItem.Checked

	// Guards against a spurious callback firing with the already-persisted
	// value (e.g. on initial menu setup) so the info dialog only shows for
	// an actual user-driven change, never on app start.
	if checked == app.IsNetworkPrintingEnabled() {
		return
	}

	logger.Debugf("Allow Network Printing toggled: %v", checked)

	if err := app.SetNetworkPrintingEnabled(checked); err != nil {
		logger.Errorf("Failed to set network printing enabled: %v", err)
		return
	}

	wailsruntime.EventsEmit(app.ctx, "network-printing-changed", checked)
}

func (app *App) ConfirmQuit() bool {
	result, err := app.dlg().Message(app.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.QuestionDialog,
		Title:         "Quit ePOS Proxy",
		Message:       "Stopping the proxy will prevent POS from printing receipts.\n\nAre you sure you want to quit?",
		Buttons:       []string{"Cancel", "Quit"},
		DefaultButton: "Cancel",
	})

	if err != nil {
		logger.Errorf("Failed to show quit dialog: %v", err)
		return false
	}

	// linux doesn't use Buttons overrides and uses No | Yes for question dialog
	if result != "Yes" && result != "Quit" {
		return false
	}

	logger.Debug("Confirmed quit action")
	return true
}
