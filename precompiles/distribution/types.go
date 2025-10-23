package distribution

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/utils"

	"cosmossdk.io/core/address"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
)

// EventSetWithdrawAddress defines the event data for the SetWithdrawAddress transaction.
type EventSetWithdrawAddress struct {
	Caller            common.Address
	WithdrawerAddress string
}

// EventWithdrawDelegatorReward defines the event data for the WithdrawDelegatorReward transaction.
type EventWithdrawDelegatorReward struct {
	DelegatorAddress common.Address
	ValidatorAddress common.Address
	Amount           *big.Int
}

// EventWithdrawValidatorRewards defines the event data for the WithdrawValidatorRewards transaction.
type EventWithdrawValidatorRewards struct {
	ValidatorAddress common.Hash
	Commission       *big.Int
}

// EventClaimRewards defines the event data for the ClaimRewards transaction.
type EventClaimRewards struct {
	DelegatorAddress common.Address
	Amount           *big.Int
}

// EventFundCommunityPool defines the event data for the FundCommunityPool transaction.
type EventFundCommunityPool struct {
	Depositor common.Address
	Denom     string
	Amount    *big.Int
}

// EventDepositValidatorRewardsPool defines the event data for the DepositValidatorRewardsPool transaction.
type EventDepositValidatorRewardsPool struct {
	Depositor        common.Address
	ValidatorAddress common.Address
	Denom            string
	Amount           *big.Int
}

// parseClaimRewardsArgs parses the arguments for the ClaimRewards method.
func parseClaimRewardsArgs(args []interface{}) (common.Address, uint32, error) {
	if len(args) != 2 {
		return common.Address{}, 0, fmt.Errorf(cmn.ErrInvalidNumberOfArgs, 2, len(args))
	}

	delegatorAddress, ok := args[0].(common.Address)
	if !ok || delegatorAddress == (common.Address{}) {
		return common.Address{}, 0, fmt.Errorf(cmn.ErrInvalidDelegator, args[0])
	}

	maxRetrieve, ok := args[1].(uint32)
	if !ok {
		return common.Address{}, 0, fmt.Errorf(cmn.ErrInvalidType, "maxRetrieve", uint32(0), args[1])
	}

	return delegatorAddress, maxRetrieve, nil
}

// NewMsgSetWithdrawAddress creates a new MsgSetWithdrawAddress instance.
func NewMsgSetWithdrawAddress(args SetWithdrawAddressCall, addrCdc address.Codec) (*distributiontypes.MsgSetWithdrawAddress, common.Address, error) {
	delegatorAddress := args.DelegatorAddress
	withdrawerAddress := args.WithdrawerAddress

	// If the withdrawer address is a hex address, convert it to a bech32 address.
	if common.IsHexAddress(withdrawerAddress) {
		var err error
		withdrawerAddress, err = sdk.Bech32ifyAddressBytes(sdk.GetConfig().GetBech32AccountAddrPrefix(), common.HexToAddress(withdrawerAddress).Bytes())
		if err != nil {
			return nil, common.Address{}, err
		}
	}

	delAddr, err := addrCdc.BytesToString(delegatorAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &distributiontypes.MsgSetWithdrawAddress{
		DelegatorAddress: delAddr,
		WithdrawAddress:  withdrawerAddress,
	}

	return msg, delegatorAddress, nil
}

// NewMsgWithdrawDelegatorReward creates a new MsgWithdrawDelegatorReward instance.
func NewMsgWithdrawDelegatorReward(args WithdrawDelegatorRewardsCall, addrCdc address.Codec) (*distributiontypes.MsgWithdrawDelegatorReward, common.Address, error) {
	delegatorAddress := args.DelegatorAddress
	if delegatorAddress == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidDelegator, args.DelegatorAddress)
	}

	validatorAddress := args.ValidatorAddress

	delAddr, err := addrCdc.BytesToString(delegatorAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	msg := &distributiontypes.MsgWithdrawDelegatorReward{
		DelegatorAddress: delAddr,
		ValidatorAddress: validatorAddress,
	}

	return msg, delegatorAddress, nil
}

// NewMsgWithdrawValidatorCommission creates a new MsgWithdrawValidatorCommission message.
func NewMsgWithdrawValidatorCommission(args WithdrawValidatorCommissionCall) (*distributiontypes.MsgWithdrawValidatorCommission, common.Address, error) {
	validatorAddress := args.ValidatorAddress

	msg := &distributiontypes.MsgWithdrawValidatorCommission{
		ValidatorAddress: validatorAddress,
	}

	validatorHexAddr, err := utils.HexAddressFromBech32String(msg.ValidatorAddress)
	if err != nil {
		return nil, common.Address{}, err
	}

	return msg, validatorHexAddr, nil
}

// NewMsgFundCommunityPool creates a new NewMsgFundCommunityPool message.
func NewMsgFundCommunityPool(args FundCommunityPoolCall, addrCdc address.Codec) (*distributiontypes.MsgFundCommunityPool, common.Address, error) {
	emptyAddr := common.Address{}
	depositorAddress := args.Depositor

	amt, err := cmn.NewSdkCoinsFromCoins(args.Amount)
	if err != nil {
		return nil, emptyAddr, fmt.Errorf(ErrInvalidAmount, "amount arg")
	}

	depAddr, err := addrCdc.BytesToString(depositorAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode depositor address: %w", err)
	}
	msg := &distributiontypes.MsgFundCommunityPool{
		Depositor: depAddr,
		Amount:    amt,
	}

	return msg, depositorAddress, nil
}

// NewMsgDepositValidatorRewardsPool creates a new MsgDepositValidatorRewardsPool message.
func NewMsgDepositValidatorRewardsPool(args DepositValidatorRewardsPoolCall, addrCdc address.Codec) (*distributiontypes.MsgDepositValidatorRewardsPool, common.Address, error) {
	depositorAddress := args.Depositor
	if depositorAddress == (common.Address{}) {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidHexAddress, args.Depositor)
	}

	validatorAddress := args.ValidatorAddress

	coins, err := cmn.ToCoins(args.Amount)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidAmount, args.Amount)
	}

	amount, err := cmn.NewSdkCoinsFromCoins(coins)
	if err != nil {
		return nil, common.Address{}, fmt.Errorf(cmn.ErrInvalidAmount, err.Error())
	}

	depAddr, err := addrCdc.BytesToString(depositorAddress.Bytes())
	if err != nil {
		return nil, common.Address{}, fmt.Errorf("failed to decode depositor address: %w", err)
	}

	msg := &distributiontypes.MsgDepositValidatorRewardsPool{
		Depositor:        depAddr,
		ValidatorAddress: validatorAddress,
		Amount:           amount,
	}

	return msg, depositorAddress, nil
}

// NewValidatorDistributionInfoRequest creates a new QueryValidatorDistributionInfoRequest  instance and does sanity
// checks on the provided arguments.
func NewValidatorDistributionInfoRequest(args ValidatorDistributionInfoCall) (*distributiontypes.QueryValidatorDistributionInfoRequest, error) {
	return &distributiontypes.QueryValidatorDistributionInfoRequest{
		ValidatorAddress: args.ValidatorAddress,
	}, nil
}

// NewValidatorOutstandingRewardsRequest creates a new QueryValidatorOutstandingRewardsRequest  instance and does sanity
// checks on the provided arguments.
func NewValidatorOutstandingRewardsRequest(args ValidatorOutstandingRewardsCall) (*distributiontypes.QueryValidatorOutstandingRewardsRequest, error) {
	return &distributiontypes.QueryValidatorOutstandingRewardsRequest{
		ValidatorAddress: args.ValidatorAddress,
	}, nil
}

// NewValidatorCommissionRequest creates a new QueryValidatorCommissionRequest  instance and does sanity
// checks on the provided arguments.
func NewValidatorCommissionRequest(args ValidatorCommissionCall) (*distributiontypes.QueryValidatorCommissionRequest, error) {
	validatorAddress := args.ValidatorAddress

	return &distributiontypes.QueryValidatorCommissionRequest{
		ValidatorAddress: validatorAddress,
	}, nil
}

// NewValidatorSlashesRequest creates a new QueryValidatorSlashesRequest  instance and does sanity
// checks on the provided arguments.
func NewValidatorSlashesRequest(args ValidatorSlashesCall) (*distributiontypes.QueryValidatorSlashesRequest, error) {
	return &distributiontypes.QueryValidatorSlashesRequest{
		ValidatorAddress: args.ValidatorAddress,
		StartingHeight:   args.StartingHeight,
		EndingHeight:     args.EndingHeight,
		Pagination:       args.PageRequest.ToPageRequest(),
	}, nil
}

// NewDelegationRewardsRequest creates a new QueryDelegationRewardsRequest  instance and does sanity
// checks on the provided arguments.
func NewDelegationRewardsRequest(args DelegationRewardsCall, addrCdc address.Codec) (*distributiontypes.QueryDelegationRewardsRequest, error) {
	delegatorAddress := args.DelegatorAddress
	validatorAddress := args.ValidatorAddress

	delAddr, err := addrCdc.BytesToString(delegatorAddress.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	return &distributiontypes.QueryDelegationRewardsRequest{
		DelegatorAddress: delAddr,
		ValidatorAddress: validatorAddress,
	}, nil
}

// NewDelegationTotalRewardsRequest creates a new QueryDelegationTotalRewardsRequest  instance and does sanity
// checks on the provided arguments.
func NewDelegationTotalRewardsRequest(args DelegationTotalRewardsCall, addrCdc address.Codec) (*distributiontypes.QueryDelegationTotalRewardsRequest, error) {
	delegatorAddress := args.DelegatorAddress
	delAddr, err := addrCdc.BytesToString(delegatorAddress.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	return &distributiontypes.QueryDelegationTotalRewardsRequest{
		DelegatorAddress: delAddr,
	}, nil
}

// NewDelegatorValidatorsRequest creates a new QueryDelegatorValidatorsRequest  instance and does sanity
// checks on the provided arguments.
func NewDelegatorValidatorsRequest(args DelegatorValidatorsCall, addrCdc address.Codec) (*distributiontypes.QueryDelegatorValidatorsRequest, error) {
	delegatorAddress := args.DelegatorAddress
	delAddr, err := addrCdc.BytesToString(delegatorAddress.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	return &distributiontypes.QueryDelegatorValidatorsRequest{
		DelegatorAddress: delAddr,
	}, nil
}

// NewDelegatorWithdrawAddressRequest creates a new QueryDelegatorWithdrawAddressRequest  instance and does sanity
// checks on the provided arguments.
func NewDelegatorWithdrawAddressRequest(args DelegatorWithdrawAddressCall, addrCdc address.Codec) (*distributiontypes.QueryDelegatorWithdrawAddressRequest, error) {
	delegatorAddress := args.DelegatorAddress
	delAddr, err := addrCdc.BytesToString(delegatorAddress.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed to decode delegator address: %w", err)
	}
	return &distributiontypes.QueryDelegatorWithdrawAddressRequest{
		DelegatorAddress: delAddr,
	}, nil
}

// NewCommunityPoolRequest creates a new QueryCommunityPoolRequest instance and does sanity
// checks on the provided arguments.
func NewCommunityPoolRequest() (*distributiontypes.QueryCommunityPoolRequest, error) {
	return &distributiontypes.QueryCommunityPoolRequest{}, nil
}

// FromResponse converts a response to a ValidatorDistributionInfo.
func (o *ValidatorDistributionInfoReturn) FromResponse(res *distributiontypes.QueryValidatorDistributionInfoResponse) *ValidatorDistributionInfoReturn {
	return &ValidatorDistributionInfoReturn{
		DistributionInfo: ValidatorDistributionInfo{
			OperatorAddress: res.OperatorAddress,
			SelfBondRewards: cmn.NewDecCoinsResponse(res.SelfBondRewards),
			Commission:      cmn.NewDecCoinsResponse(res.Commission),
		},
	}
}

// FromResponse populates the ValidatorSlashesReturn from a QueryValidatorSlashesResponse.
func (vs *ValidatorSlashesReturn) FromResponse(res *distributiontypes.QueryValidatorSlashesResponse) *ValidatorSlashesReturn {
	vs.Slashes = make([]ValidatorSlashEvent, len(res.Slashes))
	for i, s := range res.Slashes {
		vs.Slashes[i] = ValidatorSlashEvent{
			ValidatorPeriod: s.ValidatorPeriod,
			Fraction: cmn.Dec{
				Value:     s.Fraction.BigInt(),
				Precision: math.LegacyPrecision,
			},
		}
	}

	if res.Pagination != nil {
		vs.PageResponse.Total = res.Pagination.Total
		vs.PageResponse.NextKey = res.Pagination.NextKey
	}

	return vs
}

// FromResponse populates the DelegationTotalRewardsReturn from a QueryDelegationTotalRewardsResponse.
func (dtr *DelegationTotalRewardsReturn) FromResponse(res *distributiontypes.QueryDelegationTotalRewardsResponse) *DelegationTotalRewardsReturn {
	dtr.Rewards = make([]DelegationDelegatorReward, len(res.Rewards))
	for i, r := range res.Rewards {
		dtr.Rewards[i] = DelegationDelegatorReward{
			ValidatorAddress: r.ValidatorAddress,
			Reward:           cmn.NewDecCoinsResponse(r.Reward),
		}
	}
	dtr.Total = cmn.NewDecCoinsResponse(res.Total)
	return dtr
}

// FromResponse populates the CommunityPoolReturn from a QueryCommunityPoolResponse.
func (cp *CommunityPoolReturn) FromResponse(res *distributiontypes.QueryCommunityPoolResponse) *CommunityPoolReturn {
	cp.Coins = cmn.NewDecCoinsResponse(res.Pool)
	return cp
}
