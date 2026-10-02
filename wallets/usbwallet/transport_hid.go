//go:build !ledger_zemu

package usbwallet

import (
	"io"

	usb "github.com/zondax/hid"
)

// transportSupported reports whether USB HID is available on this platform.
func transportSupported() bool {
	return usb.Supported()
}

// enumerateDevices lists the USB HID devices of the given vendor.
func enumerateDevices(vendorID uint16) []usb.DeviceInfo {
	return usb.Enumerate(vendorID, 0)
}

// openDevice opens a USB HID connection to the device.
func openDevice(info usb.DeviceInfo) (io.ReadWriteCloser, error) {
	device, err := info.Open()
	if err != nil {
		return nil, err
	}
	return device, nil
}
