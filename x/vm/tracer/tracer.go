package tracer

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
)

// Tracer is the common contract for application tracers managed during
// one EVM execution. Concrete implementations own their collection policy and
// expose only the geth hooks needed by Manager.
type Tracer interface {
	Hooks() *tracing.Hooks
}

// ObservationKind identifies a raw fact, not a policy decision.
type ObservationKind string

const (
	CallEnter      ObservationKind = "call_enter"
	CallExit       ObservationKind = "call_exit"
	NativeTransfer ObservationKind = "native_transfer"
	ERC20Call      ObservationKind = "erc20_call"
	Log            ObservationKind = "log"
	BalanceChange  ObservationKind = "balance_change"
)

// UserOperationEvidence must be supplied at an authenticated execution boundary
// by an EntryPoint adapter. The generic EVM never infers it from calldata or
// sender equality. Phase distinguishes validation from account execution.
type UserOperationEvidence struct {
	EntryPoint  common.Address
	BundleIndex uint64
	Sender      common.Address
	Hash        common.Hash
	Phase       string
}

// Observation is an immutable snapshot delivered in execution order. IDs are
// execution-local and shared by every observer. CallID identifies
// the active call; ParentCallID is zero for roots. A CallEnter is an attempt,
// even if its call or an ancestor later reverts. BalanceChange is a delta,
// whereas NativeTransfer records an executed balance transfer. ERC20Call is
// request evidence; Log retains the contract emission for resolver correlation.
type Observation struct {
	ObservationID    uint64
	CallID           uint64
	ParentCallID     uint64
	Kind             ObservationKind
	Opcode           byte
	Depth            int
	Status           CallStatus
	Caller           common.Address
	Target           common.Address
	ExecutionAddress common.Address
	CodeAddress      common.Address
	CodeAddressKnown bool
	Input            []byte
	Value            *big.Int
	Output           []byte
	Gas              uint64
	Err              error
	Reverted         bool
	Log              *ethtypes.Log
	PreviousBalance  *big.Int
	CurrentBalance   *big.Int
	BalanceReason    tracing.BalanceChangeReason
	UserOperation    *UserOperationEvidence
	From             common.Address
	To               common.Address
	Amount           *big.Int
}

// Observer errors are latched until the enclosing execution is rejected. They
// survive EVM reverts, including reverts caught by an ancestor.
type Observer func(Observation) error

// CallStatus reports the local exit and current effective rollback status.
// Success is provisional while ancestors are still executing.
type CallStatus struct {
	Exited   bool
	Success  bool
	Reverted bool
	Error    error
}

// Journal is the execution journal owned by an application collector.
// Observers share its records and execution-local IDs.
type Journal interface {
	Tracer
	Observations() []Observation
	CurrentCall() (Observation, bool)
	LastObservationID() uint64
	SetCodeReader(func(common.Address) []byte) func()
	RecordNativeTransfer(common.Address, common.Address, *big.Int)
	LatchError(error)
	Error() error
	ExecutionError() error
}
