package distribution

import (
	"fmt"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	distributionkeeper "github.com/cosmos/cosmos-sdk/x/distribution/keeper"
)

const (
	// SetWithdrawAddressMethod defines the ABI method name for the distribution
	// SetWithdrawAddress transaction.
	SetWithdrawAddressMethod = "setWithdrawAddress"
	// WithdrawDelegatorRewardMethod defines the ABI method name for the distribution
	// WithdrawDelegatorReward transaction.
	WithdrawDelegatorRewardMethod = "withdrawDelegatorRewards"
	// WithdrawValidatorCommissionMethod defines the ABI method name for the distribution
	// WithdrawValidatorCommission transaction.
	WithdrawValidatorCommissionMethod = "withdrawValidatorCommission"
	// FundCommunityPoolMethod defines the ABI method name for the fundCommunityPool transaction
	FundCommunityPoolMethod = "fundCommunityPool"
	// ClaimRewardsMethod defines the ABI method name for the custom ClaimRewards transaction
	ClaimRewardsMethod = "claimRewards"
	// DepositValidatorRewardsPoolMethod defines the ABI method name for the distribution
	// DepositValidatorRewardsPool transaction
	DepositValidatorRewardsPoolMethod = "depositValidatorRewardsPool"
)

// ClaimRewards claims the rewards accumulated by a delegator from multiple or all validators.
func (p *Precompile) ClaimRewards(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	delegatorAddr, maxRetrieve, err := parseClaimRewardsArgs(args)
	if err != nil {
		return nil, err
	}

	maxVals, err := p.stakingKeeper.MaxValidators(ctx)
	if err != nil {
		return nil, err
	}
	if maxRetrieve > maxVals {
		return nil, fmt.Errorf("maxRetrieve (%d) parameter exceeds the maximum number of validators (%d)", maxRetrieve, maxVals)
	}

	msgSender := contract.Caller()
	if msgSender != delegatorAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorAddr.String())
	}

	res, err := p.stakingKeeper.GetDelegatorValidators(ctx, delegatorAddr.Bytes(), maxRetrieve)
	if err != nil {
		return nil, err
	}
	totalCoins := sdk.Coins{}
	for _, validator := range res.Validators {
		// Convert the validator operator address into an ValAddress
		valAddr, err := sdk.ValAddressFromBech32(validator.OperatorAddress)
		if err != nil {
			return nil, err
		}

		// Withdraw the rewards for each validator address
		coins, err := p.distributionKeeper.WithdrawDelegationRewards(ctx, delegatorAddr.Bytes(), valAddr)
		if err != nil {
			return nil, err
		}

		totalCoins = totalCoins.Add(coins...)
	}

	withdrawerHexAddr, err := p.getWithdrawerHexAddr(ctx, delegatorAddr)
	if err != nil {
		return nil, err
	}

	convertedAmount, err := utils.Uint256FromBigInt(evmtypes.ConvertAmountTo18DecimalsBigInt(totalCoins.AmountOf(evmtypes.GetEVMCoinDenom()).BigInt()))
	if err != nil {
		return nil, err
	}
	// check if converted amount is greater than zero
	if convertedAmount.Cmp(uint256.NewInt(0)) == 1 {
		p.SetBalanceChangeEntries(cmn.NewBalanceChangeEntry(withdrawerHexAddr, convertedAmount, cmn.Add))
	}

	if err := p.EmitClaimRewardsEvent(ctx, stateDB, delegatorAddr, totalCoins); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// SetWithdrawAddress sets the withdrawal address for a delegator (or validator self-delegation).
func (p Precompile) SetWithdrawAddress(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, delegatorHexAddr, err := NewMsgSetWithdrawAddress(args)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorHexAddr.String())
	}

	msgSrv := distributionkeeper.NewMsgServerImpl(p.distributionKeeper)
	if _, err = msgSrv.SetWithdrawAddress(ctx, msg); err != nil {
		return nil, err
	}

	if err = p.EmitSetWithdrawAddressEvent(ctx, stateDB, delegatorHexAddr, msg.WithdrawAddress); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// WithdrawDelegatorReward withdraws the rewards of a delegator from a single validator.
func (p *Precompile) WithdrawDelegatorReward(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, delegatorHexAddr, err := NewMsgWithdrawDelegatorReward(args)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != delegatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), delegatorHexAddr.String())
	}

	msgSrv := distributionkeeper.NewMsgServerImpl(p.distributionKeeper)
	res, err := msgSrv.WithdrawDelegatorReward(ctx, msg)
	if err != nil {
		return nil, err
	}

	// rewards go to the withdrawer address
	withdrawerHexAddr, err := p.getWithdrawerHexAddr(ctx, delegatorHexAddr)
	if err != nil {
		return nil, err
	}

	convertedAmount, err := utils.Uint256FromBigInt(evmtypes.ConvertAmountTo18DecimalsBigInt(res.Amount.AmountOf(evmtypes.GetEVMCoinDenom()).BigInt()))
	if err != nil {
		return nil, err
	}
	// check if converted amount is greater than zero
	if convertedAmount.Cmp(uint256.NewInt(0)) == 1 {
		p.SetBalanceChangeEntries(cmn.NewBalanceChangeEntry(withdrawerHexAddr, convertedAmount, cmn.Add))
	}

	if err = p.EmitWithdrawDelegatorRewardEvent(ctx, stateDB, delegatorHexAddr, msg.ValidatorAddress, res.Amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(cmn.NewCoinsResponse(res.Amount))
}

// WithdrawValidatorCommission withdraws the rewards of a validator.
func (p *Precompile) WithdrawValidatorCommission(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, validatorHexAddr, err := NewMsgWithdrawValidatorCommission(args)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != validatorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), validatorHexAddr.String())
	}

	msgSrv := distributionkeeper.NewMsgServerImpl(p.distributionKeeper)
	res, err := msgSrv.WithdrawValidatorCommission(ctx, msg)
	if err != nil {
		return nil, err
	}

	// commissions go to the withdrawer address
	withdrawerHexAddr, err := p.getWithdrawerHexAddr(ctx, validatorHexAddr)
	if err != nil {
		return nil, err
	}

	convertedAmount, err := utils.Uint256FromBigInt(evmtypes.ConvertAmountTo18DecimalsBigInt(res.Amount.AmountOf(evmtypes.GetEVMCoinDenom()).BigInt()))
	if err != nil {
		return nil, err
	}
	// check if converted amount is greater than zero
	if convertedAmount.Cmp(uint256.NewInt(0)) == 1 {
		p.SetBalanceChangeEntries(cmn.NewBalanceChangeEntry(withdrawerHexAddr, convertedAmount, cmn.Add))
	}

	if err = p.EmitWithdrawValidatorCommissionEvent(ctx, stateDB, msg.ValidatorAddress, res.Amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(cmn.NewCoinsResponse(res.Amount))
}

// FundCommunityPool directly fund the community pool
func (p *Precompile) FundCommunityPool(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, depositorHexAddr, err := NewMsgFundCommunityPool(args)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != depositorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), depositorHexAddr.String())
	}

	msgSrv := distributionkeeper.NewMsgServerImpl(p.distributionKeeper)
	_, err = msgSrv.FundCommunityPool(ctx, msg)
	if err != nil {
		return nil, err
	}

	convertedAmount, err := utils.Uint256FromBigInt(evmtypes.ConvertAmountTo18DecimalsBigInt(msg.Amount.AmountOf(evmtypes.GetEVMCoinDenom()).BigInt()))
	if err != nil {
		return nil, err
	}
	// check if converted amount is greater than zero
	if convertedAmount.Cmp(uint256.NewInt(0)) == 1 {
		p.SetBalanceChangeEntries(cmn.NewBalanceChangeEntry(depositorHexAddr, convertedAmount, cmn.Sub))
	}

	if err = p.EmitFundCommunityPoolEvent(ctx, stateDB, depositorHexAddr, msg.Amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// DepositValidatorRewardsPool deposits rewards into the validator rewards pool
// for a specific validator.
func (p *Precompile) DepositValidatorRewardsPool(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
	msg, depositorHexAddr, err := NewMsgDepositValidatorRewardsPool(args)
	if err != nil {
		return nil, err
	}

	msgSender := contract.Caller()
	if msgSender != depositorHexAddr {
		return nil, fmt.Errorf(cmn.ErrRequesterIsNotMsgSender, msgSender.String(), depositorHexAddr.String())
	}

	msgSrv := distributionkeeper.NewMsgServerImpl(p.distributionKeeper)
	_, err = msgSrv.DepositValidatorRewardsPool(ctx, msg)
	if err != nil {
		return nil, err
	}
	if found, evmCoinAmount := msg.Amount.Find(evmtypes.GetEVMCoinDenom()); found {
		convertedAmount, err := utils.Uint256FromBigInt(evmtypes.ConvertAmountTo18DecimalsBigInt(evmCoinAmount.Amount.BigInt()))
		if err != nil {
			return nil, err
		}
		// check if converted amount is greater than zero
		if convertedAmount.Cmp(uint256.NewInt(0)) == 1 {
			p.SetBalanceChangeEntries(cmn.NewBalanceChangeEntry(depositorHexAddr, convertedAmount, cmn.Sub))
		}
	}

	if err = p.EmitDepositValidatorRewardsPoolEvent(ctx, stateDB, depositorHexAddr, msg.ValidatorAddress, msg.Amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

// getWithdrawerHexAddr is a helper function to get the hex address
// of the withdrawer for the specified account address
func (p Precompile) getWithdrawerHexAddr(ctx sdk.Context, delegatorAddr common.Address) (common.Address, error) {
	withdrawerAccAddr, err := p.distributionKeeper.GetDelegatorWithdrawAddr(ctx, delegatorAddr.Bytes())
	if err != nil {
		return common.Address{}, err
	}

	return common.BytesToAddress(withdrawerAccAddr), nil
}
