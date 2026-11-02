---
name: obox-odoo-connection
description: >-
  Use this skill when working on the Obox-to-Odoo IoT connection, pairing lifecycle,
  JSON-RPC communication, remote action queue handling, or Odoo status reporting.
---

# Obox & Odoo IoT Connection

This skill covers the interface connecting **Obox App** to **Odoo database**, implemented in [internal/obox](file:///home/odoo/odoo/epos-proxy/internal/obox) and [internal/app_actions](file:///home/odoo/odoo/epos-proxy/internal/app_actions).

---

## Architecture Overview

```
Odoo Cloud / Local Server
    │
    ├─► 1. Pairing (POST /api/obox/pair with dbUrl & token)
    │
    ├─► 2. Long Polling / Action Queue (JSON-RPC)
    │     │
    │     ▼
    │   Obox Manager (internal/obox/obox.go)
    │     │
    │     ├─► System Actions (/odoo/health, /odoo/disconnect)
    │     │
    │     ▼
    │   Action Handler (internal/app_actions/obox.go)
    │     ├─► /odoo/discover_devices (Returns printer list)
    │     └─► /p/:id/cgi-bin/epos/service.cgi (Prints receipt)
    │
    └─◄ 3. Action Result Reported back to Odoo (reportActionResult)
```

---

## Key Workflows

### 1. Device Pairing & Credential Management
- Handled via `m.setCredentials(dbURL, token)` and persisted to configuration.
- To check current connection status from Go:
  ```go
  status := app.CheckOdooStatus()
  // returns obox.ConnectionStatus { DBURL, WebsocketStatus, LanStatus }
  ```
- To disconnect Obox from Odoo:
  ```go
  app.ConfirmDisconnectOdoo()
  ```

### 2. Action Queue Worker (`queue.go` & `actions.go`)
- Background worker polls Odoo for pending `QueueAction` records.
- Each action contains:
  - `UUID`: Action identifier.
  - `Payload`: URL, HTTP Method, and data bytes.
- Dispatches matching actions to `app_actions.OboxActionHandler`:
  - **Device Discovery**: Returns connected USB and LAN printers.
  - **Remote Print**: Forwards receipt bytes to the appropriate printer instance and returns the ePOS status XML.
- Reports outcomes using `m.reportActionResult(uuid, result)`.

---

## Testing & Verification

### 1. Unit Tests
```bash
# Test obox queue, RPC, and state machine
go test -v ./internal/obox/...

# Test app action router and printer dispatching
go test -v ./internal/app_actions/...
```

### 2. Mocking Odoo IoT Server
Use the mock implementation in [internal/testutil/mock_odoo.go](file:///home/odoo/odoo/epos-proxy/internal/testutil/mock_odoo.go):
```go
mockServer := testutil.NewMockOdooServer(t)
defer mockServer.Close()

// Sets up mock endpoints for pairing and action queue responses
```

See [protocol.md](file:///home/odoo/odoo/epos-proxy/.agents/skills/obox-odoo-connection/references/protocol.md) for JSON-RPC message schemas and timeout policies.
