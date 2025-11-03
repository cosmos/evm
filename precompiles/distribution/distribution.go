package distribution

import (
	"fmt"

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

//go:generate go run github.com/yihuang/go-abi/cmd -input abi.json -output distribution.abi.go -external-tuples Coin=cmn.Coin,Dec=cmn.Dec,DecCoin=cmn.DecCoin,PageRequest=cmn.PageRequest -imports cmn=github.com/cosmos/evm/precompiles/common

var _ vm.PrecompiledContract = &Precompile{}

// Precompile defines the precompiled contract for distribution.
type Precompile struct {
	cmn.Precompile

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
		stakingKeeper:         stakingKeeper,
		distributionKeeper:    distributionKeeper,
		distributionMsgServer: distributionMsgServer,
		distributionQuerier:   distributionQuerier,
		addrCdc:               addrCdc,
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
	// Custom transactions
	case ClaimRewardsID:
		return cmn.RunWithStateDB(ctx, p.ClaimRewards, input, stateDB, contract)
	// Distribution transactions
	case SetWithdrawAddressID:
		return cmn.RunWithStateDB(ctx, p.SetWithdrawAddress, input, stateDB, contract)
	case WithdrawDelegatorRewardsID:
		return cmn.RunWithStateDB(ctx, p.WithdrawDelegatorReward, input, stateDB, contract)
	case WithdrawValidatorCommissionID:
		return cmn.RunWithStateDB(ctx, p.WithdrawValidatorCommission, input, stateDB, contract)
	case FundCommunityPoolID:
		return cmn.RunWithStateDB(ctx, p.FundCommunityPool, input, stateDB, contract)
	case DepositValidatorRewardsPoolID:
		return cmn.RunWithStateDB(ctx, p.DepositValidatorRewardsPool, input, stateDB, contract)
	// Distribution queries
	case ValidatorDistributionInfoID:
		return cmn.Run(ctx, p.ValidatorDistributionInfo, input)
	case ValidatorOutstandingRewardsID:
		return cmn.Run(ctx, p.ValidatorOutstandingRewards, input)
	case ValidatorCommissionID:
		return cmn.Run(ctx, p.ValidatorCommission, input)
	case ValidatorSlashesID:
		return cmn.Run(ctx, p.ValidatorSlashes, input)
	case DelegationRewardsID:
		return cmn.Run(ctx, p.DelegationRewards, input)
	case DelegationTotalRewardsID:
		return cmn.Run(ctx, p.DelegationTotalRewards, input)
	case DelegatorValidatorsID:
		return cmn.Run(ctx, p.DelegatorValidators, input)
	case DelegatorWithdrawAddressID:
		return cmn.Run(ctx, p.DelegatorWithdrawAddress, input)
	case CommunityPoolID:
		return cmn.Run(ctx, p.CommunityPool, input)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, methodID)
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
func (Precompile) IsTransaction(method uint32) bool {
	switch method {
	case ClaimRewardsID,
		SetWithdrawAddressID,
		WithdrawDelegatorRewardsID,
		WithdrawValidatorCommissionID,
		FundCommunityPoolID,
		DepositValidatorRewardsPoolID:
		return true
	default:
		return false
	}
}
