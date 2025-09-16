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

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/precompiles/ics20"
	chainutil "github.com/cosmos/evm/testutil"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	evmante "github.com/cosmos/evm/x/vm/ante"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

type ICS20TransferV2TestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics20.Precompile
	chainB           *evmibctesting.TestChain
	chainBPrecompile *ics20.Precompile
}

func (suite *ICS20TransferV2TestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 2, 0, integration.SetupEvmd)
	suite.chainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(2))

	evmAppA := suite.chainA.App.(*evmd.EVMD)
	suite.chainAPrecompile = ics20.NewPrecompile(
		evmAppA.BankKeeper,
		*evmAppA.StakingKeeper,
		evmAppA.TransferKeeper,
		evmAppA.IBCKeeper.ChannelKeeper,
	)
	evmAppB := suite.chainB.App.(*evmd.EVMD)
	suite.chainBPrecompile = ics20.NewPrecompile(
		evmAppB.BankKeeper,
		*evmAppB.StakingKeeper,
		evmAppB.TransferKeeper,
		evmAppB.IBCKeeper.ChannelKeeper,
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
	// originally a basic test case from the IBC testing package, and it has been added as-is to ensure that
	// it still works properly when invoked through the ics20 precompile with ibc v2 packet.
	testCases := []struct {
		name     string
		malleate func(senderAcc evmibctesting.SenderAccount)
	}{
		{
			"transfer single denom",
			func(_ evmibctesting.SenderAccount) {
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount = evmibctesting.DefaultCoinAmount
			},
		},
		{
			"transfer amount larger than int64",
			func(_ evmibctesting.SenderAccount) {
				var ok bool
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount, ok = sdkmath.NewIntFromString("9223372036854775808") // 2^63 (one above int64)
				suite.Require().True(ok)
			},
		},
		{
			"transfer entire balance",
			func(_ evmibctesting.SenderAccount) {
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount = transfertypes.UnboundedSpendLimit()
			},
		},
		{
			"native erc20 case",
			func(senderAcc evmibctesting.SenderAccount) {
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA, senderAcc)
				sourceDenomToTransfer = nativeErc20.Denom
				msgAmount = sdkmath.NewIntFromBigInt(nativeErc20.InitialBal)
				erc20 = true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			// setup between chainA and chainB
			// NOTE:
			// pathAToB.EndpointA = endpoint on chainA
			// pathAToB.EndpointB = endpoint on chainB
			// pathAToB := evmibctesting.NewTransferPath(suite.chainA, suite.chainB)
			pathAToB := evmibctesting.NewPath(suite.chainA, suite.chainB)
			pathAToB.SetupV2()
			traceAToB := transfertypes.NewHop(transfertypes.PortID, pathAToB.EndpointB.ClientID)

			senderIdx := 1
			senderAccount := suite.chainA.SenderAccounts[senderIdx]
			senderAddr := senderAccount.SenderAccount.GetAddress()

			tc.malleate(senderAccount)

			evmAppA := suite.chainA.App.(*evmd.EVMD)

			GetBalance := func(addr sdk.AccAddress) sdk.Coin {
				ctx := suite.chainA.GetContext()
				if erc20 {
					balanceAmt := evmAppA.Erc20Keeper.BalanceOf(ctx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, nativeErc20.Account)
					return sdk.Coin{
						Denom:  nativeErc20.Denom,
						Amount: sdkmath.NewIntFromBigInt(balanceAmt),
					}
				}
				return evmAppA.BankKeeper.GetBalance(ctx, addr, sourceDenomToTransfer)
			}

			senderBalance := GetBalance(senderAddr)
			suite.Require().NoError(err)

			timeoutHeight := clienttypes.NewHeight(1, 110)
			timeoutTimestamp := uint64(suite.chainB.GetContext().BlockTime().Add(time.Hour).Unix()) //nolint:gosec // G115
			originalCoin := sdk.NewCoin(sourceDenomToTransfer, msgAmount)

			data, err := suite.chainAPrecompile.Pack("transfer",
				transfertypes.PortID,
				pathAToB.EndpointA.ClientID, // Note: should be client id on v2 packet
				originalCoin.Denom,
				originalCoin.Amount.BigInt(),
				common.BytesToAddress(senderAddr.Bytes()),        // Note: source addr should be evm hex addr
				suite.chainB.SenderAccount.GetAddress().String(), // Note: receiver should be cosmos bech32 addr
				timeoutHeight,
				timeoutTimestamp,
				"",
			)
			suite.Require().NoError(err)

			res, _, _, err := suite.chainA.SendEvmTx(senderAccount, senderIdx, suite.chainAPrecompile.Address(), big.NewInt(0), data, 0)
			suite.Require().NoError(err) // message committed
			packets, err := pathAToB.EndpointA.ParseV2PacketFromEvent(res.Events)
			suite.Require().NoError(err)
			transferAmount := msgAmount

			// check that the balance for chainA is updated
			afterSenderBalance := evmAppA.BankKeeper.GetBalance(suite.chainA.GetContext(), senderAddr, originalCoin.Denom)
			// Note: When an UnboundedSpendLimit value is sent, the spendable amount is used.
			if msgAmount.Equal(transfertypes.UnboundedSpendLimit()) {
				transferAmount = senderBalance.Amount
			}
			suite.Require().Equal(
				senderBalance.Amount.Sub(transferAmount).String(),
				afterSenderBalance.Amount.String(),
			)
			if msgAmount.Equal(transfertypes.UnboundedSpendLimit()) {
				suite.Require().True(afterSenderBalance.IsZero())
			}

			relayerAddr := suite.chainA.SenderAccounts[0].SenderAccount.GetAddress()
			relayerBalance := GetBalance(relayerAddr)

			// relay send
			err = pathAToB.RelayPacketV2(packets[0])
			suite.Require().NoError(err) // relay committed

			// There are two msgs that are sent in the relay:
			// 1. MsgAcknowledgePacket to acknowledge the packet
			// 2. Counterparty.UpdateClient to update the client
			// Both of these msgs incur a fee, so we need to account for that.
			relayPacketV2Fee := evmibctesting.FeeCoins().AmountOf(originalCoin.Denom).Mul(sdkmath.NewInt(2))
			afterRelayerBalance := GetBalance(relayerAddr)
			suite.Require().Equal(
				relayerBalance.Amount.Sub(relayPacketV2Fee).String(),
				afterRelayerBalance.Amount.String(),
			)

			escrowAddress := transfertypes.GetEscrowAddress(
				transfertypes.PortID,
				pathAToB.EndpointA.ClientID,
			)

			// check that module account escrow address has locked the tokens
			chainAEscrowBalance := evmAppA.BankKeeper.GetBalance(
				suite.chainA.GetContext(),
				escrowAddress,
				originalCoin.Denom,
			)
			suite.Require().True(transferAmount.Equal(chainAEscrowBalance.Amount))

			// check that voucher exists on chain B
			evmAppB := suite.chainB.App.(*evmd.EVMD)
			chainBDenom := transfertypes.NewDenom(originalCoin.Denom, traceAToB)
			chainBBalance := evmAppB.BankKeeper.GetBalance(
				suite.chainB.GetContext(),
				suite.chainB.SenderAccount.GetAddress(),
				chainBDenom.IBCDenom(),
			)
			coinSentFromAToB := sdk.NewCoin(chainBDenom.IBCDenom(), transferAmount)
			suite.Require().Equal(coinSentFromAToB, chainBBalance)

			// ---------------------------------------------
			// Tests for Query endpoints of ICS20 precompile
			// denoms query method
			chainBAddr := common.BytesToAddress(suite.chainB.SenderAccount.GetAddress().Bytes())
			ctxB := evmante.BuildEvmExecutionCtx(suite.chainB.GetContext())
			evmRes, err := evmAppB.EVMKeeper.CallEVM(
				ctxB,
				suite.chainBPrecompile.ABI,
				chainBAddr,
				suite.chainBPrecompile.Address(),
				false,
				nil,
				ics20.DenomsMethod,
				query.PageRequest{
					Key:        []byte{},
					Offset:     0,
					Limit:      0,
					CountTotal: false,
					Reverse:    false,
				},
			)
			suite.Require().NoError(err)
			var denomsResponse ics20.DenomsResponse
			err = suite.chainBPrecompile.UnpackIntoInterface(&denomsResponse, ics20.DenomsMethod, evmRes.Ret)
			suite.Require().NoError(err)
			suite.Require().Equal(chainBDenom, denomsResponse.Denoms[0])

			// denom query method
			evmRes, err = evmAppB.EVMKeeper.CallEVM(
				ctxB,
				suite.chainBPrecompile.ABI,
				chainBAddr,
				suite.chainBPrecompile.Address(),
				false,
				nil,
				ics20.DenomMethod,
				chainBDenom.Hash().String(),
			)
			suite.Require().NoError(err)
			var denomResponse ics20.DenomResponse
			err = suite.chainBPrecompile.UnpackIntoInterface(&denomResponse, ics20.DenomMethod, evmRes.Ret)
			suite.Require().NoError(err)
			suite.Require().Equal(chainBDenom, denomResponse.Denom)

			// denom query method not exists case
			evmRes, err = evmAppB.EVMKeeper.CallEVM(
				ctxB,
				suite.chainBPrecompile.ABI,
				chainBAddr,
				suite.chainBPrecompile.Address(),
				false,
				nil,
				ics20.DenomMethod,
				"0000000000000000000000000000000000000000000000000000000000000000",
			)
			suite.Require().NoError(err)
			err = suite.chainBPrecompile.UnpackIntoInterface(&denomResponse, ics20.DenomMethod, evmRes.Ret)
			suite.Require().NoError(err)
			// ensure empty denom struct when not exist
			suite.Require().Equal(denomResponse.Denom, transfertypes.Denom{Base: "", Trace: []transfertypes.Hop{}})

			// denom query method invalid error case
			evmRes, err = evmAppB.EVMKeeper.CallEVM(
				ctxB,
				suite.chainBPrecompile.ABI,
				chainBAddr,
				suite.chainBPrecompile.Address(),
				false,
				nil,
				ics20.DenomMethod,
				"INVALID-DENOM-HASH",
			)
			suite.Require().ErrorContains(err, vm.ErrExecutionReverted.Error())
			revertErr := chainutil.DecodeRevertReason(*evmRes)
			suite.Require().Contains(revertErr.Error(), "invalid denom trace hash")
			ctxB.GasMeter().RefundGas(ctxB.GasMeter().Limit(), "refund after error")

			// denomHash query method
			evmRes, err = evmAppB.EVMKeeper.CallEVM(
				ctxB,
				suite.chainBPrecompile.ABI,
				chainBAddr,
				suite.chainBPrecompile.Address(),
				false,
				nil,
				ics20.DenomHashMethod,
				chainBDenom.Path(),
			)
			suite.Require().NoError(err)
			var denomHashResponse transfertypes.QueryDenomHashResponse
			err = suite.chainBPrecompile.UnpackIntoInterface(&denomHashResponse, ics20.DenomHashMethod, evmRes.Ret)
			suite.Require().NoError(err)
			suite.Require().Equal(chainBDenom.Hash().String(), denomHashResponse.Hash)

			// denomHash query method not exists case
			evmRes, err = evmAppB.EVMKeeper.CallEVM(
				ctxB,
				suite.chainBPrecompile.ABI,
				chainBAddr,
				suite.chainBPrecompile.Address(),
				false,
				nil,
				ics20.DenomHashMethod,
				"transfer/channel-0/erc20:not-exists-case",
			)
			suite.Require().NoError(err)
			err = suite.chainBPrecompile.UnpackIntoInterface(&denomHashResponse, ics20.DenomHashMethod, evmRes.Ret)
			suite.Require().NoError(err)
			suite.Require().Equal(denomHashResponse.Hash, "")

			// denomHash query method invalid error case
			evmRes, err = evmAppB.EVMKeeper.CallEVM(
				ctxB,
				suite.chainBPrecompile.ABI,
				chainBAddr,
				suite.chainBPrecompile.Address(),
				false,
				nil,
				ics20.DenomHashMethod,
				"",
			)
			suite.Require().ErrorContains(err, vm.ErrExecutionReverted.Error())
			revertErr = chainutil.DecodeRevertReason(*evmRes)
			suite.Require().Contains(revertErr.Error(), "invalid denomination for cross-chain transfer")
			ctxB.GasMeter().RefundGas(ctxB.GasMeter().Limit(), "refund after error")
		})
	}
}

func TestICS20TransferV2TestSuite(t *testing.T) {
	suite.Run(t, new(ICS20TransferV2TestSuite))
}
