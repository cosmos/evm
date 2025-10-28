package ics02

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	_ "embed"

	ibcutils "github.com/cosmos/evm/ibc"
	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ vm.PrecompiledContract = (*Precompile)(nil)

const (
	// TODO: These gas values are placeholders and should be determined through proper benchmarking.

	GasUpdateClient        = 40_000
	GasVerifyMembership    = 15_000
	GasVerifyNonMembership = 15_000
	GasGetClientState      = 4_000
)

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

// Precompile defines the precompiled contract for ICS02.
type Precompile struct {
	cmn.Precompile

	abi.ABI
	clientKeeper ibcutils.ClientKeeper
	// BankKeeper is not used directly in the precompile but is needed for the balance handler.
	BankKeeper cmn.BankKeeper
}

// NewPrecompile creates a new Client Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	bankKeeper cmn.BankKeeper,
	clientKeeper ibcutils.ClientKeeper,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.GasConfig{},
			TransientKVGasConfig:  storetypes.GasConfig{},
			ContractAddress:       common.HexToAddress(evmtypes.ICS02PrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:          ABI,
		clientKeeper: clientKeeper,
		BankKeeper:   bankKeeper,
	}
}

// RequiredGas calculates the contract gas used for the
func (p Precompile) RequiredGas(input []byte) uint64 {
	// NOTE: This check avoid panicking when trying to decode the method ID
	if len(input) < 4 {
		return 0
	}

	methodID := input[:4]
	method, err := p.MethodById(methodID)
	if err != nil {
		return 0
	}

	switch method.Name {
	// ERC-20 transactions
	case UpdateClientMethod:
		return GasUpdateClient
	case VerifyMembershipMethod:
		return GasVerifyMembership
	case VerifyNonMembershipMethod:
		return GasVerifyNonMembership
	// Read-only transactions
	case GetClientStateMethod:
		return GasGetClientState
	default:
		return 0
	}
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

	switch method.Name {
	case UpdateClientMethod:
		return p.UpdateClient(ctx, contract, stateDB, method, args)
	case VerifyMembershipMethod:
		return p.VerifyMembership(ctx, contract, stateDB, method, args)
	case VerifyNonMembershipMethod:
		return p.VerifyNonMembership(ctx, contract, stateDB, method, args)
	// queries:
	case GetClientStateMethod:
		return p.GetClientState(ctx, contract, stateDB, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
func (Precompile) IsTransaction(method *abi.Method) bool {
	// GetClientStateMethod is the only query method.
	return GetClientStateMethod != method.Name
}

// Logger returns a precompile-specific logger.
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "clients")
}
