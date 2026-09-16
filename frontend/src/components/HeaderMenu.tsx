import { useContext, useEffect, useRef, useState } from "react";
import {
  ConfirmQuit,
  DownloadLogs,
  ToggleAutoStart,
  ToggleNetworkPrinting,
} from "../../wailsjs/go/main/App";
import { Quit } from "../../wailsjs/runtime/runtime";
import { ToastContext } from "../contexts/ToastContext";
import { errorText } from "../error";
import {
  BoltIcon,
  DisconnectIcon,
  DownloadIcon,
  SettingsIcon,
  WifiIcon,
} from "../functions/icon";
import { AppContext } from "../contexts/AppContext";

interface ToggleSwitchProps {
  checked: boolean;
  disabled?: boolean;
  onChange: () => void;
}

function ToggleSwitch({ checked, disabled, onChange }: ToggleSwitchProps) {
  return (
    <button
      type="button"
      role="switch"
      aria-checked={checked}
      disabled={disabled}
      onClick={(e) => {
        e.stopPropagation();
        onChange();
      }}
      className={`relative inline-flex h-5 w-9 shrink-0 cursor-pointer rounded-full border-2 border-transparent transition-colors duration-200 ease-in-out focus:outline-none focus:ring-2 focus:ring-odoo focus:ring-offset-1 disabled:opacity-50 disabled:cursor-not-allowed ${checked ? "bg-odoo" : "bg-gray-300"
        }`}
    >
      <span
        aria-hidden="true"
        className={`pointer-events-none inline-block h-4 w-4 transform rounded-full bg-white shadow-sm ring-0 transition duration-200 ease-in-out ${checked ? "translate-x-4" : "translate-x-0"
          }`}
      />
    </button>
  );
}

export default function HeaderMenu() {
  const menuRef = useRef<HTMLDivElement>(null);
  const appContext = useContext(AppContext);
  const toastContext = useContext(ToastContext);

  const [isOpen, setIsOpen] = useState(false);
  const [isLoadingAutostart, setIsLoadingAutostart] = useState(false);
  const [isLoadingNetwork, setIsLoadingNetwork] = useState(false);

  const { autoStart, networkPrintingEnabled } = appContext.data;

  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(event.target as Node)) {
        setIsOpen(false);
      }
    };

    if (isOpen) {
      document.addEventListener("mousedown", handleClickOutside);
    }
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
    };
  }, [isOpen]);

  const toggleAutostart = async () => {
    setIsLoadingAutostart(true);
    try {
      await ToggleAutoStart();
      toastContext.actions.showToast(`Auto start ${autoStart ? "Disabled" : "Enabled"}`, "success");
    } catch (err) {
      toastContext.actions.showToast(`Failed to update auto start: ${errorText(err, "unknown error")}`, "danger");
    } finally {
      setIsLoadingAutostart(false);
    }
  };

  const toggleNetworkPrinting = async () => {
    setIsLoadingNetwork(true);
    try {
      await ToggleNetworkPrinting();
      toastContext.actions.showToast(`Network printing ${networkPrintingEnabled ? "Disabled" : "Enabled"}`, "success");
    } catch (err) {
      toastContext.actions.showToast(`Failed to update network printing: ${errorText(err, "unknown error")}`, "danger");
    } finally {
      setIsLoadingNetwork(false);
    }
  };

  const handleDownloadLogs = async () => {
    setIsOpen(false);
    try {
      await DownloadLogs();
    } catch (err) {
      toastContext.actions.showToast(`Failed to download logs: ${errorText(err, "unknown error")}`, "danger");
    }
  };

  const handleQuit = async () => {
    setIsOpen(false);
    try {
      const confirmed = await ConfirmQuit();
      if (confirmed) {
        Quit();
      }
    } catch (err) {
      console.error("Failed to quit application:", err);
    }
  };

  return (
    <div className="relative" ref={menuRef}>
      <button
        type="button"
        onClick={() => setIsOpen(!isOpen)}
        title="Settings & Configuration"
        aria-label="Settings & Configuration"
        aria-expanded={isOpen}
        className={`p-2 rounded-xl text-gray-500 hover:text-odoo hover:bg-purple-50 active:scale-95 transition-all cursor-pointer flex items-center justify-center border ${isOpen
          ? "bg-purple-50 text-odoo border-purple-200"
          : "border-transparent hover:border-purple-100"
          }`}
      >
        <SettingsIcon className="w-5 h-5" />
      </button>

      {isOpen && (
        <div className="absolute right-0 top-full mt-2 w-72 max-h-[calc(100vh-80px)] overflow-y-auto bg-white text-gray-800 rounded-2xl shadow-2xl border border-gray-100 p-2 z-50">
          <div className="px-3 pt-2 pb-1.5 text-[11px] font-semibold text-gray-400 uppercase tracking-wider">
            Configuration
          </div>

          <div className="space-y-1">
            {/* Auto Start Item */}
            <div
              onClick={toggleAutostart}
              className="flex items-center justify-between gap-3 px-3 py-2.5 rounded-xl hover:bg-gray-50 cursor-pointer transition-colors"
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <div className="w-7 h-7 rounded-lg bg-purple-50 border border-purple-100/80 text-odoo flex items-center justify-center shrink-0">
                  <BoltIcon className="w-3.5 h-3.5" />
                </div>
                <div className="min-w-0">
                  <div className="text-xs font-medium text-gray-800">
                    Auto Start
                  </div>
                  <div className="text-[10px] text-gray-500 truncate">
                    Launch on system boot
                  </div>
                </div>
              </div>
              <ToggleSwitch
                checked={autoStart}
                disabled={isLoadingAutostart}
                onChange={toggleAutostart}
              />
            </div>

            {/* Network Printing Item */}
            <div
              onClick={toggleNetworkPrinting}
              className="flex items-center justify-between gap-3 px-3 py-2.5 rounded-xl hover:bg-gray-50 cursor-pointer transition-colors"
            >
              <div className="flex items-center gap-2.5 min-w-0">
                <div className="w-7 h-7 rounded-lg bg-blue-50 border border-blue-100/80 text-blue-600 flex items-center justify-center shrink-0">
                  <WifiIcon className="w-3.5 h-3.5" />
                </div>
                <div className="min-w-0">
                  <div className="text-xs font-medium text-gray-800">
                    Allow Network Printing
                  </div>
                  <div className="text-[10px] text-gray-500 truncate">
                    Accept LAN print jobs
                  </div>
                </div>
              </div>
              <ToggleSwitch
                checked={networkPrintingEnabled}
                disabled={isLoadingNetwork}
                onChange={toggleNetworkPrinting}
              />
            </div>
          </div>

          <div className="border-t border-gray-100 my-1.5" />

          <div className="space-y-1">
            {/* Download Logs Item */}
            <button
              type="button"
              onClick={handleDownloadLogs}
              className="w-full flex items-center gap-2.5 px-3 py-2 rounded-xl text-xs font-medium text-gray-700 hover:bg-gray-50 hover:text-odoo cursor-pointer transition-colors text-left"
            >
              <div className="w-7 h-7 rounded-lg bg-amber-50 border border-amber-100/80 text-amber-600 flex items-center justify-center shrink-0">
                <DownloadIcon className="w-3.5 h-3.5" />
              </div>
              <div className="min-w-0">
                <div className="text-xs font-medium text-gray-800">
                  Download Logs
                </div>
                <div className="text-[10px] text-gray-500 truncate">
                  Export logs archive
                </div>
              </div>
            </button>

            {/* Quit App Item */}
            <button
              type="button"
              onClick={handleQuit}
              className="w-full flex items-center gap-2.5 px-3 py-2 rounded-xl text-xs font-medium text-rose-600 hover:bg-rose-50 cursor-pointer transition-colors text-left"
            >
              <div className="w-7 h-7 rounded-lg bg-rose-50 border border-rose-100/80 text-rose-600 flex items-center justify-center shrink-0">
                <DisconnectIcon className="w-3.5 h-3.5" />
              </div>
              <div className="min-w-0">
                <div className="text-xs font-medium text-rose-600">
                  Quit Obox App
                </div>
                <div className="text-[10px] text-rose-400 truncate">
                  Stop and Exit App
                </div>
              </div>
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
