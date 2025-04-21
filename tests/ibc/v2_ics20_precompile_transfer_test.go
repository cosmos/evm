// Copied from https://github.com/cosmos/ibc-go/blob/7325bd2b00fd5e33d895770ec31b5be2f497d37a/modules/apps/transfer/transfer_test.go
// Why was this copied?
// This test suite was imported to validate that ExampleChain (an EVM-based chain)
// correctly supports IBC v1 token transfers using ibc-go’s Transfer module logic.
// The test ensures that ics20 precompile transfer (A → B) behave as expected across channels.
package ibc

import (
	"math/big"
	"testing"
	"time"

	storetypes "cosmossdk.io/store/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	"github.com/cosmos/evm/evmd"
	evmibctesting "github.com/cosmos/evm/ibc/testing"
	"github.com/cosmos/evm/precompiles/ics20"
	evmante "github.com/cosmos/evm/x/vm/ante"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ICS20TransferV2TestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics20.Precompile
	chainB           *evmibctesting.TestChain
}

func (suite *ICS20TransferV2TestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 1, 1)
	suite.chainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetChainID(2))

	evmAppA := suite.chainA.App.(*evmd.EVMD)
	suite.chainAPrecompile, _ = ics20.NewPrecompile(
		*evmAppA.StakingKeeper,
		evmAppA.TransferKeeper,
		evmAppA.IBCKeeper.ChannelKeeper,
		evmAppA.AuthzKeeper, // TODO: To be deprecated,
		evmAppA.EVMKeeper,
	)
}

// Constructs the following sends based on the established channels/connections
// 1 - from chainA to chainB
func (suite *ICS20TransferV2TestSuite) TestHandleMsgTransfer() {
	var (
		sourceDenomToTransfer string
		msgAmount             sdkmath.Int
		err                   error
		nativeErc20           *NativeErc20Info
		erc20                 bool
	)

	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			"transfer single denom",
			func() {
				evmApp := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmApp.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount = evmibctesting.DefaultCoinAmount
			},
		},
		{
			"transfer amount larger than int64",
			func() {
				var ok bool
				evmApp := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmApp.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount, ok = sdkmath.NewIntFromString("9223372036854775808") // 2^63 (one above int64)
				suite.Require().True(ok)
			},
		},
		{
			"transfer entire balance",
			func() {
				evmApp := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmApp.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount = transfertypes.UnboundedSpendLimit()
			},
		},
		//{
		//	"native erc20 case",
		//	func() {
		//		// TODO: To be added and verified due to the current / restriction logic. These test cases will be added and validated after PR for #61, which removes the erc20/ prefix, is merged.
		//		// TODO: Error: base denomination erc20/0x80b5a32E4F032B2a058b4F29EC95EEfEEB87aDcd cannot contain slashes for IBC v2 packet: invalid denomination for cross-chain transfer
		//		nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA)
		//		sourceDenomToTransfer = nativeErc20.Denom
		//		msgAmount = sdkmath.NewIntFromBigInt(nativeErc20.InitialBal)
		//		erc20 = true
		//	},
		//},
		// TODO: registered token pair case, after authz dependency deprecated case, query denom cases
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			// setup between chainA and chainB
			// NOTE:
			// pathAToB.EndpointA = endpoint on chainA
			// pathAToB.EndpointB = endpoint on chainB
			//pathAToB := evmibctesting.NewTransferPath(suite.chainA, suite.chainB)
			pathAToB := evmibctesting.NewPath(suite.chainA, suite.chainB)
			pathAToB.SetupV2()
			traceAToB := transfertypes.NewHop(transfertypes.PortID, pathAToB.EndpointB.ClientID)

			tc.malleate()

			evmApp := suite.chainA.App.(*evmd.EVMD)

			GetBalance := func() sdk.Coin {
				ctx := suite.chainA.GetContext()
				if erc20 {
					balanceAmt := evmApp.Erc20Keeper.BalanceOf(ctx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, nativeErc20.Account)
					return sdk.Coin{
						Denom:  nativeErc20.Denom,
						Amount: sdkmath.NewIntFromBigInt(balanceAmt),
					}
				}
				return evmApp.BankKeeper.GetBalance(
					ctx,
					suite.chainA.SenderAccount.GetAddress(),
					sourceDenomToTransfer,
				)
			}

			originalBalance := GetBalance()
			suite.Require().NoError(err)

			timeoutHeight := clienttypes.NewHeight(1, 110)
			timeoutTimestamp := uint64(suite.chainB.GetContext().BlockTime().Add(time.Hour).Unix())
			originalCoin := sdk.NewCoin(sourceDenomToTransfer, msgAmount)
			sourceAddr := common.BytesToAddress(suite.chainA.SenderAccount.GetAddress().Bytes())

			ctx := suite.chainA.GetContext()
			ctx = evmante.BuildEvmExecutionCtx(ctx).
				WithGasMeter(storetypes.NewInfiniteGasMeter())

			data, err := suite.chainAPrecompile.ABI.Pack("transfer",
				transfertypes.PortID,
				pathAToB.EndpointA.ClientID, // Note: should be client id on v2 packet
				originalCoin.Denom,
				originalCoin.Amount.BigInt(),
				sourceAddr,                                       // Note: source addr should be evm hex addr
				suite.chainB.SenderAccount.GetAddress().String(), // Note: receiver should be cosmos bech32 addr
				timeoutHeight,
				timeoutTimestamp,
				"",
			)
			suite.Require().NoError(err)

			res, err := suite.chainA.SendEvmTx(
				suite.chainA.SenderPrivKey, suite.chainAPrecompile.Address(), big.NewInt(0), data)
			suite.Require().NoError(err) // message committed
			packets, err := pathAToB.EndpointA.ParseV2PacketFromEvent(res.Events)
			suite.Require().NoError(err)

			chainABalanceBeforeRelay := GetBalance()

			transferAmount := msgAmount

			// Note: When an UnboundedSpendLimit value is sent, the spendable amount is used.
			if msgAmount.Equal(transfertypes.UnboundedSpendLimit()) {
				transferAmount = originalBalance.Amount
			}

			// relay send
			err = pathAToB.RelayPacketV2(packets[0])
			suite.Require().NoError(err) // relay committed

			escrowAddress := transfertypes.GetEscrowAddress(
				transfertypes.PortID,
				pathAToB.EndpointA.ClientID,
			)
			// check that the balance for chainA is updated
			chainABalance := evmApp.BankKeeper.GetBalance(
				suite.chainA.GetContext(),
				suite.chainA.SenderAccount.GetAddress(),
				originalCoin.Denom,
			)

			suite.Require().True(chainABalanceBeforeRelay.Amount.Equal(chainABalance.Amount))
			suite.Require().True(originalBalance.Amount.Sub(transferAmount).Equal(chainABalance.Amount))

			// check that module account escrow address has locked the tokens
			chainAEscrowBalance := evmApp.BankKeeper.GetBalance(
				suite.chainA.GetContext(),
				escrowAddress,
				originalCoin.Denom,
			)
			suite.Require().True(transferAmount.Equal(chainAEscrowBalance.Amount))

			// check that voucher exists on chain B
			chainBApp := suite.chainB.GetSimApp()

			chainBDenom := transfertypes.NewDenom(originalCoin.Denom, traceAToB)
			chainBBalance := chainBApp.BankKeeper.GetBalance(
				suite.chainB.GetContext(),
				suite.chainB.SenderAccount.GetAddress(),
				chainBDenom.IBCDenom(),
			)
			coinSentFromAToB := sdk.NewCoin(chainBDenom.IBCDenom(), transferAmount)
			suite.Require().Equal(coinSentFromAToB, chainBBalance)
		})
	}
}

func TestICS20TransferV2TestSuite(t *testing.T) {
	suite.Run(t, new(ICS20TransferV2TestSuite))
}
