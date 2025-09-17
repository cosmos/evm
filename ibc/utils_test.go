package ibc_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	cosmosevmibc "github.com/cosmos/evm/ibc"
	precompilestestutil "github.com/cosmos/evm/precompiles/testutil"
	testconstants "github.com/cosmos/evm/testutil/constants"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func init() {
	cfg := sdk.GetConfig()
	cfg.SetBech32PrefixForAccount("cosmos", "cosmospub")
}

func TestGetTransferAmount(t *testing.T) {
	testCases := []struct {
		name      string
		packet    channeltypes.Packet
		expAmount string
		expError  bool
	}{
		{
			name:      "empty packet",
			packet:    channeltypes.Packet{},
			expAmount: "",
			expError:  true,
		},
		{
			name:      "invalid packet data",
			packet:    channeltypes.Packet{Data: ibctesting.MockFailPacketData},
			expAmount: "",
			expError:  true,
		},
		{
			name: "invalid amount - empty",
			packet: channeltypes.Packet{
				Data: transfertypes.ModuleCdc.MustMarshalJSON(
					&transfertypes.FungibleTokenPacketData{
						Sender:   "cosmos1qql8ag4cluz6r4dz28p3w00dnc9w8ueulg2gmc",
						Receiver: "cosmos1x2w87cvt5mqjncav4lxy8yfreynn273x34qlwy",
						Amount:   "",
					},
				),
			},
			expAmount: "",
			expError:  true,
		},
		{
			name: "invalid amount - non-int",
			packet: channeltypes.Packet{
				Data: transfertypes.ModuleCdc.MustMarshalJSON(
					&transfertypes.FungibleTokenPacketData{
						Sender:   "cosmos1qql8ag4cluz6r4dz28p3w00dnc9w8ueulg2gmc",
						Receiver: "cosmos1x2w87cvt5mqjncav4lxy8yfreynn273x34qlwy",
						Amount:   "test",
					},
				),
			},
			expAmount: "test",
			expError:  true,
		},
		{
			name: "valid",
			packet: channeltypes.Packet{
				Data: transfertypes.ModuleCdc.MustMarshalJSON(
					&transfertypes.FungibleTokenPacketData{
						Sender:   "cosmos1qql8ag4cluz6r4dz28p3w00dnc9w8ueulg2gmc",
						Receiver: "cosmos1x2w87cvt5mqjncav4lxy8yfreynn273x34qlwy",
						Amount:   "10000",
					},
				),
			},
			expAmount: "10000",
			expError:  false,
		},
	}

	for _, tc := range testCases {
		amt, err := cosmosevmibc.GetTransferAmount(tc.packet)
		if tc.expError {
			require.Error(t, err, tc.name)
		} else {
			require.NoError(t, err, tc.name)
			require.Equal(t, tc.expAmount, amt)
		}
	}
}

func TestGetReceivedCoin(t *testing.T) {
	port := transfertypes.PortID
	chan0 := "channel-0"
	chan1 := "channel-1"
	chan2 := "channel-2"

	testCases := []struct {
		desc       string
		srcPort    string
		srcChannel string
		dstPort    string
		dstChannel string
		rawDenom   func() string
		rawAmount  string
		expCoin    func() sdk.Coin
	}{
		{
			desc:       "transfer native coin to destination which is not its source",
			srcPort:    port,
			srcChannel: chan1,
			dstPort:    port,
			dstChannel: chan0,
			rawDenom:   func() string { return "uosmo" },
			rawAmount:  "10",
			expCoin: func() sdk.Coin {
				expectedDenom := transfertypes.NewDenom(
					"uosmo",
					transfertypes.NewHop(port, chan0),
				).IBCDenom()
				return sdk.NewCoin(expectedDenom, math.NewInt(10))
			},
		},
		{
			desc:       "transfer ibc wrapped coin to destination which is its source",
			srcPort:    port,
			srcChannel: chan0,
			dstPort:    port,
			dstChannel: chan1,
			rawDenom: func() string {
				denom := transfertypes.NewDenom(
					"uatom",
					transfertypes.NewHop(port, chan0),
				)
				return denom.Path()
			},
			rawAmount: "10",
			expCoin: func() sdk.Coin {
				expectedDenom := transfertypes.NewDenom("uatom").IBCDenom()
				return sdk.NewCoin(expectedDenom, math.NewInt(10))
			},
		},
		{
			desc:       "transfer 2x ibc wrapped coin to destination which is its source",
			srcPort:    port,
			srcChannel: chan0,
			dstPort:    port,
			dstChannel: chan1,
			rawDenom: func() string {
				denom := transfertypes.NewDenom(
					"uosmo",
					transfertypes.NewHop(port, chan0),
					transfertypes.NewHop(port, chan1),
				)
				return denom.Path()
			},
			rawAmount: "10",
			expCoin: func() sdk.Coin {
				expectedDenom := transfertypes.NewDenom(
					"uosmo",
					transfertypes.NewHop(port, chan1),
				).IBCDenom()
				return sdk.NewCoin(expectedDenom, math.NewInt(10))
			},
		},
		{
			desc:       "transfer ibc wrapped coin to destination which is not its source",
			srcPort:    port,
			srcChannel: chan2,
			dstPort:    port,
			dstChannel: chan0,
			rawDenom: func() string {
				denom := transfertypes.NewDenom("uatom",
					transfertypes.NewHop(port, chan1),
				)
				return denom.Path()
			},
			rawAmount: "10",
			expCoin: func() sdk.Coin {
				expectedDenom := transfertypes.NewDenom(
					"uatom",
					transfertypes.NewHop(port, chan0),
					transfertypes.NewHop(port, chan1),
				).IBCDenom()
				return sdk.NewCoin(expectedDenom, math.NewInt(10))
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			packet := channeltypes.Packet{
				SourcePort:         tc.srcPort,
				SourceChannel:      tc.srcChannel,
				DestinationPort:    tc.dstPort,
				DestinationChannel: tc.dstChannel,
			}
			token := transfertypes.Token{
				Denom:  transfertypes.ExtractDenomFromPath(tc.rawDenom()),
				Amount: tc.rawAmount,
			}
			coin := cosmosevmibc.GetReceivedCoin(packet, token)
			require.Equal(t, tc.expCoin(), coin)
		})
	}
}

func TestGetSentCoin(t *testing.T) {
	baseDenom := testconstants.ExampleAttoDenom

	testCases := []struct {
		name      string
		rawDenom  string
		rawAmount string
		expCoin   sdk.Coin
	}{
		{
			"get unwrapped aatom coin",
			baseDenom,
			"10",
			sdk.Coin{Denom: baseDenom, Amount: math.NewInt(10)},
		},
		{
			"get ibc wrapped aatom coin",
			"transfer/channel-0/aatom",
			"10",
			sdk.Coin{Denom: precompilestestutil.AatomIbcDenom, Amount: math.NewInt(10)},
		},
		{
			"get ibc wrapped uosmo coin",
			"transfer/channel-0/uosmo",
			"10",
			sdk.Coin{Denom: precompilestestutil.UosmoIbcDenom, Amount: math.NewInt(10)},
		},
		{
			"get ibc wrapped uatom coin",
			"transfer/channel-1/uatom",
			"10",
			sdk.Coin{Denom: precompilestestutil.UatomIbcDenom, Amount: math.NewInt(10)},
		},
		{
			"get 2x ibc wrapped uatom coin",
			"transfer/channel-0/transfer/channel-1/uatom",
			"10",
			sdk.Coin{Denom: precompilestestutil.UatomOsmoIbcDenom, Amount: math.NewInt(10)},
		},
	}

	for _, tc := range testCases {
		coin := cosmosevmibc.GetSentCoin(tc.rawDenom, tc.rawAmount)
		require.Equal(t, tc.expCoin, coin)
	}
}

func TestDeriveDecimalsFromDenom(t *testing.T) {
	testCases := []struct {
		name      string
		baseDenom string
		expDec    uint8
		expFail   bool
		expErrMsg string
	}{
		{
			name:      "fail: empty string",
			baseDenom: "",
			expDec:    0,
			expFail:   true,
			expErrMsg: "Base denom cannot be an empty string",
		},
		{
			name:      "fail: invalid prefix",
			baseDenom: "nevmos",
			expDec:    0,
			expFail:   true,
			expErrMsg: "Should be either micro ('u[...]') or atto ('a[...]'); got: \"nevmos\"",
		},
		{
			name:      "success: micro 'u' prefix",
			baseDenom: "uatom",
			expDec:    6,
			expFail:   false,
			expErrMsg: "",
		},
		{
			name:      "success: atto 'a' prefix",
			baseDenom: "aatom",
			expDec:    18,
			expFail:   false,
			expErrMsg: "",
		},
	}

	for _, tc := range testCases {
		dec, err := cosmosevmibc.DeriveDecimalsFromDenom(tc.baseDenom)
		if tc.expFail {
			require.Error(t, err, tc.expErrMsg)
			require.Contains(t, err.Error(), tc.expErrMsg)
		} else {
			require.NoError(t, err)
		}
		require.Equal(t, tc.expDec, dec)
	}
}
