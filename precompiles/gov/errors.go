package gov

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
)
