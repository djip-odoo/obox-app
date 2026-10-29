import { createContext, useCallback, useEffect, useState } from "react";
import * as obox from "@bindings/obox-app/internal/obox/models";
import {
  ConfirmDisconnectOdoo,
  CheckOdooStatus,
} from "@bindings/obox-app/app";
import { Events } from "@wailsio/runtime";

export type OdooContextType = {
  data: {
    status: obox.ConnectionStatus | null;
    isLoading: boolean;
  };
  actions: {
    refreshStatus: () => Promise<void>;
    disconnectOdoo: () => Promise<boolean>;
  };
};

export const OdooContext = createContext({} as OdooContextType);

interface OdooContextWrapperProps {
  children: React.ReactNode;
}

export const OdooContextWrapper = ({ children }: OdooContextWrapperProps) => {
  const [status, setStatus] = useState<obox.ConnectionStatus | null>(null);

  const refreshStatus = useCallback(async () => {
    try {
      const odooStatus = await CheckOdooStatus();
      setStatus(odooStatus);
    } catch (error) {
      console.error("Failed to fetch Odoo status:", error);
    }
  }, []);

  const disconnectOdoo = useCallback(async () => {
    try {
      const confirmed = await ConfirmDisconnectOdoo();
      return confirmed;
    } catch (error) {
      console.error("Failed to disconnect Odoo:", error);
      throw error;
    }
  }, []);

  useEffect(() => {
    const unsubscribe = Events.On("odoo:status_changed", (ev) => {
      setStatus(ev.data);
    });

    // Initial fetch on mount
    refreshStatus();

    return () => {
      if (unsubscribe) {
        unsubscribe();
      }
    };
  }, [refreshStatus]);

  const data = { status, isLoading: status === null };

  const actions = { refreshStatus, disconnectOdoo };

  return (
    <OdooContext.Provider value={{ data, actions }}>
      {children}
    </OdooContext.Provider>
  );
};
