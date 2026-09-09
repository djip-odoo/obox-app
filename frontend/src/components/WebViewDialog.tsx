import { useContext, useEffect, useRef, useState } from "react";
import { QRCodeSVG } from "qrcode.react";
import { WebViewContext } from "../contexts/WebViewContext";
import { AppContext } from "../contexts/AppContext";
import { ToastContext } from "../contexts/ToastContext";
import Dialog, { ActionType } from "./Dialog";
import { usePINGate } from "../hooks/usePINGate";
import { useClipboard } from "../hooks/useClipboard";
import { backendService } from "../services/backend";

/**
 * Supported URL formats for Kiosk mode.
 */
export const KIOSK_URL_CONSTRAINTS = [
  {
    type: "odoo-pos-self",
    name: "Odoo Self Order / Kiosk",
    example:
      "https://your-domain.odoo.com/pos-self/204?access_token=4fcea930b2a1479a",
    validate: (url: URL) =>
      /^\/pos-self\/[a-zA-Z0-9_-]+/i.test(url.pathname),
  },
];

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

    return true
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
  const [localError, setLocalError] = useState<string | null>(null);
  const [urlSaveStatus, setUrlSaveStatus] = useState<"idle" | "saving" | "saved">("idle");
  const [cornerSaveStatus, setCornerSaveStatus] = useState<"idle" | "saving" | "saved">("idle");
  const isInputFocused = useRef(false);
  const initialUrlLoaded = useRef(false);
  const [serverUrl, setServerUrl] = useState(
    window.location.origin + "/"
  );
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

      await actions.toggleEnabled(true);

      if (isWails || isLocalhost) {
        await actions.enterKiosk();
      }
    });

    toastContext.actions.showToast(
      "Kiosk opened",
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
      "Kiosk closed",
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

  const getStatusText = () => {
    if (isKioskCurrentlyActive) {
      return "Kiosk mode is currently running.";
    }

    if (!url.trim()) {
      return "Enter a POS web application URL to get started.";
    }

    if (!isUrlValid) {
      return "The URL format is not supported.";
    }

    if (!cfg?.hasPIN) {
      return "Set an admin PIN before enabling kiosk mode.";
    }

    return "Ready to launch kiosk mode.";
  };

  const dialogActions = [
    {
      name: "close",
      label: "Close",
      onClick: cleanup,
      variant: "secondary" as ActionType,
    },
    ...(!isKioskCurrentlyActive && canEnable
      ? [
          {
            name: "open",
            label: "Open Kiosk",
            onClick: handleOpenKiosk,
            variant: "primary" as ActionType,
          },
        ]
      : []),
  ];

  return (
    <Dialog
      title="Kiosk & Remote Access"
      actions={dialogActions}
      onClose={cleanup}
      openButton={
        <button
          type="button"
          className={`
            w-full flex items-center justify-between
            rounded-xl border px-4 py-3
            transition-colors cursor-pointer
            ${isKioskCurrentlyActive
              ? "border-odoo/40 bg-odoo/5 hover:bg-odoo/10"
              : "border-gray-200 bg-white hover:bg-gray-50"
            }
          `}
        >
          <div className="flex items-center gap-3">
            <div
              className={`
                flex h-9 w-9 items-center justify-center rounded-lg
                ${isKioskCurrentlyActive
                  ? "bg-odoo text-white"
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
                Configure the kiosk screen and remote access
              </div>
            </div>
          </div>

          <span
            className={`
              rounded-full px-2.5 py-1
              text-[10px] font-semibold uppercase tracking-wide
              ${isKioskCurrentlyActive
                ? "bg-green-100 text-green-700"
                : "bg-gray-100 text-gray-500"
              }
            `}
          >
            {isKioskCurrentlyActive ? "Active" : "Inactive"}
          </span>
        </button>
      }
    >
      <div className="flex flex-col gap-6 text-gray-700">

        {/* ============================================================
            KIOSK MODE
        ============================================================ */}
        <section>
          <div className="mb-3">
            <div className="flex items-center gap-2">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-odoo/10 text-odoo">
                <svg
                  className="h-4 w-4"
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

              <div>
                <h3 className="text-sm font-semibold text-gray-900">
                  Kiosk Mode
                </h3>

                <p className="text-xs text-gray-500">
                  Display your POS web application fullscreen.
                </p>
              </div>
            </div>
          </div>

          <div
            className={`
              rounded-xl border p-4
              ${isKioskCurrentlyActive
                ? "border-green-200 bg-green-50/50"
                : "border-gray-200 bg-gray-50/50"
              }
            `}
          >
            {/* Status */}
            <div className="flex items-center justify-between gap-4">
              <div className="flex items-center gap-2.5">
                <span
                  className={`
                    h-2.5 w-2.5 rounded-full
                    ${isKioskCurrentlyActive
                      ? "bg-green-500 animate-pulse"
                      : "bg-gray-300"
                    }
                  `}
                />

                <div>
                  <div className="text-xs font-semibold text-gray-800">
                    {isKioskCurrentlyActive
                      ? "Kiosk is running"
                      : "Kiosk is not running"}
                  </div>

                  <div className="mt-0.5 text-[11px] text-gray-500">
                    {getStatusText()}
                  </div>
                </div>
              </div>

              {isKioskCurrentlyActive && (
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    onClick={handleReload}
                    title="Reload kiosk"
                    className="
                      inline-flex items-center gap-1.5
                      rounded-lg border border-gray-300
                      bg-white px-2.5 py-1.5
                      text-xs font-medium text-gray-600
                      shadow-xs transition-colors
                      hover:border-gray-400 hover:bg-gray-50
                      cursor-pointer
                    "
                  >
                    <svg
                      className={`h-3.5 w-3.5 ${reloading ? "animate-spin" : ""
                        }`}
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

                    {reloading ? "Reloading" : "Reload"}
                  </button>
                </div>
              )}
            </div>

            {/* URL */}
            <div className="mt-4">
              <div className="flex items-center justify-between mb-1.5">
                <label
                  htmlFor="kiosk-url"
                  className="block text-xs font-medium text-gray-700"
                >
                  POS Web Application URL
                </label>
                {urlSaveStatus === "saving" && (
                  <span className="inline-flex items-center gap-1 text-[11px] font-medium text-odoo">
                    <svg className="h-3 w-3 animate-spin" viewBox="0 0 24 24" fill="none">
                      <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                      <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                    </svg>
                    Saving...
                  </span>
                )}
                {urlSaveStatus === "saved" && (
                  <span className="inline-flex items-center gap-1 text-[11px] font-medium text-green-600">
                    <svg className="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                      <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                    </svg>
                    Saved
                  </span>
                )}
              </div>

              <div className="relative">
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
                    w-full rounded-lg border
                    bg-white px-3 py-2.5
                    text-sm font-mono
                    outline-none transition
                    ${localError
                      ? "border-red-300 focus:border-red-400"
                      : "border-gray-300 focus:border-odoo"
                    }
                  `}
                />
              </div>

              <p className="mt-1.5 text-[11px] text-gray-500">
                Use the URL of your Odoo POS Self Order / Kiosk page.
              </p>
            </div>

            {/* Inline error */}
            {localError && (
              <div className="mt-3 flex items-start gap-2 rounded-lg border border-red-200 bg-red-50 px-3 py-2.5 text-xs text-red-700">
                <svg
                  className="mt-0.5 h-4 w-4 shrink-0"
                  fill="currentColor"
                  viewBox="0 0 20 20"
                >
                  <path
                    fillRule="evenodd"
                    d="M18 10a8 8 0 11-16 0 8 8 0 0116 0zm-7 4a1 1 0 11-2 0 1 1 0 012 0zm-1-9a1 1 0 00-1 1v4a1 1 0 102 0V6a1 1 0 00-1-1z"
                    clipRule="evenodd"
                  />
                </svg>

                <span>{localError}</span>
              </div>
            )}

            {/* PIN warning */}
            {!cfg?.hasPIN && (
              <div className="mt-3 flex items-start gap-2 rounded-lg border border-amber-200 bg-amber-50 px-3 py-2.5 text-xs text-amber-800">
                <svg
                  className="mt-0.5 h-4 w-4 shrink-0"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    d="M12 9v3m0 4h.01M10.29 3.86l-8.1 14a2 2 0 001.73 3h16.16a2 2 0 001.73-3l-8.1-14a2 2 0 00-3.42 0z"
                  />
                </svg>

                <div>
                  <div className="font-medium">
                    Admin PIN required
                  </div>
                  <div className="mt-0.5 text-amber-700">
                    Set a PIN in the App settings before enabling kiosk mode.
                  </div>
                </div>
              </div>
            )}

            {/* Exit Corner Configuration */}
            <div className="mt-4 border-t border-gray-200 pt-3">
              <div className="flex items-center justify-between mb-1.5">
                <label className="block text-xs font-medium text-gray-700">
                  Exit Gesture Corners
                </label>
                <div className="flex items-center gap-2">
                  {cornerSaveStatus === "saving" && (
                    <span className="inline-flex items-center gap-1 text-[11px] font-medium text-odoo">
                      <svg className="h-3 w-3 animate-spin" viewBox="0 0 24 24" fill="none">
                        <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
                        <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8v8H4z" />
                      </svg>
                      Saving...
                    </span>
                  )}
                  {cornerSaveStatus === "saved" && (
                    <span className="inline-flex items-center gap-1 text-[11px] font-medium text-green-600">
                      <svg className="h-3.5 w-3.5" viewBox="0 0 20 20" fill="currentColor">
                        <path fillRule="evenodd" d="M16.707 5.293a1 1 0 010 1.414l-8 8a1 1 0 01-1.414 0l-4-4a1 1 0 011.414-1.414L8 12.586l7.293-7.293a1 1 0 011.414 0z" clipRule="evenodd" />
                      </svg>
                      Saved
                    </span>
                  )}
                  <span className="text-[11px] text-gray-400">
                    {exitCorners.length} selected
                  </span>
                </div>
              </div>

              <div className="grid grid-cols-2 gap-2 sm:grid-cols-4">
                {[
                  { id: "top-left", label: "Top Left" },
                  { id: "top-right", label: "Top Right" },
                  { id: "bottom-left", label: "Bottom Left" },
                  { id: "bottom-right", label: "Bottom Right" },
                ].map((item) => {
                  const isSelected = exitCorners.includes(item.id);
                  return (
                    <button
                      key={item.id}
                      type="button"
                      disabled={!canEnable}
                      onClick={() => toggleCorner(item.id)}
                      className={`
                        flex items-center justify-center gap-1.5 rounded-lg border px-2.5 py-2 text-xs font-medium transition-all cursor-pointer text-center
                        ${isSelected
                          ? "border-odoo bg-odoo/10 text-odoo font-semibold ring-1 ring-odoo shadow-xs"
                          : "border-gray-200 bg-white text-gray-700 hover:bg-gray-50 hover:border-gray-300"
                        }
                      `}
                    >
                      <span>{item.label}</span>
                    </button>
                  );
                })}
              </div>

              <p className="mt-1.5 text-[11px] text-gray-500">
                Tap any selected corner 4 times quickly to exit fullscreen kiosk and prompt for your admin PIN.
              </p>
            </div>

            {/* Toggle */}
            <div className="mt-4 flex items-center justify-between border-t border-gray-200 pt-3">
              <div>
                <div className="text-xs font-medium text-gray-800">
                  Open WebApp
                </div>

                <div className="text-[11px] text-gray-500">
                  Launch the configured page in fullscreen.
                </div>
              </div>

              <button
                type="button"
                disabled={!canEnable}
                onClick={handleToggleKiosk}
                aria-label={
                  isKioskCurrentlyActive
                    ? "Close WebApp"
                    : "Open WebApp"
                }
                className={`
                  relative h-6 w-11 shrink-0 rounded-full
                  transition-colors
                  ${isKioskCurrentlyActive
                    ? "bg-odoo"
                    : "bg-gray-300"
                  }
                  ${canEnable
                    ? "cursor-pointer"
                    : "cursor-not-allowed opacity-40"
                  }
                `}
              >
                <span
                  className={`
                    absolute top-1 h-4 w-4
                    rounded-full bg-white shadow-sm
                    transition-transform
                    ${isKioskCurrentlyActive
                      ? "left-6"
                      : "left-1"
                    }
                  `}
                />
              </button>
            </div>
          </div>
        </section>

        {/* ============================================================
            REMOTE ACCESS
        ============================================================ */}
        {(isWails || isLocalhost) && <section>
          <div className="mb-3">
            <div className="flex items-center gap-2">
              <div className="flex h-8 w-8 items-center justify-center rounded-lg bg-blue-50 text-blue-600">
                <svg
                  className="h-4 w-4"
                  fill="none"
                  stroke="currentColor"
                  viewBox="0 0 24 24"
                >
                  <path
                    strokeLinecap="round"
                    strokeLinejoin="round"
                    strokeWidth="2"
                    d="M12 18h.01M8 21h8a2 2 0 002-2V5a2 2 0 00-2-2H8a2 2 0 00-2 2v14a2 2 0 002 2z"
                  />
                </svg>
              </div>

              <div>
                <h3 className="text-sm font-semibold text-gray-900">
                  Remote Access
                </h3>

                <p className="text-xs text-gray-500">
                  Open ePOS Proxy from another device on the same network.
                </p>
              </div>
            </div>
          </div>

          <div className="rounded-xl border border-gray-200 bg-gray-50/50 p-4">
            <div className="flex items-center gap-5">
              {/* QR */}
              <div className="shrink-0 rounded-xl border border-gray-200 bg-white p-2.5 shadow-xs">
                <QRCodeSVG
                  value={serverUrl}
                  size={116}
                  level="M"
                />
              </div>

              {/* URL */}
              <div className="min-w-0 flex-1">
                <div className="text-xs font-semibold text-gray-800">
                  Scan to connect
                </div>

                <p className="mt-1 text-[11px] leading-relaxed text-gray-500">
                  Scan this QR code with a phone, tablet, or another
                  computer connected to the same network.
                </p>

              </div>
            </div>
            <div className="mt-3 flex gap-2">
              <input
                type="text"
                readOnly
                value={serverUrl}
                className="
                      min-w-0 flex-1 rounded-lg
                      border border-gray-300
                      bg-white px-2.5 py-2
                      text-xs font-mono text-gray-700
                      outline-none
                    "
              />

              <button
                type="button"
                onClick={() => copyServerUrl(serverUrl)}
                className="
                      inline-flex shrink-0 items-center gap-1.5
                      rounded-lg bg-odoo
                      px-3 py-2
                      text-xs font-medium text-white
                      shadow-xs transition-colors
                      hover:bg-odoo-dark
                      cursor-pointer
                    "
              >
                {copiedUrl ? (
                  <>
                    <svg
                      className="h-3.5 w-3.5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth="2"
                        d="M5 13l4 4L19 7"
                      />
                    </svg>
                  </>
                ) : (
                  <>
                    <svg
                      className="h-3.5 w-3.5"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth="2"
                        d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z"
                      />
                    </svg>
                  </>
                )}
              </button>
            </div>
          </div>
        </section>}

        {/* ============================================================
            SECURITY NOTE
        ============================================================ */}
        <div className="flex items-start gap-2.5 border-t border-gray-200 pt-4">
          <svg
            className="mt-0.5 h-4 w-4 shrink-0 text-red-400"
            fill="none"
            stroke="currentColor"
            viewBox="0 0 24 24"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              strokeWidth="2"
              d="M12 9v3m0 4h.01M12 3a9 9 0 100 18 9 9 0 000-18z"
            />
          </svg>

          <p className="text-[11px] leading-relaxed text-red-500">
            {isWails || isLocalhost
              ? `To exit fullscreen kiosk mode, tap any of the configured corners (${exitCorners.map((c) => c.replace("-", " ")).join(", ")}) 4 times quickly and enter your admin PIN.`
              : "Kiosk fullscreen mode runs on the local application (127.0.0.1). This web interface is used to manage the kiosk and remote access settings."}
          </p>
        </div>
      </div>
    </Dialog>
  );
}
