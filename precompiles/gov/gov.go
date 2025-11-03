package gov

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	_ "embed"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/core/address"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
)

//go:generate go run github.com/yihuang/go-abi/cmd -input abi.json -output gov.abi.go -external-tuples Coin=cmn.Coin,Dec=cmn.Dec,DecCoin=cmn.DecCoin,PageRequest=cmn.PageRequest,PageResponse=cmn.PageResponse -imports cmn=github.com/cosmos/evm/precompiles/common

var _ vm.PrecompiledContract = &Precompile{}

// Precompile defines the precompiled contract for gov.
type Precompile struct {
	cmn.Precompile

	govMsgServer govtypes.MsgServer
	govQuerier   govtypes.QueryServer
	codec        codec.Codec
	addrCdc      address.Codec
}

// NewPrecompile creates a new gov Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	govMsgServer govtypes.MsgServer,
	govQuerier govtypes.QueryServer,
	bankKeeper cmn.BankKeeper,
	codec codec.Codec,
	addrCdc address.Codec,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(evmtypes.GovPrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		govMsgServer: govMsgServer,
		govQuerier:   govQuerier,
		codec:        codec,
		addrCdc:      addrCdc,
	}
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	methodID, input, err := cmn.SplitMethodID(input)
	if err != nil {
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(methodID))
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
}

func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	methodID, input, err := cmn.ParseMethod(contract.Input, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	switch methodID {
	// gov transactions
	case VoteID:
		return cmn.RunWithStateDB(ctx, p.Vote, input, stateDB, contract)
	case VoteWeightedID:
		return cmn.RunWithStateDB(ctx, p.VoteWeighted, input, stateDB, contract)
	case SubmitProposalID:
		return cmn.RunWithStateDB(ctx, p.SubmitProposal, input, stateDB, contract)
	case DepositID:
		return cmn.RunWithStateDB(ctx, p.Deposit, input, stateDB, contract)
	case CancelProposalID:
		return cmn.RunWithStateDB(ctx, p.CancelProposal, input, stateDB, contract)

	// gov queries
	case GetVoteID:
		return cmn.Run(ctx, p.GetVote, input)
	case GetVotesID:
		return cmn.Run(ctx, p.GetVotes, input)
	case GetDepositID:
		return cmn.Run(ctx, p.GetDeposit, input)
	case GetDepositsID:
		return cmn.Run(ctx, p.GetDeposits, input)
	case GetTallyResultID:
		return cmn.Run(ctx, p.GetTallyResult, input)
	case GetProposalID:
		return cmn.Run(ctx, p.GetProposal, input)
	case GetProposalsID:
		return cmn.Run(ctx, p.GetProposals, input)
	case GetParamsID:
		return cmn.Run(ctx, p.GetParams, input)
	case GetConstitutionID:
		return cmn.Run(ctx, p.GetConstitution, input)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, methodID)
	}
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(methodID uint32) bool {
	switch methodID {
	case VoteID, VoteWeightedID,
		SubmitProposalID, DepositID, CancelProposalID:
		return true
	default:
		return false
	}
}
