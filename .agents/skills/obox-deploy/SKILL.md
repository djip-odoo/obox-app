---
name: obox-deploy
description: >-
  Use this skill when managing, modifying, or triggering GitHub Actions CI/CD workflows,
  release builds, signing, and artifact generation.
---

# Obox GitHub Actions & Release Pipeline

This skill covers the release automation and CI/CD pipelines executed by GitHub Actions. GitHub Actions is the **exclusive mechanism** used to generate, sign, and publish production releases for Obox App.

## GitHub Actions Architecture

The CI/CD pipeline consists of:

1. **[.github/workflows/build.yaml](file:///home/odoo/odoo/epos-proxy/.github/workflows/build.yaml)**:
   - **Trigger**: Push of a SemVer git tag matching `[0-9]+.[0-9]+.[0-9]+` (e.g., `v1.2.0` or `1.2.0`).
   - **Matrix**: Runs parallel builds for `obox-app-linux-amd64` (on `ubuntu-latest`) and `obox-app-windows-amd64` (on `windows-latest`).
   - **Release Publishing**: Automatically attaches generated `.deb` and `.exe` installers to the GitHub Release using `softprops/action-gh-release@v2`.

2. **[.github/workflows/pr-validation.yml](file:///home/odoo/odoo/epos-proxy/.github/workflows/pr-validation.yml)**:
   - **Trigger**: Pull requests targeting `main`.
   - Runs cross-platform build validation with `package: false`.

3. **[.github/actions/action.yaml](file:///home/odoo/odoo/epos-proxy/.github/actions/action.yaml)**:
   - Composite action setting up toolchains (Task, Go 1.26, Node 22, MSYS2 MinGW64 on Windows, GTK/WebKit on Linux).
   - Runs tests via `task test`.
   - Performs code signing (signtool on Windows, codesign/notarization on macOS).

See [github-actions.md](file:///home/odoo/odoo/epos-proxy/.agents/skills/obox-deploy/references/github-actions.md) for full action step details.

---

## Critical Release Rules

### 1. Release Asset Naming Convention
> [!IMPORTANT]
> The matrix `name` input in `build.yaml` **must strictly follow** the pattern:
> `obox-app-<os>-<arch>` (e.g. `obox-app-linux-amd64`, `obox-app-windows-amd64`).
>
> The in-app auto-updater (`internal/update/`) parses GitHub Releases by matching this exact asset prefix. If changed, deployed clients will fail to detect or download updates.

### 2. Code Signing Secrets
The following secrets are consumed by `.github/actions/action.yaml`:
- **Windows**:
  - `sign-windows-cert`: Base64-encoded PFX certificate.
  - `sign-windows-cert-password`: Password for the PFX certificate.
- **macOS** (when enabled):
  - `sign-macos-app-cert`, `sign-macos-app-cert-password`
  - `sign-macos-installer-cert`, `sign-macos-installer-cert-password`
  - `sign-macos-apple-id`, `sign-macos-apple-password`, `sign-macos-team-id`

---

## How to Trigger a Production Release

1. Verify local code checks and tests pass:
   ```bash
   task lint
   task test
   ```
2. Commit and push all changes to `main`.
3. Create and push a SemVer release tag:
   ```bash
   # Example for version 1.2.0
   git tag 1.2.0
   git push origin 1.2.0
   ```
4. GitHub Actions will:
   - Spin up runners for Linux and Windows.
   - Compile binaries and package them into `.deb` and NSIS installer `.exe`.
   - Sign the Windows installer with signtool.
   - Create a GitHub Release with tag `1.2.0` and attach all assets.
