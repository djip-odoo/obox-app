#!/usr/bin/env bash
# build-win.sh
# MSYS2 MINGW64 build script for Obox App (Windows)
# Run directly inside MSYS2 bash or via build-win.ps1
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
cd "${PROJECT_DIR}"

echo "===================================="
echo " OBOX APP - MSYS2 WINDOWS BUILD"
echo "===================================="

# 1. Environment & Paths
export PATH="/mingw64/bin:/usr/bin:${PATH}"

# Add Windows Go if not already in PATH
if ! command -v go >/dev/null 2>&1; then
    for go_path in "/c/Program Files/Go/bin" "/c/Go/bin"; do
        if [ -d "${go_path}" ]; then
            export PATH="${go_path}:${PATH}"
            break
        fi
    done
fi

# Add Windows Node if not already in PATH
if ! command -v node >/dev/null 2>&1; then
    for node_path in "/c/Program Files/nodejs"; do
        if [ -d "${node_path}" ]; then
            export PATH="${node_path}:${PATH}"
            break
        fi
    done
fi

# Add user Go bin for wails3
if [ -n "${USERPROFILE:-}" ]; then
    user_go_bin="$(cygpath -u "${USERPROFILE}")/go/bin"
    if [ -d "${user_go_bin}" ]; then
        export PATH="${PATH}:${user_go_bin}"
    fi
fi

# 2. Verify / Install required MSYS2 packages
REQUIRED_PKGS=(
    "mingw-w64-x86_64-gcc"
    "mingw-w64-x86_64-nsis"
    "mingw-w64-x86_64-libusb"
    "mingw-w64-x86_64-pkg-config"
)

MISSING_PKGS=()
for pkg in "${REQUIRED_PKGS[@]}"; do
    if ! pacman -Q "${pkg}" >/dev/null 2>&1; then
        MISSING_PKGS+=("${pkg}")
    fi
done

if [ ${#MISSING_PKGS[@]} -gt 0 ]; then
    echo "==> Installing missing MSYS2 packages: ${MISSING_PKGS[*]}"
    pacman -S --needed --noconfirm "${MISSING_PKGS[@]}"
fi

# 3. CGO Compiler Flags for libusb
export CGO_ENABLED="1"
export CGO_CFLAGS="-I/mingw64/include/libusb-1.0"
export CGO_LDFLAGS="-L/mingw64/lib -lusb-1.0 -static -lsetupapi"
export CC="gcc"

# 4. Verify tools
echo "==> Verifying toolchain:"
for tool in go node npm task gcc makensis wails3; do
    if command -v "${tool}" >/dev/null 2>&1; then
        echo "  [OK] ${tool}: $(command -v "${tool}")"
    else
        echo "  [MISSING] ${tool}"
    fi
done

# 5. Resolve version
CURRENT_VER="$(git describe --tags --always 2>/dev/null || echo "dev")"
BUILD_VERSION="${VERSION:-${CURRENT_VER}}"

# 6. Execute Build
echo "==> Building Windows binary and NSIS installer (version: ${BUILD_VERSION})..."
task build:windows VERSION="${BUILD_VERSION}" "$@"

echo "===================================="
echo " BUILD SUCCESSFUL!"
echo " Output artifacts in: ${PROJECT_DIR}/build/bin"
echo "===================================="
