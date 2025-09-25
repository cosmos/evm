package staking

import (
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/core/vm"

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
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
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
			return method.Outputs.Pack(big.NewInt(0), cmn.Coin{Denom: bondDenom, Amount: big.NewInt(0)})
		}

		return nil, err
	}

	out := new(DelegationOutput).FromResponse(res)

	return out.Pack(method.Outputs)
}

// UnbondingDelegation returns the delegation currently being unbonded for a delegator from
// a specific validator.
func (p Precompile) UnbondingDelegation(
	ctx sdk.Context,
	_ *vm.Contract,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	req, err := NewUnbondingDelegationRequest(args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.UnbondingDelegation(ctx, req)
	if err != nil {
		// return empty unbonding delegation output if the unbonding delegation is not found
		expError := fmt.Sprintf("unbonding delegation with delegator %s not found for validator %s", req.DelegatorAddr, req.ValidatorAddr)
		if strings.Contains(err.Error(), expError) {
			return method.Outputs.Pack(UnbondingDelegationResponse{})
		}
		return nil, err
	}

	out := new(UnbondingDelegationOutput).FromResponse(res)

	return method.Outputs.Pack(out.UnbondingDelegation)
}

// Validator returns the validator information for a given validator address.
func (p Precompile) Validator(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewValidatorRequest(args)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Validator(ctx, req)
	if err != nil {
		// return empty validator info if the validator is not found
		expError := fmt.Sprintf("validator %s not found", req.ValidatorAddr)
		if strings.Contains(err.Error(), expError) {
			return method.Outputs.Pack(DefaultValidatorInfo())
		}
		return nil, err
	}

	validatorInfo := NewValidatorInfoFromResponse(res.Validator)

	return method.Outputs.Pack(validatorInfo)
}

// Validators returns the validators information with a provided status & pagination (optional).
func (p Precompile) Validators(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewValidatorsRequest(method, args)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Validators(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(ValidatorsOutput).FromResponse(res)

	return out.Pack(method.Outputs)
}

// Redelegation returns the redelegation between two validators for a delegator.
func (p Precompile) Redelegation(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewRedelegationRequest(args)
	if err != nil {
		return nil, err
	}

	res, _ := p.stakingKeeper.GetRedelegation(ctx, req.DelegatorAddress, req.ValidatorSrcAddress, req.ValidatorDstAddress)

	out := new(RedelegationOutput).FromResponse(res)

	return method.Outputs.Pack(out.Redelegation)
}

// Redelegations returns the redelegations according to
// the specified criteria (delegator address and/or validator source address
// and/or validator destination address or all existing redelegations) with pagination.
// Pagination is only supported for querying redelegations from a source validator or to query all redelegations.
func (p Precompile) Redelegations(
	ctx sdk.Context,
	method *abi.Method,
	_ *vm.Contract,
	args []interface{},
) ([]byte, error) {
	req, err := NewRedelegationsRequest(method, args, p.addrCdc)
	if err != nil {
		return nil, err
	}

	res, err := p.stakingQuerier.Redelegations(ctx, req)
	if err != nil {
		return nil, err
	}

	out := new(RedelegationsOutput).FromResponse(res)

	return out.Pack(method.Outputs)
}
