# Obox Hardware & USB Reference

Low-level driver specifications and troubleshooting notes for POS hardware.

## Supported Epson USB Vendors & Devices

- **Vendor ID**: `0x04B8` (Seiko Epson Corp.)
- **USB Device Class**: `0x07` (Printer Device Class)
- **Common Product Families**:
  - TM-T20 / TM-T20II / TM-T20III
  - TM-T88IV / TM-T88V / TM-T88VI
  - TM-m10 / TM-m30 / TM-m30II / TM-m50
  - TM-U220 (Impact / Dot Matrix)

## Linux Permissions (udev Rules)

On Linux, raw USB access requires appropriate permissions:
```udev
# /etc/udev/rules.d/99-epson-pos.rules
SUBSYSTEM=="usb", ATTR{idVendor}=="04b8", MODE="0666"
```
After adding or modifying udev rules:
```bash
sudo udevadm control --reload-rules
sudo udevadm trigger
```

## Low-Level Debugging

1. **Verify OS-level USB visibility**:
   - Linux: `lsusb -d 04b8:`
   - macOS: `system_profiler SPUSBDataType`
   - Windows: `Get-PnpDevice -PresentOnly | Where-Object { $_.InstanceId -like "*USB*" }`
2. **Check App Logs**:
   Logs from `internal/logger` contain detailed USB discovery scans and endpoint claim results. Enable debug mode in the UI or configuration file.
