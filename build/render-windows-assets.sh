#!/usr/bin/env bash
# render-windows-assets.sh
# Dynamically renders Windows build assets (info.json, wails.exe.manifest, wails_tools.nsh)
# from templates using project and build variables.
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Variables with fallbacks
APP_NAME="${APP_NAME:-obox-app}"
PRODUCT_NAME="${PRODUCT_NAME:-Obox App}"
COMPANY_NAME="${COMPANY_NAME:-Odoo}"
COPYRIGHT="${COPYRIGHT:-(c) 2026, Odoo}"
COMMENTS="${COMMENTS:-POS Printer Proxy for Odoo}"
IDENTIFIER="${IDENTIFIER:-com.odoo.oboxapp}"

# Resolve dynamic version:
# VERSION can be full git tag (e.g. 1.2.3 or 0.1-4-g1f6020c or dev)
VERSION="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo "dev")}"

# Resolve SEMVER: Windows PE resources and NSIS require numeric X.Y.Z
if [ -n "${SEMVER:-}" ]; then
  BUILD_SEMVER="${SEMVER}"
else
  EXTRACTED="$(echo "${VERSION}" | grep -oE '^[0-9]+\.[0-9]+(\.[0-9]+)?' | awk -F. '{printf "%d.%d.%d", $1, $2, ($3==""?0:$3)}' || true)"
  BUILD_SEMVER="${EXTRACTED:-1.0.0}"
fi

OUTPUT_DIR="${PROJECT_DIR}/build/windows/installer/tmp"
mkdir -p "${OUTPUT_DIR}"

# 1. Render info.json
sed \
  -e "s|{{\.Info\.ProductVersion}}|${BUILD_SEMVER}|g" \
  -e "s|{{\.Info\.CompanyName}}|${COMPANY_NAME}|g" \
  -e "s|{{\.Info\.ProductName}}|${PRODUCT_NAME}|g" \
  -e "s|{{\.Info\.Copyright}}|${COPYRIGHT}|g" \
  -e "s|{{\.Info\.Comments}}|${COMMENTS}|g" \
  "${PROJECT_DIR}/build/windows/info.json" > "${OUTPUT_DIR}/info.json"

# 2. Render wails.exe.manifest
sed \
  -e "s|{{\.Info\.ProductVersion}}|${BUILD_SEMVER}|g" \
  -e "s|com\.wails\.\*{{\.Name}}|${IDENTIFIER}|g" \
  -e "s|com\.wails\.{{[.]Name}}|${IDENTIFIER}|g" \
  -e "s|{{\.Name}}|${APP_NAME}|g" \
  "${PROJECT_DIR}/build/windows/wails.exe.manifest" > "${OUTPUT_DIR}/wails.exe.manifest"

# 3. Render wails_tools.nsh
sed \
  -e "s|{{\.Name}}|${APP_NAME}|g" \
  -e "s|{{\.Info\.CompanyName}}|${COMPANY_NAME}|g" \
  -e "s|{{\.Info\.ProductName}}|${PRODUCT_NAME}|g" \
  -e "s|{{\.Info\.ProductVersion}}|${BUILD_SEMVER}|g" \
  -e "s|{{\.Info\.Copyright}}|${COPYRIGHT}|g" \
  "${PROJECT_DIR}/build/windows/installer/wails_tools.nsh" > "${OUTPUT_DIR}/wails_tools.nsh"
