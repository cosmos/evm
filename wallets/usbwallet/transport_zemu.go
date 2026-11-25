//go:build ledger_zemu

package usbwallet

import (
	"io"

	usb "github.com/zondax/hid"
	ledger "github.com/zondax/ledger-go"
)

// With the ledger_zemu build tag, the hub reaches a Zemu/Speculos emulator
// through ledger-go instead of USB HID, as a single Ledger Nano S.
const (
	zemuProductID = 0x0001
	zemuUsagePage = 0xffa0
)

// transportSupported reports that the emulator is always reachable.
func transportSupported() bool {
	return true
}

// enumerateDevices returns the emulated device.
func enumerateDevices(vendorID uint16) []usb.DeviceInfo {
	return []usb.DeviceInfo{{
		Path:      "zemu",
		VendorID:  vendorID,
		ProductID: zemuProductID,
		UsagePage: zemuUsagePage,
	}}
}

// openDevice connects to the emulator's gRPC APDU service.
func openDevice(usb.DeviceInfo) (io.ReadWriteCloser, error) {
	device, err := ledger.NewLedgerAdmin().Connect(0)
	if err != nil {
		return nil, err
	}
	return &apduDevice{
		exchange: func(apdu []byte) ([]byte, error) {
			// ledger-go strips the success status word the driver expects
			reply, err := device.Exchange(apdu)
			if err != nil {
				return nil, err
			}
			return append(reply, 0x90, 0x00), nil
		},
		close: device.Close,
	}, nil
}
