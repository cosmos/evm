package staking

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"

	"cosmossdk.io/core/address"
	"cosmossdk.io/math"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

const (
	// DoNotModifyCommissionRate constant used in flags to indicate that commission rate field should not be updated
	DoNotModifyCommissionRate = -1
	// DoNotModifyMinSelfDelegation constant used in flags to indicate that min self delegation field should not be updated
	DoNotModifyMinSelfDelegation = -1
)

// EventCreateValidator defines the event data for the staking CreateValidator transaction.
type EventCreateValidator struct {
	ValidatorAddress common.Address
	Value            *big.Int
}

// EventEditValidator defines the event data for the staking EditValidator transaction.
type EventEditValidator struct {
	ValidatorAddress  common.Address
	CommissionRate    *big.Int
	MinSelfDelegation *big.Int
}

// EventDelegate defines the event data for the staking Delegate transaction.
type EventDelegate struct {
	DelegatorAddress common.Address
	ValidatorAddress common.Address
	Amount           *big.Int
	NewShares        *big.Int
}

// EventUnbond defines the event data for the staking Undelegate transaction.
type EventUnbond struct {
	DelegatorAddress common.Address
	ValidatorAddress common.Address
	Amount           *big.Int
	CompletionTime   *big.Int
}

// EventRedelegate defines the event data for the staking Redelegate transaction.
type EventRedelegate struct {
	DelegatorAddress    common.Address
	ValidatorSrcAddress common.Address
	ValidatorDstAddress common.Address
	Amount              *big.Int
	CompletionTime      *big.Int
}

// EventCancelUnbonding defines the event data for the staking CancelUnbond transaction.
type EventCancelUnbonding struct {
	DelegatorAddress common.Address
	ValidatorAddress common.Address
	Amount           *big.Int
	CreationHeight   *big.Int
}

func NewDescriptionFromResponse(d stakingtypes.Description) Description {
	return Description{
		Moniker:         d.Moniker,
		Identity:        d.Identity,
		Website:         d.Website,
		SecurityContact: d.SecurityContact,
		Details:         d.Details,
	}
}

// NewMsgCreateValidator creates a new MsgCreateValidator instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgCreateValidator(args CreateValidatorCall, denom string, addrCdc address.Codec) (*stakingtypes.MsgCreateValidator, common.Address, error) {
	validatorAddress := args.ValidatorAddress
	if validatorAddress == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidValidator, args.ValidatorAddress)
	}

	pubkeyBytes, err := base64.StdEncoding.DecodeString(args.Pubkey)
	if err != nil {
		return nil, common.Address{}, err
	}

	// more details see https://github.com/cosmos/cosmos-sdk/pull/18506
	if len(pubkeyBytes) != ed25519.PubKeySize {
		return nil, common.Address{}, fmt.Errorf("consensus pubkey len is invalid, got: %d, expected: %d", len(pubkeyBytes), ed25519.PubKeySize)
	}

	var ed25519pk cryptotypes.PubKey = &ed25519.PubKey{Key: pubkeyBytes}
	pubkey, err := codectypes.NewAnyWithValue(ed25519pk)
	if err != nil {
		return nil, common.Address{}, err
	}

	delegatorAddr, err := addrCdc.BytesToString(validatorAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &stakingtypes.MsgCreateValidator{
		Description: stakingtypes.Description{
			Moniker:         args.Description.Moniker,
			Identity:        args.Description.Identity,
			Website:         args.Description.Website,
			SecurityContact: args.Description.SecurityContact,
			Details:         args.Description.Details,
		},
		Commission: stakingtypes.CommissionRates{
			Rate:          math.LegacyNewDecFromBigIntWithPrec(args.CommissionRates.Rate, math.LegacyPrecision),
			MaxRate:       math.LegacyNewDecFromBigIntWithPrec(args.CommissionRates.MaxRate, math.LegacyPrecision),
			MaxChangeRate: math.LegacyNewDecFromBigIntWithPrec(args.CommissionRates.MaxChangeRate, math.LegacyPrecision),
		},
		MinSelfDelegation: math.NewIntFromBigInt(args.MinSelfDelegation),
		DelegatorAddress:  delegatorAddr,
		ValidatorAddress:  sdk.ValAddress(validatorAddress.Bytes()).String(),
		Pubkey:            pubkey,
		Value:             sdk.Coin{Denom: denom, Amount: math.NewIntFromBigInt(args.Value)},
	}

	return msg, validatorAddress, nil
}

// NewMsgEditValidator creates a new MsgEditValidator instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgEditValidator(args EditValidatorCall) (*stakingtypes.MsgEditValidator, common.Address, error) {
	validatorHexAddr := args.ValidatorAddress
	if validatorHexAddr == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidValidator, args.ValidatorAddress)
	}

	// The default value of a variable declared using a pointer is nil, indicating that the user does not want to modify its value.
	// If the value passed in by the user is not DoNotModifyCommissionRate, which is -1, it means that the user wants to modify its value.
	var commissionRate *math.LegacyDec
	if args.CommissionRate.Cmp(big.NewInt(DoNotModifyCommissionRate)) != 0 {
		cr := math.LegacyNewDecFromBigIntWithPrec(args.CommissionRate, math.LegacyPrecision)
		commissionRate = &cr
	}

	var minSelfDelegation *math.Int
	if args.MinSelfDelegation.Cmp(big.NewInt(DoNotModifyMinSelfDelegation)) != 0 {
		msd := math.NewIntFromBigInt(args.MinSelfDelegation)
		minSelfDelegation = &msd
	}

	msg := &stakingtypes.MsgEditValidator{
		Description: stakingtypes.Description{
			Moniker:         args.Description.Moniker,
			Identity:        args.Description.Identity,
			Website:         args.Description.Website,
			SecurityContact: args.Description.SecurityContact,
			Details:         args.Description.Details,
		},
		ValidatorAddress:  sdk.ValAddress(validatorHexAddr.Bytes()).String(),
		CommissionRate:    commissionRate,
		MinSelfDelegation: minSelfDelegation,
	}

	return msg, validatorHexAddr, nil
}

// NewMsgDelegate creates a new MsgDelegate instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgDelegate(args DelegateCall, denom string, addrCdc address.Codec) (*stakingtypes.MsgDelegate, common.Address, error) {
	if args.DelegatorAddress == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDelegator, args.DelegatorAddress)
	}

	delegatorAddrStr, err := addrCdc.BytesToString(args.DelegatorAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &stakingtypes.MsgDelegate{
		DelegatorAddress: delegatorAddrStr,
		ValidatorAddress: args.ValidatorAddress,
		Amount: sdk.Coin{
			Denom:  denom,
			Amount: math.NewIntFromBigInt(args.Amount),
		},
	}

	return msg, args.DelegatorAddress, nil
}

// NewMsgUndelegate creates a new MsgUndelegate instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgUndelegate(args UndelegateCall, denom string, addrCdc address.Codec) (*stakingtypes.MsgUndelegate, common.Address, error) {
	if args.DelegatorAddress == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDelegator, args.DelegatorAddress)
	}

	delegatorAddrStr, err := addrCdc.BytesToString(args.DelegatorAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &stakingtypes.MsgUndelegate{
		DelegatorAddress: delegatorAddrStr,
		ValidatorAddress: args.ValidatorAddress,
		Amount: sdk.Coin{
			Denom:  denom,
			Amount: math.NewIntFromBigInt(args.Amount),
		},
	}

	return msg, args.DelegatorAddress, nil
}

// NewMsgRedelegate creates a new MsgRedelegate instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgRedelegate(args RedelegateCall, denom string, addrCdc address.Codec) (*stakingtypes.MsgBeginRedelegate, common.Address, error) {
	if args.DelegatorAddress == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDelegator, args.DelegatorAddress)
	}

	delegatorAddrStr, err := addrCdc.BytesToString(args.DelegatorAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &stakingtypes.MsgBeginRedelegate{
		DelegatorAddress:    delegatorAddrStr,
		ValidatorSrcAddress: args.ValidatorSrcAddress,
		ValidatorDstAddress: args.ValidatorDstAddress,
		Amount: sdk.Coin{
			Denom:  denom,
			Amount: math.NewIntFromBigInt(args.Amount),
		},
	}

	return msg, args.DelegatorAddress, nil
}

// NewMsgCancelUnbondingDelegation creates a new MsgCancelUnbondingDelegation instance and does sanity checks
// on the given arguments before populating the message.
func NewMsgCancelUnbondingDelegation(args CancelUnbondingDelegationCall, denom string, addrCdc address.Codec) (*stakingtypes.MsgCancelUnbondingDelegation, common.Address, error) {
	if args.DelegatorAddress == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDelegator, args.DelegatorAddress)
	}

	delegatorAddrStr, err := addrCdc.BytesToString(args.DelegatorAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &stakingtypes.MsgCancelUnbondingDelegation{
		DelegatorAddress: delegatorAddrStr,
		ValidatorAddress: args.ValidatorAddress,
		Amount: sdk.Coin{
			Denom:  denom,
			Amount: math.NewIntFromBigInt(args.Amount),
		},
		CreationHeight: args.CreationHeight.Int64(),
	}

	return msg, args.DelegatorAddress, nil
}

// NewDelegationRequest creates a new QueryDelegationRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewDelegationRequest(args DelegationCall, addrCdc address.Codec) (*stakingtypes.QueryDelegationRequest, error) {
	if args.DelegatorAddress == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args.DelegatorAddress)
	}

	delegatorAddrStr, err := addrCdc.BytesToString(args.DelegatorAddress.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	return &stakingtypes.QueryDelegationRequest{
		DelegatorAddr: delegatorAddrStr,
		ValidatorAddr: args.ValidatorAddress,
	}, nil
}

// NewValidatorRequest create a new QueryValidatorRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewValidatorRequest(args ValidatorCall) (*stakingtypes.QueryValidatorRequest, error) {
	if args.ValidatorAddress == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidValidator, args.ValidatorAddress)
	}

	validatorAddress := sdk.ValAddress(args.ValidatorAddress.Bytes()).String()

	return &stakingtypes.QueryValidatorRequest{ValidatorAddr: validatorAddress}, nil
}

// NewValidatorsRequest create a new QueryValidatorsRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewValidatorsRequest(args ValidatorsCall) (*stakingtypes.QueryValidatorsRequest, error) {
	if bytes.Equal(args.PageRequest.Key, []byte{0}) {
		args.PageRequest.Key = nil
	}

	return &stakingtypes.QueryValidatorsRequest{
		Status:     args.Status,
		Pagination: args.PageRequest.ToPageRequest(),
	}, nil
}

// NewRedelegationRequest create a new QueryRedelegationRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewRedelegationRequest(args RedelegateCall) (*RedelegationRequest, error) {
	if args.DelegatorAddress == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args.DelegatorAddress)
	}

	validatorSrcAddr, err := sdk.ValAddressFromBech32(args.ValidatorSrcAddress)
	if err != nil {
		return nil, err
	}

	validatorDstAddr, err := sdk.ValAddressFromBech32(args.ValidatorDstAddress)
	if err != nil {
		return nil, err
	}

	return &RedelegationRequest{
		DelegatorAddress:    args.DelegatorAddress.Bytes(), // bech32 formatted
		ValidatorSrcAddress: validatorSrcAddr,
		ValidatorDstAddress: validatorDstAddr,
	}, nil
}

// NewRedelegationsRequest create a new QueryRedelegationsRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewRedelegationsRequest(args RedelegationsCall, addrCdc address.Codec) (*stakingtypes.QueryRedelegationsRequest, error) {
	// delAddr, srcValAddr & dstValAddr
	// can be empty strings. The query will return the
	// corresponding redelegations according to the addresses specified
	// however, cannot pass all as empty strings, need to provide at least
	// the delegator address or the source validator address
	var (
		// delegatorAddr is the string representation of the delegator address
		delegatorAddr = ""
		// emptyAddr is an empty address
		emptyAddr = common.Address{}.Hex()
	)
	if args.DelegatorAddress.Hex() != emptyAddr {
		var err error
		delegatorAddr, err = addrCdc.BytesToString(args.DelegatorAddress.Bytes())
		if err != nil {
			return nil, fmt.Errorf("failed to decode delegator address: %w", err)
		}
	}

	if delegatorAddr == "" && args.SrcValidatorAddress == "" && args.DstValidatorAddress == "" ||
		delegatorAddr == "" && args.SrcValidatorAddress == "" && args.DstValidatorAddress != "" {
		return nil, errors.New("invalid query. Need to specify at least a source validator address or delegator address")
	}

	return &stakingtypes.QueryRedelegationsRequest{
		DelegatorAddr:    delegatorAddr, // bech32 formatted
		SrcValidatorAddr: args.SrcValidatorAddress,
		DstValidatorAddr: args.DstValidatorAddress,
		Pagination:       args.PageRequest.ToPageRequest(),
	}, nil
}

// RedelegationRequest is a struct that contains the information to pass into a redelegation query.
type RedelegationRequest struct {
	DelegatorAddress    sdk.AccAddress
	ValidatorSrcAddress sdk.ValAddress
	ValidatorDstAddress sdk.ValAddress
}

// RedelegationsRequest is a struct that contains the information to pass into a redelegations query.
type RedelegationsRequest struct {
	DelegatorAddress sdk.AccAddress
	MaxRetrieve      int64
}

// UnbondingDelegationResponse is a struct that contains the information about an unbonding delegation.
type UnbondingDelegationResponse struct {
	DelegatorAddress string
	ValidatorAddress string
	Entries          []UnbondingDelegationEntry
}

// FromResponse populates the DelegationReturn from a QueryDelegationResponse.
func (do *UnbondingDelegationReturn) FromResponse(res *stakingtypes.QueryUnbondingDelegationResponse) *UnbondingDelegationReturn {
	do.UnbondingDelegation.Entries = make([]UnbondingDelegationEntry, len(res.Unbond.Entries))
	do.UnbondingDelegation.ValidatorAddress = res.Unbond.ValidatorAddress
	do.UnbondingDelegation.DelegatorAddress = res.Unbond.DelegatorAddress
	for i, entry := range res.Unbond.Entries {
		do.UnbondingDelegation.Entries[i] = UnbondingDelegationEntry{
			UnbondingId:             entry.UnbondingId,
			UnbondingOnHoldRefCount: entry.UnbondingOnHoldRefCount,
			CreationHeight:          entry.CreationHeight,
			CompletionTime:          entry.CompletionTime.UTC().Unix(),
			InitialBalance:          entry.InitialBalance.BigInt(),
			Balance:                 entry.Balance.BigInt(),
		}
	}
	return do
}

// FromResponse populates the DelegationReturn from a QueryDelegationResponse.
func (do *DelegationReturn) FromResponse(res *stakingtypes.QueryDelegationResponse) *DelegationReturn {
	do.Shares = res.DelegationResponse.Delegation.Shares.BigInt()
	do.Balance = cmn.Coin{
		Denom:  res.DelegationResponse.Balance.Denom,
		Amount: res.DelegationResponse.Balance.Amount.BigInt(),
	}
	return do
}

func DefaultValidator() Validator {
	return Validator{
		Tokens:            big.NewInt(0),
		DelegatorShares:   big.NewInt(0),
		Commission:        big.NewInt(0),
		MinSelfDelegation: big.NewInt(0),
	}
}

func NewValidatorFromResponse(v stakingtypes.Validator) Validator {
	operatorAddress, err := sdk.ValAddressFromBech32(v.OperatorAddress)
	if err != nil {
		return DefaultValidator()
	}

	return Validator{
		OperatorAddress:   common.BytesToAddress(operatorAddress.Bytes()).String(),
		ConsensusPubkey:   FormatConsensusPubkey(v.ConsensusPubkey),
		Jailed:            v.Jailed,
		Status:            uint8(stakingtypes.BondStatus_value[v.Status.String()]), //#nosec G115 // enum will always be convertible to uint8
		Tokens:            v.Tokens.BigInt(),
		DelegatorShares:   v.DelegatorShares.BigInt(), // TODO: Decimal
		Description:       NewDescriptionFromResponse(v.Description),
		UnbondingHeight:   v.UnbondingHeight,
		UnbondingTime:     v.UnbondingTime.UTC().Unix(),
		Commission:        v.Commission.Rate.BigInt(),
		MinSelfDelegation: v.MinSelfDelegation.BigInt(),
	}
}

// FromResponse populates the ValidatorsReturn from a QueryValidatorsResponse.
func (vo *ValidatorsReturn) FromResponse(res *stakingtypes.QueryValidatorsResponse) *ValidatorsReturn {
	vo.Validators = make([]Validator, len(res.Validators))
	for i, v := range res.Validators {
		vo.Validators[i] = NewValidatorFromResponse(v)
	}

	vo.PageResponse = cmn.FromPageResponse(res.Pagination)
	return vo
}

// RedelegationValues is a struct to represent the key information from
// a redelegation response.
type RedelegationValues struct {
	DelegatorAddress    string
	ValidatorSrcAddress string
	ValidatorDstAddress string
	Entries             []RedelegationEntry
}

// FromResponse populates the RedelegationReturn from a QueryRedelegationsResponse.
func (ro *RedelegationReturn) FromResponse(res stakingtypes.Redelegation) *RedelegationReturn {
	ro.Redelegation.Entries = make([]RedelegationEntry, len(res.Entries))
	ro.Redelegation.DelegatorAddress = res.DelegatorAddress
	ro.Redelegation.ValidatorSrcAddress = res.ValidatorSrcAddress
	ro.Redelegation.ValidatorDstAddress = res.ValidatorDstAddress
	for i, entry := range res.Entries {
		ro.Redelegation.Entries[i] = RedelegationEntry{
			CreationHeight: entry.CreationHeight,
			CompletionTime: entry.CompletionTime.UTC().Unix(),
			InitialBalance: entry.InitialBalance.BigInt(),
			SharesDst:      entry.SharesDst.BigInt(),
		}
	}
	return ro
}

// FromResponse populates the RedelgationsReturn from a QueryRedelegationsResponse.
func (ro *RedelegationsReturn) FromResponse(res *stakingtypes.QueryRedelegationsResponse) *RedelegationsReturn {
	ro.Response = make([]RedelegationResponse, len(res.RedelegationResponses))
	for i, resp := range res.RedelegationResponses {
		// for each RedelegationResponse
		// there's a RedelegationEntryResponse array ('Entries' field)
		entries := make([]RedelegationEntryResponse, len(resp.Entries))
		for j, e := range resp.Entries {
			entries[j] = RedelegationEntryResponse{
				RedelegationEntry: RedelegationEntry{
					CreationHeight: e.RedelegationEntry.CreationHeight,
					CompletionTime: e.RedelegationEntry.CompletionTime.Unix(),
					InitialBalance: e.RedelegationEntry.InitialBalance.BigInt(),
					SharesDst:      e.RedelegationEntry.SharesDst.BigInt(),
				},
				Balance: e.Balance.BigInt(),
			}
		}

		// the Redelegation field has also an 'Entries' field of type RedelegationEntry
		redelEntries := make([]RedelegationEntry, len(resp.Redelegation.Entries))
		for j, e := range resp.Redelegation.Entries {
			redelEntries[j] = RedelegationEntry{
				CreationHeight: e.CreationHeight,
				CompletionTime: e.CompletionTime.Unix(),
				InitialBalance: e.InitialBalance.BigInt(),
				SharesDst:      e.SharesDst.BigInt(),
			}
		}

		ro.Response[i] = RedelegationResponse{
			Entries: entries,
			Redelegation: Redelegation{
				DelegatorAddress:    resp.Redelegation.DelegatorAddress,
				ValidatorSrcAddress: resp.Redelegation.ValidatorSrcAddress,
				ValidatorDstAddress: resp.Redelegation.ValidatorDstAddress,
				Entries:             redelEntries,
			},
		}
	}

	ro.PageResponse = cmn.FromPageResponse(res.Pagination)
	return ro
}

// NewUnbondingDelegationRequest creates a new QueryUnbondingDelegationRequest instance and does sanity checks
// on the given arguments before populating the request.
func NewUnbondingDelegationRequest(args UnbondingDelegationCall, addrCdc address.Codec) (*stakingtypes.QueryUnbondingDelegationRequest, error) {
	if args.DelegatorAddress == (common.Address{}) {
		return nil, fmt.Errorf(cmn.ErrInvalidDelegator, args.DelegatorAddress)
	}

	delegatorAddrStr, err := addrCdc.BytesToString(args.DelegatorAddress.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	return &stakingtypes.QueryUnbondingDelegationRequest{
		DelegatorAddr: delegatorAddrStr,
		ValidatorAddr: args.ValidatorAddress,
	}, nil
}

// checkDelegationUndelegationArgs checks the arguments for the delegation and undelegation functions.
func checkDelegationUndelegationArgs(args DelegateCall) (common.Address, string, *big.Int, error) {
	if args.DelegatorAddress == (common.Address{}) {
		return common.Address{}, "", nil, fmt.Errorf(cmn.ErrInvalidDelegator, args.DelegatorAddress)
	}

	return args.DelegatorAddress, args.ValidatorAddress, args.Amount, nil
}

// FormatConsensusPubkey format ConsensusPubkey into a base64 string
func FormatConsensusPubkey(consensusPubkey *codectypes.Any) string {
	ed25519pk, ok := consensusPubkey.GetCachedValue().(cryptotypes.PubKey)
	if ok {
		return base64.StdEncoding.EncodeToString(ed25519pk.Bytes())
	}
	return consensusPubkey.String()
}
