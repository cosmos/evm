package slashing

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

	sdk "github.com/cosmos/cosmos-sdk/types"
	slashingkeeper "github.com/cosmos/cosmos-sdk/x/slashing/keeper"
)

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

// Precompile defines the precompiled contract for slashing.
type Precompile struct {
	cmn.Precompile
	slashingKeeper slashingkeeper.Keeper
}

// LoadABI loads the slashing ABI from the embedded abi.json file
// for the slashing precompile.
func LoadABI() (abi.ABI, error) {
	return cmn.LoadABI(f, "abi.json")
}

// NewPrecompile creates a new slashing Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	slashingKeeper slashingkeeper.Keeper,
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
		slashingKeeper: slashingKeeper,
	}

	// SetAddress defines the address of the slashing precompiled contract.
	p.SetAddress(common.HexToAddress(evmtypes.SlashingPrecompileAddress))

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

// Run executes the precompiled contract slashing methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) ([]byte, error) {
	return p.ExecuteWithBalanceHandling(
		evm, contract, readOnly, p.IsTransaction,
		func(ctx sdk.Context, contract *vm.Contract, stateDB *statedb.StateDB, method *abi.Method, args []interface{}) ([]byte, error) {
			switch method.Name {
			// slashing transactions
			case UnjailMethod:
				return p.Unjail(ctx, method, stateDB, contract, args)
			// slashing queries
			case GetSigningInfoMethod:
				return p.GetSigningInfo(ctx, method, contract, args)
			case GetSigningInfosMethod:
				return p.GetSigningInfos(ctx, method, contract, args)
			default:
				return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
			}
		},
	)
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available slashing transactions are:
// - Unjail
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case UnjailMethod:
		return true
	default:
		return false
	}
}

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "slashing")
}
