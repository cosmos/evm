// Copied from https://github.com/cosmos/ibc-go/blob/7325bd2b00fd5e33d895770ec31b5be2f497d37a/modules/apps/transfer/transfer_test.go
// Why was this copied?
// This test suite was imported to validate that ExampleChain (an EVM-based chain)
// correctly supports IBC v1 token transfers using ibc-go’s Transfer module logic.
// The test ensures that ics20 precompile transfer (A → B) behave as expected across channels.
package ibc

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/precompiles/ics20"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	evmante "github.com/cosmos/evm/x/vm/ante"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
)

type ICS20TransferTestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	// testing chains used for convenience and readability
	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics20.Precompile
	chainB           *evmibctesting.TestChain
	chainBPrecompile *ics20.Precompile
}

func (suite *ICS20TransferTestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 2, 0)
	suite.chainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(2))

	evmAppA := suite.chainA.App.(*evmd.EVMD)
	suite.chainAPrecompile, _ = ics20.NewPrecompile(
		*evmAppA.StakingKeeper,
		evmAppA.TransferKeeper,
		evmAppA.IBCKeeper.ChannelKeeper,
		evmAppA.EVMKeeper,
	)
	evmAppB := suite.chainB.App.(*evmd.EVMD)
	suite.chainBPrecompile, _ = ics20.NewPrecompile(
		*evmAppB.StakingKeeper,
		evmAppB.TransferKeeper,
		evmAppB.IBCKeeper.ChannelKeeper,
		evmAppB.EVMKeeper,
	)
}

// Constructs the following sends based on the established channels/connections
// 1 - from evmChainA to chainB
func (suite *ICS20TransferTestSuite) TestHandleMsgTransfer() {
	var (
		sourceDenomToTransfer string
		msgAmount             sdkmath.Int
		err                   error
		nativeErc20           *NativeErc20Info
		erc20                 bool
	)

	// originally a basic test case from the IBC testing package, and it has been added as-is to ensure that
	// it still works properly when invoked through the ics20 precompile.
	testCases := []struct {
		name     string
		malleate func()
	}{
		{
			"transfer single denom",
			func() {
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount = evmibctesting.DefaultCoinAmount
			},
		},
		{
			"transfer amount larger than int64",
			func() {
				var ok bool
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount, ok = sdkmath.NewIntFromString("9223372036854775808") // 2^63 (one above int64)
				suite.Require().True(ok)
			},
		},
		{
			"transfer entire balance",
			func() {
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				sourceDenomToTransfer, err = evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
				msgAmount = transfertypes.UnboundedSpendLimit()
			},
		},
		{
			"native erc20 case",
			func() {
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA)
				sourceDenomToTransfer = nativeErc20.Denom
				msgAmount = sdkmath.NewIntFromBigInt(nativeErc20.InitialBal)
				erc20 = true
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			// setup between evmChainA and chainB
			// NOTE:
			// pathAToB.EndpointA = endpoint on evmChainA
			// pathAToB.EndpointB = endpoint on chainB
			pathAToB := evmibctesting.NewTransferPath(suite.chainA, suite.chainB)
			pathAToB.Setup()
			traceAToB := transfertypes.NewHop(pathAToB.EndpointB.ChannelConfig.PortID, pathAToB.EndpointB.ChannelID)

			tc.malleate()

			evmAppA := suite.chainA.App.(*evmd.EVMD)

			GetBalance := func() sdk.Coin {
				ctx := suite.chainA.GetContext()
				if erc20 {
					balanceAmt := evmAppA.Erc20Keeper.BalanceOf(ctx, nativeErc20.ContractAbi, nativeErc20.ContractAddr, nativeErc20.Account)
					return sdk.Coin{
						Denom:  nativeErc20.Denom,
						Amount: sdkmath.NewIntFromBigInt(balanceAmt),
					}
				}
				return evmAppA.BankKeeper.GetBalance(
					ctx,
					suite.chainA.SenderAccount.GetAddress(),
					sourceDenomToTransfer,
				)
			}

			originalBalance := GetBalance()
			suite.Require().NoError(err)

			timeoutHeight := clienttypes.NewHeight(1, 110)
			originalCoin := sdk.NewCoin(sourceDenomToTransfer, msgAmount)
			sourceAddr := common.BytesToAddress(suite.chainA.SenderAccount.GetAddress().Bytes())

			data, err := suite.chainAPrecompile.Pack("transfer",
				pathAToB.EndpointA.ChannelConfig.PortID,
				pathAToB.EndpointA.ChannelID,
				originalCoin.Denom,
				originalCoin.Amount.BigInt(),
				sourceAddr,                                       // source addr should be evm hex addr
				suite.chainB.SenderAccount.GetAddress().String(), // receiver should be cosmos bech32 addr
				timeoutHeight,
				uint64(0),
				"",
			)
			suite.Require().NoError(err)

			res, err := suite.chainA.SendEvmTx(
				suite.chainA.SenderPrivKey, suite.chainAPrecompile.Address(), big.NewInt(0), data)
			suite.Require().NoError(err) // message committed

			packet, err := evmibctesting.ParsePacketFromEvents(res.Events)
			suite.Require().NoError(err)

			// Get the packet data to determine the amount of tokens being transferred (needed for sending entire balance)
			packetData, err := transfertypes.UnmarshalPacketData(packet.GetData(), pathAToB.EndpointA.GetChannel().Version, "")
			suite.Require().NoError(err)
			transferAmount, ok := sdkmath.NewIntFromString(packetData.Token.Amount)
			suite.Require().True(ok)

			chainABalanceBeforeRelay := GetBalance()

			// relay send
			err = pathAToB.RelayPacket(packet)
			suite.Require().NoError(err) // relay committed

			escrowAddress := transfertypes.GetEscrowAddress(packet.GetSourcePort(), packet.GetSourceChannel())
			// check that the balance for evmChainA is updated
			chainABalance := evmAppA.BankKeeper.GetBalance(
				suite.chainA.GetContext(),
				suite.chainA.SenderAccount.GetAddress(),
				originalCoin.Denom,
			)

			suite.Require().True(chainABalanceBeforeRelay.Amount.Equal(chainABalance.Amount))
			suite.Require().True(originalBalance.Amount.Sub(transferAmount).Equal(chainABalance.Amount))

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

			// denom query method with result
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
			_, err = evmAppB.EVMKeeper.CallEVM(
				ctxB,
				suite.chainBPrecompile.ABI,
				chainBAddr,
				suite.chainBPrecompile.Address(),
				false,
				nil,
				ics20.DenomMethod,
				"INVALID-DENOM-HASH",
			)
			suite.Require().ErrorContains(err, "invalid denom trace hash")

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
			_, err = evmAppB.EVMKeeper.CallEVM(
				ctxB,
				suite.chainBPrecompile.ABI,
				chainBAddr,
				suite.chainBPrecompile.Address(),
				false,
				nil,
				ics20.DenomHashMethod,
				"",
			)
			suite.Require().ErrorContains(err, "invalid denomination for cross-chain transfer")
		})
	}
}

func TestICS20TransferTestSuite(t *testing.T) {
	suite.Run(t, new(ICS20TransferTestSuite))
}
