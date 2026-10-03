package cli

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/client"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	testHexAddr    = "0x3B98c72760f7BBa69D62ED6f48278451251948e7"
	testBech32Addr = "cosmos18wvvwfmq77a6d8tza4h5sfuy2yj3jj88yqg82a"
)

// setCosmosPrefix pins the bech32 prefix the fixtures below are encoded with.
func setCosmosPrefix() {
	sdk.GetConfig().SetBech32PrefixForAccount("cosmos", "cosmospub")
}

func TestHexToBech32(t *testing.T) {
	setCosmosPrefix()

	testCases := []struct {
		name        string
		addr        string
		expected    string
		errContains string
	}{
		{"0x hex", testHexAddr, testBech32Addr, ""},
		{"0X hex", "0X" + testHexAddr[2:], testBech32Addr, ""},
		{"hex without prefix", testHexAddr[2:], testBech32Addr, ""},
		{"bech32", testBech32Addr, "", "is not a valid hex address"},
		{"foreign prefix bech32", "evmos1ltzy54ms24v590zz37r2q9hrrdcc8eslndsqwv", "", "is not a valid hex address"},
		{"short hex", "0x3B98c7", "", "is not a valid hex address"},
		// common.HexToAddress would keep the last 20 bytes of this.
		{"32-byte hex", "0x0102030405060708090a0b0c0d0e0f101112131415161718191a1b1c1d1e1f20", "", "is not a valid hex address"},
		{"trailing newline", testHexAddr + "\n", "", `\n" is not a valid hex address`},
		{"garbage", "blabla", "", `"blabla" is not a valid hex address`},
		{"empty", "", "", `"" is not a valid hex address`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bech32, err := hexToBech32(tc.addr)
			if tc.errContains != "" {
				require.ErrorContains(t, err, tc.errContains)
				require.Empty(t, bech32)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, bech32)
		})
	}
}

func TestAccountToBech32(t *testing.T) {
	setCosmosPrefix()

	// 32-byte account, e.g. a module-derived address; must not be cropped to 20 bytes.
	const longBech32Addr = "cosmos1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5z5tpwxqergd3c8g7rusqqlvp8l"

	testCases := []struct {
		name        string
		addr        string
		expected    string
		errContains string
	}{
		{"0x hex", testHexAddr, testBech32Addr, ""},
		{"hex without prefix", testHexAddr[2:], testBech32Addr, ""},
		{"bech32", testBech32Addr, testBech32Addr, ""},
		{"uppercase bech32 is normalized", strings.ToUpper(testBech32Addr), testBech32Addr, ""},
		{"32-byte bech32 keeps its length", longBech32Addr, longBech32Addr, ""},
		{"validator operator address", "cosmosvaloper18wvvwfmq77a6d8tza4h5sfuy2yj3jj88p5ujxw", "", "invalid Bech32 prefix"},
		{"bech32 with bad checksum", "cosmos18wvvwfmq77a6d8tza4h5sfuy2yj3jj88yqg82c", "", "invalid checksum"},
		{"foreign prefix bech32", "evmos1ltzy54ms24v590zz37r2q9hrrdcc8eslndsqwv", "", "invalid Bech32 prefix"},
		{"truncated 0x hex", testHexAddr[:41], "", "is not a valid hex address"},
		{"garbage", "blabla", "", "is not a valid hex or bech32 address"},
		{"empty", "", "", `"" is not a valid hex or bech32 address`},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			bech32, err := accountToBech32(tc.addr)
			if tc.errContains != "" {
				require.ErrorContains(t, err, tc.errContains)
				require.Empty(t, bech32)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.expected, bech32)
		})
	}
}

func TestHexToBech32Cmd(t *testing.T) {
	setCosmosPrefix()

	out, err := clitestutil.ExecTestCLICmd(client.Context{}, HexToBech32Cmd(), []string{testHexAddr})
	require.NoError(t, err)
	require.Equal(t, testBech32Addr, strings.TrimSpace(out.String()))

	_, err = clitestutil.ExecTestCLICmd(client.Context{}, HexToBech32Cmd(), []string{testBech32Addr})
	require.ErrorContains(t, err, "is not a valid hex address")
}

func TestGetBankBalanceCmdRejectsInvalidAddresses(t *testing.T) {
	setCosmosPrefix()

	_, err := clitestutil.ExecTestCLICmd(client.Context{}, GetBankBalanceCmd(), []string{"blabla", "atoken"})
	require.ErrorContains(t, err, "is not a valid hex or bech32 address")
}

func TestGetERC20BalanceCmdRejectsInvalidAddresses(t *testing.T) {
	for _, args := range [][]string{
		{"blabla", testHexAddr},
		{testBech32Addr, testHexAddr},
		{testHexAddr, "blabla"},
	} {
		_, err := clitestutil.ExecTestCLICmd(client.Context{}, GetERC20BalanceCmd(), args)
		require.ErrorContains(t, err, "is not a valid hex address", args)
	}
}
