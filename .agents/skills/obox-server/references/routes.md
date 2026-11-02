# Obox Server Routes & Protocol Reference

Detailed reference for routes, headers, and response formats implemented in `internal/server`.

## Network & CORS Configuration

Odoo POS runs inside a browser (e.g., Chrome or Firefox). When configured with "Use Local Network access", browsers send CORS preflight (`OPTIONS`) requests or cross-origin POST requests to the local IP.

- The Fiber server enables permissive CORS headers to allow requests from Odoo instances (HTTP and HTTPS origins).
- Supported Methods: `GET`, `POST`, `OPTIONS`.
- Supported Headers: `Content-Type`, `SOAPAction`, `X-Requested-With`.

## Error Mapping Reference

| Condition | Endpoint | Response Type | Value |
| :--- | :--- | :--- | :--- |
| **Receipt Success** | `/cgi-bin/epos/service.cgi` | XML | `<response success="true" code="" status=""/>` |
| **Printer Missing** | `/cgi-bin/epos/service.cgi` | XML | `<response success="false" code="EX_BADPORT" status=""/>` |
| **Print Timeout** | `/cgi-bin/epos/service.cgi` | XML | `<response success="false" code="EX_TIMEOUT" status=""/>` |
| **Malformed XML** | `/cgi-bin/epos/service.cgi` | XML | `<response success="false" code="SCHEMA_ERROR" status=""/>` |
| **Label Success** | `/p/:id/pstprnt` | HTTP Status | `200 OK` |
| **Empty Label Data** | `/p/:id/pstprnt` | HTTP Status | `400 Bad Request` |
| **Queue Full** | `/p/:id/pstprnt` | HTTP Status | `429 Too Many Requests` |
| **Hardware Failure** | `/p/:id/pstprnt` | HTTP Status | `500 Internal Server Error` |
