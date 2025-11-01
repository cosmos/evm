package keeper

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	authante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// CheckSenderBalance validates that the tx cost value is positive and that the
// sender has enough funds to pay for the fees and value of the transaction.
func CheckSenderBalance(
	balance sdkmath.Int,
	ethTx *ethtypes.Transaction,
) error {
	cost := ethTx.Cost()

	if cost.Sign() < 0 {
		return errorsmod.Wrapf(
			errortypes.ErrInvalidCoins,
			"tx cost (%s) is negative and invalid", cost,
		)
	}

	if balance.IsNegative() || balance.BigInt().Cmp(cost) < 0 {
		return errorsmod.Wrapf(
			errortypes.ErrInsufficientFunds,
			"sender balance < tx cost (%s < %s)", balance, cost,
		)
	}
	return nil
}

// DeductTxCostsFromUserBalance deducts the fees from the user balance.
func (k *Keeper) DeductTxCostsFromUserBalance(
	ctx sdk.Context,
	fees sdk.Coins,
	from common.Address,
) error {
	// fetch sender account
	signerAcc, err := authante.GetSignerAcc(ctx, k.accountKeeper, from.Bytes())
	if err != nil {
		return errorsmod.Wrapf(err, "account not found for sender %s", from)
	}

	// Deduct fees from the user balance. Notice that it is used
	// the bankWrapper to properly convert fees from the 18 decimals
	// representation to the original one before calling into the bank keeper.
	if k.virtualFeeCollection {
		err = DeductFees(k.bankWrapper, k, ctx, signerAcc, fees)
	} else {
		err = authante.DeductFees(k.bankWrapper, ctx, signerAcc, fees)
	}
	if err != nil {
		return errorsmod.Wrapf(err, "failed to deduct full gas cost %s from the user %s balance", fees, from)
	}

	return nil
}

// VerifyFee is used to return the fee for the given transaction data in sdk.Coins. It checks that the
// gas limit is not reached, the gas limit is higher than the intrinsic gas and that the
// base fee is higher than the gas fee cap.
func VerifyFee(
	ethTx *ethtypes.Transaction,
	denom string,
	baseFee *big.Int,
	homestead, istanbul, shanghai, isCheckTx bool,
) (sdk.Coins, error) {
	isContractCreation := ethTx.To() == nil

	gasLimit := ethTx.Gas()

	var accessList ethtypes.AccessList
	if ethTx.AccessList() != nil {
		accessList = ethTx.AccessList()
	}

	authList := ethTx.SetCodeAuthorizations()
	intrinsicGas, err := core.IntrinsicGas(ethTx.Data(), accessList, authList, isContractCreation, homestead,
		istanbul, shanghai)
	if err != nil {
		return nil, errorsmod.Wrapf(
			err,
			"failed to retrieve intrinsic gas, contract creation = %t; homestead = %t, istanbul = %t",
			isContractCreation, homestead, istanbul,
		)
	}

	// intrinsic gas verification during CheckTx
	if isCheckTx && gasLimit < intrinsicGas {
		return nil, errorsmod.Wrapf(
			errortypes.ErrOutOfGas,
			"gas limit too low: %d (gas limit) < %d (intrinsic gas)", gasLimit, intrinsicGas,
		)
	}

	if baseFee != nil && ethTx.GasFeeCap().Cmp(baseFee) < 0 {
		return nil, errorsmod.Wrapf(errortypes.ErrInsufficientFee,
			"the tx gasfeecap is lower than the tx baseFee: %s (gasfeecap), %s (basefee) ",
			ethTx.GasFeeCap(),
			baseFee)
	}

	gasTip, _ := ethTx.EffectiveGasTip(baseFee)
	price := new(big.Int).Add(gasTip, baseFee)
	gas := new(big.Int).SetUint64(ethTx.Gas())
	feeAmt := gas.Mul(gas, price)
	if feeAmt.Sign() == 0 {
		// zero fee, no need to deduct
		return sdk.Coins{}, nil
	}

	return sdk.Coins{{Denom: denom, Amount: sdkmath.NewIntFromBigInt(feeAmt)}}, nil
}

// DeductFees deducts fees from the given account.
func DeductFees(bankKeeper types.BankKeeper, vmKeeper types.VMKeeper, ctx sdk.Context, acc sdk.AccountI, fees sdk.Coins) error {
	if !fees.IsValid() {
		return errorsmod.Wrapf(errortypes.ErrInsufficientFee, "invalid fee amount: %s", fees)
	}
	evmCoinInfo := vmKeeper.GetEvmCoinInfo(ctx)
	for _, coin := range fees {
		md, found := bankKeeper.GetDenomMetaData(ctx, coin.Denom)
		if !found {
			// DenomMetadata being set for the evm coin is enforced in genesis
			continue
		}
		for _, du := range md.DenomUnits {
			if du.Denom == evmCoinInfo.Denom && du.Exponent != evmCoinInfo.Decimals {
				panic(
					fmt.Sprintf(
						"Cannot use virtual fee collection for denom, %s, which has a different exponent, %d, than the evm coin's %d",
						du.Denom, du.Exponent, evmCoinInfo.Decimals))
			}
		}
	}

	err := bankKeeper.SendCoinsFromAccountToModuleVirtual(ctx, acc.GetAddress(), authtypes.FeeCollectorName, fees)
	if err != nil {
		return errorsmod.Wrap(errortypes.ErrInsufficientFunds, err.Error())
	}

	return nil
}
