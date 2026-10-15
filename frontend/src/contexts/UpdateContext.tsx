import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useRef,
  useState,
  type ReactNode,
} from "react";
import {
  CancelUpdate,
  CheckForUpdate,
  DownloadUpdate,
  MarkUpdateSeen,
} from "../../wailsjs/go/main/App";
import { update } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime/runtime";
import { AppContext } from "./AppContext";

// Matches backend State constants in internal/update/types.go
export type UpdateState =
  | "idle"
  | "failed"
  | "update_available"
  | "checking"
  | "downloading"
  | "installing";

const MIN_CHECK_DURATION = 600; // ms minimum duration to prevent rapid state flickering

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => setTimeout(resolve, ms));
}

interface ProgressData {
  state?: UpdateState;
  downloaded: number;
  total: number;
  percent?: number;
}

export interface UpdateContextType {
  data: {
    info: update.Info;
    progress: number;
  };
  setters: {
    setInfo: (info: update.Info) => void;
    setBannerDismissed: (dismissed: boolean) => void;
    setStatus: (status: UpdateState) => void;
  };
  actions: {
    checkForUpdate: () => Promise<void>;
    handleUpdate: () => Promise<void>;
    handleCancel: () => Promise<void>;
    handleDismiss: () => Promise<void>;
  };
}

export const UpdateContext = createContext<UpdateContextType>({} as UpdateContextType);

export function useUpdate() {
  return useContext(UpdateContext);
}

export function getFriendlyErrorMessage(err: string): string {
  if (!err) {
    return "We couldn't check for updates. Please check your internet connection and try again.";
  }

  const lower = err.toLowerCase();

  if (lower.includes("rate limit")) {
    return "Too many update requests were made. Please try again later.";
  }

  if (
    lower.includes("offline") ||
    lower.includes("no such host") ||
    lower.includes("connection refused") ||
    lower.includes("network is unreachable") ||
    lower.includes("no route to host")
  ) {
    return "We couldn't connect to the update service. Please check your internet connection and try again.";
  }

  if (lower.includes("timeout") || lower.includes("deadline")) {
    return "The update check took too long. Please try again.";
  }

  if (lower.includes("stalled")) {
    return "The update download was interrupted. Please check your internet connection and try again.";
  }

  if (lower.includes("forbidden") || lower.includes("403")) {
    return "We couldn't access the update service right now. Please try again later.";
  }

  if (lower.includes("no compatible asset") || lower.includes("no asset")) {
    return "No update is available for your device.";
  }

  return "We couldn't check for updates. Please try again later.";
}

interface UpdateContextWrapperProps {
  children: ReactNode;
}

export const UpdateContextWrapper = ({ children }: UpdateContextWrapperProps) => {
  const appContext = useContext(AppContext);
  const version = appContext.data.build.version || "dev";
  const [info, setInfo] = useState<update.Info>({
    state: "checking",
    error: "",
    currentVersion: version,
    latestVersion: version,
    notes: "",
    seen: true,
  });

  const [progress, setProgress] = useState(0);

  const inFlightRef = useRef(false);
  const hasCheckedStartupRef = useRef(false);

  const checkForUpdate = useCallback(async () => {
    if (inFlightRef.current) {
      return;
    }

    inFlightRef.current = true;
    setProgress(0);
    setInfo((prev) => ({ ...prev, state: "checking", error: "" }));

    const startTime = Date.now();

    const finish = async (newInfo: update.Info) => {
      const elapsed = Date.now() - startTime;
      if (elapsed < MIN_CHECK_DURATION) {
        await sleep(MIN_CHECK_DURATION - elapsed);
      }
      setInfo(newInfo);
      inFlightRef.current = false;
    };

    if (typeof navigator !== "undefined" && !navigator.onLine) {
      const offlineMsg = "Cannot reach update server. Please check your internet connection.";
      await finish({ ...info, state: "failed", error: offlineMsg });
      return;
    }

    try {
      const result = await CheckForUpdate();
      if (!result) {
        await finish({ ...info, state: "failed", error: "Failed to check for updates." });
        return;
      }

      await finish(result);
    } catch (err: any) {
      console.error("Failed to check for updates:", err);
      const rawErr = typeof err === "string" ? err : err.message || String(err) || "Could not check for updates.";
      await finish({ ...info, state: "failed", error: getFriendlyErrorMessage(rawErr) });
    }
  }, [info]);

  // Abort stalled downloads immediately when connection drops
  useEffect(() => {
    const handleOffline = () => {
      if (inFlightRef.current) {
        CancelUpdate().catch(() => { });
        inFlightRef.current = false;
        setInfo((prev) => ({ ...prev, state: "failed", error: "Network connection lost during download. Please check your internet connection." }));
      }
    };

    window.addEventListener("offline", handleOffline);
    return () => {
      window.removeEventListener("offline", handleOffline);
    };
  }, []);

  // Listen for backend progress events
  useEffect(() => {
    const unsubscribeProgress = EventsOn("update-progress", (data: ProgressData) => {
      if (!data) return;
      if (data.state) {
        setInfo((prev) => (prev ? { ...prev, state: data.state! } : prev));
      }
      if (typeof data.percent === "number") {
        setProgress(data.percent);
      } else if (data.total > 0) {
        setProgress(Math.round((data.downloaded / data.total) * 100));
      }
    });

    return () => {
      unsubscribeProgress();
    };
  }, []);

  // Automatic startup check (skipped in dev mode)
  const isAppLoading = appContext.data.isLoading;
  const build = appContext.data.build;
  const isDev = build.dev ?? false;
  const appVersion = build.version || "local";

  useEffect(() => {
    if (hasCheckedStartupRef.current || isAppLoading) {
      return;
    }
    hasCheckedStartupRef.current = true;

    if (isDev) {
      setInfo({
        state: "idle",
        error: "",
        currentVersion: appVersion,
        latestVersion: appVersion,
        notes: "",
        seen: true,
      });
      return;
    }

    checkForUpdate();
  }, [isAppLoading, isDev, appVersion, checkForUpdate]);

  const handleUpdate = useCallback(async () => {
    if (inFlightRef.current) {
      return;
    }
    if (typeof navigator !== "undefined" && !navigator.onLine) {
      setInfo((prev) => ({ ...prev, state: "failed", error: "No internet connection available. Please check your network and try again." }));
      return;
    }

    inFlightRef.current = true;
    setProgress(0);
    setInfo((prev) => (prev ? { ...prev, state: "downloading", error: "" } : prev));

    try {
      await DownloadUpdate();
    } catch (err: any) {
      inFlightRef.current = false;
      const rawMsg = typeof err === "string" ? err : err.message || String(err) || "Update failed. Please try again.";
      setInfo((prev) => ({ ...prev, state: "failed", error: getFriendlyErrorMessage(rawMsg) }));
    }
  }, []);

  const handleCancel = useCallback(async () => {
    try {
      await CancelUpdate().catch(() => { });
    } catch (err) {
      console.error("Failed to cancel update:", err);
    }
    inFlightRef.current = false;
    setProgress(0);
    setInfo((prev) =>
      prev ? { ...prev, state: prev.state === "update_available" ? "update_available" : "idle" } : prev
    );
  }, []);

  const handleDismiss = useCallback(async () => {
    if (info.latestVersion) {
      await MarkUpdateSeen(info.latestVersion).catch(() => { });
    }
    setInfo((prev) => (prev ? { ...prev, seen: true } : prev));
  }, [info]);

  const data = { info, progress };

  const setters = {
    setInfo,
    setBannerDismissed: (seen: boolean) => setInfo((prev) => (prev ? { ...prev, seen } : prev)),
    setStatus: (state: UpdateState) => setInfo((prev) => (prev ? { ...prev, state } : prev)),
  };

  const actions = { checkForUpdate, handleUpdate, handleCancel, handleDismiss };

  return (
    <UpdateContext.Provider value={{ data, setters, actions }}>
      {children}
    </UpdateContext.Provider>
  );
};
