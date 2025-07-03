package keeper

import (
	"bytes"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (k *Keeper) AddPreinstalls(ctx sdk.Context, preinstalls []types.Preinstall) error {
	for _, preinstall := range preinstalls {
		address := common.HexToAddress(preinstall.Address)
		accAddress := sdk.AccAddress(address.Bytes())

		if len(preinstall.Code) == 0 {
			return errorsmod.Wrapf(types.ErrInvalidPreinstall, "preinstall %s has no code", preinstall.Address)
		}

		codeHash := crypto.Keccak256Hash(common.FromHex(preinstall.Code)).Bytes()
		if types.IsEmptyCodeHash(codeHash) {
			return errorsmod.Wrapf(types.ErrInvalidPreinstall, "preinstall %s has empty code hash", preinstall.Address)
		}

		existingCodeHash := k.GetCodeHash(ctx, address)
		if !types.IsEmptyCodeHash(existingCodeHash.Bytes()) && !bytes.Equal(existingCodeHash.Bytes(), codeHash) {
			return errorsmod.Wrapf(types.ErrInvalidPreinstall, "preinstall %s already has a code hash with a different code hash", preinstall.Address)
		}

		// check that the account is not already set
		if acc := k.accountKeeper.GetAccount(ctx, accAddress); acc != nil {
			return errorsmod.Wrapf(types.ErrInvalidPreinstall, "preinstall %s already has an account in account keeper", preinstall.Address)
		}
		// create account with the account keeper
		account := k.accountKeeper.NewAccountWithAddress(ctx, accAddress)
		k.accountKeeper.SetAccount(ctx, account)

		k.SetCodeHash(ctx, address.Bytes(), codeHash)

		k.SetCode(ctx, codeHash, common.FromHex(preinstall.Code))

		// We are not setting any storage for preinstalls, so we skip that step.
	}
	return nil
}
