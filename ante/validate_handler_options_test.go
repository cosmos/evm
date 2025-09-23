package ante_test

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/ante"
	ethante "github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/types"
)

//nolint:thelper // RunValidateHandlerOptionsTest is not a helper function; it's an externally called benchmark entry point
func RunValidateHandlerOptionsTest(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	nw := network.NewUnitTestNetwork(create, options...)
	cases := []struct {
		name    string
		options ante.HandlerOptions
		expPass bool
	}{
		{
			"fail - empty options",
			ante.HandlerOptions{},
			false,
		},
		{
			"fail - empty account keeper",
			ante.HandlerOptions{
				Cdc:           nw.App.AppCodec(),
				AccountKeeper: nil,
			},
			false,
		},
		{
			"fail - empty bank keeper",
			ante.HandlerOptions{
				Cdc:           nw.App.AppCodec(),
				AccountKeeper: nw.App.GetAccountKeeper(),
				BankKeeper:    nil,
			},
			false,
		},
		{
			"fail - empty IBC keeper",
			ante.HandlerOptions{
				Cdc:           nw.App.AppCodec(),
				AccountKeeper: nw.App.GetAccountKeeper(),
				BankKeeper:    nw.App.GetBankKeeper(),
				IBCKeeper:     nil,
			},
			false,
		},
		{
			"fail - empty fee market keeper",
			ante.HandlerOptions{
				Cdc:             nw.App.AppCodec(),
				AccountKeeper:   nw.App.GetAccountKeeper(),
				BankKeeper:      nw.App.GetBankKeeper(),
				IBCKeeper:       nw.App.GetIBCKeeper(),
				FeeMarketKeeper: nil,
			},
			false,
		},
		{
			"fail - empty EVM keeper",
			ante.HandlerOptions{
				Cdc:             nw.App.AppCodec(),
				AccountKeeper:   nw.App.GetAccountKeeper(),
				BankKeeper:      nw.App.GetBankKeeper(),
				IBCKeeper:       nw.App.GetIBCKeeper(),
				FeeMarketKeeper: nw.App.GetFeeMarketKeeper(),
				EvmKeeper:       nil,
			},
			false,
		},
		{
			"fail - empty signature gas consumer",
			ante.HandlerOptions{
				Cdc:             nw.App.AppCodec(),
				AccountKeeper:   nw.App.GetAccountKeeper(),
				BankKeeper:      nw.App.GetBankKeeper(),
				IBCKeeper:       nw.App.GetIBCKeeper(),
				FeeMarketKeeper: nw.App.GetFeeMarketKeeper(),
				EvmKeeper:       nw.App.GetEVMKeeper(),
				SigGasConsumer:  nil,
			},
			false,
		},
		{
			"fail - empty signature mode handler",
			ante.HandlerOptions{
				Cdc:             nw.App.AppCodec(),
				AccountKeeper:   nw.App.GetAccountKeeper(),
				BankKeeper:      nw.App.GetBankKeeper(),
				IBCKeeper:       nw.App.GetIBCKeeper(),
				FeeMarketKeeper: nw.App.GetFeeMarketKeeper(),
				EvmKeeper:       nw.App.GetEVMKeeper(),
				SigGasConsumer:  ante.SigVerificationGasConsumer,
				SignModeHandler: nil,
			},
			false,
		},
		{
			"fail - empty tx fee checker",
			ante.HandlerOptions{
				Cdc:             nw.App.AppCodec(),
				AccountKeeper:   nw.App.GetAccountKeeper(),
				BankKeeper:      nw.App.GetBankKeeper(),
				IBCKeeper:       nw.App.GetIBCKeeper(),
				FeeMarketKeeper: nw.App.GetFeeMarketKeeper(),
				EvmKeeper:       nw.App.GetEVMKeeper(),
				SigGasConsumer:  ante.SigVerificationGasConsumer,
				SignModeHandler: nw.App.GetTxConfig().SignModeHandler(),
				TxFeeChecker:    nil,
			},
			false,
		},
		{
			"fail - empty pending tx listener",
			ante.HandlerOptions{
				Cdc:                    nw.App.AppCodec(),
				AccountKeeper:          nw.App.GetAccountKeeper(),
				BankKeeper:             nw.App.GetBankKeeper(),
				ExtensionOptionChecker: types.HasDynamicFeeExtensionOption,
				EvmKeeper:              nw.App.GetEVMKeeper(),
				FeegrantKeeper:         nw.App.GetFeeGrantKeeper(),
				IBCKeeper:              nw.App.GetIBCKeeper(),
				FeeMarketKeeper:        nw.App.GetFeeMarketKeeper(),
				SignModeHandler:        nw.GetEncodingConfig().TxConfig.SignModeHandler(),
				SigGasConsumer:         ante.SigVerificationGasConsumer,
				MaxTxGasWanted:         40000000,
				TxFeeChecker:           ethante.NewDynamicFeeChecker(nw.App.GetFeeMarketKeeper()),
				PendingTxListener:      nil,
			},
			false,
		},
		{
			"success - default app options",
			ante.HandlerOptions{
				Cdc:                    nw.App.AppCodec(),
				AccountKeeper:          nw.App.GetAccountKeeper(),
				BankKeeper:             nw.App.GetBankKeeper(),
				ExtensionOptionChecker: types.HasDynamicFeeExtensionOption,
				EvmKeeper:              nw.App.GetEVMKeeper(),
				FeegrantKeeper:         nw.App.GetFeeGrantKeeper(),
				IBCKeeper:              nw.App.GetIBCKeeper(),
				FeeMarketKeeper:        nw.App.GetFeeMarketKeeper(),
				SignModeHandler:        nw.GetEncodingConfig().TxConfig.SignModeHandler(),
				SigGasConsumer:         ante.SigVerificationGasConsumer,
				MaxTxGasWanted:         40000000,
				TxFeeChecker:           ethante.NewDynamicFeeChecker(nw.App.GetFeeMarketKeeper()),
				PendingTxListener:      func(hash common.Hash) {},
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

func TestValidateHandlerOptions(t *testing.T) {
	RunValidateHandlerOptionsTest(t, integration.CreateEvmd)
}
