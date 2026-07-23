package staking

import (
	cmn "github.com/cosmos/evm/precompiles/common"

	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const (
	// ErrNoDelegationFound is raised when no delegation is found for the given delegator and validator addresses.
	ErrNoDelegationFound = "delegation with delegator %s not found for validator %s"
	// ErrCannotCallFromContract is raised when a function cannot be called from a smart contract.
	ErrCannotCallFromContract = "this method can only be called directly to the precompile, not from a smart contract"

	// Solidity custom error names defined in StakingI (and IPrecompile where shared).
	SolidityErrBondDenomQueryFailed           = "BondDenomQueryFailed"
	SolidityErrCannotCallFromContract         = "CannotCallFromContract"
	SolidityErrInvalidDescription             = "InvalidDescription"
	SolidityErrInvalidCommission              = "InvalidCommission"
	SolidityErrRedelegationsInputUnpackFailed = "RedelegationsInputUnpackFailed"
	SolidityErrInvalidRedelegationsQuery      = "InvalidRedelegationsQuery"

	SolidityErrStakingValidatorNotFound               = "StakingValidatorNotFound"
	SolidityErrStakingValidatorOwnerExists            = "StakingValidatorOwnerExists"
	SolidityErrStakingValidatorPubKeyExists           = "StakingValidatorPubKeyExists"
	SolidityErrStakingValidatorPubKeyTypeNotSupported = "StakingValidatorPubKeyTypeNotSupported"
	SolidityErrStakingValidatorJailed                 = "StakingValidatorJailed"
	SolidityErrStakingCommissionNegative              = "StakingCommissionNegative"
	SolidityErrStakingCommissionHuge                  = "StakingCommissionHuge"
	SolidityErrStakingCommissionGTMaxRate             = "StakingCommissionGTMaxRate"
	SolidityErrStakingCommissionUpdateTime            = "StakingCommissionUpdateTime"
	SolidityErrStakingCommissionChangeRateNegative    = "StakingCommissionChangeRateNegative"
	SolidityErrStakingCommissionChangeRateGTMaxRate   = "StakingCommissionChangeRateGTMaxRate"
	SolidityErrStakingCommissionGTMaxChangeRate       = "StakingCommissionGTMaxChangeRate"
	SolidityErrStakingSelfDelegationBelowMinimum      = "StakingSelfDelegationBelowMinimum"
	SolidityErrStakingMinSelfDelegationDecreased      = "StakingMinSelfDelegationDecreased"
	SolidityErrStakingNoDelegation                    = "StakingNoDelegation"
	SolidityErrStakingInsufficientShares              = "StakingInsufficientShares"
	SolidityErrStakingUnbondingDelegationNotFound     = "StakingUnbondingDelegationNotFound"
	SolidityErrStakingMaxUnbondingDelegationEntries   = "StakingMaxUnbondingDelegationEntries"
	SolidityErrStakingSelfRedelegation                = "StakingSelfRedelegation"
	SolidityErrStakingTinyRedelegationAmount          = "StakingTinyRedelegationAmount"
	SolidityErrStakingBadRedelegationDst              = "StakingBadRedelegationDst"
	SolidityErrStakingTransitiveRedelegation          = "StakingTransitiveRedelegation"
	SolidityErrStakingMaxRedelegationEntries          = "StakingMaxRedelegationEntries"
	SolidityErrStakingDelegatorShareExRateInvalid     = "StakingDelegatorShareExRateInvalid"
	SolidityErrStakingCommissionLTMinRate             = "StakingCommissionLTMinRate"
	SolidityErrStakingBadRedelegationSrc              = "StakingBadRedelegationSrc"
)

var stakingErrorMappings = cmn.CosmosErrorMappings{
	cmn.NewCosmosErrorMapping(stakingtypes.ErrNoValidatorFound, SolidityErrStakingValidatorNotFound),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrValidatorOwnerExists, SolidityErrStakingValidatorOwnerExists),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrValidatorPubKeyExists, SolidityErrStakingValidatorPubKeyExists),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrValidatorPubKeyTypeNotSupported, SolidityErrStakingValidatorPubKeyTypeNotSupported),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrValidatorJailed, SolidityErrStakingValidatorJailed),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrCommissionNegative, SolidityErrStakingCommissionNegative),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrCommissionHuge, SolidityErrStakingCommissionHuge),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrCommissionGTMaxRate, SolidityErrStakingCommissionGTMaxRate),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrCommissionUpdateTime, SolidityErrStakingCommissionUpdateTime),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrCommissionChangeRateNegative, SolidityErrStakingCommissionChangeRateNegative),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrCommissionChangeRateGTMaxRate, SolidityErrStakingCommissionChangeRateGTMaxRate),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrCommissionGTMaxChangeRate, SolidityErrStakingCommissionGTMaxChangeRate),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrSelfDelegationBelowMinimum, SolidityErrStakingSelfDelegationBelowMinimum),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrMinSelfDelegationDecreased, SolidityErrStakingMinSelfDelegationDecreased),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrNoDelegation, SolidityErrStakingNoDelegation),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrInsufficientShares, SolidityErrStakingInsufficientShares),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrMaxUnbondingDelegationEntries, SolidityErrStakingMaxUnbondingDelegationEntries),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrSelfRedelegation, SolidityErrStakingSelfRedelegation),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrTinyRedelegationAmount, SolidityErrStakingTinyRedelegationAmount),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrBadRedelegationDst, SolidityErrStakingBadRedelegationDst),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrTransitiveRedelegation, SolidityErrStakingTransitiveRedelegation),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrMaxRedelegationEntries, SolidityErrStakingMaxRedelegationEntries),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrDelegatorShareExRateInvalid, SolidityErrStakingDelegatorShareExRateInvalid),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrCommissionLTMinRate, SolidityErrStakingCommissionLTMinRate),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrBadRedelegationSrc, SolidityErrStakingBadRedelegationSrc),
}

func ErrorMappings() cmn.CosmosErrorMappings {
	return stakingErrorMappings.Clone()
}
