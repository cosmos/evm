package distribution

import (
	cmn "github.com/cosmos/evm/precompiles/common"

	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const (
	// ErrDifferentValidator is raised when the origin address is not the same as the validator address.
	ErrDifferentValidator = "origin address %s is not the same as validator address %s"
	// ErrInvalidAmount is raised when the given sdk coins amount is invalid
	ErrInvalidAmount = "invalid amount %s"

	// Solidity custom error names defined in DistributionI.
	SolidityErrDistributionInputInvalid                 = "DistributionInputInvalid"
	SolidityErrDistributionValidatorSlashesUnpackFailed = "DistributionValidatorSlashesUnpackFailed"
	SolidityErrClaimRewardsMaxRetrieveExceeded          = "ClaimRewardsMaxRetrieveExceeded"

	SolidityErrDistributionSetWithdrawAddressDisabled      = "DistributionSetWithdrawAddressDisabled"
	SolidityErrDistributionEmptyDelegationDistributionInfo = "DistributionEmptyDelegationDistributionInfo"
	SolidityErrDistributionNoValidatorDistributionInfo     = "DistributionNoValidatorDistributionInfo"
	SolidityErrDistributionNoValidatorCommission           = "DistributionNoValidatorCommission"
	SolidityErrDistributionNoValidatorExists               = "DistributionNoValidatorExists"
	SolidityErrDistributionNoDelegationExists              = "DistributionNoDelegationExists"
)

var distributionErrorMappings = cmn.CosmosErrorMappings{
	cmn.NewCosmosErrorMapping(distributiontypes.ErrSetWithdrawAddrDisabled, SolidityErrDistributionSetWithdrawAddressDisabled),
	cmn.NewCosmosErrorMapping(distributiontypes.ErrEmptyDelegationDistInfo, SolidityErrDistributionEmptyDelegationDistributionInfo),
	cmn.NewCosmosErrorMapping(distributiontypes.ErrNoValidatorDistInfo, SolidityErrDistributionNoValidatorDistributionInfo),
	cmn.NewCosmosErrorMapping(distributiontypes.ErrNoValidatorCommission, SolidityErrDistributionNoValidatorCommission),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrNoValidatorFound, SolidityErrDistributionNoValidatorExists),
	cmn.NewCosmosErrorMapping(stakingtypes.ErrNoDelegation, SolidityErrDistributionNoDelegationExists),
}

func ErrorMappings() cmn.CosmosErrorMappings {
	return distributionErrorMappings.Clone()
}
