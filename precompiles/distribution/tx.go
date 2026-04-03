package distribution

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// SetWithdrawAddressMethod defines the ABI method name for the distribution
	// SetWithdrawAddress transaction.
	SetWithdrawAddressMethod = "setWithdrawAddress"
	// WithdrawDelegatorRewardMethod defines the ABI method name for the distribution
	// WithdrawDelegatorReward transaction.
	WithdrawDelegatorRewardMethod = "withdrawDelegatorRewards"
	// WithdrawValidatorCommissionMethod defines the ABI method name for the distribution
	// WithdrawValidatorCommission transaction.
	WithdrawValidatorCommissionMethod = "withdrawValidatorCommission"
	// FundCommunityPoolMethod defines the ABI method name for the fundCommunityPool transaction
	FundCommunityPoolMethod = "fundCommunityPool"
	// ClaimRewardsMethod defines the ABI method name for the custom ClaimRewards transaction
	ClaimRewardsMethod = "claimRewards"
	// DepositValidatorRewardsPoolMethod defines the ABI method name for the distribution
	// DepositValidatorRewardsPool transaction
	DepositValidatorRewardsPoolMethod = "depositValidatorRewardsPool"
	// ClaimRewardsMaxValidatorsQueryMethod is the method label for MaxValidators keeper query.
	ClaimRewardsMaxValidatorsQueryMethod = "MaxValidators"
	// ClaimRewardsGetDelegatorValidatorsQueryMethod is the method label for GetDelegatorValidators query.
	ClaimRewardsGetDelegatorValidatorsQueryMethod = "GetDelegatorValidators"
	// ClaimRewardsWithdrawDelegationRewardsMsgMethod is the method label for WithdrawDelegationRewards call.
	ClaimRewardsWithdrawDelegationRewardsMsgMethod = "WithdrawDelegationRewards"
)

// ClaimRewards claims the rewards accumulated by a delegator from multiple or all validators.
func (p *Precompile) ClaimRewards(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	delegatorAddr, maxRetrieve, err := parseClaimRewardsArgs(args)
	if err != nil {
		return nil, err
	}

	maxVals, err := p.stakingKeeper.MaxValidators(ctx)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, ClaimRewardsMaxValidatorsQueryMethod, err.Error())
	}
	if maxRetrieve > maxVals {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, SolidityErrClaimRewardsMaxRetrieveExceeded, maxRetrieve, maxVals)
	}

	msgSender := contract.Caller()
	if msgSender != delegatorAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, delegatorAddr)
	}

	res, err := p.stakingKeeper.GetDelegatorValidators(ctx, delegatorAddr.Bytes(), maxRetrieve)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrQueryFailed, ClaimRewardsGetDelegatorValidatorsQueryMethod, err.Error())
	}
	totalCoins := sdk.Coins{}
	for _, validator := range res.Validators {
		// Convert the validator operator address into an ValAddress
		valAddr, err := sdk.ValAddressFromBech32(validator.OperatorAddress)
		if err != nil {
			return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrInvalidAddress, validator.OperatorAddress)
		}

		// Withdraw the rewards for each validator address
		coins, err := p.distributionKeeper.WithdrawDelegationRewards(ctx, delegatorAddr.Bytes(), valAddr)
		if err != nil {
			return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, ClaimRewardsWithdrawDelegationRewardsMsgMethod, err.Error())
		}

		totalCoins = totalCoins.Add(coins...)
	}

	if err := p.EmitClaimRewardsEvent(ctx, stateDB, delegatorAddr, totalCoins); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, ClaimRewardsMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}

// SetWithdrawAddress sets the withdrawal address for a delegator (or validator self-delegation).
func (p Precompile) SetWithdrawAddress(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, delegatorHexAddr, err := NewMsgSetWithdrawAddress(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, delegatorHexAddr)
	}

	if _, err = p.distributionMsgServer.SetWithdrawAddress(ctx, msg); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, SetWithdrawAddressMethod, err.Error())
	}

	if err = p.EmitSetWithdrawAddressEvent(ctx, stateDB, delegatorHexAddr, msg.WithdrawAddress); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, SetWithdrawAddressMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}

// WithdrawDelegatorReward withdraws the rewards of a delegator from a single validator.
func (p *Precompile) WithdrawDelegatorReward(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, delegatorHexAddr, err := NewMsgWithdrawDelegatorReward(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, delegatorHexAddr)
	}

	res, err := p.distributionMsgServer.WithdrawDelegatorReward(ctx, msg)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, WithdrawDelegatorRewardMethod, err.Error())
	}

	if err = p.EmitWithdrawDelegatorRewardEvent(ctx, stateDB, delegatorHexAddr, msg.ValidatorAddress, res.Amount); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, WithdrawDelegatorRewardMethod, err.Error())
	}

	return method.Outputs.Pack(cmn.NewCoinsResponse(res.Amount))
}

// WithdrawValidatorCommission withdraws the rewards of a validator.
func (p *Precompile) WithdrawValidatorCommission(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, validatorHexAddr, err := NewMsgWithdrawValidatorCommission(args)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != validatorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, validatorHexAddr)
	}

	res, err := p.distributionMsgServer.WithdrawValidatorCommission(ctx, msg)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, WithdrawValidatorCommissionMethod, err.Error())
	}

	if err = p.EmitWithdrawValidatorCommissionEvent(ctx, stateDB, msg.ValidatorAddress, res.Amount); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, WithdrawValidatorCommissionMethod, err.Error())
	}

	return method.Outputs.Pack(cmn.NewCoinsResponse(res.Amount))
}

// FundCommunityPool directly fund the community pool
func (p *Precompile) FundCommunityPool(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, depositorHexAddr, err := NewMsgFundCommunityPool(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != depositorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, depositorHexAddr)
	}

	_, err = p.distributionMsgServer.FundCommunityPool(ctx, msg)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, FundCommunityPoolMethod, err.Error())
	}

	if err = p.EmitFundCommunityPoolEvent(ctx, stateDB, depositorHexAddr, msg.Amount); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, FundCommunityPoolMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}

// DepositValidatorRewardsPool deposits rewards into the validator rewards pool
// for a specific validator.
func (p *Precompile) DepositValidatorRewardsPool(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, depositorHexAddr, err := NewMsgDepositValidatorRewardsPool(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != depositorHexAddr {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrRequesterIsNotMsgSender, msgSender, depositorHexAddr)
	}

	_, err = p.distributionMsgServer.DepositValidatorRewardsPool(ctx, msg)
	if err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrMsgServerFailed, DepositValidatorRewardsPoolMethod, err.Error())
	}

	if err = p.EmitDepositValidatorRewardsPoolEvent(ctx, stateDB, depositorHexAddr, msg.ValidatorAddress, msg.Amount); err != nil {
		return nil, cmn.NewRevertWithSolidityError(p.ABI, cmn.SolidityErrEventEmitFailed, DepositValidatorRewardsPoolMethod, err.Error())
	}

	return method.Outputs.Pack(true)
}
