package op

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/mempool"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"
)

type ProposalBuilderConfig struct {
	// RebuildTimeout is the duration to wait in between trying to build a new
	// proposal.
	RebuildTimeout time.Duration
}

var DefaultConfig = ProposalBuilderConfig{
	RebuildTimeout: 100 * time.Millisecond,
}

const (
	proposalPerHeightKey        = "optimistic_proposal_proposals_per_height"
	proposalCreationDurationKey = "optimistic_proposal_creation_duration"
	worseProposalsKey           = "optimistic_proposal_worse_proposals"
	prepareProposalWaitDuration = "optimistic_prepare_proposal_wait_duration"
)

// ProposalBuilder maintains the current 'best' proposal that is available, by
// continuously calling the PrepareProposalHandler as soon as possible for a
// new height.
//
// The amount of time in between creating new proposals can be
// configured via the RebuildTimeout.
type ProposalBuilder struct {
	// proposalHeight is the height that the proposal builder is building
	// proposals for
	proposalHeight int64

	// latestProposal is the current best proposal for proposalHeight
	latestProposal *abci.ResponsePrepareProposal
	// lock protects latestProposal and proposalHeight
	lock sync.Mutex

	// proposalsCreated signals if any proposal has been created for this
	// height
	proposalsCreated chan struct{}

	// done signals the main async loop to stop
	done   chan struct{}
	isDone atomic.Bool

	config *ProposalBuilderConfig
	logger log.Logger
}

// NewProposalBuilder creates and starts a new ProposalBuilder instance.
func NewProposalBuilder(
	height int64,
	config *ProposalBuilderConfig,
	logger log.Logger,
) *ProposalBuilder {
	return &ProposalBuilder{
		proposalHeight:   height,
		latestProposal:   &abci.ResponsePrepareProposal{},
		logger:           logger.With(log.ModuleKey, "proposalbuilder"),
		done:             make(chan struct{}),
		proposalsCreated: make(chan struct{}),
		config:           config,
	}
}

// PrepareProposalHandler returns the latest proposal in the ProposalBuilder,
// conforming to the sdk.PrepareProposalHandler signature.
func (pb *ProposalBuilder) PrepareProposalHandler(ctx sdk.Context, _ *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	pb.waitForProposalCreation()
	return pb.LatestProposal(), nil
}

// LatestProposal returns the latest proposal in the ProposalBuilder.
func (pb *ProposalBuilder) LatestProposal() *abci.ResponsePrepareProposal {
	pb.lock.Lock()
	defer pb.lock.Unlock()
	return pb.latestProposal
}

// Stop signals the ProposalBuilder to stop creating new proposals. No new
// proposals will be created after this.
func (pb *ProposalBuilder) Stop() {
	close(pb.done)
	pb.isDone.Store(true)
}

// BuildAsync will kick off a background routine that will create new proposals
// at the ProposalsBuilders height, making them available via LatestProposal.
func (pb *ProposalBuilder) BuildAsync(
	ctx sdk.Context,
	evmMempool *mempool.ExperimentalEVMMempool,
	txVerifier baseapp.ProposalTxVerifier,
	app *baseapp.BaseApp,
) {
	// create a cancellable sdk context
	cancellable, cancel := context.WithCancel(ctx.Context())
	ctx = ctx.WithContext(cancellable)

	var (
		// ticker will tick every time we should start building a new proposal
		ticker           = time.NewTicker(time.Nanosecond)
		proposalsCreated = 0
	)

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				proposalContext := pb.newProposalContext(ctx, app)
				pb.logger.Debug("building a new proposal", "height", proposalContext.BlockHeight())

				proposalsCreated++
				telemetry.SetGauge(float32(proposalsCreated), proposalPerHeightKey)

				start := time.Now()
				proposal := pb.build(proposalContext, evmMempool, txVerifier, app)
				pb.setLatestProposal(proposalContext, time.Since(start), proposal)
				telemetry.MeasureSince(start, proposalCreationDurationKey) //nolint:staticcheck

				ticker.Reset(pb.config.RebuildTimeout)
			case <-pb.done:
				pb.logger.Info("OP received done signal, cancelling context for any proposals being built", "for_height", ctx.BlockHeight())
				// tell any in progress operations to stop
				cancel()
				return
			}
		}
	}()
}

// build creates a new proposal.
func (pb *ProposalBuilder) build(
	ctx sdk.Context,
	evmMempool *mempool.ExperimentalEVMMempool,
	txVerifier baseapp.ProposalTxVerifier,
	app *baseapp.BaseApp,
) *abci.ResponsePrepareProposal {
	// create a per 'build' instance of the ProposalHandler since the way the
	// internal tx selector is used is not thread safe
	abciProposalHandler := baseapp.NewDefaultProposalHandler(evmMempool, txVerifier)
	abciProposalHandler.SetSignerExtractionAdapter(
		mempool.NewEthSignerExtractionAdapter(
			sdkmempool.NewDefaultSignerExtractionAdapter(),
		),
	)
	abciProposalHandler.SetTxSelector(mempool.NewNoCopyProposalTxSelector())

	// TODO: there is an issue that must be solved here.
	// CometBFT typically supplies this value in the
	// PrepareProposalRequest, it uses the MaxBlockBytes as
	// used here, and then subtracts away the space that it
	// knows will be used for storing encoding info, header
	// info, validator commits, and evidence. We do not have
	// access to that info at this point.
	//
	// One possible solution would be to introduce a new ABCI
	// method for CometBFT to inform the application of the
	// smaller MaxTxBytes immediately after Commit is
	// processed, which is when we need to start building
	// proposals.
	const blockOverheadBytes = 2000 // 2000 will suffice for comets overhead assuming 5 vals and no evidence
	maxTxBytes := app.GetConsensusParams(ctx).Block.MaxBytes - blockOverheadBytes

	// TODO: there is a lot of state setup done in baseapp that happens
	// before the PrepareProposalHandler is called. I don't think this
	// is actually required here, but this should be investigated more.
	resp, err := abciProposalHandler.PrepareProposalHandler()(ctx, &abci.RequestPrepareProposal{
		MaxTxBytes: maxTxBytes,
		Height:     ctx.BlockHeight(),
	})
	if err != nil {
		pb.logger.Error("proposal builder failed to prepare proposal", "height", ctx.BlockHeight(), "err", err)
		resp = &abci.ResponsePrepareProposal{}
	}
	return resp
}

// newProposalContext creates a branched context from ctx that contains all the
// relevant info to create a proposal (gas meter, height, consensus params).
func (pb *ProposalBuilder) newProposalContext(ctx sdk.Context, app *baseapp.BaseApp) sdk.Context {
	// cache context so that we do not directly modify chains context and have
	// a unique context per prepare proposal request
	proposalContext, _ := ctx.CacheContext()

	// add a gas meter to the context with the chains max block gas if
	// available
	meter := storetypes.NewInfiniteGasMeter()
	if maxGas := app.GetMaximumBlockGas(proposalContext); maxGas > 0 {
		meter = storetypes.NewGasMeter(maxGas)
	}

	// return an updated context
	return proposalContext.
		WithBlockHeight(pb.GetHeight()).
		WithConsensusParams(app.GetConsensusParams(proposalContext)).
		WithBlockGasMeter(meter)
}

// setLatestProposal updates the PendingBuilders LatestProposal, if the
// response is valid for the PendingBuilders current height and it is 'better'
// than the current LatestProposal.
func (pb *ProposalBuilder) setLatestProposal(ctx sdk.Context, dur time.Duration, resp *abci.ResponsePrepareProposal) {
	pb.lock.Lock()
	defer pb.lock.Unlock()

	// only if there are more txs in the new proposal than the current latest
	// proposal will we replace it (since proposals are created in goroutines,
	// a very early goroutine that is spawned could get unlucky scheduling and
	// finish after a later goroutine and replace a better proposal with a tiny
	// one, this check avoids that)
	newTxsCount := len(resp.GetTxs())
	oldTxsCount := len(pb.latestProposal.GetTxs())

	if newTxsCount != oldTxsCount {
		pb.logger.Info("found new best proposal", "num_txs", len(resp.Txs), "height", ctx.BlockHeight(), "dur", dur.String())
	}
	pb.latestProposal = resp
	pb.signalPropsoalDone()
}

func (pb *ProposalBuilder) signalPropsoalDone() {
	// if the proposals created channel is empty, push a value onto it to
	// signal that a proposal has been created for pb.proposalHeight. If there
	// is already a value in there, do nothing since this means a proposal has
	// already been created for this height, and signaling again does not
	// matter (this is only used to signal that atleast one proposal has been
	// created at pb.proposalHeight).
	select {
	case pb.proposalsCreated <- struct{}{}:
	default:
	}
}

func (pb *ProposalBuilder) waitForProposalCreation() {
	defer func(t0 time.Time) { telemetry.MeasureSince(t0, prepareProposalWaitDuration) }(time.Now()) //nolint:staticcheck

	// try and pull a value off of the chan that signals if a proposal has been
	// created. a value will be pushed to a channel once atleast one proposals
	// have been created.

	// TODO: do we want to timeout and return an empty proposal after some
	// time?
	<-pb.proposalsCreated
}

func (pb *ProposalBuilder) IsStopped() bool {
	return pb.isDone.Load()
}

func (pb *ProposalBuilder) GetHeight() int64 {
	pb.lock.Lock()
	defer pb.lock.Unlock()
	return pb.proposalHeight
}
