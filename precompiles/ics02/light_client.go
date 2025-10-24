package ics02

import (
	"embed"
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	ibcutils "github.com/cosmos/evm/ibc"
	cmn "github.com/cosmos/evm/precompiles/common"
	clientstypes "github.com/cosmos/evm/x/ibc/clients/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ vm.PrecompiledContract = (*Precompile)(nil)

const (
	// abiPath defines the path to the LightClient precompile ABI JSON file.
	abiPath = "abi.json"

	// TODO: These gas values are placeholders and should be determined through proper benchmarking.

	GasUpdateClient        = 40_000
	GasVerifyMembership    = 15_000
	GasVerifyNonMembership = 15_000
	GasMisbehaviour        = 50_000
	GasGetClientState      = 4_000
)

var (
	// Embed abi json file to the executable binary. Needed when importing as dependency.
	//
	//go:embed abi.json
	f   embed.FS
	ABI abi.ABI
)

func init() {
	var err error
	ABI, err = cmn.LoadABI(f, abiPath)
	if err != nil {
		panic(err)
	}
}

// LoadABI loads the ILightClient ABI from the embedded abi.json file
// for the ics02 precompile.
func LoadABI() (abi.ABI, error) {
	return cmn.LoadABI(f, abiPath)
}

// Precompile defines the precompiled contract for ICS02.
type Precompile struct {
	cmn.Precompile

	abi.ABI
	clientPrecompile clientstypes.ClientPrecompile
	clientKeeper     ibcutils.ClientKeeper
	// BankKeeper is not used directly in the precompile but is needed for the balance handler.
	BankKeeper cmn.BankKeeper
}

// NewPrecompile creates a new Client Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	clientPrecompile clientstypes.ClientPrecompile,
	bankKeeper cmn.BankKeeper,
	clientKeeper ibcutils.ClientKeeper,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.GasConfig{},
			TransientKVGasConfig:  storetypes.GasConfig{},
			ContractAddress:       clientPrecompile.GetContractAddress(),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:              ABI,
		clientPrecompile: clientPrecompile,
		clientKeeper:     clientKeeper,
		BankKeeper:       bankKeeper,
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
	case MisbehaviourMethod:
		return GasMisbehaviour
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

	var bz []byte

	switch method.Name {
	case UpdateClientMethod:
		bz, err = p.UpdateClient(ctx, contract, stateDB, method, args)
	case VerifyMembershipMethod:
		bz, err = p.VerifyMembership(ctx, contract, stateDB, method, args)
	case VerifyNonMembershipMethod:
		bz, err = p.VerifyNonMembership(ctx, contract, stateDB, method, args)
	case MisbehaviourMethod:
		bz, err = p.Misbehaviour(ctx, contract, stateDB, method, args)
	// queries:
	case GetClientStateMethod:
		bz, err = p.GetClientState(ctx, method, contract, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
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
