package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/encoding"
	evmaddress "github.com/cosmos/evm/encoding/address"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	clitestutil "github.com/cosmos/cosmos-sdk/testutil/cli"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// TestSendTxCmdAddresses runs the send command with --generate-only and checks
// the addresses that end up in the generated MsgSend.
func TestSendTxCmdAddresses(t *testing.T) {
	enc := encoding.MakeConfig(9001)
	banktypes.RegisterInterfaces(enc.InterfaceRegistry)
	ac := evmaddress.NewEvmCodec("cosmos")
	clientCtx := client.Context{}.
		WithCodec(enc.Codec).
		WithTxConfig(enc.TxConfig).
		WithInterfaceRegistry(enc.InterfaceRegistry).
		WithChainID("test-1")

	const from = "0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E"
	const fromBech = "cosmos10jmp6sgh4cc6zt3e8gw05wavvejgr5pwsjskvv"

	for _, tc := range []struct {
		name, to, wantTo, errContains string
	}{
		{"bech32 preserved", "cosmos18wvvwfmq77a6d8tza4h5sfuy2yj3jj88yqg82a", "cosmos18wvvwfmq77a6d8tza4h5sfuy2yj3jj88yqg82a", ""},
		{"0x hex", "0x3B98c72760f7BBa69D62ED6f48278451251948e7", "cosmos18wvvwfmq77a6d8tza4h5sfuy2yj3jj88yqg82a", ""},
		{"32-byte preserved", "cosmos1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5z5tpwxqergd3c8g7rusqqlvp8l", "cosmos1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5z5tpwxqergd3c8g7rusqqlvp8l", ""},
		{"foreign prefix", "evmos1ltzy54ms24v590zz37r2q9hrrdcc8eslndsqwv", "", "invalid to-address"},
		{"garbage", "blabla", "", "invalid to-address"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			out, err := clitestutil.ExecTestCLICmd(clientCtx, NewSendTxCmd(ac), []string{
				from, tc.to, "1stake", "--" + flags.FlagGenerateOnly,
			})
			if tc.errContains != "" {
				require.ErrorContains(t, err, tc.errContains)
				return
			}
			require.NoError(t, err)
			var tx struct {
				Body struct {
					Messages []struct {
						From string `json:"from_address"`
						To   string `json:"to_address"`
					} `json:"messages"`
				} `json:"body"`
			}
			require.NoError(t, json.Unmarshal(out.Bytes(), &tx), out.String())
			require.Len(t, tx.Body.Messages, 1)
			require.Equal(t, fromBech, tx.Body.Messages[0].From)
			require.Equal(t, tc.wantTo, tx.Body.Messages[0].To)
		})
	}
}
