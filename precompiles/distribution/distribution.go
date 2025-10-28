package distribution

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

	sdk "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
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

// Precompile defines the precompiled contract for distribution.
type Precompile struct {
	cmn.Precompile

	abi.ABI
	distributionKeeper    cmn.DistributionKeeper
	distributionMsgServer distributiontypes.MsgServer
	distributionQuerier   distributiontypes.QueryServer
	stakingKeeper         cmn.StakingKeeper
	addrCdc               address.Codec
}

// NewPrecompile creates a new distribution Precompile instance as a
// PrecompiledContract interface.
func NewPrecompile(
	distributionKeeper cmn.DistributionKeeper,
	distributionMsgServer distributiontypes.MsgServer,
	distributionQuerier distributiontypes.QueryServer,
	stakingKeeper cmn.StakingKeeper,
	bankKeeper cmn.BankKeeper,
	addrCdc address.Codec,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(evmtypes.DistributionPrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:                   ABI,
		stakingKeeper:         stakingKeeper,
		distributionKeeper:    distributionKeeper,
		distributionMsgServer: distributionMsgServer,
		distributionQuerier:   distributionQuerier,
		addrCdc:               addrCdc,
	}
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
	// Custom transactions
	case ClaimRewardsMethod:
		bz, err = p.ClaimRewards(ctx, contract, stateDB, method, args)
	// Distribution transactions
	case SetWithdrawAddressMethod:
		bz, err = p.SetWithdrawAddress(ctx, contract, stateDB, method, args)
	case WithdrawDelegatorRewardMethod:
		bz, err = p.WithdrawDelegatorReward(ctx, contract, stateDB, method, args)
	case WithdrawValidatorCommissionMethod:
		bz, err = p.WithdrawValidatorCommission(ctx, contract, stateDB, method, args)
	case FundCommunityPoolMethod:
		bz, err = p.FundCommunityPool(ctx, contract, stateDB, method, args)
	case DepositValidatorRewardsPoolMethod:
		bz, err = p.DepositValidatorRewardsPool(ctx, contract, stateDB, method, args)
	// Distribution queries
	case ValidatorDistributionInfoMethod:
		bz, err = p.ValidatorDistributionInfo(ctx, contract, method, args)
	case ValidatorOutstandingRewardsMethod:
		bz, err = p.ValidatorOutstandingRewards(ctx, contract, method, args)
	case ValidatorCommissionMethod:
		bz, err = p.ValidatorCommission(ctx, contract, method, args)
	case ValidatorSlashesMethod:
		bz, err = p.ValidatorSlashes(ctx, contract, method, args)
	case DelegationRewardsMethod:
		bz, err = p.DelegationRewards(ctx, contract, method, args)
	case DelegationTotalRewardsMethod:
		bz, err = p.DelegationTotalRewards(ctx, contract, method, args)
	case DelegatorValidatorsMethod:
		bz, err = p.DelegatorValidators(ctx, contract, method, args)
	case DelegatorWithdrawAddressMethod:
		bz, err = p.DelegatorWithdrawAddress(ctx, contract, method, args)
	case CommunityPoolMethod:
		bz, err = p.CommunityPool(ctx, contract, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
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
