package keeper

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"strconv"

	"github.com/hashicorp/go-metrics"

	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/telemetry"
	sdk "github.com/cosmos/cosmos-sdk/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

var _ types.MsgServer = &Keeper{}

// EthereumTx implements the gRPC MsgServer interface. It receives a transaction which is then
// executed (i.e applied) against the go-ethereum EVM. The provided SDK Context is set to the Keeper
// so that it can implements and call the StateDB methods without receiving it as a function
// parameter.
func (k *Keeper) EthereumTx(goCtx context.Context, msg *types.MsgEthereumTx) (*types.MsgEthereumTxResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	sender := msg.From
	tx := msg.AsTransaction()
	txIndex := k.GetTxIndexTransient(ctx)

	labels := []metrics.Label{
		telemetry.NewLabel("tx_type", fmt.Sprintf("%d", tx.Type())),
	}
	if tx.To() == nil {
		labels = append(labels, telemetry.NewLabel("execution", "create"))
	} else {
		labels = append(labels, telemetry.NewLabel("execution", "call"))
	}

	response, err := k.ApplyTransaction(ctx, tx)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to apply transaction")
	}

	defer func() {
		telemetry.IncrCounterWithLabels(
			[]string{"tx", "msg", "ethereum_tx", "total"},
			1,
			labels,
		)

		if response.GasUsed != 0 {
			telemetry.IncrCounterWithLabels(
				[]string{"tx", "msg", "ethereum_tx", "gas_used", "total"},
				float32(response.GasUsed),
				labels,
			)

			// Observe which users define a gas limit >> gas used. Note, that
			// gas_limit and gas_used are always > 0
			gasLimit := math.LegacyNewDec(int64(tx.Gas()))                        //#nosec G115 -- int overflow is not a concern here -- tx gas is not going to exceed int64 max value
			gasRatio, err := gasLimit.QuoInt64(int64(response.GasUsed)).Float64() //#nosec G115 -- int overflow is not a concern here -- gas used is not going to exceed int64 max value
			if err == nil {
				telemetry.SetGaugeWithLabels(
					[]string{"tx", "msg", "ethereum_tx", "gas_limit", "per", "gas_used"},
					float32(gasRatio),
					labels,
				)
			}
		}
	}()

	attrs := []sdk.Attribute{
		sdk.NewAttribute(sdk.AttributeKeyAmount, tx.Value().String()),
		// add event for ethereum transaction hash format
		sdk.NewAttribute(types.AttributeKeyEthereumTxHash, response.Hash),
		// add event for index of valid ethereum tx
		sdk.NewAttribute(types.AttributeKeyTxIndex, strconv.FormatUint(txIndex, 10)),
		// add event for eth tx gas used, we can't get it from cosmos tx result when it contains multiple eth tx msgs.
		sdk.NewAttribute(types.AttributeKeyTxGasUsed, strconv.FormatUint(response.GasUsed, 10)),
	}

	if len(ctx.TxBytes()) > 0 {
		// add event for tendermint transaction hash format
		hash := cmttypes.Tx(ctx.TxBytes()).Hash()
		attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyTxHash, hex.EncodeToString(hash)))
	}

	if to := tx.To(); to != nil {
		attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyRecipient, to.Hex()))
	}

	if response.Failed() {
		attrs = append(attrs, sdk.NewAttribute(types.AttributeKeyEthereumTxFailed, response.VmError))
	}

	txLogAttrs := make([]sdk.Attribute, len(response.Logs))
	for i, log := range response.Logs {
		value, err := json.Marshal(log)
		if err != nil {
			return nil, errorsmod.Wrap(err, "failed to encode log")
		}
		txLogAttrs[i] = sdk.NewAttribute(types.AttributeKeyTxLog, string(value))
	}

	// emit events
	ctx.EventManager().EmitEvents(sdk.Events{
		sdk.NewEvent(
			types.EventTypeEthereumTx,
			attrs...,
		),
		sdk.NewEvent(
			types.EventTypeTxLog,
			txLogAttrs...,
		),
		sdk.NewEvent(
			sdk.EventTypeMessage,
			sdk.NewAttribute(sdk.AttributeKeyModule, types.AttributeValueCategory),
			sdk.NewAttribute(sdk.AttributeKeySender, sender),
			sdk.NewAttribute(types.AttributeKeyTxType, fmt.Sprintf("%d", tx.Type())),
		),
	})

	return response, nil
}

// UpdateParams implements the gRPC MsgServer interface. When an UpdateParams
// proposal passes, it updates the module parameters. The update can only be
// performed if the requested authority is the Cosmos SDK governance module
// account.
func (k *Keeper) UpdateParams(goCtx context.Context, req *types.MsgUpdateParams) (*types.MsgUpdateParamsResponse, error) {
	if k.authority.String() != req.Authority {
		return nil, errorsmod.Wrapf(govtypes.ErrInvalidSigner, "invalid authority, expected %s, got %s", k.authority.String(), req.Authority)
	}

	ctx := sdk.UnwrapSDKContext(goCtx)
	if err := k.SetParams(ctx, req.Params); err != nil {
		return nil, err
	}

	return &types.MsgUpdateParamsResponse{}, nil
}

func (k *Keeper) MigrateAccount(goCtx context.Context, req *types.MsgMigrateAccount) (*types.MsgMigrateAccountResponse, error) {
	ctx := sdk.UnwrapSDKContext(goCtx)

	const (
		// all active validators in the network
		maxValidators = 180
		// 99.99th percentile of tokens held per account
		maxTokens = 20
		// >99th percentile of feegrants granted per granter
		maxFeeGrants = 55
		// >99th percentile of authz grants granted per granter
		maxAuthzGrants = 5
	)

	// todo: should someone need to sign with both keys?
	// one key signing is probably fine - this functions with similar effect to bank send.
	// on the other hand, may be good for users to confirm that the other address is actually functional;
	// hmm

	originalAddress, err := sdk.AccAddressFromBech32(req.OriginalAddress)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid original address: %s", err)
	}

	if k.bankWrapper.BlockedAddr(originalAddress) {
		return nil, errorsmod.Wrapf(sdkerrors.ErrUnauthorized, "%s is not allowed to receive external funds", originalAddress)
	}

	newAddress, err := sdk.AccAddressFromBech32(req.NewAddress)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid new address: %s", err)
	}

	if k.bankWrapper.BlockedAddr(newAddress) {
		return nil, errorsmod.Wrapf(sdkerrors.ErrUnauthorized, "%s is not allowed to receive external funds", newAddress)
	}

	// call the before-delegation-modified hook
	if err := k.MigrateAccountHooks().BeforeAll(ctx, originalAddress, newAddress); err != nil {
		return nil, err
	}

	// delegations need to be migrated first to withdraw any staking rewards to handle them in the bank migration
	err = k.migrateDelegations(ctx, originalAddress, maxValidators, newAddress)
	if err != nil {
		return nil, err
	}

	if err := k.MigrateAccountHooks().AfterMigrateDelegations(ctx, originalAddress, newAddress); err != nil {
		return nil, err
	}

	err = k.migrateBankTokens(err, ctx, originalAddress, maxTokens, newAddress)
	if err != nil {
		return nil, err
	}

	if err := k.MigrateAccountHooks().AfterMigrateBankTokens(ctx, originalAddress, newAddress); err != nil {
		return nil, err
	}

	err = k.migrateFeeGrants(ctx, originalAddress, maxFeeGrants, newAddress)
	if err != nil {
		return nil, err
	}

	if err := k.MigrateAccountHooks().AfterMigrateFeeGrants(ctx, originalAddress, newAddress); err != nil {
		return nil, err
	}

	err = k.migrateAuthzGrants(ctx, originalAddress, maxAuthzGrants)
	if err != nil {
		return nil, err
	}

	if err := k.MigrateAccountHooks().AfterAll(ctx, originalAddress, newAddress); err != nil {
		return nil, err
	}

	return &types.MsgMigrateAccountResponse{}, nil
}
