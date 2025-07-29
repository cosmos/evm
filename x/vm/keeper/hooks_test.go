package keeper_test

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/x/vm/keeper"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// LogRecordHook records all the logs
type LogRecordHook struct {
	Logs []*ethtypes.Log
}

func (dh *LogRecordHook) PostTxProcessing(_ sdk.Context, _ common.Address, _ core.Message, receipt *ethtypes.Receipt) error {
	dh.Logs = receipt.Logs
	return nil
}

// FailureHook always fail
type FailureHook struct{}

func (dh *FailureHook) PostTxProcessing(_ sdk.Context, _ common.Address, _ core.Message, _ *ethtypes.Receipt) error {
	return errors.New("post tx processing failed")
}

func (suite *KeeperTestSuite) TestEvmHooks() {
	testCases := []struct {
		msg       string
		setupHook func() types.EvmHooks
		expFunc   func(hook types.EvmHooks, result error)
	}{
		{
			"log collect hook",
			func() types.EvmHooks {
				return &LogRecordHook{}
			},
			func(hook types.EvmHooks, result error) {
				suite.Require().NoError(result)
				suite.Require().Equal(1, len((hook.(*LogRecordHook).Logs)))
			},
		},
		{
			"always fail hook",
			func() types.EvmHooks {
				return &FailureHook{}
			},
			func(_ types.EvmHooks, result error) {
				suite.Require().Error(result)
			},
		},
	}

	for _, tc := range testCases {
		suite.SetupTest()
		hook := tc.setupHook()
		suite.network.App.EVMKeeper.SetHooks(keeper.NewMultiEvmHooks(hook))

		k := suite.network.App.EVMKeeper
		ctx := suite.network.GetContext()
		txHash := common.BigToHash(big.NewInt(1))
		vmdb := statedb.New(ctx, k, statedb.NewTxConfig(
			common.BytesToHash(ctx.HeaderHash()),
			txHash,
			0,
			0,
		))

		vmdb.AddLog(&ethtypes.Log{
			Topics:  []common.Hash{},
			Address: suite.keyring.GetAddr(0),
		})
		logs := vmdb.Logs()
		receipt := &ethtypes.Receipt{
			TxHash: txHash,
			Logs:   logs,
		}
		result := k.PostTxProcessing(ctx, suite.keyring.GetAddr(0), core.Message{}, receipt)

		tc.expFunc(hook, result)
	}
}

func (suite *KeeperTestSuite) TestPostTxProcessingFailureLogReversion() {
	suite.SetupTest()

	// Set up the failing hook
	hook := &FailureHook{}
	suite.network.App.EVMKeeper.SetHooks(keeper.NewMultiEvmHooks(hook))

	k := suite.network.App.EVMKeeper
	ctx := suite.network.GetContext()

	// Fund the sender
	sender := suite.keyring.GetKey(0)
	recipient := suite.keyring.GetAddr(1)
	baseDenom := types.GetEVMCoinDenom()
	coins := sdk.NewCoins(sdk.NewCoin(baseDenom, sdkmath.NewInt(1e18)))
	err := suite.network.App.BankKeeper.MintCoins(ctx, "mint", coins)
	suite.Require().NoError(err)
	err = suite.network.App.BankKeeper.SendCoinsFromModuleToAccount(ctx, "mint", sender.AccAddr, coins)
	suite.Require().NoError(err)

	// Store original transient state
	originalBloom := k.GetBlockBloomTransient(ctx)
	originalLogSize := k.GetLogSizeTransient(ctx)

	// Create a simple transfer transaction
	transferArgs := types.EvmTxArgs{
		To:       &recipient,
		Amount:   big.NewInt(100),
		GasLimit: 21000,
		GasPrice: big.NewInt(1000000000),
	}
	tx, err := suite.factory.GenerateSignedEthTx(sender.Priv, transferArgs)
	suite.Require().NoError(err)
	msg := tx.GetMsgs()[0].(*types.MsgEthereumTx)

	// Execute transaction - should fail in PostTxProcessing
	res, err := k.EthereumTx(ctx, msg)

	// Verify the transaction execution itself doesn't error, but PostTxProcessing fails
	suite.Require().NoError(err, "EthereumTx should not return error")
	suite.Require().NotNil(res)
	suite.Require().NotEmpty(res.VmError, "Should have VmError due to PostTxProcessing failure")
	suite.Require().Contains(res.VmError, "failed to execute post transaction processing")

	// Critical test: Verify logs are completely cleared
	suite.Require().Nil(res.Logs, "res.Logs should be nil after PostTxProcessing failure")

	// Critical test: Verify transient state was not updated when PostTx failed
	finalBloom := k.GetBlockBloomTransient(ctx)
	finalLogSize := k.GetLogSizeTransient(ctx)

	suite.Require().Equal(originalBloom.String(), finalBloom.String(),
		"BlockBloomTransient should not be updated when PostTxProcessing fails")
	suite.Require().Equal(originalLogSize, finalLogSize,
		"LogSizeTransient should not be updated when PostTxProcessing fails")
}
