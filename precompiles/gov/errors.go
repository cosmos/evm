package gov

import (
	cmn "github.com/cosmos/evm/precompiles/common"

	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

const (
	// ErrInvalidVoter is raised when the voter address is not valid.
	ErrInvalidVoter = "invalid voter address: %s"
	// ErrInvalidProposalID invalid proposal id.
	ErrInvalidProposalID = "invalid proposal id %d "
	// ErrInvalidPageRequest invalid page request.
	ErrInvalidPageRequest = "invalid page request"
	// ErrInvalidOption invalid option.
	ErrInvalidOption = "invalid option %s "
	// ErrInvalidMetadata invalid metadata.
	ErrInvalidMetadata = "invalid metadata %s "
	// ErrInvalidWeightedVoteOptions invalid weighted vote options.
	ErrInvalidWeightedVoteOptions = "invalid weighted vote options %s "
	// ErrInvalidWeightedVoteOption invalid weighted vote option.
	ErrInvalidWeightedVoteOption = "invalid weighted vote option %s "
	// ErrInvalidWeightedVoteOptionType invalid weighted vote option type.
	ErrInvalidWeightedVoteOptionType = "invalid weighted vote option type %s "
	// ErrInvalidWeightedVoteOptionWeight invalid weighted vote option weight.
	ErrInvalidWeightedVoteOptionWeight = "invalid weighted vote option weight %s "
	// ErrInvalidProposalJSON invalid proposal json.
	ErrInvalidProposalJSON = "invalid proposal json %s "
	// ErrInvalidProposer invalid proposer.
	ErrInvalidProposer = "invalid proposer %s"
	// ErrInvalidDepositor invalid depositor address.
	ErrInvalidDepositor = "invalid depositor address: %s"
	// ErrInvalidDeposits invalid deposits.
	ErrInvalidDeposits = "invalid deposits %s "

	// SolidityErrGovInputInvalid is defined in IGov.sol.
	SolidityErrGovInputInvalid = "GovInputInvalid"
	// SolidityErrInvalidProposalJSON is defined in IGov.sol.
	SolidityErrInvalidProposalJSON = "InvalidProposalJSON"
	// SolidityErrInvalidOption is defined in IGov.sol.
	SolidityErrInvalidOption = "InvalidOption"
	// SolidityErrInvalidMetadata is defined in IGov.sol.
	SolidityErrInvalidMetadata = "InvalidMetadata"
	// SolidityErrVotesInputUnpackFailed is defined in IGov.sol.
	SolidityErrVotesInputUnpackFailed = "VotesInputUnpackFailed"
	// SolidityErrDepositsInputUnpackFailed is defined in IGov.sol.
	SolidityErrDepositsInputUnpackFailed = "DepositsInputUnpackFailed"
	// SolidityErrProposalsInputUnpackFailed is defined in IGov.sol.
	SolidityErrProposalsInputUnpackFailed = "ProposalsInputUnpackFailed"
	// SolidityErrWeightedVoteOptionsUnpackFailed is defined in IGov.sol.
	SolidityErrWeightedVoteOptionsUnpackFailed = "WeightedVoteOptionsUnpackFailed"
	// SolidityErrInvalidProposalID is defined in IGov.sol.
	SolidityErrInvalidProposalID = "InvalidProposalID"

	SolidityErrGovNoProposalMessages        = "GovNoProposalMessages"
	SolidityErrGovInvalidProposalContent    = "GovInvalidProposalContent"
	SolidityErrGovInvalidProposalMessage    = "GovInvalidProposalMessage"
	SolidityErrGovInvalidSigner             = "GovInvalidSigner"
	SolidityErrGovUnroutableProposalMessage = "GovUnroutableProposalMessage"
	SolidityErrGovNoProposalHandlerExists   = "GovNoProposalHandlerExists"
	SolidityErrGovMetadataTooLong           = "GovMetadataTooLong"
	SolidityErrGovSummaryTooLong            = "GovSummaryTooLong"
	SolidityErrGovMinimumDepositTooSmall    = "GovMinimumDepositTooSmall"
	SolidityErrGovInvalidDepositDenom       = "GovInvalidDepositDenom"
	SolidityErrGovInactiveProposal          = "GovInactiveProposal"
	SolidityErrGovInvalidVote               = "GovInvalidVote"
	SolidityErrGovInvalidProposal           = "GovInvalidProposal"
	SolidityErrGovInvalidProposer           = "GovInvalidProposer"
	SolidityErrGovVotingPeriodEnded         = "GovVotingPeriodEnded"
)

var govErrorMappings = cmn.CosmosErrorMappings{
	cmn.NewCosmosErrorMapping(govtypes.ErrNoProposalMsgs, SolidityErrGovNoProposalMessages),
	cmn.NewCosmosErrorMapping(govtypes.ErrInvalidProposalContent, SolidityErrGovInvalidProposalContent),
	cmn.NewCosmosErrorMapping(govtypes.ErrInvalidProposalMsg, SolidityErrGovInvalidProposalMessage),
	cmn.NewCosmosErrorMapping(govtypes.ErrInvalidSigner, SolidityErrGovInvalidSigner),
	cmn.NewCosmosErrorMapping(govtypes.ErrUnroutableProposalMsg, SolidityErrGovUnroutableProposalMessage),
	cmn.NewCosmosErrorMapping(govtypes.ErrNoProposalHandlerExists, SolidityErrGovNoProposalHandlerExists),
	cmn.NewCosmosErrorMapping(govtypes.ErrMetadataTooLong, SolidityErrGovMetadataTooLong),
	cmn.NewCosmosErrorMapping(govtypes.ErrSummaryTooLong, SolidityErrGovSummaryTooLong),
	cmn.NewCosmosErrorMapping(govtypes.ErrMinDepositTooSmall, SolidityErrGovMinimumDepositTooSmall),
	cmn.NewCosmosErrorMapping(govtypes.ErrInvalidDepositDenom, SolidityErrGovInvalidDepositDenom),
	cmn.NewCosmosErrorMapping(govtypes.ErrInactiveProposal, SolidityErrGovInactiveProposal),
	cmn.NewCosmosErrorMapping(govtypes.ErrInvalidVote, SolidityErrGovInvalidVote),
	cmn.NewCosmosErrorMapping(govtypes.ErrInvalidProposal, SolidityErrGovInvalidProposal),
	cmn.NewCosmosErrorMapping(govtypes.ErrInvalidProposer, SolidityErrGovInvalidProposer),
	cmn.NewCosmosErrorMapping(govtypes.ErrVotingPeriodEnded, SolidityErrGovVotingPeriodEnded),
}

func ErrorMappings() cmn.CosmosErrorMappings {
	return govErrorMappings.Clone()
}
