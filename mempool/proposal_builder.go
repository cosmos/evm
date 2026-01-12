package mempool

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/core"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	"github.com/cosmos/cosmos-sdk/telemetry"
	sdkmempool "github.com/cosmos/cosmos-sdk/types/mempool"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type ProposalBuilderConfig struct {
	// RebuildTimeout is the duration to wait in between trying to build a new
	// proposal.
	RebuildTimeout time.Duration
}

const (
	DefaultRebuildTimeout = 100 * time.Millisecond

	inflightProposalsKey        = "proposalbuilder_inflight_proposals"
	proposalPerHeightKey        = "proposalbuilder_proposals_per_height"
	proposalCreationDurationKey = "proposalbuilder_proposal_creation_duration"
)

// ProposalBuilder maintains the current 'best' proposal that is available, by
// continuously calling the PrepareProposalHandler as soon as possible for a
// new height.
//
// The amount of time in between creating new proposals can be
// configured via the RebuildTimeout.
type ProposalBuilder struct {
	evmMempool *ExperimentalEVMMempool
	txVerifier baseapp.ProposalTxVerifier

	// chain is a reference to the blockchain.
	chain *Blockchain

	// app is a reference to the running application.
	app *baseapp.BaseApp

	// proposalHeight is the height that the proposals are currently being
	// created for.
	proposalHeight int64

	// latestProposal is the current best proposal for proposalHeight.
	latestProposal *abci.ResponsePrepareProposal

	// lock protects proposalHeight and latestProposal.
	lock sync.Mutex

	// done will be closed when we should stop processing.
	done chan struct{}

	config *ProposalBuilderConfig
	logger log.Logger
}

// NewProposalBuilder creates and starts a new ProposalBuilder instance.
func NewProposalBuilder(
	evmMempool *ExperimentalEVMMempool,
	txVerifier baseapp.ProposalTxVerifier,
	chain *Blockchain,
	app *baseapp.BaseApp,
	logger log.Logger,
	config *ProposalBuilderConfig,
) *ProposalBuilder {
	if config.RebuildTimeout == time.Duration(0) {
		config.RebuildTimeout = DefaultRebuildTimeout
	}

	pb := &ProposalBuilder{
		evmMempool:     evmMempool,
		txVerifier:     txVerifier,
		app:            app,
		chain:          chain,
		logger:         logger.With(log.ModuleKey, "proposalbuilder"),
		latestProposal: &abci.ResponsePrepareProposal{},
		done:           make(chan struct{}),
		config:         config,
	}
	go pb.loop()
	return pb
}

// PrepareProposalHandler returns the latest proposal in the ProposalBuilder,
// conforming to the sdk.PrepareProposalHandler signature.
func (pb *ProposalBuilder) PrepareProposalHandler(_ sdk.Context, _ *abci.RequestPrepareProposal) (*abci.ResponsePrepareProposal, error) {
	return pb.LatestProposal(), nil
}

// LatestProposal returns the latest proposal in the ProposalBuilder.
func (pb *ProposalBuilder) LatestProposal() *abci.ResponsePrepareProposal {
	pb.lock.Lock()
	defer pb.lock.Unlock()

	// TODO: Here we should inform the main loop that it does not need to
	// continue building new proposals at this height (since likely this
	// proposal will be accepted). However, if the consensus round increments
	// because this proposal was rejected, we will need to rebuild a new
	// proposal, and if we have stopped the main loop that will never happen.
	// Thus to do this optimization we would also need to restart the loop on
	// round increments.
	return pb.latestProposal
}

// Stop signals the ProposalBuilder to stop creating new proposals. No new
// proposals will be created after this.
func (pb *ProposalBuilder) Stop() {
	close(pb.done)
}

// loop is the main loop that will create new proposals.
func (pb *ProposalBuilder) loop() {
	// subscribe to new chain head events on the chain
	newHeadCh := make(chan core.ChainHeadEvent)
	sub := pb.chain.SubscribeChainHeadEvent(newHeadCh)
	defer sub.Unsubscribe()

	// ticker will tick every time we should start building  new proposal
	ticker := time.NewTicker(pb.config.RebuildTimeout)
	defer ticker.Stop()

	var (
		latestContext      sdk.Context
		inflightProposals  int
		proposalsPerHeight int
	)
	for {
		select {
		case event := <-newHeadCh:
			pb.logger.Debug("resetting latest proposal, received new header", "height", event.Header.Number.Int64())
			pb.resetLatestProposal()

			// fetch latest context on chain. we know this has been updated
			// since the newHeadCh receives events from the chain only after
			// its context has been updated
			ctx, err := pb.chain.GetLatestContext()
			if err != nil {
				pb.logger.Error("error getting latest context", "height", event.Header.Number, "err", err)
			}
			latestContext = ctx
			if !ctx.IsZero() {
				// proposals should be processed at one height after the height
				// they are being build on top of
				pb.lock.Lock()
				pb.proposalHeight = latestContext.BlockHeight() + 1
				pb.lock.Unlock()
			}

			proposalsPerHeight = 0

			// reset ticker by creating a new one, since it may have been
			// stopped
			pb.logger.Debug("done resetting proposal")
			ticker.Reset(pb.config.RebuildTimeout)
		case <-ticker.C:
			// rebuild proposal
			if latestContext.IsZero() {
				continue
			}

			// running a goroutine here to avoid a situation where we get stuck
			// in a long proposal and that blocks us from receiving newHead
			// events.
			//
			// if we are unable to receive newHead events, then that may cause
			// this loop to be multiple heights behind once the call to
			// prepareProposal finishes. Then, we will call prepareProposal on
			// a height in the past, which will panic.
			proposalContext := pb.newProposalContext(latestContext)
			pb.logger.Debug("building a new proposal", "for_height", proposalContext.BlockHeight())

			// metrics
			proposalsPerHeight++
			telemetry.SetGauge(float32(proposalsPerHeight), proposalPerHeightKey) // TODO: change to historgram when we get otel
			inflightProposals++
			telemetry.SetGauge(float32(inflightProposals), inflightProposalsKey)

			go func() {
				defer func(t0 time.Time) { telemetry.MeasureSince(t0, proposalCreationDurationKey) }(time.Now())

				// We create a per goroutine instance of the ProposalHandler
				// since it the internal txSelector is not thread safe.
				abciProposalHandler := baseapp.NewDefaultProposalHandler(pb.evmMempool, pb.txVerifier)
				abciProposalHandler.SetSignerExtractionAdapter(
					NewEthSignerExtractionAdapter(
						sdkmempool.NewDefaultSignerExtractionAdapter(),
					),
				)
				abciProposalHandler.SetTxSelector(NewNoCopyProposalTxSelector())

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
				maxTxBytes := pb.app.GetConsensusParams(proposalContext).Block.MaxBytes - blockOverheadBytes

				req := &abci.RequestPrepareProposal{
					MaxTxBytes: maxTxBytes,
					Height:     proposalContext.BlockHeight(),
				}

				// TODO: there is a lot of state setup done in baseapp that happens
				// before the PrepareProposalHandler is called. I don't think this
				// is actually required here, but this should be investigated more.
				resp, err := abciProposalHandler.PrepareProposalHandler()(proposalContext, req)
				if err != nil {
					pb.logger.Error("failed to prepare proposal", "height", req.Height, "err", err)
					resp = &abci.ResponsePrepareProposal{}
				}
				pb.setLatestProposal(proposalContext.BlockHeight(), resp)
				pb.logger.Debug("created a new proposal", "for_height", proposalContext.BlockHeight())

				inflightProposals--
				telemetry.SetGauge(float32(inflightProposals), inflightProposalsKey)
			}()
			ticker.Reset(pb.config.RebuildTimeout)
		case <-pb.done:
			return
		}
	}
}

// newProposalContext creates a branched context from ctx that contains all the
// relevant info to create a proposal (gas meter, height, consensus params).
func (pb *ProposalBuilder) newProposalContext(ctx sdk.Context) sdk.Context {
	// cache context so that we do not directly modify chains context and have
	// a unique context per prepare proposal request
	proposalContext, _ := ctx.CacheContext()

	// add a gas meter to the context with the chains max block gas if
	// available
	meter := storetypes.NewInfiniteGasMeter()
	if maxGas := pb.app.GetMaximumBlockGas(proposalContext); maxGas > 0 {
		meter = storetypes.NewGasMeter(maxGas)
	}

	// return an updated context
	return proposalContext.
		WithBlockHeight(pb.getHeight()).
		WithConsensusParams(pb.app.GetConsensusParams(proposalContext)).
		WithBlockGasMeter(meter)
}

// setLatestProposal updates the PendingBuilders LatestProposal, if the
// response is valid for the PendingBuilders current height and it is 'better'
// than the current LatestProposal.
func (pb *ProposalBuilder) setLatestProposal(height int64, resp *abci.ResponsePrepareProposal) {
	pb.lock.Lock()
	defer pb.lock.Unlock()

	// it is possible that the chain has already moved on by the time that this
	// proposal finished processing. if so disregard this proposal since its
	// for an old height.
	if height < pb.proposalHeight {
		return
	}

	// only if there are more txs in the new proposal than the current latest
	// proposal will we replace it (since proposals are created in goroutines,
	// a very early goroutine that is spawned could get unlucky scheduling and
	// finish after a later goroutine and replace a better proposal with a tiny
	// one, this check avoids that)
	if len(resp.Txs) <= len(pb.latestProposal.Txs) {
		return
	}

	pb.logger.Info("found new best proposal", "num_txs", len(resp.Txs))
	pb.latestProposal = resp
}

func (pb *ProposalBuilder) resetLatestProposal() {
	pb.lock.Lock()
	defer pb.lock.Unlock()
	pb.latestProposal = &abci.ResponsePrepareProposal{}
}

func (pb *ProposalBuilder) getHeight() int64 {
	pb.lock.Lock()
	defer pb.lock.Unlock()
	return pb.proposalHeight
}
