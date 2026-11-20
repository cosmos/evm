package usbwallet

import (
	"bytes"
	"testing"

	gethaccounts "github.com/ethereum/go-ethereum/accounts"
	"github.com/stretchr/testify/require"
)

func TestLedgerSignTypedMessage(t *testing.T) {
	domainHash := bytes.Repeat([]byte{0x0d}, 32)
	messageHash := bytes.Repeat([]byte{0x0e}, 32)
	// V || R || S, as the Ethereum app replies
	reply := append([]byte{0x1b}, bytes.Repeat([]byte{0x5a}, 64)...)

	var apdu []byte
	driver := &ledgerDriver{device: &apduDevice{
		exchange: func(command []byte) ([]byte, error) {
			apdu = command
			return append(bytes.Clone(reply), 0x90, 0x00), nil
		},
	}}

	// both the request and the reply span several HID frames
	sig, err := driver.ledgerSignTypedMessage(gethaccounts.DefaultBaseDerivationPath, domainHash, messageHash)
	require.NoError(t, err)

	path := []byte{
		5,
		0x80, 0, 0, 44,
		0x80, 0, 0, 60,
		0x80, 0, 0, 0,
		0, 0, 0, 0,
		0, 0, 0, 0,
	}
	data := append(append(path, domainHash...), messageHash...)
	require.Equal(t, append([]byte{0xe0, byte(ledgerOpSignTypedMessage), 0, 0, 85}, data...), apdu)
	require.Equal(t, append(reply[1:], reply[0]), sig)
}

func TestLedgerExchangeShortReply(t *testing.T) {
	// a reply too short for the status word used to panic
	for _, reply := range [][]byte{nil, {0x90}} {
		driver := &ledgerDriver{device: &apduDevice{
			exchange: func([]byte) ([]byte, error) { return reply, nil },
		}}
		_, err := driver.ledgerVersion()
		require.ErrorIs(t, err, errLedgerReplyLacksStatusWord)
	}
}
