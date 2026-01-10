package mempool

import (
	"sync"
	"time"

	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/core"
)

const (
	DefaultRebuildDuration = 100 * time.Millisecond
)

type ProposalBuilder struct {
	// prepareProposal creates a new proposal.
	prepareProposal sdk.PrepareProposalHandler

	// chain is a reference to the blockchain.
	chain *Blockchain

	// app is a reference to the running application.
	app *baseapp.BaseApp

	// rebuildDuration is the duration to wait in between trying to build a new
	// proposal.
	rebuildDuration time.Duration

	// proposalHeight is the height that the proposals are currently being
	// created for.
	proposalHeight int64

	// latestProposal is the current best proposal for proposalHeight.
	latestProposal *abci.ResponsePrepareProposal

	// lock protects proposalHeight and latestProposal.
	lock sync.Mutex

	logger log.Logger
	done   chan struct{}
}

func NewProposalBuilder(
	prepareProposal sdk.PrepareProposalHandler,
	chain *Blockchain,
	app *baseapp.BaseApp,
	rebuildDuration time.Duration,
	logger log.Logger,
) *ProposalBuilder {
	if rebuildDuration == time.Duration(0) {
		rebuildDuration = DefaultRebuildDuration
	}

	pb := &ProposalBuilder{
		prepareProposal: prepareProposal,
		app:             app,
		chain:           chain,
		rebuildDuration: rebuildDuration,
		logger:          logger.With(log.ModuleKey, "ProposalBuilder"),
		latestProposal:  &abci.ResponsePrepareProposal{},
		done:            make(chan struct{}),
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

func (pb *ProposalBuilder) loop() {
	// subscribe to new chain head events on the chain
	newHeadCh := make(chan core.ChainHeadEvent)
	sub := pb.chain.SubscribeChainHeadEvent(newHeadCh)
	defer sub.Unsubscribe()

	// ticker will tick every time we should start building  new proposal
	ticker := time.NewTicker(pb.rebuildDuration)
	defer ticker.Stop()

	var latestContext sdk.Context
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

			// reset ticker by creating a new one, since it may have been
			// stopped
			pb.logger.Debug("done resetting proposal")
			ticker.Reset(pb.rebuildDuration)
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
			go func() {
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
				resp, err := pb.prepareProposal(proposalContext, req)
				if err != nil {
					pb.logger.Error("failed to prepare proposal", "height", req.Height, "err", err)
					resp = &abci.ResponsePrepareProposal{}
				}
				pb.setLatestProposal(proposalContext.BlockHeight(), resp)
				pb.logger.Debug("created a new proposal", "for_height", proposalContext.BlockHeight())
			}()
			ticker.Reset(pb.rebuildDuration)
		case <-pb.done:
			return
		}
	}
}

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
