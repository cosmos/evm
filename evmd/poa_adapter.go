package evmd

import (
	"context"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"

	"cosmossdk.io/core/address"
	"cosmossdk.io/math"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	poakeeper "github.com/cosmos/cosmos-sdk/enterprise/poa/x/poa/keeper"
	poatypes "github.com/cosmos/cosmos-sdk/enterprise/poa/x/poa/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// POAStakingAdapter wraps the POA keeper to satisfy the various StakingKeeper
// interfaces required by genutil, evidence, EVM, ERC20, and common precompiles.
type POAStakingAdapter struct {
	keeper           *poakeeper.Keeper
	bondDenomFn      func() string
	consAddressCodec address.Codec
	valAddressCodec  address.Codec
}

// NewPOAStakingAdapter creates a new POAStakingAdapter.
func NewPOAStakingAdapter(
	keeper *poakeeper.Keeper,
	bondDenomFn func() string,
	consAddressCodec address.Codec,
	valAddressCodec address.Codec,
) *POAStakingAdapter {
	return &POAStakingAdapter{
		keeper:           keeper,
		bondDenomFn:      bondDenomFn,
		consAddressCodec: consAddressCodec,
		valAddressCodec:  valAddressCodec,
	}
}

// BondDenom returns the configured bond denomination.
func (a *POAStakingAdapter) BondDenom(_ context.Context) (string, error) {
	return a.bondDenomFn(), nil
}

// ValidatorAddressCodec returns the validator address codec.
func (a *POAStakingAdapter) ValidatorAddressCodec() address.Codec {
	return a.valAddressCodec
}

// ConsensusAddressCodec returns the consensus address codec.
func (a *POAStakingAdapter) ConsensusAddressCodec() address.Codec {
	return a.consAddressCodec
}

// GetHistoricalInfo is not supported in POA mode.
func (a *POAStakingAdapter) GetHistoricalInfo(_ context.Context, _ int64) (stakingtypes.HistoricalInfo, error) {
	return stakingtypes.HistoricalInfo{}, stakingtypes.ErrNoHistoricalInfo
}

// GetValidatorByConsAddr converts a POA validator to a staking Validator.
func (a *POAStakingAdapter) GetValidatorByConsAddr(ctx context.Context, consAddr sdk.ConsAddress) (stakingtypes.Validator, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	poaVal, err := a.keeper.GetValidatorByConsAddress(sdkCtx, consAddr)
	if err != nil {
		return stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound
	}
	return poaValidatorToStakingValidator(poaVal), nil
}

// ValidatorByConsAddr satisfies the evidence module's StakingKeeper interface.
func (a *POAStakingAdapter) ValidatorByConsAddr(ctx context.Context, consAddr sdk.ConsAddress) (stakingtypes.ValidatorI, error) {
	v, err := a.GetValidatorByConsAddr(ctx, consAddr)
	if err != nil {
		return nil, err
	}
	return v, nil
}

// GetParams returns default staking params (used by evidence module).
func (a *POAStakingAdapter) GetParams(_ context.Context) (stakingtypes.Params, error) {
	return stakingtypes.DefaultParams(), nil
}

// ApplyAndReturnValidatorSetUpdates satisfies the genutil StakingKeeper interface.
func (a *POAStakingAdapter) ApplyAndReturnValidatorSetUpdates(ctx context.Context) ([]abci.ValidatorUpdate, error) {
	sdkCtx := sdk.UnwrapSDKContext(ctx)
	return a.keeper.ReapValidatorUpdates(sdkCtx), nil
}

// MaxValidators returns the max number of validators (precompile StakingKeeper).
func (a *POAStakingAdapter) MaxValidators(_ context.Context) (uint32, error) {
	return 100, nil
}

// GetDelegatorValidators returns empty since POA has no delegations.
func (a *POAStakingAdapter) GetDelegatorValidators(_ context.Context, _ sdk.AccAddress, _ uint32) (stakingtypes.Validators, error) {
	return stakingtypes.Validators{}, nil
}

// GetRedelegation returns an error since POA has no redelegations.
func (a *POAStakingAdapter) GetRedelegation(_ context.Context, _ sdk.AccAddress, _, _ sdk.ValAddress) (stakingtypes.Redelegation, error) {
	return stakingtypes.Redelegation{}, stakingtypes.ErrNoRedelegation
}

// GetValidator returns an error since POA doesn't use val-address based lookup.
func (a *POAStakingAdapter) GetValidator(_ context.Context, _ sdk.ValAddress) (stakingtypes.Validator, error) {
	return stakingtypes.Validator{}, stakingtypes.ErrNoValidatorFound
}

// poaValidatorToStakingValidator converts a POA validator to a staking Validator
// so the adapters can satisfy interfaces that return stakingtypes.Validator.
func poaValidatorToStakingValidator(v poatypes.Validator) stakingtypes.Validator {
	sv := stakingtypes.Validator{
		ConsensusPubkey: v.PubKey,
		Status:          stakingtypes.Bonded,
		Tokens:          math.NewInt(v.Power),
		DelegatorShares: math.LegacyNewDec(v.Power),
	}
	if v.Metadata != nil {
		sv.OperatorAddress = v.Metadata.OperatorAddress
		sv.Description = stakingtypes.Description{Moniker: v.Metadata.Moniker}
	}
	return sv
}

// ---------------------------------------------------------------------------
// POASlashingAdapter — no-op slashing keeper for the evidence module.
// ---------------------------------------------------------------------------

// POASlashingAdapter satisfies evidencetypes.SlashingKeeper with no-ops.
// In POA mode there is no slashing — double-sign evidence is recorded but
// penalties are not applied.
type POASlashingAdapter struct{}

func (POASlashingAdapter) GetPubkey(_ context.Context, _ cryptotypes.Address) (cryptotypes.PubKey, error) {
	return nil, nil
}

func (POASlashingAdapter) IsTombstoned(_ context.Context, _ sdk.ConsAddress) bool { return false }

func (POASlashingAdapter) HasValidatorSigningInfo(_ context.Context, _ sdk.ConsAddress) bool {
	return false
}

func (POASlashingAdapter) Tombstone(_ context.Context, _ sdk.ConsAddress) error { return nil }

func (POASlashingAdapter) Slash(_ context.Context, _ sdk.ConsAddress, _ math.LegacyDec, _ int64, _ int64) error {
	return nil
}

func (POASlashingAdapter) SlashWithInfractionReason(_ context.Context, _ sdk.ConsAddress, _ math.LegacyDec, _ int64, _ int64, _ stakingtypes.Infraction) error {
	return nil
}

func (POASlashingAdapter) SlashFractionDoubleSign(_ context.Context) (math.LegacyDec, error) {
	return math.LegacyZeroDec(), nil
}

func (POASlashingAdapter) Jail(_ context.Context, _ sdk.ConsAddress) error { return nil }

func (POASlashingAdapter) JailUntil(_ context.Context, _ sdk.ConsAddress, _ time.Time) error {
	return nil
}

// ---------------------------------------------------------------------------
// POAGovDistributionAdapter — no-op distribution keeper for the gov module.
// ---------------------------------------------------------------------------

// POAGovDistributionAdapter satisfies govtypes.DistributionKeeper with a no-op.
type POAGovDistributionAdapter struct{}

func (POAGovDistributionAdapter) FundCommunityPool(_ context.Context, _ sdk.Coins, _ sdk.AccAddress) error {
	return nil
}

