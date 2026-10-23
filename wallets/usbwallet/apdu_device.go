package usbwallet

import (
	"encoding/binary"
	"errors"
	"io"
)

const hidFrameSize = 64

// apduDevice turns the HID frames the Ledger driver writes into an APDU to
// exchange, like with the Zemu emulator, and the reply back into frames.
type apduDevice struct {
	exchange func(apdu []byte) ([]byte, error) // returns the reply with its status word
	close    func() error

	apdu  []byte // APDU reassembled from the written frames, cap is its announced length
	reply []byte // HID frames of the last reply, not yet read
}

// Write consumes one HID frame: channel (2 bytes), tag (1 byte), sequence
// (2 bytes), then the payload; the first frame's payload starts with the
// 2-byte APDU length.
func (d *apduDevice) Write(frame []byte) (int, error) {
	if len(frame) < 5 {
		return 0, errors.New("apdu device: short HID frame")
	}
	payload := frame[5:]
	if binary.BigEndian.Uint16(frame[3:5]) == 0 {
		if len(payload) < 2 {
			return 0, errors.New("apdu device: short HID frame")
		}
		d.apdu = make([]byte, 0, binary.BigEndian.Uint16(payload))
		payload = payload[2:]
	}
	d.apdu = append(d.apdu, payload[:min(len(payload), cap(d.apdu)-len(d.apdu))]...)
	if len(d.apdu) < cap(d.apdu) {
		return len(frame), nil
	}

	reply, err := d.exchange(d.apdu)
	d.apdu = nil
	if err != nil {
		return 0, err
	}
	d.reply = hidFrames(reply)
	return len(frame), nil
}

// Read returns the reply frames produced by the last Write.
func (d *apduDevice) Read(p []byte) (int, error) {
	if len(d.reply) == 0 {
		return 0, io.EOF
	}
	n := copy(p, d.reply)
	d.reply = d.reply[n:]
	return n, nil
}

// Close releases the underlying connection.
func (d *apduDevice) Close() error {
	return d.close()
}

// hidFrames splits a reply, status word included, into 64-byte HID frames.
func hidFrames(reply []byte) []byte {
	//#nosec G115 -- APDU replies are bounded well below 64KiB
	msg := binary.BigEndian.AppendUint16(nil, uint16(len(reply)))
	msg = append(msg, reply...)

	var frames []byte
	for seq := 0; len(msg) > 0; seq++ {
		frame := make([]byte, hidFrameSize)
		copy(frame, []byte{0x01, 0x01, 0x05})
		//#nosec G115 -- the sequence number is bounded by the reply length
		binary.BigEndian.PutUint16(frame[3:5], uint16(seq))
		msg = msg[copy(frame[5:], msg):]
		frames = append(frames, frame...)
	}
	return frames
}
