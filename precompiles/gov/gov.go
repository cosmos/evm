package gov

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
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

var _ vm.PrecompiledContract = &Precompile{}

var (
	// Embed abi json file to the executable binary. Needed when importing as dependency.
	//
	//go:embed abi.json
	f   []byte
	ABI abi.ABI
)

func init() {
	var err error
	ABI, err = abi.JSON(bytes.NewReader(f))
	if err != nil {
		panic(err)
	}
}

// Precompile defines the precompiled contract for gov.
type Precompile struct {
	cmn.Precompile

	abi.ABI
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
		ABI:          ABI,
		govMsgServer: govMsgServer,
		govQuerier:   govQuerier,
		codec:        codec,
		addrCdc:      addrCdc,
	}
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(input) < 4 {
		return 0
	}
	methodID := input[:4]

	method, err := p.MethodById(methodID)
	if err != nil {
		// This should never happen since this method is going to fail during Run
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
}

func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	method, args, err := cmn.SetupABI(p.ABI, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	var bz []byte

	switch method.Name {
	// gov transactions
	case VoteMethod:
		bz, err = p.Vote(ctx, contract, stateDB, method, args)
	case VoteWeightedMethod:
		bz, err = p.VoteWeighted(ctx, contract, stateDB, method, args)
	case SubmitProposalMethod:
		bz, err = p.SubmitProposal(ctx, contract, stateDB, method, args)
	case DepositMethod:
		bz, err = p.Deposit(ctx, contract, stateDB, method, args)
	case CancelProposalMethod:
		bz, err = p.CancelProposal(ctx, contract, stateDB, method, args)

	// gov queries
	case GetVoteMethod:
		bz, err = p.GetVote(ctx, method, contract, args)
	case GetVotesMethod:
		bz, err = p.GetVotes(ctx, method, contract, args)
	case GetDepositMethod:
		bz, err = p.GetDeposit(ctx, method, contract, args)
	case GetDepositsMethod:
		bz, err = p.GetDeposits(ctx, method, contract, args)
	case GetTallyResultMethod:
		bz, err = p.GetTallyResult(ctx, method, contract, args)
	case GetProposalMethod:
		bz, err = p.GetProposal(ctx, method, contract, args)
	case GetProposalsMethod:
		bz, err = p.GetProposals(ctx, method, contract, args)
	case GetParamsMethod:
		bz, err = p.GetParams(ctx, method, contract, args)
	case GetConstitutionMethod:
		bz, err = p.GetConstitution(ctx, method, contract, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case VoteMethod, VoteWeightedMethod,
		SubmitProposalMethod, DepositMethod, CancelProposalMethod:
		return true
	default:
		return false
	}
}
