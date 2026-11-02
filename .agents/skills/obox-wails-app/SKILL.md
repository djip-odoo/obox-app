---
name: obox-wails-app
description: >-
  Use this skill when managing the Wails v3 application lifecycle, Go RPC methods,
  self-updater integration, TypeScript bindings generation, event emission, or frontend React integration.
---

# Obox Wails App, Frontend Bridge & Updater

This skill covers the **application level**: connecting the Go backend service ([app.go](file:///home/odoo/odoo/epos-proxy/app.go), [main.go](file:///home/odoo/odoo/epos-proxy/main.go)), the **Wails v3 self-updater** ([internal/update](file:///home/odoo/odoo/epos-proxy/internal/update)), and the React / TypeScript frontend ([frontend/src](file:///home/odoo/odoo/epos-proxy/frontend/src)).

## Architecture & Communication Flow

```
React Frontend (frontend/src/)
   │
   ├─► Calls RPC functions  ──────────►  frontend/bindings/obox-app/app.ts
   │                                           │ (Wails v3 IPC)
   │                                           ▼
   │                                      App Struct Methods (app.go)
   │                                           │
   │                                           ├─► In-App Updater (internal/update)
   │                                           └─► System Tray & Dialogs
   │
   └─◄ Listens to Events    ◄──────────  a.emit("event:name", payload)
```

---

## 1. Exported RPC Methods on `App`

Any public method on the `*App` struct in [app.go](file:///home/odoo/odoo/epos-proxy/app.go) is automatically exposed to the frontend:

| Go Method | TypeScript Signature | Purpose |
| :--- | :--- | :--- |
| `AppVariable()` | `AppVariable(): Promise<AppVariable>` | Initial state: AppID, server info, system info, build info, settings |
| `Printers()` | `Printers(): Promise<DiscoveryResult>` | Discovers connected USB and configured LAN printers |
| `AddLANPrinter(ip)` | `AddLANPrinter(ip: string): Promise<void>` | Registers and validates a new LAN printer IP |
| `ConfirmRemoveLANPrinter(ip)` | `ConfirmRemoveLANPrinter(ip: string): Promise<boolean>` | Displays confirmation dialog & removes LAN printer |
| `CheckLANPrinterStatus(ip)` | `CheckLANPrinterStatus(ip: string): Promise<boolean>` | Checks live socket reachability on port 9100 |
| `ToggleAutoStart()` | `ToggleAutoStart(): Promise<void>` | Toggles OS system autostart |
| `ToggleNetworkPrinting()` | `ToggleNetworkPrinting(): Promise<void>` | Toggles whether external devices can print over LAN |
| `ToggleDebugMode()` | `ToggleDebugMode(): Promise<void>` | Switches debug logging on or off |
| `DownloadLogs()` | `DownloadLogs(): Promise<void>` | Opens file save dialog and exports log bundle |
| `CheckOdooStatus()` | `CheckOdooStatus(): Promise<ConnectionStatus>` | Verifies connection to Odoo instance |
| `CheckForUpdate()` | `CheckForUpdate(): Promise<Info>` | Queries GitHub releases for newer version |
| `DownloadUpdate()` | `DownloadUpdate(): Promise<string>` | Downloads platform release package with progress tracking |
| `ApplyUpdate()` | `ApplyUpdate(): Promise<void>` | Applies downloaded package and restarts the app |
| `CancelUpdate()` | `CancelUpdate(): Promise<void>` | Aborts in-progress update download |
| `MarkUpdateSeen(tag)` | `MarkUpdateSeen(tag: string): Promise<void>` | Remembers dismissed update notification |

---

## 2. In-App Updater Subsystem

The updater is built into the Wails app using `github.com/wailsapp/wails/v3/pkg/updater`:

1. **Listener**: `app.go` attaches an `u.SetStatusListener(...)` during startup that invokes `app.notifyUpdateStatus(info)`.
2. **Event Broadcast**: Pushes `update:status` events to the React UI when updates are discovered, downloading (with percentage progress), or ready to apply.
3. **Application**: On user confirmation, `ApplyUpdate()` replaces the binary or launches the installer and restarts the application cleanly.

See [updater.md](file:///home/odoo/odoo/epos-proxy/.agents/skills/obox-wails-app/references/updater.md) for full updater lifecycle and status schemas.

---

## 3. Event System (`emit` & Listeners)

The backend pushes real-time status updates using `a.emit(eventName, data...)`:

- **`printer:connected`**: Emitted when a USB printer is connected, disconnected, or status updates.
- **`server:status`**: Emitted when the local Fiber server IP, port, or health changes.
- **`update:status`**: Emitted with `update.Info` during checking, downloading, or ready-to-install phases.

**Frontend Event Consumption**:
```typescript
import { Events } from "@wailsio/runtime";

Events.On("printer:connected", (data) => {
  console.log("Printer status changed:", data);
});

Events.On("update:status", (info) => {
  console.log("Update state:", info);
});
```

---

## 4. Bindings Synchronization Workflow

Whenever you add, remove, or modify an exported method or struct in [app.go](file:///home/odoo/odoo/epos-proxy/app.go):

1. **Regenerate Bindings**:
   ```bash
   task generate:bindings
   ```
   This invokes `wails3 generate bindings -ts` and updates [frontend/bindings](file:///home/odoo/odoo/epos-proxy/frontend/bindings).

2. **Verify Frontend Types**:
   ```bash
   npm --prefix frontend run typecheck
   ```

3. **Check Linter**:
   ```bash
   task lint
   ```

See [bindings.md](file:///home/odoo/odoo/epos-proxy/.agents/skills/obox-wails-app/references/bindings.md) for data struct definitions and Wails v3 service lifecycle notes.
