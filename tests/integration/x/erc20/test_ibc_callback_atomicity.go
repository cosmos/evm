package erc20

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"
	transfertypes "github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"
	channeltypes "github.com/cosmos/ibc-go/v11/modules/core/04-channel/types"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestConvertCoinToERC20FromPacketFailureIsAtomic checks that when the
// ERC20 reconversion performed by the timeout / error-ack callbacks fails and
// the callback returns nil, the callback leaves no partial writes: the sender
// keeps the refunded bank coin, the erc20 module account holds no extra coins,
// and ERC20 balances are unchanged.
func (s *KeeperTestSuite) TestConvertCoinToERC20FromPacketFailureIsAtomic() {
	const amount = int64(10)

	type snapshot struct {
		senderCoin  math.Int
		moduleCoin  math.Int
		supply      math.Int
		senderERC20 *big.Int
		moduleERC20 *big.Int
	}

	testCases := []struct {
		name         string
		contractType int
		// afterFund runs after the module and sender are funded and before the callback.
		afterFund func(ctx sdk.Context, contract common.Address)
		callback  func(ctx sdk.Context, data transfertypes.FungibleTokenPacketData) error
	}{
		{
			name:         "timeout - token transfer reverts (paused token)",
			contractType: contractMinterBurner,
			afterFund: func(ctx sdk.Context, contract common.Address) {
				stateDB := statedb.New(ctx, s.network.App.GetEVMKeeper(), statedb.NewEmptyTxConfig())
				_, err := s.network.App.GetEVMKeeper().CallEVM(ctx, stateDB, contracts.ERC20MinterBurnerDecimalsContract.ABI, s.keyring.GetAddr(0), contract, true, false, nil, "pause")
				s.Require().NoError(err)
			},
			callback: func(ctx sdk.Context, data transfertypes.FungibleTokenPacketData) error {
				return s.network.App.GetErc20Keeper().OnTimeoutPacket(ctx, channeltypes.Packet{}, data)
			},
		},
		{
			name:         "error ack - token transfer reverts (paused token)",
			contractType: contractMinterBurner,
			afterFund: func(ctx sdk.Context, contract common.Address) {
				stateDB := statedb.New(ctx, s.network.App.GetEVMKeeper(), statedb.NewEmptyTxConfig())
				_, err := s.network.App.GetEVMKeeper().CallEVM(ctx, stateDB, contracts.ERC20MinterBurnerDecimalsContract.ABI, s.keyring.GetAddr(0), contract, true, false, nil, "pause")
				s.Require().NoError(err)
			},
			callback: func(ctx sdk.Context, data transfertypes.FungibleTokenPacketData) error {
				ack := channeltypes.NewErrorAcknowledgement(fmt.Errorf("error ack"))
				return s.network.App.GetErc20Keeper().OnAcknowledgementPacket(ctx, channeltypes.Packet{}, data, ack)
			},
		},
		{
			name:         "timeout - post-transfer balance check fails",
			contractType: contractDirectBalanceManipulation,
			afterFund:    func(sdk.Context, common.Address) {},
			callback: func(ctx sdk.Context, data transfertypes.FungibleTokenPacketData) error {
				return s.network.App.GetErc20Keeper().OnTimeoutPacket(ctx, channeltypes.Packet{}, data)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()

			contractAddr, err := s.setupRegisterERC20Pair(tc.contractType)
			s.Require().NoError(err)

			ctx := s.network.GetContext()
			erc20Keeper := s.network.App.GetErc20Keeper()
			bankKeeper := s.network.App.GetBankKeeper()
			erc20ABI := contracts.ERC20MinterBurnerDecimalsContract.ABI

			id := erc20Keeper.GetTokenPairID(ctx, contractAddr.String())
			pair, found := erc20Keeper.GetTokenPair(ctx, id)
			s.Require().True(found)
			s.Require().True(pair.IsNativeERC20())

			senderPk := secp256k1.GenPrivKey()
			sender := sdk.AccAddress(senderPk.PubKey().Address())
			moduleAddr := s.network.App.GetAccountKeeper().GetModuleAddress(types.ModuleName)

			// ERC20 held by the module (escrowed on the outbound conversion) and
			// the refunded bank coin held by the sender.
			stateDB := statedb.New(ctx, s.network.App.GetEVMKeeper(), statedb.NewEmptyTxConfig())
			_, err = s.network.App.GetEVMKeeper().CallEVM(ctx, stateDB, erc20ABI, s.keyring.GetAddr(0), contractAddr, true, false, nil, "mint", types.ModuleAddress, big.NewInt(100))
			s.Require().NoError(err)
			err = testutil.FundAccount(ctx, bankKeeper, sender, sdk.NewCoins(sdk.NewCoin(pair.Denom, math.NewInt(100))))
			s.Require().NoError(err)

			tc.afterFund(ctx, contractAddr)

			// Reads use a fresh gas meter: a reverted EVM call consumes the
			// remaining gas of the context it ran in.
			take := func() snapshot {
				ctx := ctx.WithGasMeter(storetypes.NewInfiniteGasMeter())
				return snapshot{
					senderCoin:  bankKeeper.GetBalance(ctx, sender, pair.Denom).Amount,
					moduleCoin:  bankKeeper.GetBalance(ctx, moduleAddr, pair.Denom).Amount,
					supply:      bankKeeper.GetSupply(ctx, pair.Denom).Amount,
					senderERC20: erc20Keeper.BalanceOf(ctx, erc20ABI, contractAddr, common.BytesToAddress(sender.Bytes())),
					moduleERC20: erc20Keeper.BalanceOf(ctx, erc20ABI, contractAddr, types.ModuleAddress),
				}
			}

			before := take()
			s.Require().NotNil(before.senderERC20)
			s.Require().NotNil(before.moduleERC20)

			data := transfertypes.NewFungibleTokenPacketData(pair.Denom, math.NewInt(amount).String(), sender.String(), sender.String(), "")
			err = tc.callback(ctx, data)
			s.Require().NoError(err, "callback swallows the conversion error")

			var sawFailedEvent bool
			for _, ev := range ctx.EventManager().Events() {
				if ev.Type == types.EventTypeFailedConvertERC20 {
					sawFailedEvent = true
				}
			}
			s.Require().True(sawFailedEvent, "expected conversion to fail and emit %s", types.EventTypeFailedConvertERC20)

			after := take()
			s.T().Logf("before: senderCoin=%s moduleCoin=%s supply=%s senderERC20=%s moduleERC20=%s; after: senderCoin=%s moduleCoin=%s supply=%s senderERC20=%s moduleERC20=%s", before.senderCoin, before.moduleCoin, before.supply, before.senderERC20, before.moduleERC20, after.senderCoin, after.moduleCoin, after.supply, after.senderERC20, after.moduleERC20)
			s.Require().Equal(before.senderCoin.String(), after.senderCoin.String(), "sender bank balance must be unchanged after a failed conversion")
			s.Require().Equal(before.moduleCoin.String(), after.moduleCoin.String(), "erc20 module bank balance must be unchanged after a failed conversion")
			s.Require().Equal(before.supply.String(), after.supply.String(), "bank supply must be unchanged after a failed conversion")
			s.Require().Equal(before.senderERC20.String(), after.senderERC20.String(), "sender ERC20 balance must be unchanged after a failed conversion")
			s.Require().Equal(before.moduleERC20.String(), after.moduleERC20.String(), "module ERC20 balance must be unchanged after a failed conversion")
		})
	}
}
