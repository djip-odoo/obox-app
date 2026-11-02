---
name: obox-dev
description: >-
  Use this skill when developing, testing, linting, running, or modifying the
  Obox App desktop application (Go backend + Wails v3 + React/TypeScript frontend).
---

# Obox App Development Workflow

This skill guides development, code generation, linting, testing, and debugging for **Obox App** (`epos-proxy`), an Epson POS printer proxy for Odoo Point of Sale.

## Quick Reference Commands

All primary developer tasks are automated with [Taskfile.yaml](file:///home/odoo/odoo/epos-proxy/Taskfile.yaml):

| Action | Command | Details |
| :--- | :--- | :--- |
| **Start Dev Mode** | `task dev` | Generates TS bindings and launches Wails v3 dev server on port 5173 with hot reload |
| **Generate Bindings** | `task generate:bindings` | Regenerates TypeScript bindings in `frontend/bindings` from Go services |
| **Run Linter** | `task lint` | Checks Go formatting (`gofmt`), frontend TypeScript types, and ESLint |
| **Fix Lint Issues** | `task lint:fix` | Runs `gofmt -s -w .` and ESLint auto-fix |
| **Run Tests** | `task test` | Builds frontend assets (`frontend/dist`) and runs `go test -v ./...` |
| **Build Frontend** | `task frontend:build` | Installs deps, regenerates bindings, and runs Vite build |
| **Clean Artifacts** | `task clean` | Cleans `build/bin`, `frontend/dist`, coverage, and temporary build files |

---

## Architecture Overview

Obox App is built with **Wails v3**:

1. **Go Backend** (`main.go`, `app.go`, `internal/`):
   - [main.go](file:///home/odoo/odoo/epos-proxy/main.go): Application entry point, window lifecycle, single-instance mutex, and system tray integration.
   - [app.go](file:///home/odoo/odoo/epos-proxy/app.go): Main Wails service (`App`) exposing RPC methods and emitting frontend events.
   - [internal/server](file:///home/odoo/odoo/epos-proxy/internal/server): Fiber v3 HTTP server handling local Odoo ePOS requests (`/cgi-bin/epos/service.cgi`, `/pstprnt`).
   - [internal/printer](file:///home/odoo/odoo/epos-proxy/internal/printer): USB printer discovery, libusb (`gousb`) interface, queue management, and LAN printer support.
   - [internal/escpos](file:///home/odoo/odoo/epos-proxy/internal/escpos): ePOS XML parser and ESC/POS command builder.
   - [internal/config](file:///home/odoo/odoo/epos-proxy/internal/config): Persistent user settings (auto-start, network printing, debug mode).
   - [internal/logger](file:///home/odoo/odoo/epos-proxy/internal/logger): Lumberjack rolling log manager.

2. **Frontend** (`frontend/`):
   - React 18, TypeScript, TailwindCSS v4, Vite.
   - Uses generated bindings in [frontend/bindings](file:///home/odoo/odoo/epos-proxy/frontend/bindings) to invoke Go methods directly.

See [architecture.md](file:///home/odoo/odoo/epos-proxy/.agents/skills/obox-dev/references/architecture.md) for detailed module documentation.

---

## Standard Development Workflows

### 1. Modifying Go Service Methods / Adding RPC Methods

When you add or update methods on the `App` struct in [app.go](file:///home/odoo/odoo/epos-proxy/app.go):
1. Keep the method exported (PascalCase) with appropriate input/return types.
2. Run `task generate:bindings` to re-sync TypeScript declarations.
3. Import the updated methods in the frontend from `frontend/bindings/obox-app/app.ts`.

### 2. Running & Fixing Code Checks

Before submitting changes or committing:
```bash
# Verify formatting, types, and linter rules
task lint

# Automatically resolve formatting or lint warnings
task lint:fix
```

### 3. Running Unit and Integration Tests

The Go binary embeds the frontend bundle (`//go:embed all:frontend/dist` in [main.go](file:///home/odoo/odoo/epos-proxy/main.go)). Running `task test` automatically ensures `frontend:build` is run first.

To run tests manually:
```bash
# Full test suite with frontend build
task test

# Run a specific package test directly (if frontend/dist already exists)
go test -v ./internal/server/...
go test -v ./internal/escpos/...
go test -v ./internal/printer/...
```

---

## Troubleshooting Common Dev Issues

- **Missing `frontend/dist` on `go test` or `go build`**:
  Run `task frontend:build` first to populate the embedded directory.
- **Linux WebKit sandbox errors**:
  Linux environments may require `WEBKIT_DISABLE_SANDBOX_THIS_IS_DANGEROUS=1`, which is set by default in `main.go:init()`. Ensure `libwebkit2gtk-4.1-0` or `libwebkitgtk-6.0-dev` is installed.
- **libusb header errors**:
  Ensure system libusb development headers are installed (`sudo apt install libusb-1.0-0-dev` on Debian/Ubuntu, or `brew install libusb` on macOS).
