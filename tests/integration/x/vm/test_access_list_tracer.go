package vm

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm/server/config"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestAccessListTracer runs TraceCall with the access list tracer used by
// eth_createAccessList against a contract that reads another account's balance
// and one of its own storage slots.
func (s *KeeperTestSuite) TestAccessListTracer() {
	s.SetupTest()
	k := s.Network.App.GetEVMKeeper()
	ctx := s.Network.GetContext()

	touched := common.HexToAddress("0x1111111111111111111111111111111111111111")
	contract := common.HexToAddress("0x2222222222222222222222222222222222222222")
	slot := common.BigToHash(common.Big1)

	// PUSH20 <touched> BALANCE POP PUSH1 0x01 SLOAD POP STOP
	code := append(append([]byte{byte(vm.PUSH20)}, touched.Bytes()...),
		byte(vm.BALANCE), byte(vm.POP), byte(vm.PUSH1), 0x01, byte(vm.SLOAD), byte(vm.POP), byte(vm.STOP))
	codeHash := crypto.Keccak256(code)
	s.Require().NoError(k.SetAccount(ctx, contract, statedb.Account{Nonce: 1, CodeHash: codeHash}))
	k.SetCode(ctx, codeHash, code)

	reverting := common.HexToAddress("0x3333333333333333333333333333333333333333")
	// PUSH1 0x00 PUSH1 0x00 REVERT
	revertCode := []byte{byte(vm.PUSH1), 0x00, byte(vm.PUSH1), 0x00, byte(vm.REVERT)}
	revertCodeHash := crypto.Keccak256(revertCode)
	s.Require().NoError(k.SetAccount(ctx, reverting, statedb.Account{Nonce: 1, CodeHash: revertCodeHash}))
	k.SetCode(ctx, revertCodeHash, revertCode)

	from := s.Keyring.GetAddr(0)
	traceTo := func(to common.Address, accessList ethtypes.AccessList) types.AccessListTracerResult {
		args, err := json.Marshal(&types.TransactionArgs{From: &from, To: &to, AccessList: &accessList})
		s.Require().NoError(err)
		tracerConfig, err := json.Marshal(types.AccessListTracerConfig{
			AccessList: accessList,
			Excludes:   []common.Address{from, to},
		})
		s.Require().NoError(err)

		res, err := k.TraceCall(ctx, &types.QueryTraceCallRequest{
			Args:            args,
			GasCap:          config.DefaultGasCap,
			TraceConfig:     &types.TraceConfig{Tracer: types.AccessListTracerName, TracerJsonConfig: string(tracerConfig)},
			BlockNumber:     ctx.BlockHeight(),
			BlockTime:       ctx.BlockTime(),
			BlockHash:       common.BytesToHash(ctx.HeaderHash()).Hex(),
			ProposerAddress: sdk.ConsAddress(ctx.BlockHeader().ProposerAddress),
			ChainId:         s.Network.GetEIP155ChainID().Int64(),
		})
		s.Require().NoError(err)

		var result types.AccessListTracerResult
		s.Require().NoError(json.Unmarshal(res.Data, &result))
		return result
	}
	trace := func(accessList ethtypes.AccessList) types.AccessListTracerResult {
		return traceTo(contract, accessList)
	}

	expected := ethtypes.AccessList{
		{Address: touched, StorageKeys: []common.Hash{}},
		{Address: contract, StorageKeys: []common.Hash{slot}},
	}

	// the first run collects the accessed account and slot
	first := trace(nil)
	s.Require().Empty(first.Error)
	s.Require().ElementsMatch(expected, first.AccessList)
	s.Require().NotZero(first.GasUsed)

	// running again with that list returns the same list (what eth_createAccessList iterates on)
	second := trace(first.AccessList)
	s.Require().Empty(second.Error)
	s.Require().ElementsMatch(expected, second.AccessList)
	s.Require().NotZero(second.GasUsed)

	// the EVM error of the call is reported with the list
	reverted := traceTo(reverting, nil)
	s.Require().Equal(vm.ErrExecutionReverted.Error(), reverted.Error)
	s.Require().Empty(reverted.AccessList)
}
