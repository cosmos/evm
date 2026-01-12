package keeper

import (
	"github.com/ethereum/go-ethereum/common"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	evmtrace "github.com/cosmos/evm/trace"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GetCoinbaseAddress converts the block proposer's validator operator address to an Ethereum address
// for use as block.coinbase in the EVM.
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
		return common.Address{}, errorsmod.Wrapf(
			stakingtypes.ErrNoValidatorFound,
			"failed to retrieve validator from block proposer address %s. Error: %s",
			proposerAddress.String(),
			err.Error(),
		)
	}

	bz, err := sdk.ValAddressFromBech32(validator.GetOperator())
	if err != nil {
		return common.Address{}, errorsmod.Wrapf(
			err,
			"failed to convert validator operator address %s to bytes",
			validator.GetOperator(),
		)
	}
	return common.BytesToAddress(bz), nil
}

// GetProposerAddress returns current block proposer's address when provided proposer address is empty.
func GetProposerAddress(ctx sdk.Context, proposerAddress sdk.ConsAddress) sdk.ConsAddress {
	if len(proposerAddress) == 0 {
		proposerAddress = ctx.BlockHeader().ProposerAddress
	}
	return proposerAddress
}
