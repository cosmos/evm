package distribution

import (
	"github.com/yihuang/go-abi"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// ValidatorDistributionInfoMethod defines the ABI method name for the
	// ValidatorDistributionInfo query.
	ValidatorDistributionInfoMethod = "validatorDistributionInfo"
	// ValidatorOutstandingRewardsMethod defines the ABI method name for the
	// ValidatorOutstandingRewards query.
	ValidatorOutstandingRewardsMethod = "validatorOutstandingRewards"
	// ValidatorCommissionMethod defines the ABI method name for the
	// ValidatorCommission query.
	ValidatorCommissionMethod = "validatorCommission"
	// ValidatorSlashesMethod defines the ABI method name for the
	// ValidatorSlashes query.
	ValidatorSlashesMethod = "validatorSlashes"
	// DelegationRewardsMethod defines the ABI method name for the
	// DelegationRewards query.
	DelegationRewardsMethod = "delegationRewards"
	// DelegationTotalRewardsMethod defines the ABI method name for the
	// DelegationTotalRewards query.
	DelegationTotalRewardsMethod = "delegationTotalRewards"
	// DelegatorValidatorsMethod defines the ABI method name for the
	// DelegatorValidators query.
	DelegatorValidatorsMethod = "delegatorValidators"
	// DelegatorWithdrawAddressMethod defines the ABI method name for the
	// DelegatorWithdrawAddress query.
	DelegatorWithdrawAddressMethod = "delegatorWithdrawAddress"
	// CommunityPoolMethod defines the ABI method name for the
	// CommunityPool query.
	CommunityPoolMethod = "communityPool"
)

// ValidatorDistributionInfo returns the distribution info for a validator.
func (p Precompile) ValidatorDistributionInfo(
	ctx sdk.Context,
	args *ValidatorDistributionInfoCall,
) (*ValidatorDistributionInfoReturn, error) {
	req, err := NewValidatorDistributionInfoRequest(*args)
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.ValidatorDistributionInfo(ctx, req)
	if err != nil {
		return nil, err
	}

	return new(ValidatorDistributionInfoReturn).FromResponse(res), nil
}

// ValidatorOutstandingRewards returns the outstanding rewards for a validator.
func (p Precompile) ValidatorOutstandingRewards(
	ctx sdk.Context,
	args *ValidatorOutstandingRewardsCall,
) (*ValidatorOutstandingRewardsReturn, error) {
	req, err := NewValidatorOutstandingRewardsRequest(*args)
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.ValidatorOutstandingRewards(ctx, req)
	if err != nil {
		return nil, err
	}

	return &ValidatorOutstandingRewardsReturn{cmn.NewDecCoinsResponse(res.Rewards.Rewards)}, nil
}

// ValidatorCommission returns the commission for a validator.
func (p Precompile) ValidatorCommission(
	ctx sdk.Context,
	args *ValidatorCommissionCall,
) (*ValidatorCommissionReturn, error) {
	req, err := NewValidatorCommissionRequest(*args)
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.ValidatorCommission(ctx, req)
	if err != nil {
		return nil, err
	}

	return &ValidatorCommissionReturn{cmn.NewDecCoinsResponse(res.Commission.Commission)}, nil
}

// ValidatorSlashes returns the slashes for a validator.
func (p Precompile) ValidatorSlashes(
	ctx sdk.Context,
	args *ValidatorSlashesCall,
) (*ValidatorSlashesReturn, error) {
	req, err := NewValidatorSlashesRequest(*args)
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.ValidatorSlashes(ctx, req)
	if err != nil {
		return nil, err
	}

	return new(ValidatorSlashesReturn).FromResponse(res), nil
}

// DelegationRewards returns the total rewards accrued by a delegation.
func (p Precompile) DelegationRewards(
	ctx sdk.Context,
	args *DelegationRewardsCall,
) (*DelegationRewardsReturn, error) {
	req, err := NewDelegationRewardsRequest(*args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.DelegationRewards(ctx, req)
	if err != nil {
		return nil, err
	}

	return &DelegationRewardsReturn{cmn.NewDecCoinsResponse(res.Rewards)}, nil
}

// DelegationTotalRewards returns the total rewards accrued by a delegation.
func (p Precompile) DelegationTotalRewards(
	ctx sdk.Context,
	args *DelegationTotalRewardsCall,
) (*DelegationTotalRewardsReturn, error) {
	req, err := NewDelegationTotalRewardsRequest(*args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.DelegationTotalRewards(ctx, req)
	if err != nil {
		return nil, err
	}

	return new(DelegationTotalRewardsReturn).FromResponse(res), nil
}

// DelegatorValidators returns the validators a delegator is bonded to.
func (p Precompile) DelegatorValidators(
	ctx sdk.Context,
	args *DelegatorValidatorsCall,
) (*DelegatorValidatorsReturn, error) {
	req, err := NewDelegatorValidatorsRequest(*args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.DelegatorValidators(ctx, req)
	if err != nil {
		return nil, err
	}

	return &DelegatorValidatorsReturn{res.Validators}, nil
}

// DelegatorWithdrawAddress returns the withdraw address for a delegator.
func (p Precompile) DelegatorWithdrawAddress(
	ctx sdk.Context,
	args *DelegatorWithdrawAddressCall,
) (*DelegatorWithdrawAddressReturn, error) {
	req, err := NewDelegatorWithdrawAddressRequest(*args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.DelegatorWithdrawAddress(ctx, req)
	if err != nil {
		return nil, err
	}

	return &DelegatorWithdrawAddressReturn{res.WithdrawAddress}, nil
}

// CommunityPool returns the community pool coins.
func (p Precompile) CommunityPool(
	ctx sdk.Context,
	_ *abi.EmptyTuple,
) (*CommunityPoolReturn, error) {
	req, err := NewCommunityPoolRequest()
	if err != nil {
		return nil, err
	}

	res, err := p.distributionQuerier.CommunityPool(ctx, req)
	if err != nil {
		return nil, err
	}

	return new(CommunityPoolReturn).FromResponse(res), nil
}
