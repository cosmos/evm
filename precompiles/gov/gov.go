package gov

import (
	"embed"
	"fmt"
	"github.com/cosmos/evm/x/vm/statedb"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govkeeper "github.com/cosmos/cosmos-sdk/x/gov/keeper"
)

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

// Precompile defines the precompiled contract for gov.
type Precompile struct {
	cmn.Precompile
	govKeeper govkeeper.Keeper
	codec     codec.Codec
}

// LoadABI loads the gov ABI from the embedded abi.json file
// for the gov precompile.
func LoadABI() (abi.ABI, error) {
	return cmn.LoadABI(f, "abi.json")
}

// NewPrecompile creates a new gov Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	govKeeper govkeeper.Keeper,
	codec codec.Codec,
) (*Precompile, error) {
	abi, err := LoadABI()
	if err != nil {
		return nil, err
	}

	p := &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  abi,
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
		},
		govKeeper: govKeeper,
		codec:     codec,
	}

	// SetAddress defines the address of the gov precompiled contract.
	p.SetAddress(common.HexToAddress(evmtypes.GovPrecompileAddress))

	return p, nil
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

// Run executes the precompiled contract gov methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) ([]byte, error) {
	return p.ExecuteWithBalanceHandling(
		evm, contract, readOnly, p.IsTransaction,
		func(ctx sdk.Context, contract *vm.Contract, stateDB *statedb.StateDB, method *abi.Method, args []interface{}) ([]byte, error) {
			switch method.Name {
			// gov transactions
			case VoteMethod:
				return p.Vote(ctx, contract, stateDB, method, args)
			case VoteWeightedMethod:
				return p.VoteWeighted(ctx, contract, stateDB, method, args)
			case SubmitProposalMethod:
				return p.SubmitProposal(ctx, contract, stateDB, method, args)
			case DepositMethod:
				return p.Deposit(ctx, contract, stateDB, method, args)
			case CancelProposalMethod:
				return p.CancelProposal(ctx, contract, stateDB, method, args)

			// gov queries
			case GetVoteMethod:
				return p.GetVote(ctx, method, contract, args)
			case GetVotesMethod:
				return p.GetVotes(ctx, method, contract, args)
			case GetDepositMethod:
				return p.GetDeposit(ctx, method, contract, args)
			case GetDepositsMethod:
				return p.GetDeposits(ctx, method, contract, args)
			case GetTallyResultMethod:
				return p.GetTallyResult(ctx, method, contract, args)
			case GetProposalMethod:
				return p.GetProposal(ctx, method, contract, args)
			case GetProposalsMethod:
				return p.GetProposals(ctx, method, contract, args)
			case GetParamsMethod:
				return p.GetParams(ctx, method, contract, args)
			case GetConstitutionMethod:
				return p.GetConstitution(ctx, method, contract, args)
			default:
				return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
			}
		},
	)
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

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "gov")
}
