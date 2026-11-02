# Obox & Odoo Protocol Reference

Detailed specification of the pairing lifecycle, JSON-RPC schema, and action queue execution between Obox and Odoo.

## 1. Credentials & Pairing Handshake

- **Pairing Trigger**: Odoo IoT settings or setup wizard calls `POST /api/obox/pair` on the local Obox server with payload:
  ```json
  {
    "dbUrl": "https://my-odoo-db.odoo.com",
    "token": "<pairing-token>"
  }
  ```
- **Credential Storage**: Saved securely via `config.Manager.SaveOdooCredentials(dbURL, token)`.
- **Startup Sync**: On app startup, `obox.NewManager` automatically reads existing credentials and starts the action worker goroutine.

---

## 2. JSON-RPC 2.0 Payload Format

All requests to Odoo follow JSON-RPC 2.0:

```json
{
  "jsonrpc": "2.0",
  "method": "call",
  "id": 1,
  "params": { ... }
}
```

### Action Result Reporting
When an action received from the Odoo queue finishes, Obox reports the outcome back:
```go
m.reportActionResult(action.UUID, result)
```
- Success payloads: Map or stringified XML receipt response.
- Error payloads: `{"error": "message"}`.

---

## 3. Supported Queue Action Paths

| Path | Handler Location | Behavior |
| :--- | :--- | :--- |
| `/odoo/health` | `internal/obox/actions.go` | Responds `{"status": "ok"}` and triggers background ping |
| `/odoo/restart` | `internal/obox/actions.go` | Returns error: `Restart is not supported on Obox app` |
| `/odoo/disconnect` | `internal/obox/actions.go` | Clears credentials, stops worker, sets status to `disconnected` |
| `/odoo/discover_devices` | `internal/app_actions/obox.go` | Returns array of discovered printers: `[{"name": "...", "identifier": "...", "type": "printer"}]` |
| `/(?:usb/v1/printer\|p)/:id/cgi-bin/epos/service.cgi` | `internal/app_actions/obox.go` | Extracts printer ID, dispatches print job, and returns ePOS response XML |

---

## 4. Connection States

The manager tracks two independent states:
- **`websocketStatus`**: WebSocket tunnel status (if configured).
- **`lanStatus`**: LAN polling status.

Values:
- `disconnected`: No credentials or user explicitly disconnected.
- `connecting`: In the process of handshaking/polling Odoo.
- `connected`: Active heartbeat received within `lanContactTimeout` (30 seconds).
