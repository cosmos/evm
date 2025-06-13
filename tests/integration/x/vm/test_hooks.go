package vm

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/x/vm/keeper"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// LogRecordHook records all the logs
type LogRecordHook struct {
	Logs []*ethtypes.Log
}

func (dh *LogRecordHook) PostTxProcessing(ctx sdk.Context, sender common.Address, msg core.Message, receipt *ethtypes.Receipt) error {
	dh.Logs = receipt.Logs
	return nil
}

// FailureHook always fail
type FailureHook struct{}

func (dh FailureHook) PostTxProcessing(ctx sdk.Context, sender common.Address, msg core.Message, receipt *ethtypes.Receipt) error {
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
			func(hook types.EvmHooks, result error) {
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
			common.BytesToHash(ctx.HeaderHash()),
			txHash,
			0,
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
