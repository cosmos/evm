package vm

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	gethvm "github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
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

func (s *KeeperTestSuite) TestEvmHooks() {
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
				s.Require().NoError(result)
				s.Require().Equal(1, len((hook.(*LogRecordHook).Logs)))
			},
		},
		{
			"always fail hook",
			func() types.EvmHooks {
				return &FailureHook{}
			},
			func(_ types.EvmHooks, result error) {
				s.Require().Error(result)
			},
		},
	}

	for _, tc := range testCases {
		s.SetupTest()
		hook := tc.setupHook()
		s.Network.App.GetEVMKeeper().SetHooks(keeper.NewMultiEvmHooks(hook))

		k := s.Network.App.GetEVMKeeper()
		ctx := s.Network.GetContext()
		txHash := common.BigToHash(big.NewInt(1))
		vmdb := statedb.New(ctx, k, statedb.NewTxConfig(
			txHash,
			0,
		))

		vmdb.AddLog(&ethtypes.Log{
			Topics:  []common.Hash{},
			Address: s.Keyring.GetAddr(0),
		})
		logs := vmdb.Logs()
		receipt := &ethtypes.Receipt{
			TxHash: txHash,
			Logs:   logs,
		}
		result := k.PostTxProcessing(ctx, s.Keyring.GetAddr(0), core.Message{}, receipt)

		tc.expFunc(hook, result)
	}
}

func (s *KeeperTestSuite) TestPostTxProcessingFailureLogReversion() {
	s.SetupTest()

	// Set up the failing hook
	hook := &FailureHook{}
	s.Network.App.GetEVMKeeper().SetHooks(keeper.NewMultiEvmHooks(hook))

	k := s.Network.App.GetEVMKeeper()
	ctx := s.Network.GetContext()

	// Fund the sender
	sender := s.Keyring.GetKey(0)
	recipient := s.Keyring.GetAddr(1)
	baseDenom := types.GetEVMCoinDenom()
	coins := sdk.NewCoins(sdk.NewCoin(baseDenom, sdkmath.NewInt(1e18)))
	err := s.Network.App.GetBankKeeper().MintCoins(ctx, "mint", coins)
	s.Require().NoError(err)
	err = s.Network.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, "mint", sender.AccAddr, coins)
	s.Require().NoError(err)

	// Create a simple transfer transaction
	transferArgs := types.EvmTxArgs{
		To:       &recipient,
		Amount:   big.NewInt(100),
		GasLimit: 21000,
		GasPrice: big.NewInt(1000000000),
	}
	tx, err := s.Factory.GenerateSignedEthTx(sender.Priv, transferArgs)
	s.Require().NoError(err)
	msg := tx.GetMsgs()[0].(*types.MsgEthereumTx)

	// Execute transaction - should fail in PostTxProcessing
	res, err := k.EthereumTx(ctx, msg)

	// Verify the transaction execution itself doesn't error, but PostTxProcessing fails
	s.Require().NoError(err, "EthereumTx should not return error")
	s.Require().NotNil(res)
	s.Require().NotEmpty(res.VmError, "Should have VmError due to PostTxProcessing failure")
	s.Require().Contains(res.VmError, "failed to execute post transaction processing")

	// Critical test: Verify logs are completely cleared
	s.Require().Nil(res.Logs, "res.Logs should be nil after PostTxProcessing failure")
}

func (s *KeeperTestSuite) TestPostTxProcessingRevertData() {
	customErr := cmn.NewRevertWithSolidityError(cmn.SharedErrorABI, cmn.SolidityErrInvalidAmount, "hook amount")
	var carrier cmn.RevertDataCarrier
	s.Require().ErrorAs(customErr, &carrier)
	revertData := carrier.RevertData()
	concreteErr := types.NewExecErrorWithReason(revertData)

	testCases := []struct {
		name   string
		err    error
		revert bool
	}{
		{"custom error", customErr, true},
		{"wrapped custom error", fmt.Errorf("outer: %w", customErr), true},
		{"joined custom error", errors.Join(errors.New("other hook error"), customErr), true},
		{"concrete revert error", concreteErr, true},
		{"wrapped concrete revert error", fmt.Errorf("outer: %w", concreteErr), true},
		{"ordinary error", errors.New("hook failed"), false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			k := s.Network.App.GetEVMKeeper()
			ctx := s.Network.GetContext()
			var hookReceipt *ethtypes.Receipt
			k.SetHooks(keeper.NewMultiEvmHooks(&testHooks{
				postProcessing: func(_ sdk.Context, _ common.Address, _ core.Message, receipt *ethtypes.Receipt) error {
					hookReceipt = receipt
					receipt.Logs = []*ethtypes.Log{{Address: s.Keyring.GetAddr(0)}}
					receipt.Bloom = ethtypes.CreateBloom(receipt)
					return tc.err
				},
			}))

			recipient := s.Keyring.GetAddr(1)
			bank := s.Network.App.GetBankKeeper()
			balanceBefore := bank.GetBalance(ctx, s.Keyring.GetAccAddr(1), types.GetEVMCoinDenom())
			tx, err := s.Factory.GenerateSignedEthTx(s.Keyring.GetPrivKey(0), types.EvmTxArgs{
				To:       &recipient,
				Amount:   big.NewInt(100),
				GasLimit: 21000,
				GasPrice: big.NewInt(1000000000),
			})
			s.Require().NoError(err)
			res, err := k.EthereumTx(ctx, tx.GetMsgs()[0].(*types.MsgEthereumTx))
			s.Require().NoError(err)
			s.Require().NotNil(res)
			if tc.revert {
				s.Require().Equal(gethvm.ErrExecutionReverted.Error(), res.VmError)
				s.Require().Equal(revertData, res.Ret)
			} else {
				s.Require().Contains(res.VmError, "failed to execute post transaction processing")
				s.Require().Contains(res.VmError, tc.err.Error())
				s.Require().Empty(res.Ret)
			}
			s.Require().NotNil(hookReceipt)
			s.Require().Nil(res.Logs)
			s.Require().Nil(hookReceipt.Logs)
			s.Require().Equal(ethtypes.Bloom{}, hookReceipt.Bloom)
			s.Require().Equal(balanceBefore, bank.GetBalance(ctx, s.Keyring.GetAccAddr(1), types.GetEVMCoinDenom()))
		})
	}
}
