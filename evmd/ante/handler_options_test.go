package ante_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/ante"
	ethante "github.com/cosmos/evm/ante/evm"
	chainante "github.com/cosmos/evm/evmd/ante"
	"github.com/cosmos/evm/testutil/integration/os/network"
	"github.com/cosmos/evm/types"
)

func TestValidateHandlerOptions(t *testing.T) {
	nw := network.NewUnitTestNetwork()
	cases := []struct {
		name    string
		options chainante.HandlerOptions
		expPass bool
	}{
		{
			"fail - empty options",
			chainante.HandlerOptions{},
			false,
		},
		{
			"fail - empty account keeper",
			chainante.HandlerOptions{
				Cdc:           nw.App.AppCodec(),
				AccountKeeper: nil,
			},
			false,
		},
		{
			"fail - empty bank keeper",
			chainante.HandlerOptions{
				Cdc:           nw.App.AppCodec(),
				AccountKeeper: nw.App.AccountKeeper,
				BankKeeper:    nil,
			},
			false,
		},
		{
			"fail - empty IBC keeper",
			chainante.HandlerOptions{
				Cdc:           nw.App.AppCodec(),
				AccountKeeper: nw.App.AccountKeeper,
				BankKeeper:    nw.App.BankKeeper,
				IBCKeeper:     nil,
			},
			false,
		},
		{
			"fail - empty fee market keeper",
			chainante.HandlerOptions{
				Cdc:             nw.App.AppCodec(),
				AccountKeeper:   nw.App.AccountKeeper,
				BankKeeper:      nw.App.BankKeeper,
				IBCKeeper:       nw.App.IBCKeeper,
				FeeMarketKeeper: nil,
			},
			false,
		},
		{
			"fail - empty EVM keeper",
			chainante.HandlerOptions{
				Cdc:             nw.App.AppCodec(),
				AccountKeeper:   nw.App.AccountKeeper,
				BankKeeper:      nw.App.BankKeeper,
				IBCKeeper:       nw.App.IBCKeeper,
				FeeMarketKeeper: nw.App.FeeMarketKeeper,
				EvmKeeper:       nil,
			},
			false,
		},
		{
			"fail - empty signature gas consumer",
			chainante.HandlerOptions{
				Cdc:             nw.App.AppCodec(),
				AccountKeeper:   nw.App.AccountKeeper,
				BankKeeper:      nw.App.BankKeeper,
				IBCKeeper:       nw.App.IBCKeeper,
				FeeMarketKeeper: nw.App.FeeMarketKeeper,
				EvmKeeper:       nw.App.EVMKeeper,
				SigGasConsumer:  nil,
			},
			false,
		},
		{
			"fail - empty signature mode handler",
			chainante.HandlerOptions{
				Cdc:             nw.App.AppCodec(),
				AccountKeeper:   nw.App.AccountKeeper,
				BankKeeper:      nw.App.BankKeeper,
				IBCKeeper:       nw.App.IBCKeeper,
				FeeMarketKeeper: nw.App.FeeMarketKeeper,
				EvmKeeper:       nw.App.EVMKeeper,
				SigGasConsumer:  ante.SigVerificationGasConsumer,
				SignModeHandler: nil,
			},
			false,
		},
		{
			"fail - empty tx fee checker",
			chainante.HandlerOptions{
				Cdc:             nw.App.AppCodec(),
				AccountKeeper:   nw.App.AccountKeeper,
				BankKeeper:      nw.App.BankKeeper,
				IBCKeeper:       nw.App.IBCKeeper,
				FeeMarketKeeper: nw.App.FeeMarketKeeper,
				EvmKeeper:       nw.App.EVMKeeper,
				SigGasConsumer:  ante.SigVerificationGasConsumer,
				SignModeHandler: nw.App.GetTxConfig().SignModeHandler(),
				TxFeeChecker:    nil,
			},
			false,
		},
		{
			"success - default app options",
			chainante.HandlerOptions{
				Cdc:                    nw.App.AppCodec(),
				AccountKeeper:          nw.App.AccountKeeper,
				BankKeeper:             nw.App.BankKeeper,
				ExtensionOptionChecker: types.HasDynamicFeeExtensionOption,
				EvmKeeper:              nw.App.EVMKeeper,
				FeegrantKeeper:         nw.App.FeeGrantKeeper,
				IBCKeeper:              nw.App.IBCKeeper,
				FeeMarketKeeper:        nw.App.FeeMarketKeeper,
				SignModeHandler:        nw.GetEncodingConfig().TxConfig.SignModeHandler(),
				SigGasConsumer:         ante.SigVerificationGasConsumer,
				MaxTxGasWanted:         40000000,
				TxFeeChecker:           ethante.NewDynamicFeeChecker(nw.App.FeeMarketKeeper),
			},
			true,
		},
	}

	for _, tc := range cases {
		err := tc.options.Validate()
		if tc.expPass {
			require.NoError(t, err, tc.name)
		} else {
			require.Error(t, err, tc.name)
		}
	}
}
