import { useContext } from "react";
import { AppContext } from "../contexts/AppContext";
import { ToastContext } from "../contexts/ToastContext";
import Dialog, { type ActionType } from "./Dialog";
import { DownloadLogs } from "../../wailsjs/go/main/App";
import { EventsEmit } from "../../wailsjs/runtime/runtime";

export default function AboutDialog() {
  const appContext = useContext(AppContext);
  const toastContext = useContext(ToastContext);

  const { version, buildTime, commit, os, isWindows, isMac, isLinux, serverIsRunning } = appContext.data;

  const displayVersion = version || "local";
  const formattedVersion = displayVersion.startsWith("v") ? displayVersion : `v${displayVersion}`;

  const osName = isWindows
    ? "Windows"
    : isMac
      ? "macOS"
      : isLinux
        ? "Linux"
        : os || "Unknown";

  const formattedBuildTime = (() => {
    if (!buildTime || buildTime === "unknown") return null;
    try {
      const d = new Date(buildTime);
      if (isNaN(d.getTime())) return buildTime;
      return d.toLocaleDateString(undefined, {
        year: "numeric",
        month: "short",
        day: "numeric",
        hour: "2-digit",
        minute: "2-digit",
      });
    } catch {
      return buildTime;
    }
  })();

  const handleCheckUpdates = () => {
    EventsEmit("check-for-update-requested");
    toastContext.actions?.showToast("Checking for updates...");
  };

  const handleDownloadLogs = async () => {
    try {
      await DownloadLogs();
    } catch (err) {
      console.error("Failed to download logs:", err);
      toastContext.actions?.showToast("Failed to download logs", "danger");
    }
  };

  return (
    <Dialog
      title="About Obox App"
      showTitleDivider={true}
      actions={[
        {
          name: "ok",
          label: "OK",
          variant: "primary" as ActionType,
        },
      ]}
      openButton={
        <button
          type="button"
          className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full text-xs font-medium text-gray-700 bg-white/95 hover:bg-white border border-gray-200/90 hover:border-odoo/50 shadow-xs hover:shadow-sm hover:text-odoo transition-all duration-150 cursor-pointer select-none group focus:outline-none focus:ring-2 focus:ring-odoo/20"
          title="View About & System Status"
        >
          <span
            className={`w-2 h-2 rounded-full shrink-0 transition-colors ${
              serverIsRunning ? "bg-emerald-500 ring-2 ring-emerald-100" : "bg-red-400 ring-2 ring-red-100"
            }`}
          />
          <span className="text-gray-400 font-mono text-[11px] group-hover:text-odoo/80 transition-colors">
            {formattedVersion}
          </span>
          <svg
            className="w-3.5 h-3.5 text-gray-400 group-hover:text-odoo transition-colors ml-0.5"
            fill="none"
            viewBox="0 0 24 24"
            stroke="currentColor"
            strokeWidth="2"
          >
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
            />
          </svg>
        </button>
      }
    >
      <div className="space-y-4">
        <div className="flex items-center gap-3 pb-1">
          <div className="w-11 h-11 rounded-2xl bg-odoo/10 text-odoo flex items-center justify-center shrink-0 border border-odoo/15 shadow-2xs">
            <svg
              className="w-6 h-6"
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
              strokeWidth="2"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M17 17h2a2 2 0 002-2v-4a2 2 0 00-2-2H5a2 2 0 00-2 2v4a2 2 0 002 2h2m2 4h6a2 2 0 002-2v-4a2 2 0 00-2-2H9a2 2 0 00-2 2v4a2 2 0 002 2zm8-12V5a2 2 0 00-2-2H9a2 2 0 00-2 2v4h10z"
              />
            </svg>
          </div>
          <div className="min-w-0">
            <div className="text-base font-semibold text-gray-900 leading-tight">Obox App</div>
            <div className="text-xs text-gray-500 mt-0.5">Odoo POS Hardware & Printing Service</div>
          </div>
        </div>

        {/* System Details Card */}
        <div className="bg-gray-50/90 rounded-xl p-3.5 border border-gray-200/80 text-xs space-y-2.5">
          <div className="flex items-center justify-between">
            <span className="text-gray-500 font-medium">Service Status</span>
            <span
              className={`inline-flex items-center gap-1.5 px-2.5 py-0.5 rounded-full text-xs font-medium ${
                serverIsRunning
                  ? "bg-emerald-50 text-emerald-700 border border-emerald-200/80"
                  : "bg-red-50 text-red-700 border border-red-200/80"
              }`}
            >
              <span
                className={`w-1.5 h-1.5 rounded-full ${
                  serverIsRunning ? "bg-emerald-500 animate-pulse" : "bg-red-500"
                }`}
              />
              {serverIsRunning ? "Active" : "Stopped"}
            </span>
          </div>

          <div className="flex items-center justify-between">
            <span className="text-gray-500 font-medium">Version</span>
            <span className="font-medium text-gray-800 font-mono bg-white px-2 py-0.5 rounded border border-gray-200/60 shadow-2xs">
              {displayVersion}
            </span>
          </div>

          <div className="flex items-center justify-between">
            <span className="text-gray-500 font-medium">Operating System</span>
            <span className="font-medium text-gray-800">{osName}</span>
          </div>

          {formattedBuildTime && (
            <div className="flex items-center justify-between">
              <span className="text-gray-500 font-medium">Build Date</span>
              <span className="font-mono text-gray-700">{formattedBuildTime}</span>
            </div>
          )}

          {commit && commit !== "none" && (
            <div className="flex items-center justify-between">
              <span className="text-gray-500 font-medium">Commit</span>
              <span className="font-mono text-gray-700 bg-white px-1.5 py-0.5 rounded border border-gray-200/60">
                {commit.slice(0, 7)}
              </span>
            </div>
          )}
        </div>

        {/* Action Buttons */}
        <div className="grid grid-cols-2 gap-2 pt-1">
          <button
            type="button"
            onClick={handleCheckUpdates}
            className="inline-flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium text-gray-700 bg-white hover:bg-gray-50 active:bg-gray-100 border border-gray-200 rounded-xl shadow-2xs hover:border-gray-300 transition-colors"
          >
            <svg className="w-3.5 h-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
            </svg>
            Check Updates
          </button>

          <button
            type="button"
            onClick={handleDownloadLogs}
            className="inline-flex items-center justify-center gap-1.5 px-3 py-2 text-xs font-medium text-gray-700 bg-white hover:bg-gray-50 active:bg-gray-100 border border-gray-200 rounded-xl shadow-2xs hover:border-gray-300 transition-colors"
          >
            <svg className="w-3.5 h-3.5 text-gray-500" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth="2" d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-4l-4 4m0 0l-4-4m4 4V4" />
            </svg>
            Download Logs
          </button>
        </div>
      </div>
    </Dialog>
  );
}
