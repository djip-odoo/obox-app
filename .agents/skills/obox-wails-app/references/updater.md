# Wails v3 Self-Updater Reference

The self-update subsystem ([internal/update](file:///home/odoo/odoo/epos-proxy/internal/update)) is built on top of the **Wails v3 Updater engine** (`github.com/wailsapp/wails/v3/pkg/updater`) and integrated directly into the `App` service in [app.go](file:///home/odoo/odoo/epos-proxy/app.go).

## Update Lifecycle

```
Wails App Startup (app.go)
  │
  ├── 1. Check For Update
  │     ├── Provider: github.com/wailsapp/wails/v3/pkg/updater/providers/github
  │     ├── Compares buildinfo.Version with GitHub Release SemVer tags
  │     └── Emits `update:status` (Status: "available", release notes, version)
  │
  ├── 2. Download Update (User clicks "Download" in React UI)
  │     ├── Downloads matching platform asset: `obox-app-<os>-<arch>`
  │     ├── Streams progress percentage to UI via `update:status`
  │     └── Stages downloaded binary/installer to temp directory
  │
  └── 3. Apply Update & Restart (User clicks "Restart to update")
        ├── Linux: Replaces current executable or executes .deb
        ├── Windows: Launches NSIS installer with elevated prompt
        └── Calls `app.restart.Store(true)` and restarts application
```

---

## RPC Methods on `App`

Exposed to frontend via `frontend/bindings/obox-app/app.ts`:

- `CheckForUpdate() update.Info`: Checks for newer releases (cached with 30s cooldown).
- `DownloadUpdate() (string, error)`: Streams asset download and emits progress.
- `ApplyUpdate() error`: Performs OS-level file replacement and triggers app restart.
- `CancelUpdate()`: Cancels in-progress download context.
- `MarkUpdateSeen(tag string) error`: Remembers dismissed update notifications.

---

## Event Payload (`update:status`)

The frontend listens for real-time update events via `Events.On("update:status", ...)`:

```typescript
type UpdateStatus = 
  | "checking"
  | "available"
  | "up-to-date"
  | "downloading"
  | "ready"
  | "error";

interface UpdateInfo {
  status: UpdateStatus;
  version: string;
  releaseNotes: string;
  progress?: number; // 0 - 100 during downloading
  error?: string;
}
```
