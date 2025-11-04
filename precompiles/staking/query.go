package staking

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// DelegationMethod defines the ABI method name for the staking Delegation
	// query.
	DelegationMethod = "delegation"
	// UnbondingDelegationMethod defines the ABI method name for the staking
	// UnbondingDelegationMethod query.
	UnbondingDelegationMethod = "unbondingDelegation"
	// ValidatorMethod defines the ABI method name for the staking
	// Validator query.
	ValidatorMethod = "validator"
	// ValidatorsMethod defines the ABI method name for the staking
	// Validators query.
	ValidatorsMethod = "validators"
	// RedelegationMethod defines the ABI method name for the staking
	// Redelegation query.
	RedelegationMethod = "redelegation"
	// RedelegationsMethod defines the ABI method name for the staking
	// Redelegations query.
	RedelegationsMethod = "redelegations"
)

// Delegation returns the delegation that a delegator has with a specific validator.
func (p Precompile) Delegation(
	ctx sdk.Context,
	args DelegationCall,
) (*DelegationReturn, error) {
	req, err := NewDelegationRequest(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Delegation(ctx, req)
	if err != nil {
		// If there is no delegation found, return the response with zero values.
		if strings.Contains(err.Error(), fmt.Sprintf(ErrNoDelegationFound, req.DelegatorAddr, req.ValidatorAddr)) {
			bondDenom, err := p.stakingKeeper.BondDenom(ctx)
			if err != nil {
				return nil, err
			}
			return &DelegationReturn{
				Shares:  big.NewInt(0),
				Balance: cmn.Coin{Denom: bondDenom, Amount: big.NewInt(0)},
			}, nil
		}

		return nil, err
	}

	return new(DelegationReturn).FromResponse(res), nil
}

// UnbondingDelegation returns the delegation currently being unbonded for a delegator from
// a specific validator.
func (p Precompile) UnbondingDelegation(
	ctx sdk.Context,
	args UnbondingDelegationCall,
) (*UnbondingDelegationReturn, error) {
	req, err := NewUnbondingDelegationRequest(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.UnbondingDelegation(ctx, req)
	if err != nil {
		// return empty unbonding delegation output if the unbonding delegation is not found
		expError := fmt.Sprintf("unbonding delegation with delegator %s not found for validator %s", req.DelegatorAddr, req.ValidatorAddr)
		if strings.Contains(err.Error(), expError) {
			return &UnbondingDelegationReturn{}, nil
		}
		return nil, err
	}

	return new(UnbondingDelegationReturn).FromResponse(res), nil
}

// Validator returns the validator information for a given validator address.
func (p Precompile) Validator(
	ctx sdk.Context,
	args ValidatorCall,
) (*ValidatorReturn, error) {
	req, err := NewValidatorRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Validator(ctx, req)
	if err != nil {
		// return empty validator info if the validator is not found
		expError := fmt.Sprintf("validator %s not found", req.ValidatorAddr)
		if strings.Contains(err.Error(), expError) {
			return &ValidatorReturn{DefaultValidator()}, nil
		}
		return nil, err
	}

	validatorInfo := NewValidatorFromResponse(res.Validator)

	return &ValidatorReturn{validatorInfo}, nil
}

// Validators returns the validators information with a provided status & pagination (optional).
func (p Precompile) Validators(
	ctx sdk.Context,
	args ValidatorsCall,
) (*ValidatorsReturn, error) {
	req, err := NewValidatorsRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Validators(ctx, req)
	if err != nil {
		return nil, err
	}

	return new(ValidatorsReturn).FromResponse(res), nil
}

// Redelegation returns the redelegation between two validators for a delegator.
func (p Precompile) Redelegation(
	ctx sdk.Context,
	args RedelegationCall,
) (*RedelegationReturn, error) {
	if args.DelegatorAddress == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args.DelegatorAddress)
	}

	validatorSrcAddr, err := sdk.ValAddressFromBech32(args.SrcValidatorAddress)
	if err != nil {
		return nil, err
	}

	validatorDstAddr, err := sdk.ValAddressFromBech32(args.DstValidatorAddress)
	if err != nil {
		return nil, err
	}

	res, _ := p.stakingKeeper.GetRedelegation(ctx, args.DelegatorAddress.Bytes(), validatorSrcAddr, validatorDstAddr)
	return new(RedelegationReturn).FromResponse(res), nil
}

// Redelegations returns the redelegations according to
// the specified criteria (delegator address and/or validator source address
// and/or validator destination address or all existing redelegations) with pagination.
// Pagination is only supported for querying redelegations from a source validator or to query all redelegations.
func (p Precompile) Redelegations(
	ctx sdk.Context,
	args RedelegationsCall,
) (*RedelegationsReturn, error) {
	req, err := NewRedelegationsRequest(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Redelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	return new(RedelegationsReturn).FromResponse(res), nil
}
