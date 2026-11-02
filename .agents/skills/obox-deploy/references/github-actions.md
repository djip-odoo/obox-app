# GitHub Actions Pipeline Reference

Detailed breakdown of steps and configurations within `.github/actions/action.yaml`.

## Multi-Platform Execution Pipeline

### Linux (`ubuntu-latest`)
1. **Toolchain Setup**:
   - `arduino/setup-task@v2` (Task v3)
   - `actions/setup-go@v5` (Go 1.26 with module cache enabled)
   - `actions/setup-node@v4` (Node 22)
2. **System Dependencies**:
   - `task ci:deps:linux`: Installs `libgtk-4-dev`, `libwebkitgtk-6.0-dev`, `libusb-1.0-0-dev`.
   - `task ci:deps:wails`: Installs Wails v3 CLI.
3. **Build & Package**:
   - `task build:ci BUILD_NAME="$BUILD_NAME"` -> invokes `task package:deb`.
   - Generates `.desktop` file and calls `nfpm` with `build/linux/nfpm/nfpm.yaml`.
4. **Validation**:
   - Executes `task test`.

### Windows (`windows-latest`)
1. **Toolchain Setup**:
   - Runs in an MSYS2 MinGW64 shell via `msys2/setup-msys2@v2`.
   - Installed packages: `mingw-w64-x86_64-libusb`, `mingw-w64-x86_64-gcc`, `mingw-w64-x86_64-pkg-config`, `mingw-w64-x86_64-nsis`.
2. **CGO Compilation Flags**:
   - `CGO_ENABLED="1"`
   - `CGO_CFLAGS="-I/mingw64/include/libusb-1.0"`
   - `CGO_LDFLAGS="-L/mingw64/lib -lusb-1.0 -static -lsetupapi"`
3. **Packaging**:
   - `task build:windows`: Compiles `.exe`, downloads WebView2 bootstrapper, and runs `makensis` on `build/windows/installer/project.nsi`.
4. **Code Signing**:
   - Decodes `WINDOWS_CERT` to `certificate.pfx`.
   - Locates newest Windows Kits 10 `signtool.exe`.
   - Signs installer with SHA256 timestamping against `http://ts.ssl.com`.

### Artifact Publishing
- Uploads build outputs from `build/bin/*` via `actions/upload-artifact@v4`.
- If triggered by tag (`refs/tags/`), publishes release assets via `softprops/action-gh-release@v2`.
