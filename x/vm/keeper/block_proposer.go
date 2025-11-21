package keeper

import (
	"github.com/ethereum/go-ethereum/common"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// GetCoinbaseAddress returns the block proposer's validator operator address.
func (k Keeper) GetCoinbaseAddress(ctx sdk.Context, proposerAddress sdk.ConsAddress) (common.Address, error) {
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
