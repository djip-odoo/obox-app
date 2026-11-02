# Test Patterns & Guidelines

## 1. Zero Production Code Pollution (Strict Requirement)

Production code in `internal/`, `app.go`, and `main.go` must remain clean and free from test-specific artifacts.

### ❌ What is FORBIDDEN in Production Code:
- Adding exported fields or methods solely used by tests (e.g. `func (a *App) SetMockWebserverForTest(...)`).
- Adding boolean flags like `IsTesting bool` or `if os.Getenv("GO_TEST") == "1"`.
- Modifying production constructor signatures to accept test mocks unless that parameter is already a documented interface needed for production dependency injection.
- Adding dummy return values or skipping error checks in production files to satisfy test runs.

### ✅ What is REQUIRED:
- All test utilities, mock devices, and fake servers must live in:
  - `*_test.go` files (compiled only during test execution).
  - [internal/testutil](file:///home/odoo/odoo/epos-proxy/internal/testutil) (shared test helpers).
- Use `t.Setenv("HOME", t.TempDir())` to isolate configuration directories without changing config logic.
- Use `testutil.GetFreePort(t)` to allocate real unbound TCP ports for server tests.

---

## 2. Using `internal/testutil` Assertions

Avoid pulling external assertion libraries. Use the existing helpers in [internal/testutil/assert.go](file:///home/odoo/odoo/epos-proxy/internal/testutil/assert.go):

```go
import "obox-app/internal/testutil"

// Errors
testutil.ExpectedNoError(t, err)
testutil.ExpectedErrorContains(t, err, "sub-string")

// Equality & Values
testutil.ExpectedEqual(t, actual, expected)
testutil.ExpectedNotEqual(t, actual, expected)
testutil.ExpectedTrue(t, condition)
testutil.ExpectedFalse(t, condition)
testutil.ExpectedNotNil(t, object)
testutil.ExpectedBytesEqual(t, gotBytes, wantBytes)

// Dynamic Ports
port := testutil.GetFreePort(t)
```

---

## 3. Mocking Hardware & Odoo POS

- **USB & LAN Mock Devices**: Use [internal/testutil/mockdevices.go](file:///home/odoo/odoo/epos-proxy/internal/testutil/mockdevices.go) to simulate Epson printers and raw byte writes.
- **Odoo IoT Server Mock**: Use [internal/testutil/mock_odoo.go](file:///home/odoo/odoo/epos-proxy/internal/testutil/mock_odoo.go) to simulate Odoo cloud pairing and status polling without contacting external networks.
