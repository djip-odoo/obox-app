import { useCallback, useContext, useEffect, useRef, useState } from "react";
import {
  ApplyUpdate,
  CheckForUpdate,
  DownloadUpdate,
  LastSeenUpdate,
  MarkUpdateSeen,
} from "../../wailsjs/go/main/App";
import { update } from "../../wailsjs/go/models";
import { BrowserOpenURL, EventsOn } from "../../wailsjs/runtime/runtime";
import { ToastContext } from "../contexts/ToastContext";
import CloseButton from "./CloseButton";

interface ProgressData {
  downloaded: number;
  total: number;
  percent?: number;
}

export default function UpdateBanner() {
  const toastContext = useContext(ToastContext);
  const [info, setInfo] = useState<update.Info | null>(null);
  const [downloading, setDownloading] = useState(false);
  const [applying, setApplying] = useState(false);
  const [progress, setProgress] = useState(0);
  const [failed, setFailed] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  const inFlightRef = useRef(false);

  const runCheck = useCallback(async (manual: boolean) => {
    if (inFlightRef.current) {
      return;
    }

    try {
      const result = await CheckForUpdate();
      if (!result || !result.checkOk) {
        if (manual) {
          toastContext.actions?.showToast(
            result?.error || "Could not check for updates. Please try again later.",
            "danger"
          );
        }
        return;
      }

      if (!result.available) {
        setInfo(null);
        if (manual) {
          toastContext.actions?.showToast("You're up to date.", "success");
        }
        return;
      }

      // If check is automatic (on startup), respect user's previously dismissed version
      if (!manual) {
        const dismissed = await LastSeenUpdate().catch(() => "");
        if (dismissed && dismissed === result.latestVersion) {
          return;
        }
      }

      setInfo(result);
      setFailed(false);
      setErrorMessage("");
    } catch (err: any) {
      console.error("Failed to check for updates:", err);
      if (manual) {
        toastContext.actions?.showToast("Could not check for updates.", "danger");
      }
    }
  }, [toastContext.actions]);

  useEffect(() => {
    // 1. One initial check after mounting
    runCheck(false);

    // 2. Listen for backend background check notifications
    const unsubscribeAvailable = EventsOn("update-available", (updateInfo: update.Info) => {
      if (updateInfo && updateInfo.available && !inFlightRef.current) {
        setInfo(updateInfo);
      }
    });

    // 3. Listen for download progress
    const unsubscribeProgress = EventsOn("update-progress", (data: ProgressData) => {
      if (!data) return;
      if (typeof data.percent === "number") {
        setProgress(data.percent);
      } else if (data.total > 0) {
        setProgress(Math.round((data.downloaded / data.total) * 100));
      }
    });

    // 4. Listen for manual update check requests from menu or About dialog
    const unsubscribeRequest = EventsOn("check-for-update-requested", () => {
      runCheck(true);
    });

    return () => {
      unsubscribeAvailable();
      unsubscribeProgress();
      unsubscribeRequest();
    };
  }, [runCheck]);

  const handleUpdate = useCallback(async () => {
    if (inFlightRef.current) {
      return;
    }
    inFlightRef.current = true;
    setDownloading(true);
    setFailed(false);
    setErrorMessage("");
    setProgress(0);

    try {
      // Step 1: Download & Verify
      await DownloadUpdate();
      setDownloading(false);
      setApplying(true);

      // Step 2: Apply & Restart
      // ApplyUpdate starts replacement and triggers app shutdown.
      await ApplyUpdate().catch(() => {
        // App termination during apply is expected
      });
    } catch (err: any) {
      inFlightRef.current = false;
      setDownloading(false);
      setApplying(false);
      setFailed(true);
      const msg = err?.message || String(err) || "Update failed. Please try again.";
      setErrorMessage(msg);
      toastContext.actions?.showToast("Update failed", "danger");
    }
  }, [toastContext.actions]);

  const handleDismiss = useCallback(async () => {
    if (info?.latestVersion) {
      await MarkUpdateSeen(info.latestVersion).catch(() => {});
    }
    setInfo(null);
    setFailed(false);
  }, [info]);

  const showBanner = !!info && info.available;

  return (
    <>
      {showBanner && !downloading && !applying && (
        <div className="fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-sm font-semibold text-gray-800">
                Update available: {info.latestVersion}
              </p>
              <p className="text-xs text-gray-500 mt-0.5">
                You are running version {info.currentVersion}.
              </p>
            </div>
            <CloseButton onClick={handleDismiss} />
          </div>

          {info.notes && info.notes.length > 0 && (
            <p className="text-xs text-gray-600 mt-2 whitespace-pre-line line-clamp-3">
              {info.notes}
            </p>
          )}

          {failed ? (
            <div className="mt-3 p-3 bg-red-50 border border-red-200 rounded-lg text-xs space-y-2">
              <p className="font-semibold text-red-700">
                {errorMessage || "Update failed. You can download it directly:"}
              </p>
              {info.downloadUrl && (
                <div className="space-y-2">
                  <a
                    href={info.downloadUrl}
                    onClick={(e) => {
                      e.preventDefault();
                      BrowserOpenURL(info.downloadUrl);
                    }}
                    className="text-odoo font-medium underline break-all block hover:opacity-80"
                    title="Open download link"
                  >
                    {info.downloadUrl}
                  </a>
                  <button
                    type="button"
                    onClick={() => BrowserOpenURL(info.downloadUrl)}
                    className="w-full px-3 py-1.5 rounded-md bg-odoo text-white text-xs font-medium hover:opacity-90 transition-opacity cursor-pointer"
                  >
                    Download Directly
                  </button>
                </div>
              )}
              <div className="flex items-center gap-2 pt-1">
                <button
                  type="button"
                  onClick={handleUpdate}
                  className="flex-1 px-3 py-1 text-center text-xs font-medium text-odoo bg-white border border-odoo/30 rounded-md hover:bg-odoo/5 transition-colors cursor-pointer"
                >
                  Retry Update
                </button>
                <button
                  type="button"
                  onClick={handleDismiss}
                  className="px-3 py-1 text-center text-xs font-medium text-gray-500 hover:text-gray-800 transition-colors cursor-pointer"
                >
                  Later
                </button>
              </div>
            </div>
          ) : (
            <div className="flex items-center gap-2 mt-3">
              <button
                type="button"
                onClick={handleUpdate}
                className="flex-1 px-4 py-2 rounded-lg bg-odoo text-white text-sm font-medium hover:opacity-90 transition-opacity cursor-pointer text-center"
              >
                Download & Install
              </button>
              <button
                type="button"
                onClick={handleDismiss}
                className="px-3 py-2 rounded-lg border border-gray-200 text-gray-600 text-sm font-medium hover:bg-gray-50 transition-colors cursor-pointer"
              >
                Later
              </button>
            </div>
          )}
        </div>
      )}

      {showBanner && downloading && (
        <div className="fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
          <p className="text-sm font-semibold text-gray-800">
            Downloading update {info.latestVersion}…
          </p>
          <div className="mt-3">
            <div className="h-2 rounded-full bg-gray-200 overflow-hidden">
              <div
                className="h-full bg-odoo transition-all duration-200"
                style={{ width: `${Math.max(progress, 2)}%` }}
              />
            </div>
            <p className="text-xs text-gray-500 mt-1 font-mono">{progress}% downloaded</p>
          </div>
        </div>
      )}

      {applying && info && (
        <div className="fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
          <div className="flex items-center gap-3">
            <div className="w-5 h-5 border-2 border-odoo border-t-transparent rounded-full animate-spin shrink-0" />
            <div>
              <p className="text-sm font-semibold text-gray-800">
                Installing update {info.latestVersion}…
              </p>
              <p className="text-xs text-gray-500 mt-0.5">
                The app will close to complete the update.
              </p>
            </div>
          </div>
        </div>
      )}
    </>
  );
}