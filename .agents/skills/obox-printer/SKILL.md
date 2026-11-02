---
name: obox-printer
description: >-
  Use this skill when modifying or debugging the low-level printer subsystem,
  USB device discovery, LAN printer sockets, device ID parsing, or print queues.
---

# Obox Low-Level Printer Subsystem

This skill covers the low-level hardware communication, USB enumeration, LAN socket communication, device ID decoding, and concurrent print queue management inside [internal/printer](file:///home/odoo/odoo/epos-proxy/internal/printer).

## Subsystem Architecture

```
Manager (manager.go)
  ├── Discover / Device Cache (cache.go, discover.go)
  │     ├── USB Enumeration (usb.go / usb_windows.go)
  │     │     └── Device ID Parser (device_id.go, known_printer.go)
  │     └── LAN Verification (lan.go - TCP port 9100)
  │
  └── Printer Instance (printer.go)
        ├── Open / Claim Endpoint (USB Bulk OUT or TCP Socket)
        └── Async Job Queue (Enqueue -> Worker Goroutine -> Write)
```

---

## Hardware Protocols & Device Drivers

### 1. USB Communication
- **Linux & macOS ([usb.go](file:///home/odoo/odoo/epos-proxy/internal/printer/usb.go))**:
  - Uses `google/gousb` (wrapper over `libusb-1.0`).
  - Scans for Epson Vendor ID `0x04B8` and standard USB printer class `0x07`.
  - Claims interface, opens endpoint, and streams bulk data.
- **Windows ([usb_windows.go](file:///home/odoo/odoo/epos-proxy/internal/printer/usb_windows.go))**:
  - Uses WMI and Windows SetupAPI to enumerate installed USB printer devices without requiring custom WinUSB/libusb-win32 filter drivers (preventing conflicts with native Epson Windows drivers).

### 2. Device ID Parsing ([device_id.go](file:///home/odoo/odoo/epos-proxy/internal/printer/device_id.go))
- Reads IEEE 1284 string descriptors returned by the device.
- Parses keys:
  - `MFG` / `MANUFACTURER`: Device manufacturer (e.g., `EPSON`).
  - `MDL` / `MODEL`: Model name (e.g., `TM-T20III`, `TM-T88VI`, `TM-m30`).
  - `CMD`: Command set (e.g., `ESCPOS`).
- Verified against [known_printer.go](file:///home/odoo/odoo/epos-proxy/internal/printer/known_printer.go) to ensure hardware compatibility.

### 3. LAN Network Printers ([lan.go](file:///home/odoo/odoo/epos-proxy/internal/printer/lan.go))
- Communicates via raw TCP socket on standard RAW port `9100`.
- Probes availability via `net.DialTimeout(LANConnectTimeout)` (3 seconds).
- IDs are encoded as hexadecimal strings of the IP address (`EncodeLANPrinterID`).

---

## Print Queue & Concurrency Management

In [printer.go](file:///home/odoo/odoo/epos-proxy/internal/printer/printer.go) and [manager.go](file:///home/odoo/odoo/epos-proxy/internal/printer/manager.go):
- Each physical printer has an isolated asynchronous job channel.
- `WriteAsync(printerId, data)` enqueues the job and returns `(<-chan JobResult, error)`.
- **Buffer Protection**:
  - If jobs exceed queue capacity, `Enqueue` returns `printer.ErrQueueFull`.
  - The HTTP server translates `ErrQueueFull` to `HTTP 429 Too Many Requests`.

---

## Verification & Testing

```bash
# Run all unit tests for the printer subsystem
go test -v ./internal/printer/...

# Test specific USB or LAN components
go test -v ./internal/printer/lan_test.go ./internal/printer/lan.go
go test -v ./internal/printer/device_id_test.go ./internal/printer/device_id.go
```

See [hardware.md](file:///home/odoo/odoo/epos-proxy/.agents/skills/obox-printer/references/hardware.md) for USB vendor/model mappings and debugging steps.
