package utils_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/utils"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

func TestStringToBytes(t *testing.T) {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("cosmos", "cosmospub")
	addrBz := common.HexToAddress(hex).Bytes()

	testCases := []struct {
		name      string
		cdcPrefix string
		input     string
		expBz     []byte
		expErr    error
	}{
		{
			"success: valid bech32 address",
			"cosmos",
			bech32,
			addrBz,
			nil,
		},
		{
			"success: valid hex address",
			"cosmos",
			hex,
			addrBz,
			nil,
		},
		{
			"failure: invalid bech32 address (wrong prefix)",
			"evmos",
			bech32,
			nil,
			sdkerrors.ErrLogic.Wrapf("hrp does not match bech32 prefix: expected '%s' got '%s'", "evmos", "cosmos"),
		},
		{
			"failure: invalid bech32 address (too long)",
			"cosmos",
			"cosmos10jmp6sgh4cc6zt3e8gw05wavvejgr5pwsjskvvv", // extra char at the end
			nil,
			sdkerrors.ErrUnknownAddress,
		},
		{
			"failure: invalid bech32 address (invalid format)",
			"cosmos",
			"cosmos10jmp6sgh4cc6zt3e8gw05wavvejgr5pwsjskv", // missing last char
			nil,
			sdkerrors.ErrUnknownAddress,
		},
		{
			"failure: invalid hex address (odd length)",
			"cosmos",
			"0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02", // missing last char
			nil,
			sdkerrors.ErrUnknownAddress,
		},
		{
			"failure: invalid hex address (even length)",
			"cosmos",
			"0x7cB61D4117AE31a12E393a1Cfa3BaC666481D0", // missing last 2 char
			nil,
			sdkerrors.ErrUnknownAddress,
		},
		{
			"failure: invalid hex address (too long)",
			"cosmos",
			"0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E00", // extra 2 char at the end
			nil,
			sdkerrors.ErrUnknownAddress,
		},
		{
			"failure: empty string",
			"cosmos",
			"",
			nil,
			sdkerrors.ErrInvalidAddress.Wrap("empty address string is not allowed"),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cdc := utils.NewEvmCodec(tc.cdcPrefix)
			bz, err := cdc.StringToBytes(tc.input)
			if tc.expErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expBz, bz)
			} else {
				require.ErrorContains(t, err, tc.expErr.Error())
			}
		})
	}
}

func TestBytesToString(t *testing.T) {
	// Keep the same fixtures as your StringToBytes test
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("cosmos", "cosmospub")

	addrBz := common.HexToAddress(hex).Bytes() // 20 bytes
	zeroAddr := common.Address{}.Hex()         // "0x000..."

	// Helper codec (used only where we want to derive bytes from the bech32 string)
	cdc := utils.NewEvmCodec("cosmos")

	type tc struct {
		name   string
		input  func() []byte
		expHex string
	}

	testCases := []tc{
		{
			name: "success: from 20-byte input (hex-derived)",
			input: func() []byte {
				return addrBz
			},
			expHex: common.HexToAddress(hex).Hex(), // checksummed
		},
		{
			name: "success: from bech32-derived bytes",
			input: func() []byte {
				bz, err := cdc.StringToBytes(bech32)
				require.NoError(t, err)
				require.Len(t, bz, 20)
				return bz
			},
			expHex: common.HexToAddress(hex).Hex(), // same address as above
		},
		{
			name: "success: empty slice -> zero address",
			input: func() []byte {
				return []byte{}
			},
			expHex: zeroAddr,
		},
		{
			name: "success: shorter than 20 bytes -> left-padded to zeroes",
			input: func() []byte {
				// Drop first byte -> 19 bytes; common.BytesToAddress pads on the left
				return addrBz[1:]
			},
			expHex: common.BytesToAddress(addrBz[1:]).Hex(),
		},
		{
			name: "success: longer than 20 bytes -> rightmost 20 used",
			input: func() []byte {
				// Prepend one byte; common.BytesToAddress will use last 20 bytes (which are addrBz)
				return append([]byte{0xAA}, addrBz...)
			},
			expHex: common.BytesToAddress(append([]byte{0xAA}, addrBz...)).Hex(),
		},
		{
			name: "success: much longer (32 bytes) -> rightmost 20 used",
			input: func() []byte {
				prefix := make([]byte, 12) // 12 + 20 = 32
				return append(prefix, addrBz...)
			},
			expHex: common.BytesToAddress(append(make([]byte, 12), addrBz...)).Hex(),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			codec := utils.NewEvmCodec("cosmos")

			got, err := codec.BytesToString(tc.input())
			require.NoError(t, err)
			require.Equal(t, tc.expHex, got)
		})
	}
}
