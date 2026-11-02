---
name: obox-server
description: >-
  Use this skill when working on the local HTTP server, routing, ePOS print service,
  raw label printing, or Obox pairing endpoints.
---

# Obox Local HTTP Server & Routing

This skill covers the local Fiber v3 web server ([internal/server](file:///home/odoo/odoo/epos-proxy/internal/server)), its request lifecycle, ePOS print routing, raw label handling, and Obox device pairing.

## Server Overview

- **Engine**: Fiber v3 (`github.com/gofiber/fiber/v3`) in [server.go](file:///home/odoo/odoo/epos-proxy/internal/server/server.go).
- **Lifecycle**: Constructed and managed in [app.go](file:///home/odoo/odoo/epos-proxy/app.go). Starts on port `8069` (or dynamic port) and binds to all interfaces to allow local network requests from Odoo Point of Sale.
- **Dependencies**: Injects `printer.Manager` (for dispatching print jobs) and `obox.Manager` (for pairing/device info).

---

## Registered Routes

### 1. ePOS Receipt Printing (`route_epos.go`)

- **`POST /cgi-bin/epos/service.cgi`**:
  Auto-selects the primary connected printer and prints the submitted ePOS XML receipt.
- **`POST /p/:printerId/cgi-bin/epos/service.cgi`**:
  Prints to the specified printer ID.

**Handler Flow**:
```go
func printReceipt(printerMgr *printer.Manager, ctx fiber.Ctx, printerID string) error {
    return ctx.XML(printerMgr.PrintReceipt(printerID, ctx.Body()))
}
```
Returns an ePOS XML response:
- Success: `<response success="true" code="" status=""/>`
- Failure (e.g. printer offline): `<response success="false" code="EX_BADPORT" status=""/>`

### 2. Raw Label Printing (`route_epos.go`)

- **`POST /p/:printerId/pstprnt`**:
  Accepts raw byte payloads (e.g. ZPL/ESC label commands).

**HTTP Status Codes**:
- `200 OK`: Job queued and printed successfully.
- `400 Bad Request`: Empty request body.
- `429 Too Many Requests`: `printer.ErrQueueFull` (async print buffer saturated).
- `500 Internal Server Error`: Hardware write or communication failure.

### 3. Obox Device Pairing (`route_obox.go`)

- **`GET /api/obox/info`**:
  Returns JSON metadata about the Obox instance (App ID, server status, printer list).
- **`POST /api/obox/pair`**:
  Pairs the Obox instance with Odoo IoT / Point of Sale server.

---

## Testing & Verifying Routes

### 1. Run Server Unit Tests
```bash
go test -v ./internal/server/...
```

### 2. Test ePOS Print via cURL
```bash
curl -i -X POST http://localhost:8069/cgi-bin/epos/service.cgi \
  -H "Content-Type: text/xml; charset=utf-8" \
  -d '<epos-print><text>Hello from Odoo POS</text><feed line="2"/><cut/></epos-print>'
```

### 3. Test Raw Label via cURL
```bash
curl -i -X POST http://localhost:8069/p/default/pstprnt \
  -H "Content-Type: application/octet-stream" \
  --data-binary "RAW_LABEL_DATA"
```

See [routes.md](file:///home/odoo/odoo/epos-proxy/.agents/skills/obox-server/references/routes.md) for detailed route schemas and CORS configuration.
