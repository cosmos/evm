package distribution

import (
	"embed"
	"fmt"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmkeeper "github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	storetypes "cosmossdk.io/store/types"

	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
)

var _ vm.PrecompiledContract = &Precompile{}

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

// Precompile defines the precompiled contract for distribution.
type Precompile struct {
	cmn.Precompile
	distributionKeeper distributionkeeper.Keeper
	stakingKeeper      stakingkeeper.Keeper
	evmKeeper          *evmkeeper.Keeper
}

// NewPrecompile creates a new distribution Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	distributionKeeper distributionkeeper.Keeper,
	stakingKeeper stakingkeeper.Keeper,
	evmKeeper *evmkeeper.Keeper,
) (*Precompile, error) {
	newAbi, err := cmn.LoadABI(f, "abi.json")
	if err != nil {
		return nil, fmt.Errorf("error loading the distribution ABI %s", err)
	}

	p := &Precompile{
		Precompile: cmn.Precompile{
			ABI:                  newAbi,
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
		},
		stakingKeeper:      stakingKeeper,
		distributionKeeper: distributionKeeper,
		evmKeeper:          evmKeeper,
	}

	// SetAddress defines the address of the distribution compile contract.
	p.SetAddress(common.HexToAddress(evmtypes.DistributionPrecompileAddress))

	return p, nil
}

// RequiredGas calculates the precompiled contract's base gas rate.
func (p Precompile) RequiredGas(input []byte) uint64 {
	// TODO: refactor this to be used in the common precompile method on a separate PR

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

// Run executes the precompiled contract distribution methods defined in the ABI.
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readOnly bool) ([]byte, error) {
	return p.SetupAndRun(evm, contract, readOnly, p.IsTransaction, p.HandleMethod)
}

// HandleMethod handles the execution of each method
func (p Precompile) HandleMethod(ctx sdk.Context, contract *vm.Contract, stateDB vm.StateDB, method *abi.Method, args []interface{}) ([]byte, error) {
	switch method.Name {
	// Custom transactions
	case ClaimRewardsMethod:
		return p.ClaimRewards(ctx, contract, stateDB, method, args)
	// Distribution transactions
	case SetWithdrawAddressMethod:
		return p.SetWithdrawAddress(ctx, contract, stateDB, method, args)
	case WithdrawDelegatorRewardMethod:
		return p.WithdrawDelegatorReward(ctx, contract, stateDB, method, args)
	case WithdrawValidatorCommissionMethod:
		return p.WithdrawValidatorCommission(ctx, contract, stateDB, method, args)
	case FundCommunityPoolMethod:
		return p.FundCommunityPool(ctx, contract, stateDB, method, args)
	case DepositValidatorRewardsPoolMethod:
		return p.DepositValidatorRewardsPool(ctx, contract, stateDB, method, args)
	// Distribution queries
	case ValidatorDistributionInfoMethod:
		return p.ValidatorDistributionInfo(ctx, contract, method, args)
	case ValidatorOutstandingRewardsMethod:
		return p.ValidatorOutstandingRewards(ctx, contract, method, args)
	case ValidatorCommissionMethod:
		return p.ValidatorCommission(ctx, contract, method, args)
	case ValidatorSlashesMethod:
		return p.ValidatorSlashes(ctx, contract, method, args)
	case DelegationRewardsMethod:
		return p.DelegationRewards(ctx, contract, method, args)
	case DelegationTotalRewardsMethod:
		return p.DelegationTotalRewards(ctx, contract, method, args)
	case DelegatorValidatorsMethod:
		return p.DelegatorValidators(ctx, contract, method, args)
	case DelegatorWithdrawAddressMethod:
		return p.DelegatorWithdrawAddress(ctx, contract, method, args)
	case CommunityPoolMethod:
		return p.CommunityPool(ctx, contract, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
//
// Available distribution transactions are:
//   - ClaimRewards
//   - SetWithdrawAddress
//   - WithdrawDelegatorReward
//   - WithdrawValidatorCommission
//   - FundCommunityPool
//   - DepositValidatorRewardsPool
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case ClaimRewardsMethod,
		SetWithdrawAddressMethod,
		WithdrawDelegatorRewardMethod,
		WithdrawValidatorCommissionMethod,
		FundCommunityPoolMethod,
		DepositValidatorRewardsPoolMethod:
		return true
	default:
		return false
	}
}
