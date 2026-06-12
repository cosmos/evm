package address_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	evmaddress "github.com/cosmos/evm/encoding/address"

	"cosmossdk.io/core/address"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

const (
	hex    = "0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E"
	bech32 = "cosmos10jmp6sgh4cc6zt3e8gw05wavvejgr5pwsjskvv"
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
			cdc := evmaddress.NewEvmCodec(tc.cdcPrefix)
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

	// Helper codec (used only where we want to derive bytes from the bech32 string)
	var cdc address.Codec

	type tc struct {
		name   string
		input  func() []byte
		expRes string
		expErr error
	}

	testCases := []tc{
		{
			name: "success: from 20-byte input (hex-derived)",
			input: func() []byte {
				cdc = evmaddress.NewEvmCodec("cosmos")
				return addrBz
			},
			expRes: bech32,
			expErr: nil,
		},
		{
			name: "success: from bech32-derived bytes",
			input: func() []byte {
				cdc = evmaddress.NewEvmCodec("cosmos")
				bz, err := cdc.StringToBytes(bech32)
				require.NoError(t, err)
				require.Len(t, bz, 20)
				return bz
			},
			expRes: bech32,
			expErr: nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := cdc.BytesToString(tc.input())
			if tc.expErr == nil {
				require.NoError(t, err)
				require.Equal(t, tc.expRes, got)
			} else {
				require.ErrorContains(t, err, tc.expErr.Error())
			}
		})
	}
}
