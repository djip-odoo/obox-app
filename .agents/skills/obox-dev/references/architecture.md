# Obox App Architecture Reference

This document details the internal components and data flow of **Obox App**.

## Data Flow Diagram

```
Odoo POS (Browser / App)
    │
    │ HTTP POST (XML ePOS / raw label)
    ▼
Fiber v3 Webserver (internal/server)
    │
    ├─► XML Parser (internal/escpos)
    │     Converts <epos-print> XML nodes into ESC/POS byte sequence
    │
    ▼
Printer Manager (internal/printer)
    │
    ├─► Async Print Queue (Channel-based worker)
    │
    ▼
USB Device Interface (gousb / libusb)
    │
    ▼
Physical Epson POS Printer
```

## Core Modules

### 1. `main.go` & `app.go`
- **Application Setup**: Initializes logger, loads configuration from the user's config directory, sets up single instance guard (`obox-app-single-instance`), and starts the Wails v3 application.
- **Window Management**: Configures translucent backdrop on macOS, window size boundaries, and intercepts `events.Common.WindowClosing` to minimize to tray instead of quitting.
- **Events & State**: Emits reactive events to the UI:
  - `printer:connected`: emitted on printer status change.
  - `server:status`: server health and active IP.
  - `update:status`: new application releases.

### 2. `internal/server` (Fiber v3 HTTP Proxy)
Listens on the configured port (default: 8069 or dynamic) for Odoo Point of Sale requests:
- `POST /cgi-bin/epos/service.cgi`: Default receipt printing endpoint. Auto-selects the primary connected printer.
- `POST /p/:printerId/cgi-bin/epos/service.cgi`: Printer-specific receipt printing.
- `POST /p/:printerId/pstprnt`: Raw label printing endpoint.
- `GET/POST /api/obox/...`: Pairing and device discovery endpoints for Odoo IoT / Obox flows.

### 3. `internal/escpos` (ESC/POS & XML Engine)
- Parses ePOS-Print XML schemas into standard ESC/POS bytes.
- Supports tags:
  - `<text align="..." font="..." em="..." ul="..." dw="..." dh="...">`: Text formatting
  - `<feed line="...">`: Line feed
  - `<cut>`: Paper cut
  - `<pulse>`: Cash drawer kick pulse
  - `<image align="..." width="..." height="...">`: Base64 raster image conversion
- Generates compliant ePOS XML response packets: `<response success="true" code="" status=""/>`.

### 4. `internal/printer` (USB & Hardware Layer)
- Discovers Epson USB devices by vendor/product IDs and USB printer device class.
- Manages USB endpoints, interface claims, and background polling for disconnect/reconnect events.
- Employs an asynchronous queue to prevent printer buffer overruns and returns `429 Too Many Requests` (`printer.ErrQueueFull`) when saturated.

### 5. `internal/config`
- Persists user preferences to OS standard application config paths:
  - Linux: `~/.config/obox-app/config.json`
  - macOS: `~/Library/Application Support/obox-app/config.json`
  - Windows: `%APPDATA%\obox-app\config.json`
