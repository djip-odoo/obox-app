import { useContext } from "react";
import NetworkIpDialog from "./NetworkIpDialog";
import PrinterListItem from "./PrinterListItem";
import { PrinterContext } from "../contexts/PrinterContext";
import { AppContext } from "../contexts/AppContext";
import { ToastContext } from "../contexts/ToastContext";
import { PINContext } from "../contexts/PINContext";
import { WebViewContext } from "../contexts/WebViewContext";
import { backendService } from "../services/backend";
import WebViewDialog from "./WebViewDialog";
// import TroubleshootDialog from "./TroubleshootDialog";

export default function PrinterList() {
  const printerContext = useContext(PrinterContext);
  const { data: { isWindows, isKioskMode } } = useContext(AppContext);
  const toastContext = useContext(ToastContext);
  const { showPINDialog } = useContext(PINContext);
  const { data: webViewData, actions: webViewActions } = useContext(WebViewContext);

  const { printers, fetchError } = printerContext.data;
  const errorMessage = fetchError ?? printers?.errorMsg;

  const isLocalhost =
    typeof window !== "undefined" &&
    (window.location.hostname === "127.0.0.1" ||
      window.location.hostname === "localhost");

  const isWindowsKioskServer =
    Boolean(isWindows) &&
    Boolean(isKioskMode) &&
    isLocalhost;

  const handleQuitServer = async () => {
    const ok = await showPINDialog();
    if (!ok) return;

    try {
      await backendService.quitServer();
      toastContext.actions.showToast("ePOS Proxy server stopped", "success");
    } catch (err: unknown) {
      toastContext.actions.showToast("Failed to stop server: " + String(err), "danger");
    }
  };

  return (
    <>
      <div className="w-full max-w-full sm:max-w-md md:max-w-lg lg:max-w-xl bg-white/95 rounded-2xl shadow-sm border border-gray-200/80 overflow-hidden p-4 sm:p-6">
        {printers && (printers.printers.length > 0 || printers.unavailablePrinters.length > 0) && (
          <div>
            <ul className="divide-y divide-gray-200">
              {printers.printers.map((printer) => (
                <PrinterListItem
                  key={printer.id}
                  printer={printer}
                  isOnline={true}
                />
              ))}
              {printers.unavailablePrinters.map((printer) => (
                <PrinterListItem
                  key={printer.name}
                  printer={printer}
                  isOnline={false}
                />
              ))}
            </ul>
          </div>
        )}

        {!printers ? (
          !fetchError && (
            <div className="py-6 text-center">
              <div className="font-medium text-base sm:text-lg text-gray-700">
                Searching for printers...
              </div>
            </div>
          )
        ) : (
          printers.printers.length === 0 &&
          printers.unavailablePrinters.length === 0 && (
            <div className="py-6 text-center">
              <div className="font-medium text-base sm:text-lg text-gray-700">
                No printers found
              </div>
              <div className="mt-2 text-xs sm:text-sm text-gray-500">
                Make sure your printer is powered on and connected via USB or LAN.
              </div>
            </div>
          )
        )}

        {errorMessage && (
          <div className="text-danger text-xs sm:text-sm mt-3 text-center bg-red-50 border border-red-200 rounded-lg p-2.5">
            Error: {errorMessage}
          </div>
        )}
      </div>

      <div className="mt-4 sm:mt-6 w-full flex flex-col gap-3 max-w-full sm:max-w-md md:max-w-lg lg:max-w-xl">
        <NetworkIpDialog />
        <WebViewDialog />
        {/* <TroubleshootDialog /> */}
        {isWindowsKioskServer && (
          <>
            <button
              type="button"
              onClick={handleQuitServer}
              className="w-full flex items-center justify-between rounded-xl border border-red-200 bg-white px-4 py-3 text-red-600 hover:bg-red-50 transition-colors cursor-pointer shadow-2xs"
            >
              <div className="flex items-center gap-3">
                <div className="flex h-9 w-9 items-center justify-center rounded-lg bg-red-50 text-red-600">
                  <svg
                    className="h-5 w-5"
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth="2"
                      d="M17 16l4-4m0 0l-4-4m4 4H7m6 4v1a3 3 0 01-3 3H6a3 3 0 01-3-3V7a3 3 0 013-3h4a3 3 0 013 3v1"
                    />
                  </svg>
                </div>
                <div className="text-left">
                  <div className="text-sm font-semibold text-gray-800">
                    Stop ePOS Proxy
                  </div>
                  <div className="text-xs text-gray-500">
                    Shut down background server process
                  </div>
                </div>
              </div>
              <span className="rounded-full bg-red-50 border border-red-200 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wide text-red-600">
                Quit
              </span>
            </button>
          </>
        )}
      </div>
    </>
  );
}
