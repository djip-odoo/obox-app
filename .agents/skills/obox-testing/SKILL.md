---
name: obox-testing
description: >-
  Use this skill when writing, running, debugging, or maintaining tests for Obox App.
  Strictly enforces that production source code must NEVER be modified or added to just to satisfy a test case.
---

# Obox Testing & Quality Assurance

This skill guides testing workflows, assertions, test isolation, and test runner targets for **Obox App**.

---

> [!CAUTION]
> ### STRICT PROHIBITION: NEVER ADD PRODUCTION SOURCE CODE JUST FOR TEST CASES
> 
> - **Zero Test Pollution in Production**: You must **NEVER** add methods, exported fields, dummy hooks, mock parameters, or test flags (e.g. `isTest`, `mockMode`) to production files (`app.go`, `main.go`, `internal/*`) solely to make a test pass or satisfy a test requirement.
> - **Tests Must Test Real Code**: Tests must exercise actual production code paths as designed. If an interface is needed, it must be part of the natural architecture, not a temporary hack for a single test.
> - **Keep All Test Logic in Test Files**: All mocks, fake implementations, stubs, and assertions **MUST** reside exclusively in `*_test.go` files or inside the [internal/testutil](file:///home/odoo/odoo/epos-proxy/internal/testutil) package.

---

## Testing Commands

All test runs are managed via [Taskfile.yaml](file:///home/odoo/odoo/epos-proxy/Taskfile.yaml):

| Target | Command | Notes |
| :--- | :--- | :--- |
| **Full Test Suite** | `task test` | Automatically builds `frontend:build` first so that `//go:embed all:frontend/dist` succeeds, then executes `go test -v ./...` |
| **Specific Package Tests** | `go test -v ./internal/<package>/...` | Fast test execution for isolated packages |
| **Single Test Method** | `go test -v -run <TestName> ./...` | Runs only tests matching `<TestName>` |
| **Frontend Typecheck** | `npm --prefix frontend run typecheck` | Checks TypeScript compilation with `tsc --noEmit` |
| **Frontend Lint** | `npm --prefix frontend run lint` | Runs ESLint checks |

---

## Writing Tests: Best Practices

### 1. Test Isolation
- **Environment Isolation**: Always isolate configuration files using temporary directories so tests do not read or overwrite user preferences:
  ```go
  t.Setenv("HOME", t.TempDir())
  ```
- **Port Allocation**: Never hardcode port numbers in tests. Use:
  ```go
  port := testutil.GetFreePort(t)
  ```

### 2. Using `internal/testutil` Assertions
Never import third-party assertion libraries. Use the built-in assertions in [internal/testutil/assert.go](file:///home/odoo/odoo/epos-proxy/internal/testutil/assert.go):
- `testutil.ExpectedNoError(t, err)`
- `testutil.ExpectedErrorContains(t, err, "expected error fragment")`
- `testutil.ExpectedEqual(t, got, want)`
- `testutil.ExpectedNotEqual(t, got, want)`
- `testutil.ExpectedTrue(t, condition)`
- `testutil.ExpectedFalse(t, condition)`
- `testutil.ExpectedNotNil(t, object)`
- `testutil.ExpectedBytesEqual(t, gotBytes, wantBytes)`

### 3. Mocking Hardware & Network
- Hardware mock interfaces (e.g. simulated Epson printers) are provided in [internal/testutil/mockdevices.go](file:///home/odoo/odoo/epos-proxy/internal/testutil/mockdevices.go).
- Odoo IoT mock endpoints are provided in [internal/testutil/mock_odoo.go](file:///home/odoo/odoo/epos-proxy/internal/testutil/mock_odoo.go).

See [test-patterns.md](file:///home/odoo/odoo/epos-proxy/.agents/skills/obox-testing/references/test-patterns.md) for full code examples and guidelines.
