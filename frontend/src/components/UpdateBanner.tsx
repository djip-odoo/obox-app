import { useCallback, useContext, useEffect, useRef, useState } from "react";
import {
  ApplyUpdate,
  CheckForUpdate,
  DownloadUpdate,
  LastSeenUpdate,
  MarkUpdateSeen,
} from "../../wailsjs/go/main/App";
import { main } from "../../wailsjs/go/models";
import { BrowserOpenURL, EventsOn } from "../../wailsjs/runtime/runtime";
import { ToastContext } from "../contexts/ToastContext";
import CloseButton from "./CloseButton";

interface ProgressData {
  downloaded: number;
  total: number;
}

export default function UpdateBanner() {
  const toastContext = useContext(ToastContext);
  const [info, setInfo] = useState<main.UpdateInfo | null>(null);
  const [downloading, setDownloading] = useState(false);
  const [applying, setApplying] = useState(false);
  const [progress, setProgress] = useState(0);
  const [failed, setFailed] = useState(false);
  const applyingRef = useRef(false);
  const downloadingRef = useRef(false);

  const runCheck = useCallback(async (manual: boolean) => {
    if (applyingRef.current || downloadingRef.current) {
      return;
    }

    try {
      const result = await CheckForUpdate();
      if (!result || !result.checkOk) {
        if (manual) {
          toastContext.actions.showToast("Could not check for updates", "danger");
        }
        return;
      }

      if (!result.available) {
        setInfo(null);
        if (manual) {
          toastContext.actions.showToast("You are up to date", "success");
        }
        return;
      }

      // Auto-shown banners are offered once per release; a manual check always
      // shows the banner so the user can update on their own terms.
      if (!manual) {
        const seen = await LastSeenUpdate();
        if (result.latestVersion === seen) {
          return;
        }
      }

      setInfo(result);
      await MarkUpdateSeen(result.latestVersion).catch(() => {
        // Best-effort: a failure only means the banner may appear again later.
      });
    } catch (err) {
      console.error("Failed to check for updates", err);
      if (manual) {
        toastContext.actions.showToast("Could not check for updates", "danger");
      }
    }
  }, []);

  useEffect(() => {
    // One auto-check per app start.
    runCheck(false);

    const unsubscribe = EventsOn("update-progress", (data: ProgressData) => {
      if (data && data.total > 0) {
        setProgress(Math.round((data.downloaded / data.total) * 100));
      }
    });
    const unsubscribeRequest = EventsOn("check-for-update-requested", () => {
      runCheck(true);
    });

    return () => {
      unsubscribe();
      unsubscribeRequest();
    };
  }, [runCheck]);

  const handleUpdate = useCallback(async () => {
    if (applyingRef.current) {
      return;
    }
    applyingRef.current = true;
    setDownloading(true);
    setFailed(false);
    try {
      await DownloadUpdate();
      setDownloading(false);
      setApplying(true);
      // ApplyUpdate replaces the binary and restarts the app; the call may not
      // resolve because the app quits while it runs.
      await ApplyUpdate().catch(() => {
        // The app is quitting during the apply; treat it as success.
      });
    } catch (err) {
      applyingRef.current = false;
      setDownloading(false);
      setApplying(false);
      setFailed(true);
      console.error("Update failed", err);
      toastContext.actions.showToast("Update failed", "danger");
    }
  }, []);

  const showBanner = !!info && info.available;

  return (
    <>
      {showBanner && !downloading && !applying && (
        <div className="fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <p className="text-sm font-semibold text-gray-800">
                Obox App {info.latestVersion} is available
              </p>
              <p className="text-xs text-gray-500 mt-0.5">
                You are running version {info.currentVersion}.
              </p>
            </div>
            <CloseButton
              onClick={() => {
                setInfo(null);
                setFailed(false);
              }}
            />
          </div>

          {info.notes && info.notes.length > 0 && (
            <p className="text-xs text-gray-600 mt-2 whitespace-pre-line line-clamp-3">
              {info.notes}
            </p>
          )}

          {failed ? (
            <div className="mt-3 p-3 bg-red-50 border border-red-200 rounded-lg text-xs space-y-2">
              <p className="font-semibold text-red-700">
                Update failed. You can download it directly:
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
                    className="w-full px-3 py-1.5 rounded-md bg-odoo text-white text-xs font-medium hover:opacity-90 transition-opacity"
                  >
                    Download Directly
                  </button>
                </div>
              )}
              <button
                type="button"
                onClick={handleUpdate}
                className="w-full mt-1 px-3 py-1 text-center text-gray-600 hover:text-gray-900 text-xs font-medium"
              >
                Retry Automatic Update
              </button>
            </div>
          ) : (
            <button
              type="button"
              onClick={handleUpdate}
              className="mt-3 w-full px-4 py-2 rounded-lg bg-odoo text-white text-sm font-medium hover:opacity-90 transition-opacity"
            >
              Update Now
            </button>
          )}
        </div>
      )}

      {showBanner && downloading && (
        <div className="fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
          <p className="text-sm font-semibold text-gray-800">
            Downloading {info.latestVersion}
          </p>
          <div className="mt-3">
            <div className="h-2 rounded-full bg-gray-200 overflow-hidden">
              <div
                className="h-full bg-odoo transition-all duration-200"
                style={{ width: `${Math.max(progress, 2)}%` }}
              />
            </div>
            <p className="text-xs text-gray-500 mt-1">{progress}% downloaded</p>
          </div>
        </div>
      )}

      {applying && info && (
        <div className="fixed bottom-4 right-4 z-40 w-full max-w-sm p-4 rounded-xl shadow-lg border border-amber-200 bg-white">
          <p className="text-sm font-semibold text-gray-800">
            Installing version {info.latestVersion}
          </p>
          <p className="text-xs text-gray-500 mt-0.5">
            The app will close to complete the update.
          </p>
        </div>
      )}
    </>
  );
}