package printer

import (
	"testing"

	"epos-proxy/internal/config"
	"epos-proxy/internal/testutil"
)

func TestListBluetoothPrinters(t *testing.T) {
	cfg := &config.Manager{
		Data: config.AppConfig{
			BluetoothPrinters: []config.BluetoothPrinterConfig{
				{Address: "11:22:33:44:55:66", Name: "Kitchen Printer"},
				{Address: "AA:BB:CC:DD:EE:FF", Name: ""},
			},
		},
	}

	mgr := NewManager(cfg)
	printers := mgr.ListBluetoothPrinters()
	testutil.ExpectedLen(t, printers, 2)
	testutil.ExpectedEqual(t, printers[0].Address, "11:22:33:44:55:66")
	testutil.ExpectedEqual(t, printers[0].Name, "Kitchen Printer")
	testutil.ExpectedEqual(t, printers[0].Id, encodeBluetoothPrinterID("11:22:33:44:55:66"))

	testutil.ExpectedEqual(t, printers[1].Address, "AA:BB:CC:DD:EE:FF")
	testutil.ExpectedEqual(t, printers[1].Name, "BT - AA:BB:CC:DD:EE:FF")
	testutil.ExpectedEqual(t, printers[1].Id, encodeBluetoothPrinterID("AA:BB:CC:DD:EE:FF"))
}

func TestAddAndRemoveBluetoothPrinter(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	cfg, err := config.NewManager()
	testutil.ExpectedNoError(t, err)

	mgr := NewManager(cfg)

	// 1. Invalid address validation error
	err = mgr.AddBluetoothPrinter("not-valid-address", "Test Printer")
	testutil.ExpectedError(t, err)

	// 2. Empty address
	err = mgr.AddBluetoothPrinter("", "Test Printer")
	testutil.ExpectedError(t, err)

	// 3. Remove printer
	const mac = "AA:BB:CC:DD:EE:FF"
	err = cfg.AddBluetoothPrinter(mac, "Test Printer")
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedLen(t, cfg.GetBluetoothPrinters(), 1)

	// Remove via RemoveBluetoothPrinter
	err = mgr.RemoveBluetoothPrinter(mac)
	testutil.ExpectedNoError(t, err)
	testutil.ExpectedLen(t, cfg.GetBluetoothPrinters(), 0)
}

func TestListBluetoothPrinters_NilConfig(t *testing.T) {
	mgr := NewManager(nil)
	printers := mgr.ListBluetoothPrinters()
	testutil.ExpectedLen(t, printers, 0)
}

func TestIsBluetoothPrinterConfigured(t *testing.T) {
	mgrNil := NewManager(nil)
	testutil.ExpectedFalse(t, mgrNil.isBluetoothPrinterConfigured("11:22:33:44:55:66"))

	cfg := &config.Manager{
		Data: config.AppConfig{
			BluetoothPrinters: []config.BluetoothPrinterConfig{
				{Address: "11:22:33:44:55:66", Name: "Printer 1"},
			},
		},
	}
	mgr := NewManager(cfg)
	testutil.ExpectedTrue(t, mgr.isBluetoothPrinterConfigured("11:22:33:44:55:66"))
	testutil.ExpectedFalse(t, mgr.isBluetoothPrinterConfigured("AA:BB:CC:DD:EE:FF"))

	// Manager.Get should reject unconfigured Bluetooth printer
	unconfiguredID := encodeBluetoothPrinterID("AA:BB:CC:DD:EE:FF")
	_, err := mgr.Get(unconfiguredID)
	testutil.ExpectedError(t, err)
}
