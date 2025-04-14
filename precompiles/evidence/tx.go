package evidence

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	evidencekeeper "cosmossdk.io/x/evidence/keeper"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// SubmitEvidence implements the evidence submission logic for the evidence precompile.
func (p Precompile) SubmitEvidence(
	ctx sdk.Context,
	origin common.Address,
	_ *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, err := NewMsgSubmitEvidence(origin, args)
	if err != nil {
		return nil, err
	}

	msgServer := evidencekeeper.NewMsgServerImpl(p.evidenceKeeper)
	res, err := msgServer.SubmitEvidence(ctx, msg)
	if err != nil {
		return nil, err
	}

	if err = p.EmitSubmitEvidenceEvent(ctx, stateDB, origin, res.Hash); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}
