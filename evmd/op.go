package evmd

import (
	"context"
	"fmt"
	"time"

	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/op"
)

const (
	commitToProposeDuruationKey = "op_commit_to_propose_duration"
)

func (app *EVMD) NewOptimisticPrepareProposalHandler(defaultHandler sdk.PrepareProposalHandler) sdk.PrepareProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
		height := req.Height

		if height > 1 {
			telemetry.MeasureSince(app.lastCommitTime, commitToProposeDuruationKey)
		}

		// optimistic proposal not running for this height or could have
		// already been stopped or never started, i.e. genesis, or incremented
		// rounds, fallback to creating a proposal synchronously
		if height == 1 || (height > 1 && app.optimisticProp.IsStopped()) {
			app.Logger().Warn("no optimistic proposal instance running at height, falling back to synchronous proposal", "height", height)
			return defaultHandler(ctx, req)
		}

		// sanity check, optimistic proposal should always be at the same
		// height that comet thinks we are at
		if app.optimisticProp.GetHeight() != height {
			panic(fmt.Errorf("mismatch between optimistic proposal height %d and prepare proposal request height %d", app.optimisticProp.GetHeight(), req.GetHeight()))
		}

		// get latest proposal
		resp, err := app.optimisticProp.PrepareProposalHandler(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("getting proposal from optimistic proposal handler: %w", err)
		}

		// stop the builder
		//
		// NOTE: if this proposal is rejected or the consensus round increments
		// for some other reason, then we may need to produce another proposal.
		// However, we opt to simply stop the builder here to account for the
		// most common case where there are no round increments and we would
		// rather save resources and not create any more resources.
		app.Logger().Info("returned optimistic proposal, stopping optimistic proposal builder", "height", height)
		app.stopOptimisticProposals(height)

		return resp, nil
	}
}

// NewOptimisticProcessProposalHandler returns a function that can be used as a
// ProcessProposalHandler. This handler will stop optimistic proposals (if they
// are running) after the defaultHandler has been invoked and accepted a
// proposal.
func (app *EVMD) NewOptimisticProcessProposalHandler(defaultHandler sdk.ProcessProposalHandler) sdk.ProcessProposalHandler {
	return func(ctx sdk.Context, req *abci.RequestProcessProposal) (*abci.ResponseProcessProposal, error) {
		resp, err := defaultHandler(ctx, req)
		if req.GetHeight() == 1 {
			return resp, err
		}

		// if we are accepting the proposal, we assuming that we are an honest
		// node and others will also accept the proposal, thus it is unlikely
		// that we will increment rounds and need to propose here, and can
		// stop optimistically building proposals.

		// if we were the proposer then we are already stopped
		isRunning := !app.optimisticProp.IsStopped()

		// if we did not accept the proposal, then we should not stop creating
		// new ones since we will likely inc rounds
		proposalAccepted := resp != nil && resp.Status == abci.ResponseProcessProposal_ACCEPT
		if isRunning && proposalAccepted {
			app.Logger().Info("accepted proposal, stopping optimistic proposal builder", "height", req.GetHeight())
			app.stopOptimisticProposals(req.GetHeight())
		}

		return resp, err
	}
}

var _ storetypes.ABCIListener = (*OptimisticProposalCommitListener)(nil)

// OptimisticProposalCommitListener is a struct that can be registered on the
// StreamingManager as a ABCIListener. The OptimisticProposalCommitListener
// sets up optimistic proposals when a Commit is seen.
type OptimisticProposalCommitListener struct {
	app    *EVMD
	config *op.ProposalBuilderConfig
}

// NewOptimisticProposalCommitListener creates a new OptimisticProposalCommitListener.
func NewOptimisticProposalCommitListener(app *EVMD, config *op.ProposalBuilderConfig) *OptimisticProposalCommitListener {
	return &OptimisticProposalCommitListener{app, config}
}

// ListenCommit resets the optimistic proposal instance on the application to
// one that will create proposals for the next height and starts it.
func (listener *OptimisticProposalCommitListener) ListenCommit(
	ctx context.Context,
	res abci.ResponseCommit,
	changeSet []*storetypes.StoreKVPair,
) error {
	listener.app.lastCommitTime = time.Now()

	// swap out the basectx in the sdk context to be one that is disconnected
	// from its parent. when commit returns the base context will be cancelled,
	// which we do not want, since we rely on context cancellation via a
	// timeout in prepare proposal to know when to stop collecting txs.
	sdkctx := sdk.UnwrapSDKContext(ctx)
	sdkctx = sdkctx.WithContext(context.Background())

	currHeight := sdkctx.BlockHeight()
	nextHeight := sdkctx.BlockHeight() + 1

	// if we have an old optimistic proposal instance still running, stop it.
	// this will happen when we are not the proposer and the old instance was
	// never stopped during prepare proposal.
	if currHeight != 1 && !listener.app.optimisticProp.IsStopped() {
		// TODO: should we panic here? when would this happen? maybe only in
		// some scenario where were never call prepare proposal or process
		// proposal, maybe blocksync?
		listener.app.Logger().Warn("optimistic proposal still running during commit, stopping now", "height", currHeight)
		listener.app.stopOptimisticProposals(currHeight)
	}

	listener.app.Logger().Info(
		"creating proposal builder",
		"height", nextHeight,
	)

	// create instance and start building proposals for the next height
	listener.app.optimisticProp = op.NewProposalBuilder(
		nextHeight,
		listener.config,
		listener.app.Logger(),
	)
	listener.app.optimisticProp.BuildAsync(
		sdkctx,
		listener.app.EVMMempool,
		NewNoCheckProposalTxVerifier(listener.app.BaseApp),
		listener.app.BaseApp,
	)
	return nil
}

// ListenFinalizeBlock is a noop, needed to implement the ABCIListener interface.
func (_ *OptimisticProposalCommitListener) ListenFinalizeBlock(
	ctx context.Context,
	req abci.RequestFinalizeBlock,
	res abci.ResponseFinalizeBlock,
) error {
	return nil
}

func (app *EVMD) stopOptimisticProposals(height int64) {
	if height == 1 {
		// no optimistic proposal running at this height, since there is no
		// previous commit that would have started the optimistic proposal
		// instance
		return
	}
	app.optimisticProp.Stop()
}
