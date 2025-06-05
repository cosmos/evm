package ante

import (
	"math/big"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/server/config"
	"github.com/cosmos/evm/testutil"
	testconstants "github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (s *EvmAnteTestSuite) TestGasWantedDecorator() {
	s.WithFeemarketEnabled(true)
	s.SetupTest()
	ctx := s.GetNetwork().GetContext()
	dec := evm.NewGasWantedDecorator(s.GetNetwork().App.GetEVMKeeper(), s.GetNetwork().App.GetFeeMarketKeeper())
	from, fromPrivKey := utiltx.NewAddrKey()
	to := utiltx.GenerateAddress()
	denom := evmtypes.GetEVMCoinDenom()

	testCases := []struct {
		name              string
		expectedGasWanted uint64
		malleate          func() sdk.Tx
		expPass           bool
	}{
		{
			"Cosmos Tx",
			TestGasLimit,
			func() sdk.Tx {
				testMsg := banktypes.MsgSend{
					FromAddress: "cosmos1x8fhpj9nmhqk8z9kpgjt95ck2xwyue0ptzkucp",
					ToAddress:   "cosmos1dx67l23hz9l0k9hcher8xz04uj7wf3yu26l2yn",
					Amount:      sdk.Coins{sdk.Coin{Amount: sdkmath.NewInt(10), Denom: denom}},
				}
				txBuilder := s.CreateTestCosmosTxBuilder(sdkmath.NewInt(10), denom, &testMsg)
				return txBuilder.GetTx()
			},
			true,
		},
		{
			"Ethereum Legacy Tx",
			TestGasLimit,
			func() sdk.Tx {
				txArgs := evmtypes.EvmTxArgs{
					To:       &to,
					GasPrice: big.NewInt(0),
					GasLimit: TestGasLimit,
				}
				return s.CreateTxBuilder(fromPrivKey, txArgs).GetTx()
			},
			true,
		},
		{
			"Ethereum Access List Tx",
			TestGasLimit,
			func() sdk.Tx {
				emptyAccessList := ethtypes.AccessList{}
				txArgs := evmtypes.EvmTxArgs{
					To:       &to,
					GasPrice: big.NewInt(0),
					GasLimit: TestGasLimit,
					Accesses: &emptyAccessList,
				}
				return s.CreateTxBuilder(fromPrivKey, txArgs).GetTx()
			},
			true,
		},
		{
			"Ethereum Dynamic Fee Tx (EIP1559)",
			TestGasLimit,
			func() sdk.Tx {
				emptyAccessList := ethtypes.AccessList{}
				txArgs := evmtypes.EvmTxArgs{
					To:        &to,
					GasPrice:  big.NewInt(0),
					GasFeeCap: big.NewInt(100),
					GasLimit:  TestGasLimit,
					GasTipCap: big.NewInt(50),
					Accesses:  &emptyAccessList,
				}
				return s.CreateTxBuilder(fromPrivKey, txArgs).GetTx()
			},
			true,
		},
		{
			"EIP712 message",
			200000,
			func() sdk.Tx {
				amount := sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20)))
				gas := uint64(200000)
				acc := s.GetNetwork().App.GetAccountKeeper().NewAccountWithAddress(ctx, from.Bytes())
				s.Require().NoError(acc.SetSequence(1))
				s.GetNetwork().App.GetAccountKeeper().SetAccount(ctx, acc)
				builder, err := s.CreateTestEIP712TxBuilderMsgSend(acc.GetAddress(), fromPrivKey, ctx.ChainID(), config.DefaultEVMChainID, gas, amount)
				s.Require().NoError(err)
				return builder.GetTx()
			},
			true,
		},
		{
			"Cosmos Tx - gasWanted > max block gas",
			TestGasLimit,
			func() sdk.Tx {
				denom := testconstants.ExampleAttoDenom
				testMsg := banktypes.MsgSend{
					FromAddress: "cosmos1x8fhpj9nmhqk8z9kpgjt95ck2xwyue0ptzkucp",
					ToAddress:   "cosmos1dx67l23hz9l0k9hcher8xz04uj7wf3yu26l2yn",
					Amount:      sdk.Coins{sdk.Coin{Amount: sdkmath.NewInt(10), Denom: denom}},
				}
				txBuilder := s.CreateTestCosmosTxBuilder(sdkmath.NewInt(10), testconstants.ExampleAttoDenom, &testMsg)
				limit := types.BlockGasLimit(ctx)
				txBuilder.SetGasLimit(limit + 5)
				return txBuilder.GetTx()
			},
			false,
		},
	}

	// cumulative gas wanted from all test transactions in the same block
	var expectedGasWanted uint64

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			_, err := dec.AnteHandle(ctx, tc.malleate(), false, testutil.NoOpNextFn)
			if tc.expPass {
				s.Require().NoError(err)

				gasWanted := s.GetNetwork().App.GetFeeMarketKeeper().GetTransientGasWanted(ctx)
				expectedGasWanted += tc.expectedGasWanted
				s.Require().Equal(expectedGasWanted, gasWanted)
			} else {
				// TODO: check for specific error message
				s.Require().Error(err)
			}
		})
	}
}
