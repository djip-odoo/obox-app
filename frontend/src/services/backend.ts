import { detectWails } from "../contexts/AppContext";
import {
  AddLANPrinter,
  AppVariable,
  CheckLANPrinterStatus,
  ConfirmRemoveLANPrinter,
  DisableAutostart,
  EnableAutostart,
  GetTroubleshootInfo,
  GetWebViewConfig,
  IsAutostartEnabled,
  IsNetworkPrintingEnabled,
  Printers,
  SetNetworkPrintingEnabled,
  SetWebViewEnabled,
  SetWebViewExitCorners,
  SetWebViewPIN,
  SetWebViewURL,
  SetWindowFullscreen,
  ValidateWebViewPIN,
  IsPendingPinAuth,
  CompletePinAuth,
  NavigateToWebApp,
  ReturnToWailsApp,
  SetWailsAppURL,
} from "../../wailsjs/go/main/App";
import {
  apiAddLANPrinter,
  apiCashDrawer,
  apiCreatePINSession,
  apiGetAppVariable,
  apiGetLANPrinterStatus,
  apiGetPrinters,
  apiGetTroubleshootInfo,
  apiGetWebViewConfig,
  apiRemoveLANPrinter,
  apiSetWebViewEnabled,
  apiSetWebViewExitCorners,
  apiSetWebViewURL,
  apiTestPrint,
  ApiAppVariable,
  ApiPrinter,
  ApiPrintersResponse,
  ApiTroubleshootInfo,
  ApiWebViewConfig,
  apiReloadKiosk,
  apiQuitApp,
} from "../api/client";
import { executePrint } from "../functions/executePrint";
import { main } from "../../wailsjs/go/models";

export interface IBackendService {
  readonly isWails: boolean;

  // App & System Info
  getAppVariable(): Promise<main.AppVariable | ApiAppVariable>;
  getTroubleshootInfo(): Promise<main.TroubleshootInfo | ApiTroubleshootInfo>;
  getNetworkPrintingEnabled(): Promise<boolean>;
  setNetworkPrintingEnabled(v: boolean): Promise<void>;
  getAutostart(): Promise<boolean>;
  setAutostart(v: boolean): Promise<void>;

  // Printers
  getPrinters(): Promise<main.Printers | ApiPrintersResponse>;
  checkLANPrinterStatus(ip: string): Promise<{ online: boolean }>;
  addLANPrinter(ip: string): Promise<void>;
  removeLANPrinter(ip: string): Promise<boolean>;
  testPrint(printer: main.Printer | ApiPrinter): Promise<void>;
  openCashDrawer(printer: main.Printer | ApiPrinter): Promise<void>;

  // Kiosk & Webview
  getWebViewConfig(): Promise<main.WebViewConfig | ApiWebViewConfig>;
  setWebViewURL(url: string): Promise<void>;
  setWebViewEnabled(enabled: boolean): Promise<void>;
  setWebViewExitCorners(corners: string[]): Promise<void>;
  setWebViewPIN(pin: string): Promise<void>;
  validatePIN(pin: string): Promise<boolean>;
  setWindowFullscreen(fullscreen: boolean): Promise<void>;
  reloadKiosk(): Promise<void>;
  quitServer(): Promise<void>;
  isPendingPinAuth(): Promise<boolean>;
  completePinAuth(success: boolean): Promise<void>;
  navigateToWebApp(): Promise<void>;
  returnToWailsApp(): Promise<void>;
  setWailsAppURL(url: string): Promise<void>;
}

class WailsBackendService implements IBackendService {
  readonly isWails = true;

  getAppVariable(): Promise<main.AppVariable> {
    return AppVariable();
  }

  getTroubleshootInfo(): Promise<main.TroubleshootInfo> {
    return GetTroubleshootInfo();
  }

  getNetworkPrintingEnabled(): Promise<boolean> {
    return IsNetworkPrintingEnabled();
  }

  setNetworkPrintingEnabled(v: boolean): Promise<void> {
    return SetNetworkPrintingEnabled(v);
  }

  getAutostart(): Promise<boolean> {
    return IsAutostartEnabled();
  }

  async setAutostart(v: boolean): Promise<void> {
    if (v) {
      await EnableAutostart();
    } else {
      await DisableAutostart();
    }
  }

  getPrinters(): Promise<main.Printers> {
    return Printers();
  }

  async checkLANPrinterStatus(ip: string): Promise<{ online: boolean }> {
    const online = await CheckLANPrinterStatus(ip);
    return { online: Boolean(online) };
  }

  addLANPrinter(ip: string): Promise<void> {
    return AddLANPrinter(ip);
  }

  async removeLANPrinter(ip: string): Promise<boolean> {
    const confirmed = await ConfirmRemoveLANPrinter(ip);
    return Boolean(confirmed);
  }

  async testPrint(printer: main.Printer | ApiPrinter): Promise<void> {
    await executePrint(printer as main.Printer);
  }

  async openCashDrawer(printer: main.Printer | ApiPrinter): Promise<void> {
    await executePrint(printer as main.Printer, true);
  }

  getWebViewConfig(): Promise<main.WebViewConfig> {
    return GetWebViewConfig();
  }

  setWebViewURL(url: string): Promise<void> {
    return SetWebViewURL(url);
  }

  setWebViewEnabled(enabled: boolean): Promise<void> {
    return SetWebViewEnabled(enabled);
  }

  setWebViewExitCorners(corners: string[]): Promise<void> {
    return SetWebViewExitCorners(corners);
  }

  setWebViewPIN(pin: string): Promise<void> {
    return SetWebViewPIN(pin);
  }

  validatePIN(pin: string): Promise<boolean> {
    return ValidateWebViewPIN(pin);
  }

  setWindowFullscreen(fullscreen: boolean): Promise<void> {
    return SetWindowFullscreen(fullscreen);
  }

  reloadKiosk(): Promise<void> {
    const wailsApp = (window as unknown as { go?: { main?: { App?: { ReloadKiosk?: () => Promise<void> } } } })?.go?.main?.App;
    if (wailsApp?.ReloadKiosk) {
      return wailsApp.ReloadKiosk();
    }
    return Promise.resolve();
  }

  quitServer(): Promise<void> {
    const wailsApp = (window as unknown as { go?: { main?: { App?: { Quit?: () => Promise<void> } } } })?.go?.main?.App;
    if (wailsApp?.Quit) {
      return wailsApp.Quit();
    }
    return Promise.resolve();
  }

  isPendingPinAuth(): Promise<boolean> {
    return IsPendingPinAuth();
  }

  completePinAuth(success: boolean): Promise<void> {
    return CompletePinAuth(success);
  }

  navigateToWebApp(): Promise<void> {
    return NavigateToWebApp();
  }

  returnToWailsApp(): Promise<void> {
    return ReturnToWailsApp();
  }

  setWailsAppURL(url: string): Promise<void> {
    return SetWailsAppURL(url);
  }
}

class RemoteBackendService implements IBackendService {
  readonly isWails = false;

  getAppVariable(): Promise<ApiAppVariable> {
    return apiGetAppVariable();
  }

  getTroubleshootInfo(): Promise<ApiTroubleshootInfo> {
    return apiGetTroubleshootInfo();
  }

  getNetworkPrintingEnabled(): Promise<boolean> {
    return Promise.resolve(false);
  }

  setNetworkPrintingEnabled(): Promise<void> {
    return Promise.resolve();
  }

  getAutostart(): Promise<boolean> {
    return Promise.resolve(false);
  }

  setAutostart(): Promise<void> {
    return Promise.resolve();
  }

  getPrinters(): Promise<ApiPrintersResponse> {
    return apiGetPrinters();
  }

  checkLANPrinterStatus(ip: string): Promise<{ online: boolean }> {
    return apiGetLANPrinterStatus(ip);
  }

  async addLANPrinter(ip: string): Promise<void> {
    await apiAddLANPrinter(ip);
  }

  async removeLANPrinter(ip: string): Promise<boolean> {
    await apiRemoveLANPrinter(ip);
    return true;
  }

  async testPrint(printer: main.Printer | ApiPrinter): Promise<void> {
    await apiTestPrint(printer.id);
  }

  async openCashDrawer(printer: main.Printer | ApiPrinter): Promise<void> {
    await apiCashDrawer(printer.id);
  }

  getWebViewConfig(): Promise<ApiWebViewConfig> {
    return apiGetWebViewConfig();
  }

  async setWebViewURL(url: string): Promise<void> {
    await apiSetWebViewURL(url);
  }

  async setWebViewEnabled(enabled: boolean): Promise<void> {
    await apiSetWebViewEnabled(enabled);
  }

  async setWebViewExitCorners(corners: string[]): Promise<void> {
    await apiSetWebViewExitCorners(corners);
  }

  setWebViewPIN(): Promise<void> {
    return Promise.reject(new Error("PIN configuration is only permitted on the desktop application"));
  }

  validatePIN(pin: string): Promise<boolean> {
    return apiCreatePINSession(pin);
  }

  setWindowFullscreen(): Promise<void> {
    return Promise.resolve();
  }

  async reloadKiosk(): Promise<void> {
    await apiReloadKiosk();
  }

  async quitServer(): Promise<void> {
    await apiQuitApp();
  }

  isPendingPinAuth(): Promise<boolean> {
    return Promise.resolve(false);
  }

  completePinAuth(): Promise<void> {
    return Promise.resolve();
  }

  navigateToWebApp(): Promise<void> {
    return Promise.resolve();
  }

  returnToWailsApp(): Promise<void> {
    return Promise.resolve();
  }

  setWailsAppURL(): Promise<void> {
    return Promise.resolve();
  }
}

class DynamicBackendService implements IBackendService {
  private wails = new WailsBackendService();
  private remote = new RemoteBackendService();

  private get service(): IBackendService {
    return detectWails() ? this.wails : this.remote;
  }

  get isWails(): boolean {
    return detectWails();
  }

  getAppVariable(): Promise<main.AppVariable | ApiAppVariable> {
    return this.service.getAppVariable();
  }

  getTroubleshootInfo(): Promise<main.TroubleshootInfo | ApiTroubleshootInfo> {
    return this.service.getTroubleshootInfo();
  }

  getNetworkPrintingEnabled(): Promise<boolean> {
    return this.service.getNetworkPrintingEnabled();
  }

  setNetworkPrintingEnabled(v: boolean): Promise<void> {
    return this.service.setNetworkPrintingEnabled(v);
  }

  getAutostart(): Promise<boolean> {
    return this.service.getAutostart();
  }

  setAutostart(v: boolean): Promise<void> {
    return this.service.setAutostart(v);
  }

  getPrinters(): Promise<main.Printers | ApiPrintersResponse> {
    return this.service.getPrinters();
  }

  checkLANPrinterStatus(ip: string): Promise<{ online: boolean }> {
    return this.service.checkLANPrinterStatus(ip);
  }

  addLANPrinter(ip: string): Promise<void> {
    return this.service.addLANPrinter(ip);
  }

  removeLANPrinter(ip: string): Promise<boolean> {
    return this.service.removeLANPrinter(ip);
  }

  testPrint(printer: main.Printer | ApiPrinter): Promise<void> {
    return this.service.testPrint(printer);
  }

  openCashDrawer(printer: main.Printer | ApiPrinter): Promise<void> {
    return this.service.openCashDrawer(printer);
  }

  getWebViewConfig(): Promise<main.WebViewConfig | ApiWebViewConfig> {
    return this.service.getWebViewConfig();
  }

  setWebViewURL(url: string): Promise<void> {
    return this.service.setWebViewURL(url);
  }

  setWebViewEnabled(enabled: boolean): Promise<void> {
    return this.service.setWebViewEnabled(enabled);
  }

  setWebViewExitCorners(corners: string[]): Promise<void> {
    return this.service.setWebViewExitCorners(corners);
  }

  setWebViewPIN(pin: string): Promise<void> {
    return this.service.setWebViewPIN(pin);
  }

  validatePIN(pin: string): Promise<boolean> {
    return this.service.validatePIN(pin);
  }

  setWindowFullscreen(fullscreen: boolean): Promise<void> {
    return this.service.setWindowFullscreen(fullscreen);
  }

  reloadKiosk(): Promise<void> {
    return this.service.reloadKiosk();
  }

  quitServer(): Promise<void> {
    return this.service.quitServer();
  }

  isPendingPinAuth(): Promise<boolean> {
    return this.service.isPendingPinAuth();
  }

  completePinAuth(success: boolean): Promise<void> {
    return this.service.completePinAuth(success);
  }

  navigateToWebApp(): Promise<void> {
    return this.service.navigateToWebApp();
  }

  returnToWailsApp(): Promise<void> {
    return this.service.returnToWailsApp();
  }

  setWailsAppURL(url: string): Promise<void> {
    return this.service.setWailsAppURL(url);
  }
}

export const backendService: IBackendService = new DynamicBackendService();
