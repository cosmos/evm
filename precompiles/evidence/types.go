package evidence

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	evidencetypes "cosmossdk.io/x/evidence/types"

	"github.com/cosmos/cosmos-sdk/types/query"
)

const (
	// SubmitEvidenceMethod defines the ABI method name for the evidence SubmitEvidence
	// transaction.
	SubmitEvidenceMethod = "submitEvidence"
	// EvidenceMethod defines the ABI method name for the Evidence query.
	EvidenceMethod = "evidence"
	// GetAllEvidenceMethod defines the ABI method name for the GetAllEvidence query.
	GetAllEvidenceMethod = "getAllEvidence"
)

// EventSubmitEvidence defines the event data for the SubmitEvidence transaction.
type EventSubmitEvidence struct {
	Submitter common.Address
	Hash      []byte
}

// SingleEvidenceOutput defines the output for the Evidence query.
type SingleEvidenceOutput struct {
	Evidence EquivocationData
}

// AllEvidenceOutput defines the output for the GetAllEvidence query.
type AllEvidenceOutput struct {
	Evidence     []EquivocationData
	PageResponse query.PageResponse
}

// EquivocationData represents the Solidity Equivocation struct
type EquivocationData struct {
	Height           int64  `abi:"height"`
	Time             int64  `abi:"time"`
	Power            int64  `abi:"power"`
	ConsensusAddress string `abi:"consensusAddress"`
}

// ToEquivocation converts the EquivocationData to a types.Equivocation
func (e EquivocationData) ToEquivocation() *evidencetypes.Equivocation {
	return &evidencetypes.Equivocation{
		Height:           e.Height,
		Time:             time.Unix(e.Time, 0).UTC(),
		Power:            e.Power,
		ConsensusAddress: e.ConsensusAddress,
	}
}

type SubmitEquivocationInput struct {
	Equivocation EquivocationData
}

// NewMsgSubmitEvidence creates a new MsgSubmitEvidence instance.
func NewMsgSubmitEvidence(method *abi.Method, args []interface{}) (*evidencetypes.MsgSubmitEvidence, common.Address, error) {
	emptyAddr := common.Address{}
	if len(args) != 2 {
		return nil, emptyAddr, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	submitterAddress, ok := args[0].(common.Address)
	if !ok {
		return nil, emptyAddr, fmt.Errorf("invalid submitter address")
	}

	inputs := method.Inputs[1:] // Skip the first input which is the submitter address
	args = args[1:]
	var input SubmitEquivocationInput
	if err := inputs.Copy(&input, args); err != nil {
		return nil, emptyAddr, fmt.Errorf("failed to copy inputs: %w", err)
	}

	equivocation := input.Equivocation.ToEquivocation()
	if equivocation == nil {
		return nil, emptyAddr, fmt.Errorf("invalid equivocation evidence")
	}

	msg, err := evidencetypes.NewMsgSubmitEvidence(
		submitterAddress.Bytes(),
		equivocation,
	)
	if err != nil {
		return nil, emptyAddr, fmt.Errorf("failed to create evidence message: %w", err)
	}

	return msg, submitterAddress, nil
}
