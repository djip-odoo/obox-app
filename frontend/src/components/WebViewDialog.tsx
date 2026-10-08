import { useContext, useEffect, useRef, useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { WebViewContext } from "../contexts/WebViewContext";
import { AppContext } from "../contexts/AppContext";
import { ToastContext } from "../contexts/ToastContext";
import Dialog, { ActionType } from "./Dialog";
import { usePINGate } from "../hooks/usePINGate";
import { useClipboard } from "../hooks/useClipboard";
import { backendService } from "../services/backend";

export function isValidUrl(urlStr: string): boolean {
  const trimmed = urlStr.trim();
  if (!trimmed) {
    return false;
  }
  try {
    const parsed = new URL(trimmed);
    if (parsed.protocol !== "http:" && parsed.protocol !== "https:") {
      return false;
    }
    if (!parsed.hostname) {
      return false;
    }
    return true;
  } catch {
    return false;
  }
}

export default function WebViewDialog() {
  const { data: { isWails, isWindows, isKioskMode } } = useContext(AppContext);
  const toastContext = useContext(ToastContext);
  const { data, actions } = useContext(WebViewContext);
  const gate = usePINGate();
  const cfg = data.config;

  const [url, setUrl] = useState(cfg?.url ?? "");
  const [exitCorners, setExitCorners] = useState<string[]>(() => {
    if (cfg?.exitCorners && cfg.exitCorners.length > 0) {
      return cfg.exitCorners;
    }
    return ["top-right"];
  });
  const [fullscreen, setFullscreen] = useState<boolean>(() => cfg?.fullscreen ?? true);
  const [localError, setLocalError] = useState<string | null>(null);
  const [urlSaveStatus, setUrlSaveStatus] = useState<"idle" | "saving" | "saved">("idle");
  const [cornerSaveStatus, setCornerSaveStatus] = useState<"idle" | "saving" | "saved">("idle");
  const [fullscreenSaveStatus, setFullscreenSaveStatus] = useState<"idle" | "saving" | "saved">("idle");
  const isInputFocused = useRef(false);
  const initialUrlLoaded = useRef(false);
  const [serverUrl, setServerUrl] = useState(window.location.origin + "/");
  const [reloading, setReloading] = useState(false);

  const { copied: copiedUrl, copy: copyServerUrl } = useClipboard({
    successMessage: "Server URL copied to clipboard",
    errorMessage: "Failed to copy URL",
  });

  const isLocalhost =
    typeof window !== "undefined" &&
    (window.location.hostname === "127.0.0.1" ||
      window.location.hostname === "localhost");

  const isUrlValid = isValidUrl(url);

  const isKioskCurrentlyActive = (isWails || isLocalhost)
    ? data.isKioskActive
    : Boolean(cfg?.enabled);

  const canEnable = Boolean(isUrlValid && cfg?.hasPIN);

  /*
   * Keep URL and Exit Corner fields synchronized with configuration.
   */
  useEffect(() => {
    if (cfg?.url !== undefined) {
      if (!initialUrlLoaded.current || (!isInputFocused.current && urlSaveStatus === "idle")) {
        setUrl(cfg.url);
        initialUrlLoaded.current = true;
      }
    }
  }, [cfg?.url]);

  /*
   * 1-second debounce auto-save for POS Web Application URL textfield.
   */
  useEffect(() => {
    if (!initialUrlLoaded.current) {
      if (cfg?.url !== undefined) {
        initialUrlLoaded.current = true;
      }
      return;
    }

    const trimmed = url.trim();

    if (!trimmed) {
      setLocalError("URL cannot be empty.");
      setUrlSaveStatus("idle");
      return;
    }

    if (!isValidUrl(trimmed)) {
      setLocalError(
        "Enter a valid HTTP or HTTPS URL for an Odoo POS Self Order page."
      );
      setUrlSaveStatus("idle");
      return;
    }

    setLocalError(null);

    if (trimmed === cfg?.url) {
      setUrlSaveStatus("idle");
      return;
    }

    const timer = setTimeout(async () => {
      setUrlSaveStatus("saving");
      try {
        await gate(async () => {
          await actions.saveURL(trimmed);
        });
        setUrlSaveStatus("saved");
        setTimeout(() => {
          setUrlSaveStatus("idle");
        }, 2000);
      } catch (err: unknown) {
        setLocalError(String(err) || "Failed to auto-save URL.");
        setUrlSaveStatus("idle");
      }
    }, 1000);

    return () => clearTimeout(timer);
  }, [url, cfg?.url]);

  const remoteCornersKey = (cfg?.exitCorners || []).join(",");
  useEffect(() => {
    if (cfg?.exitCorners && cfg.exitCorners.length > 0 && cornerSaveStatus === "idle") {
      setExitCorners(cfg.exitCorners);
    }
  }, [remoteCornersKey]);

  useEffect(() => {
    if (cfg?.fullscreen !== undefined && fullscreenSaveStatus === "idle") {
      setFullscreen(cfg.fullscreen);
    }
  }, [cfg?.fullscreen]);

  /*
   * Load the local server address used for remote access.
   */
  useEffect(() => {
    const fetchServerInfo = async () => {
      try {
        const info = await backendService.getTroubleshootInfo();
        if (info?.localIp && info?.port) {
          setServerUrl(`http://${info.localIp}:${info.port}/`);
        } else {
          setServerUrl(window.location.origin + "/");
        }
      } catch {
        setServerUrl(window.location.origin + "/");
      }
    };

    fetchServerInfo();
  }, []);

  const toggleCorner = async (id: string) => {
    let next: string[];
    if (exitCorners.includes(id)) {
      if (exitCorners.length <= 1) {
        return; // Keep at least one corner selected
      }
      next = exitCorners.filter((c) => c !== id);
    } else {
      next = [...exitCorners, id];
    }
    setExitCorners(next);

    setCornerSaveStatus("saving");
    try {
      await gate(async () => {
        await actions.saveExitCorners(next);
      });
      setCornerSaveStatus("saved");
      setTimeout(() => {
        setCornerSaveStatus("idle");
      }, 2000);
    } catch (err: unknown) {
      setLocalError(String(err) || "Failed to auto-save exit corners.");
      setCornerSaveStatus("idle");
    }
  };

  const handleToggleFullscreen = async (forcedVal?: boolean) => {
    const nextVal = forcedVal !== undefined ? forcedVal : !fullscreen;
    if (nextVal === fullscreen && forcedVal !== undefined) {
      return;
    }
    setFullscreen(nextVal);
    setFullscreenSaveStatus("saving");
    try {
      await gate(async () => {
        await actions.saveFullscreen(nextVal);
      });
      setFullscreenSaveStatus("saved");
      toastContext.actions.showToast(
        nextVal ? "Fullscreen mode enabled" : "Fullscreen mode disabled (windowed mode)",
        "success"
      );
      setTimeout(() => {
        setFullscreenSaveStatus("idle");
      }, 2000);
    } catch (err: unknown) {
      setFullscreen(!nextVal);
      setLocalError(String(err) || "Failed to update fullscreen mode.");
      setFullscreenSaveStatus("idle");
    }
  };

  const handleOpenKiosk = async () => {
    const targetUrl = url.trim() || cfg?.url;

    if (!targetUrl || !isValidUrl(targetUrl)) {
      setLocalError("Enter a valid kiosk URL first.");
      return;
    }

    await gate(async () => {
      if (url.trim() && url.trim() !== cfg?.url) {
        await actions.saveURL(url.trim());
      }
      await actions.saveExitCorners(exitCorners);
      await actions.saveFullscreen(fullscreen);

      await actions.toggleEnabled(true);

      if (isWails || isLocalhost) {
        await actions.enterKiosk();
      }
    });

    toastContext.actions.showToast(
      "Kiosk launched",
      "success"
    );
  };

  const handleCloseKiosk = async () => {
    await gate(async () => {
      await actions.toggleEnabled(false);

      if (isWails || isLocalhost) {
        await actions.exitKiosk();
      }
    });

    toastContext.actions.showToast(
      "Kiosk stopped",
      "success"
    );
  };

  const handleToggleKiosk = async () => {
    if (isKioskCurrentlyActive) {
      await handleCloseKiosk();
    } else {
      await handleOpenKiosk();
    }
  };

  const handleReload = async () => {
    setReloading(true);

    try {
      const res = await gate(async () => {
        await actions.reloadKiosk();
        return true;
      });
      if (res) {
        toastContext.actions.showToast("Kiosk view reloaded", "success");
      }
    } catch {
      toastContext.actions.showToast("Failed to reload kiosk", "danger");
    } finally {
      setTimeout(() => {
        setReloading(false);
      }, 600);
    }
  };

  const cleanup = () => {
    setLocalError(null);
  };

  const dialogActions = [
    {
      name: "close",
      label: "Close",
      onClick: cleanup,
      variant: "secondary" as ActionType,
    },
    ...(isKioskCurrentlyActive
      ? [
          {
            name: "closeKiosk",
            label: "Stop Kiosk",
            onClick: handleCloseKiosk,
            variant: "danger" as ActionType,
          },
        ]
      : canEnable
      ? [
          {
            name: "open",
            label: fullscreen ? "Launch Fullscreen Kiosk" : "Launch Windowed WebApp",
            onClick: handleOpenKiosk,
            variant: "primary" as ActionType,
          },
        ]
      : []),
  ];

  return (
    <Dialog
      title="Kiosk & Remote Access"
      size="4xl"
      showTitleDivider
      actions={dialogActions}
      onClose={cleanup}
      openButton={
        <button
          type="button"
          className={`
            w-full flex items-center justify-between
            rounded-xl border px-4 py-3
            transition-colors cursor-pointer shadow-2xs
            ${isKioskCurrentlyActive
              ? "border-odoo/40 bg-odoo/5 hover:bg-odoo/10"
              : "border-gray-200 bg-white hover:bg-gray-50"
            }
          `}
        >
          <div className="flex items-center gap-3">
            <div
              className={`
                flex h-9 w-9 items-center justify-center rounded-lg transition-colors
                ${isKioskCurrentlyActive
                  ? "bg-odoo text-white shadow-xs"
                  : "bg-gray-100 text-gray-500"
                }
              `}
            >
              <svg
                className="h-5 w-5"
                fill="none"
                stroke="currentColor"
                viewBox="0 0 24 24"
              >
                <rect
                  x="2"
                  y="3"
                  width="20"
                  height="14"
                  rx="2"
                  strokeWidth="2"
                />
                <line
                  x1="8"
                  y1="21"
                  x2="16"
                  y2="21"
                  strokeWidth="2"
                />
                <line
                  x1="12"
                  y1="17"
                  x2="12"
                  y2="21"
                  strokeWidth="2"
                />
              </svg>
            </div>

            <div className="text-left">
              <div className="text-sm font-semibold text-gray-800">
                Kiosk & Remote Access
              </div>

              <div className="text-xs text-gray-500">
                Configure kiosk screen, display mode, gestures & remote pairing
              </div>
            </div>
          </div>

          <div className="flex items-center gap-2">
            <span
              className={`
                inline-flex items-center gap-1.5 rounded-full px-2.5 py-1
                text-[10px] font-semibold uppercase tracking-wider
                ${isKioskCurrentlyActive
                  ? "bg-green-100 text-green-700 ring-1 ring-green-300"
                  : "bg-gray-100 text-gray-500"
                }
              `}
            >
              <span
                className={`h-1.5 w-1.5 rounded-full ${
                  isKioskCurrentlyActive ? "bg-green-500 animate-pulse" : "bg-gray-400"
                }`}
              />
              {isKioskCurrentlyActive ? "Active" : "Inactive"}
            </span>
          </div>
        </button>
      }
    >
      <div className="flex flex-col gap-5 py-1 text-gray-700">

        {/* ── Top Status & Quick Action Banner ───────────────────────────────── */}
        <div
          className={`
            relative overflow-hidden rounded-2xl border p-4 sm:p-5 transition-all
            ${isKioskCurrentlyActive
              ? "border-green-200/80 bg-gradient-to-br from-green-50/80 via-emerald-50/40 to-white shadow-xs"
              : "border-gray-200 bg-gradient-to-br from-gray-50 via-slate-50/50 to-white"
            }
          `}
        >
          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
            <div className="flex items-start gap-3.5">
              <div
                className={`
                  flex h-11 w-11 shrink-0 items-center justify-center rounded-xl transition-colors
                  ${isKioskCurrentlyActive
                    ? "bg-green-600 text-white shadow-sm ring-4 ring-green-100"
                    : "bg-odoo/10 text-odoo ring-4 ring-purple-50"
                  }
                `}
              >
                {isKioskCurrentlyActive ? (
                  <svg className="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                  </svg>
                ) : (
                  <svg className="h-6 w-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <rect x="2" y="3" width="20" height="14" rx="2" strokeWidth="2" />
                    <line x1="8" y1="21" x2="16" y2="21" strokeWidth="2" />
                    <line x1="12" y1="17" x2="12" y2="21" strokeWidth="2" />
                  </svg>
                )}
              </div>

              <div>
                <div className="flex items-center gap-2">
                  <h3 className="text-base font-semibold text-gray-900">
                    {isKioskCurrentlyActive ? "POS WebApp is Running" : "POS Kiosk is Idle"}
                  </h3>

                  <span
                    className={`
                      inline-flex items-center gap-1 rounded-full px-2 py-0.5 text-[11px] font-semibold
                      ${isKioskCurrentlyActive
                        ? "bg-green-100 text-green-800"
                        : "bg-gray-100 text-gray-600"
                      }
                    `}
                  >
                    <span
                      className={`h-1.5 w-1.5 rounded-full ${
                        isKioskCurrentlyActive ? "bg-green-500 animate-pulse" : "bg-gray-400"
                      }`}
                    />
                    {isKioskCurrentlyActive
                      ? fullscreen
                        ? "Fullscreen Active"
                        : "Windowed Active"
                      : "Standby"}
                  </span>
                </div>

                <p className="mt-1 text-xs text-gray-500 max-w-xl">
                  {isKioskCurrentlyActive
                    ? `Currently displaying your POS page in ${
                        fullscreen ? "borderless fullscreen kiosk mode" : "standard windowed mode"
                      }. Use the 4-tap corner gesture or hotkey to exit.`
                    : "Configure your Odoo self-ordering terminal URL, display mode, gesture corners, and network remote access below."}
                </p>
              </div>
            </div>

            {/* Quick action buttons */}
            <div className="flex items-center gap-2 self-start sm:self-center shrink-0">
              {isKioskCurrentlyActive && (
                <button
                  type="button"
                  onClick={handleReload}
                  disabled={reloading}
                  className="
                    inline-flex items-center gap-1.5 rounded-lg border border-gray-300
                    bg-white px-3 py-2 text-xs font-medium text-gray-700 shadow-2xs
                    hover:bg-gray-50 hover:border-gray-400 cursor-pointer transition
                  "
                >
                  <svg
                    className={`h-3.5 w-3.5 ${reloading ? "animate-spin text-odoo" : "text-gray-500"}`}
                    fill="none"
                    stroke="currentColor"
                    viewBox="0 0 24 24"
                  >
                    <path
                      strokeLinecap="round"
                      strokeLinejoin="round"
                      strokeWidth="2"
                      d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"
                    />
                  </svg>
                  {reloading ? "Reloading..." : "Reload View"}
                </button>
              )}

              <button
                type="button"
                disabled={!isKioskCurrentlyActive && !canEnable}
                onClick={handleToggleKiosk}
                className={`
                  inline-flex items-center gap-2 rounded-lg px-3.5 py-2 text-xs font-semibold shadow-xs transition-all cursor-pointer
                  ${isKioskCurrentlyActive
                    ? "border border-red-200 bg-white text-red-600 hover:bg-red-50 hover:border-red-300"
                    : canEnable
                    ? "bg-odoo text-white hover:bg-odoo-dark"
                    : "bg-gray-100 text-gray-400 cursor-not-allowed border border-gray-200"
                  }
                `}
              >
                {isKioskCurrentlyActive ? (
                  <>
                    <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M6 18L18 6M6 6l12 12" />
                    </svg>
                    Close Kiosk
                  </>
                ) : (
                  <>
                    <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M14.752 11.168l-3.197-2.132A1 1 0 0010 9.87v4.263a1 1 0 001.555.832l3.197-2.132a1 1 0 000-1.664z" />
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                    </svg>
                    Launch Kiosk
                  </>
                )}
              </button>
            </div>
          </div>
        </div>

        {/* ── Two Column Responsive Control Grid ─────────────────────────────── */}
        <div className="grid grid-cols-1 lg:grid-cols-12 gap-5">

          {/* ══════════════════════════════════════════════════════════════════
              LEFT COLUMN: WebApp URL & Display Mode (Col Span 7)
          ══════════════════════════════════════════════════════════════════ */}
          <div className="lg:col-span-7 flex flex-col gap-5">

            {/* ── Section: Web Application Endpoint ── */}
            <div className="rounded-2xl border border-gray-200/90 bg-white p-4 sm:p-5 shadow-2xs">
              <div className="flex items-center justify-between pb-3 border-b border-gray-100 mb-3.5">
                <div className="flex items-center gap-2">
                  <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-odoo/10 text-odoo">
                    <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M21 12a9 9 0 01-9 9m9-9a9 9 0 00-9-9m9 9H3m9 9a9 9 0 01-9-9m9 9c1.657 0 3-4.03 3-9s-1.343-9-3-9m0 18c-1.657 0-3-4.03-3-9s1.343-9 3-9m-9 9a9 9 0 019-9" />
                    </svg>
                  </div>
                  <div>
                    <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700">
                      POS Web Application URL
                    </h4>
                  </div>
                </div>

                {/* Auto-save indicator */}
                <div>
                  {urlSaveStatus === "saving" && (
                    <span className="inline-flex items-center gap-1.5 rounded-full bg-purple-50 px-2 py-0.5 text-[11px] font-medium text-odoo">
                      <svg className="h-3 w-3 animate-spin" viewBox="0 0 24 24" fill="none">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                      </svg>
                      Saving...
                    </span>
                  )}
                  {urlSaveStatus === "saved" && (
                    <span className="inline-flex items-center gap-1 rounded-full bg-green-50 px-2 py-0.5 text-[11px] font-medium text-green-700">
                      <svg className="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                        <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                      </svg>
                      Saved
                    </span>
                  )}
                  {urlSaveStatus === "idle" && isUrlValid && (
                    <span className="inline-flex items-center gap-1 text-[11px] text-gray-400">
                      Auto-saves as you type
                    </span>
                  )}
                </div>
              </div>

              <div>
                <label htmlFor="kiosk-url" className="sr-only">
                  POS Web Application URL
                </label>

                <div className="relative">
                  <div className="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3 text-gray-400">
                    <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                    </svg>
                  </div>

                  <input
                    id="kiosk-url"
                    type="url"
                    value={url}
                    placeholder="https://your-domain.odoo.com/pos-self/204?access_token=..."
                    onFocus={() => {
                      isInputFocused.current = true;
                    }}
                    onBlur={() => {
                      isInputFocused.current = false;
                    }}
                    onChange={(e) => {
                      isInputFocused.current = true;
                      setUrl(e.target.value);
                      setLocalError(null);
                    }}
                    className={`
                      w-full rounded-xl border bg-gray-50/50 py-2.5 pl-9 pr-3 text-xs sm:text-sm font-mono
                      outline-none transition-all placeholder:text-gray-400 placeholder:font-sans
                      ${localError
                        ? "border-red-300 focus:border-red-400 focus:bg-white focus:ring-2 focus:ring-red-100"
                        : isUrlValid
                        ? "border-gray-200 focus:border-odoo focus:bg-white focus:ring-2 focus:ring-odoo/10"
                        : "border-gray-200 focus:border-odoo focus:bg-white"
                      }
                    `}
                  />
                </div>

                {/* Validation and Helper row */}
                <div className="mt-2 flex flex-col sm:flex-row sm:items-center justify-between gap-1 text-[11px]">
                  {localError ? (
                    <span className="flex items-center gap-1 font-medium text-red-600">
                      <svg className="h-3.5 w-3.5 shrink-0" viewBox="0 0 20 20" fill="currentColor">
                        <path fillRule="evenodd" d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z" clipRule="evenodd" />
                      </svg>
                      {localError}
                    </span>
                  ) : isUrlValid ? (
                    <span className="flex items-center gap-1 text-green-700">
                      <svg className="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                        <path fillRule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clipRule="evenodd" />
                      </svg>
                      Valid HTTP/HTTPS POS destination URL
                    </span>
                  ) : (
                    <span className="text-gray-500">
                      Target URL for your Odoo POS Self Order kiosk screen
                    </span>
                  )}
                </div>
              </div>

              {/* Admin PIN warning if missing */}
              {!cfg?.hasPIN && (
                <div className="mt-3 flex items-start gap-2.5 rounded-xl border border-amber-200 bg-amber-50/80 p-3 text-xs text-amber-900">
                  <svg className="mt-0.5 h-4 w-4 shrink-0 text-amber-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 9v3m0 4h.01M10.29 3.86l-8.1 14a2 2 0 001.73 3h16.16a2 2 0 001.73-3l-8.1-14a2 2 0 00-3.42 0z" />
                  </svg>
                  <div>
                    <span className="font-semibold text-amber-950">Admin PIN Configuration Required</span>
                    <p className="mt-0.5 text-[11px] text-amber-800 leading-relaxed">
                      Before launching kiosk mode, set a 4-digit PIN in the App settings. The PIN protects this application from unauthorized exits by customers.
                    </p>
                  </div>
                </div>
              )}
            </div>

            {/* ── Section: Display Mode Selection (Fullscreen vs Windowed) ── */}
            <div className="rounded-2xl border border-gray-200/90 bg-white p-4 sm:p-5 shadow-2xs">
              <div className="flex items-center justify-between pb-3 border-b border-gray-100 mb-3.5">
                <div className="flex items-center gap-2">
                  <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-blue-50 text-blue-600">
                    <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M9.75 17L9 20l-1 1h8l-1-1-.75-3M3 13h18M5 17h14a2 2 0 002-2V5a2 2 0 00-2-2H5a2 2 0 00-2 2v10a2 2 0 002 2z" />
                    </svg>
                  </div>
                  <div>
                    <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700">
                      Display Mode
                    </h4>
                  </div>
                </div>

                {fullscreenSaveStatus === "saving" && (
                  <span className="inline-flex items-center gap-1.5 rounded-full bg-purple-50 px-2 py-0.5 text-[11px] font-medium text-odoo">
                    <svg className="h-3 w-3 animate-spin" viewBox="0 0 24 24" fill="none">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                    </svg>
                    Updating...
                  </span>
                )}
                {fullscreenSaveStatus === "saved" && (
                  <span className="inline-flex items-center gap-1 rounded-full bg-green-50 px-2 py-0.5 text-[11px] font-medium text-green-700">
                    <svg className="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                      <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                    </svg>
                    Saved
                  </span>
                )}
              </div>

              {/* Mode choice cards */}
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {/* Mode Option 1: Fullscreen Kiosk */}
                <button
                  type="button"
                  disabled={!canEnable}
                  onClick={() => handleToggleFullscreen(true)}
                  className={`
                    group relative flex flex-col justify-between rounded-xl border p-3.5 text-left transition-all cursor-pointer
                    ${fullscreen
                      ? "border-odoo bg-odoo/5 ring-2 ring-odoo/20 shadow-xs"
                      : "border-gray-200 bg-white hover:bg-gray-50/80 hover:border-gray-300"
                    }
                    ${!canEnable ? "opacity-50 cursor-not-allowed" : ""}
                  `}
                >
                  <div>
                    <div className="flex items-center justify-between">
                      <div
                        className={`flex h-8 w-8 items-center justify-center rounded-lg ${
                          fullscreen ? "bg-odoo text-white" : "bg-gray-100 text-gray-500 group-hover:text-gray-700"
                        }`}
                      >
                        <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 8V4m0 0h4M4 4l5 5m11-1V4m0 0h-4m4 0l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
                        </svg>
                      </div>

                      <span
                        className={`inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-semibold ${
                          fullscreen ? "bg-odoo text-white" : "bg-gray-100 text-gray-500"
                        }`}
                      >
                        {fullscreen ? "Selected" : "Select"}
                      </span>
                    </div>

                    <div className="mt-2.5">
                      <div className="text-xs font-bold text-gray-900">
                        Fullscreen Kiosk
                      </div>
                      <p className="mt-1 text-[11px] text-gray-500 leading-snug">
                        Locks to full display with no titlebar or OS window frame. Ideal for customer self-order stands.
                      </p>
                    </div>
                  </div>

                  <div className="mt-3 flex items-center gap-1.5 text-[10px] font-medium text-odoo">
                    <span className="h-1.5 w-1.5 rounded-full bg-odoo" />
                    Recommended for POS hardware
                  </div>
                </button>

                {/* Mode Option 2: Standard Window */}
                <button
                  type="button"
                  disabled={!canEnable}
                  onClick={() => handleToggleFullscreen(false)}
                  className={`
                    group relative flex flex-col justify-between rounded-xl border p-3.5 text-left transition-all cursor-pointer
                    ${!fullscreen
                      ? "border-odoo bg-odoo/5 ring-2 ring-odoo/20 shadow-xs"
                      : "border-gray-200 bg-white hover:bg-gray-50/80 hover:border-gray-300"
                    }
                    ${!canEnable ? "opacity-50 cursor-not-allowed" : ""}
                  `}
                >
                  <div>
                    <div className="flex items-center justify-between">
                      <div
                        className={`flex h-8 w-8 items-center justify-center rounded-lg ${
                          !fullscreen ? "bg-odoo text-white" : "bg-gray-100 text-gray-500 group-hover:text-gray-700"
                        }`}
                      >
                        <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                          <rect x="3" y="3" width="18" height="18" rx="2" strokeWidth="2" />
                          <line x1="3" y1="9" x2="21" y2="9" strokeWidth="2" />
                        </svg>
                      </div>

                      <span
                        className={`inline-flex items-center rounded-full px-2 py-0.5 text-[10px] font-semibold ${
                          !fullscreen ? "bg-odoo text-white" : "bg-gray-100 text-gray-500"
                        }`}
                      >
                        {!fullscreen ? "Selected" : "Select"}
                      </span>
                    </div>

                    <div className="mt-2.5">
                      <div className="text-xs font-bold text-gray-900">
                        Windowed Mode
                      </div>
                      <p className="mt-1 text-[11px] text-gray-500 leading-snug">
                        Runs inside a standard application window with title bar, minimize, and move controls.
                      </p>
                    </div>
                  </div>

                  <div className="mt-3 flex items-center gap-1.5 text-[10px] font-medium text-gray-500">
                    <span className="h-1.5 w-1.5 rounded-full bg-gray-400" />
                    Best for cashier PCs & testing
                  </div>
                </button>
              </div>
            </div>
          </div>

          {/* ══════════════════════════════════════════════════════════════════
              RIGHT COLUMN: Gestures, Security & Remote Access (Col Span 5)
          ══════════════════════════════════════════════════════════════════ */}
          <div className="lg:col-span-5 flex flex-col gap-5">

            {/* ── Section: Interactive Exit Gesture & Security ── */}
            <div className="rounded-2xl border border-gray-200/90 bg-white p-4 sm:p-5 shadow-2xs">
              <div className="flex items-center justify-between pb-3 border-b border-gray-100 mb-3.5">
                <div className="flex items-center gap-2">
                  <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-emerald-50 text-emerald-600">
                    <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 11c0 3.517-1.009 6.799-2.753 9.571m-3.44-2.04l.054-.09A13.916 13.916 0 008 11a4 4 0 118 0c0 1.017-.07 2.019-.203 3m-2.118 6.844A21.88 21.88 0 0015.171 17m3.839 1.132c.645-2.266.99-4.659.99-7.132A8 8 0 008 4.07M3 15.364c.64-1.319 1-2.8 1-4.364 0-1.457.39-2.823 1.07-4" />
                    </svg>
                  </div>
                  <div>
                    <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700">
                      Exit Gesture Screen
                    </h4>
                  </div>
                </div>

                <div className="flex items-center gap-1.5">
                  {cornerSaveStatus === "saving" && (
                    <span className="text-[11px] font-medium text-odoo">Saving...</span>
                  )}
                  {cornerSaveStatus === "saved" && (
                    <span className="text-[11px] font-medium text-green-600">Saved</span>
                  )}
                  <span className="rounded-full bg-gray-100 px-2 py-0.5 text-[10px] font-bold text-gray-600">
                    {exitCorners.length} / 4
                  </span>
                </div>
              </div>

              {/* Visual Simulated POS Display with Clickable Corners */}
              <div className="relative mx-auto w-full rounded-xl border border-gray-300 bg-gray-900 p-2 shadow-inner">
                <div className="relative aspect-16/10 w-full overflow-hidden rounded-lg bg-gradient-to-br from-slate-900 via-gray-900 to-black p-2 flex flex-col justify-between border border-gray-800">

                  {/* Corner Buttons overlaying the simulated screen */}
                  <div className="flex justify-between items-center z-10">
                    {/* Top Left */}
                    <button
                      type="button"
                      disabled={!canEnable}
                      onClick={() => toggleCorner("top-left")}
                      title="Toggle Top-Left exit tap corner"
                      className={`
                        flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-bold transition-all cursor-pointer
                        ${exitCorners.includes("top-left")
                          ? "bg-odoo text-white ring-2 ring-odoo/40 shadow-sm"
                          : "bg-gray-800/90 text-gray-400 border border-dashed border-gray-700 hover:text-gray-200"
                        }
                      `}
                    >
                      <span>↖ Top Left</span>
                      {exitCorners.includes("top-left") && (
                        <svg className="h-2.5 w-2.5" viewBox="0 0 20 20" fill="currentColor">
                          <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                        </svg>
                      )}
                    </button>

                    {/* Top Right */}
                    <button
                      type="button"
                      disabled={!canEnable}
                      onClick={() => toggleCorner("top-right")}
                      title="Toggle Top-Right exit tap corner"
                      className={`
                        flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-bold transition-all cursor-pointer
                        ${exitCorners.includes("top-right")
                          ? "bg-odoo text-white ring-2 ring-odoo/40 shadow-sm"
                          : "bg-gray-800/90 text-gray-400 border border-dashed border-gray-700 hover:text-gray-200"
                        }
                      `}
                    >
                      {exitCorners.includes("top-right") && (
                        <svg className="h-2.5 w-2.5" viewBox="0 0 20 20" fill="currentColor">
                          <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                        </svg>
                      )}
                      <span>Top Right ↗</span>
                    </button>
                  </div>

                  {/* Center Screen Mockup Indicator */}
                  <div className="my-auto text-center px-4">
                    <div className="inline-flex h-7 w-7 items-center justify-center rounded-full bg-white/5 text-odoo-light mb-1">
                      <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z" />
                      </svg>
                    </div>
                    <div className="text-[11px] font-medium text-gray-300">
                      Tap Active Corner 4x Quickly
                    </div>
                    <div className="text-[9px] text-gray-500 mt-0.5">
                      Click corners to activate / deactivate
                    </div>
                  </div>

                  {/* Bottom Corners */}
                  <div className="flex justify-between items-center z-10">
                    {/* Bottom Left */}
                    <button
                      type="button"
                      disabled={!canEnable}
                      onClick={() => toggleCorner("bottom-left")}
                      title="Toggle Bottom-Left exit tap corner"
                      className={`
                        flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-bold transition-all cursor-pointer
                        ${exitCorners.includes("bottom-left")
                          ? "bg-odoo text-white ring-2 ring-odoo/40 shadow-sm"
                          : "bg-gray-800/90 text-gray-400 border border-dashed border-gray-700 hover:text-gray-200"
                        }
                      `}
                    >
                      <span>↙ Bottom Left</span>
                      {exitCorners.includes("bottom-left") && (
                        <svg className="h-2.5 w-2.5" viewBox="0 0 20 20" fill="currentColor">
                          <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                        </svg>
                      )}
                    </button>

                    {/* Bottom Right */}
                    <button
                      type="button"
                      disabled={!canEnable}
                      onClick={() => toggleCorner("bottom-right")}
                      title="Toggle Bottom-Right exit tap corner"
                      className={`
                        flex items-center gap-1 rounded-md px-2 py-1 text-[10px] font-bold transition-all cursor-pointer
                        ${exitCorners.includes("bottom-right")
                          ? "bg-odoo text-white ring-2 ring-odoo/40 shadow-sm"
                          : "bg-gray-800/90 text-gray-400 border border-dashed border-gray-700 hover:text-gray-200"
                        }
                      `}
                    >
                      {exitCorners.includes("bottom-right") && (
                        <svg className="h-2.5 w-2.5" viewBox="0 0 20 20" fill="currentColor">
                          <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                        </svg>
                      )}
                      <span>Bottom Right ↘</span>
                    </button>
                  </div>
                </div>
              </div>

              {/* Gesture Instructions & Emergency Shortcut */}
              <div className="mt-3.5 space-y-2">
                <div className="flex items-center gap-2 rounded-xl bg-gray-50 px-3 py-2 text-xs text-gray-600">
                  <span className="font-semibold text-gray-800">Exit Gesture:</span>
                  <span className="text-[11px] text-gray-500">
                    Tap any highlighted corner 4 times within 1 second.
                  </span>
                </div>

                <div className="flex items-center justify-between rounded-xl bg-purple-50/60 px-3 py-2 text-[11px] text-odoo-dark border border-purple-100">
                  <span className="font-medium">Emergency Exit Shortcut:</span>
                  <kbd className="rounded-md border border-purple-200 bg-white px-2 py-0.5 font-mono text-[10px] font-bold shadow-2xs">
                    Ctrl + Alt + S
                  </kbd>
                </div>
              </div>
            </div>

            {/* ── Section: Remote Management Access (LAN Pairing) ── */}
            {(isWails || isLocalhost) && (
              <div className="rounded-2xl border border-gray-200/90 bg-white p-4 sm:p-5 shadow-2xs">
                <div className="flex items-center justify-between pb-3 border-b border-gray-100 mb-3.5">
                  <div className="flex items-center gap-2">
                    <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-indigo-50 text-indigo-600">
                      <svg className="h-4 w-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z" />
                      </svg>
                    </div>
                    <div>
                      <h4 className="text-xs font-bold uppercase tracking-wider text-gray-700">
                        Remote Management
                      </h4>
                    </div>
                  </div>

                  <span className="rounded-full bg-blue-50 px-2 py-0.5 text-[10px] font-semibold text-blue-700">
                    Local Wi-Fi
                  </span>
                </div>

                <div className="flex items-center gap-4">
                  {/* QR Code Card */}
                  <div className="shrink-0 rounded-xl border border-gray-200 bg-white p-2 shadow-xs">
                    <QRCodeSVG value={serverUrl} size={88} level="M" />
                  </div>

                  {/* Remote Instructions & URL */}
                  <div className="min-w-0 flex-1">
                    <div className="text-xs font-bold text-gray-800">
                      Scan to Pair Device
                    </div>
                    <p className="mt-0.5 text-[11px] text-gray-500 leading-snug">
                      Connect any phone, tablet, or PC on this LAN to manage printers & kiosk remotely.
                    </p>

                    <div className="mt-2.5 flex items-center gap-1.5">
                      <input
                        type="text"
                        readOnly
                        value={serverUrl}
                        className="
                          min-w-0 flex-1 rounded-lg border border-gray-200 bg-gray-50 px-2.5 py-1.5
                          text-xs font-mono text-gray-700 outline-none
                        "
                      />
                      <button
                        type="button"
                        onClick={() => copyServerUrl(serverUrl)}
                        title="Copy LAN address"
                        className="
                          inline-flex shrink-0 items-center gap-1 rounded-lg bg-odoo px-2.5 py-1.5
                          text-xs font-medium text-white shadow-2xs hover:bg-odoo-dark cursor-pointer transition
                        "
                      >
                        {copiedUrl ? (
                          <>
                            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M5 13l4 4L19 7" />
                            </svg>
                            <span>Copied</span>
                          </>
                        ) : (
                          <>
                            <svg className="h-3.5 w-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
                            </svg>
                            <span>Copy</span>
                          </>
                        )}
                      </button>
                    </div>
                  </div>
                </div>
              </div>
            )}
          </div>
        </div>

        {/* ── Bottom Security Information Note ───────────────────────────────── */}
        <div className="flex items-start gap-2.5 rounded-xl border border-gray-200/80 bg-gray-50/60 p-3 text-xs text-gray-600">
          <svg className="mt-0.5 h-4 w-4 shrink-0 text-odoo" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
          </svg>
          <p className="text-[11px] leading-relaxed text-gray-500">
            {isWails || isLocalhost
              ? `When ${fullscreen ? "fullscreen " : ""}kiosk mode is running, user navigation is secured. To return to this management console, tap any active corner 4 times quickly or press Ctrl + Alt + S and verify your 4-digit admin PIN.`
              : "This web interface allows remote administrative access across your local network. Kiosk screen launch runs strictly on the host desktop workstation (127.0.0.1)."}
          </p>
        </div>

      </div>
    </Dialog>
  );
}
