import { useContext } from "react";
import { AppContext } from "../contexts/AppContext";
import Dialog, { type ActionType } from "./Dialog";

export default function AboutDialog() {
  const appContext = useContext(AppContext);
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

  return (
    <Dialog
      title="About ePOS Proxy"
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
          className="text-xs text-gray-400 hover:text-odoo cursor-pointer transition-colors focus:outline-none select-none tracking-wide"
        >
          {formattedVersion}
        </button>
      }
    >
      <div className="space-y-4">
        <div className="flex items-center gap-3 pb-1">
          <div className="w-10 h-10 rounded-xl bg-odoo/10 text-odoo flex items-center justify-center shrink-0">
            <svg
              className="w-5 h-5"
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
          <div>
            <div className="font-semibold text-gray-900 leading-tight">ePOS Proxy</div>
            <div className="text-xs text-gray-500">Odoo POS Hardware Service</div>
          </div>
        </div>

        <p className="text-sm text-gray-600 leading-relaxed">
          ePOS Proxy connects Odoo Point of Sale directly to USB and Network receipt printers.
        </p>

        <div className="bg-gray-50 rounded-xl p-3 border border-gray-200/70 text-xs space-y-2">
          <div className="flex items-center justify-between">
            <span className="text-gray-500">Version</span>
            <span className="font-medium text-gray-800 font-mono">{displayVersion}</span>
          </div>

          {buildTime && buildTime !== "unknown" && (
            <div className="flex items-center justify-between">
              <span className="text-gray-500">Build Time</span>
              <span className="font-medium text-gray-800 font-mono">{buildTime}</span>
            </div>
          )}

          {commit && commit !== "none" && (
            <div className="flex items-center justify-between">
              <span className="text-gray-500">Commit</span>
              <span className="font-medium text-gray-800 font-mono">{commit.slice(0, 7)}</span>
            </div>
          )}

          <div className="flex items-center justify-between">
            <span className="text-gray-500">Platform</span>
            <span className="font-medium text-gray-800">{osName}</span>
          </div>

          <div className="flex items-center justify-between">
            <span className="text-gray-500">Status</span>
            <span className="inline-flex items-center gap-1.5 font-medium text-gray-800">
              <span
                className={`w-2 h-2 rounded-full ${
                  serverIsRunning ? "bg-emerald-500" : "bg-red-500"
                }`}
              />
              {serverIsRunning ? "Running" : "Stopped"}
            </span>
          </div>
        </div>
      </div>
    </Dialog>
  );
}
