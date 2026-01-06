package keeper

import (
	"github.com/ethereum/go-ethereum/common"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	evmtrace "github.com/cosmos/evm/trace"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// GetCoinbaseAddress returns the block proposer's validator operator address.
// Returns zero address if any error occurs.
func (k Keeper) GetCoinbaseAddress(ctx sdk.Context, proposerAddress sdk.ConsAddress) (_ common.Address, err error) {
	ctx, span := ctx.StartSpan(tracer, "GetCoinbaseAddress", trace.WithAttributes(
		attribute.String("proposer_address", proposerAddress.String()),
	))
	defer func() { evmtrace.EndSpanErr(span, err) }()
	proposerAddress = GetProposerAddress(ctx, proposerAddress)
	if len(proposerAddress) == 0 {
		// it's ok that proposer address don't exsits in some contexts like CheckTx.
		return common.Address{}, nil
	}
	validator, err := k.stakingKeeper.GetValidatorByConsAddr(ctx, proposerAddress)
	if err != nil {
		return common.Address{}, nil
	}

	coinbase := common.BytesToAddress([]byte(validator.GetOperator()))
	return coinbase, nil
}

// GetProposerAddress returns current block proposer's address when provided proposer address is empty.
func GetProposerAddress(ctx sdk.Context, proposerAddress sdk.ConsAddress) sdk.ConsAddress {
	if len(proposerAddress) == 0 {
		proposerAddress = ctx.BlockHeader().ProposerAddress
	}
	return proposerAddress
}
