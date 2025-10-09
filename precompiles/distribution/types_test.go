package distribution

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	evmaddress "github.com/cosmos/evm/encoding/address"
	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

const validatorAddr = "cosmosvaloper1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5a3kaax"

func TestNewMsgSetWithdrawAddress(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	withdrawerBech32 := "cosmos1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5lzv7xu"
	withdrawerHex := "0xABCDEF1234567890123456789012345678901234"

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	expectedWithdrawerFromHex, err := sdk.Bech32ifyAddressBytes(
		sdk.GetConfig().GetBech32AccountAddrPrefix(),
		common.HexToAddress(withdrawerHex).Bytes(),
	)
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           []interface{}
		wantErr        bool
		errMsg         string
		wantDelegator  string
		wantWithdrawer string
	}{
		{
			name:           "valid with bech32 withdrawer",
			args:           []interface{}{delegatorAddr, withdrawerBech32},
			wantErr:        false,
			wantDelegator:  expectedDelegatorAddr,
			wantWithdrawer: withdrawerBech32,
		},
		{
			name:           "valid with hex withdrawer",
			args:           []interface{}{delegatorAddr, withdrawerHex},
			wantErr:        false,
			wantDelegator:  expectedDelegatorAddr,
			wantWithdrawer: expectedWithdrawerFromHex,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{delegatorAddr, withdrawerBech32, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 3),
		},
		{
			name:    "invalid delegator type",
			args:    []interface{}{"not-an-address", withdrawerBech32},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, "not-an-address"),
		},
		{
			name:    "empty delegator address",
			args:    []interface{}{common.Address{}, withdrawerBech32},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, common.Address{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgSetWithdrawAddress(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, delegatorAddr, returnAddr)
				require.Equal(t, tt.wantDelegator, msg.DelegatorAddress)
				require.Equal(t, tt.wantWithdrawer, msg.WithdrawAddress)
			}
		})
	}
}

func TestNewMsgWithdrawDelegatorReward(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name          string
		args          []interface{}
		wantErr       bool
		errMsg        string
		wantDelegator string
		wantValidator string
	}{
		{
			name:          "valid",
			args:          []interface{}{delegatorAddr, validatorAddr},
			wantErr:       false,
			wantDelegator: expectedDelegatorAddr,
			wantValidator: validatorAddr,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "invalid delegator type",
			args:    []interface{}{"not-an-address", validatorAddr},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, "not-an-address"),
		},
		{
			name:    "empty delegator address",
			args:    []interface{}{common.Address{}, validatorAddr},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, common.Address{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgWithdrawDelegatorReward(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, delegatorAddr, returnAddr)
				require.Equal(t, tt.wantDelegator, msg.DelegatorAddress)
				require.Equal(t, tt.wantValidator, msg.ValidatorAddress)
			}
		})
	}
}

func TestNewMsgFundCommunityPool(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	depositorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validCoins := []cmn.Coin{{Denom: "stake", Amount: big.NewInt(1000)}}

	expectedDepositorAddr, err := addrCodec.BytesToString(depositorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name          string
		args          []interface{}
		wantErr       bool
		errMsg        string
		wantDepositor string
	}{
		{
			name:          "valid",
			args:          []interface{}{depositorAddr, validCoins},
			wantErr:       false,
			wantDepositor: expectedDepositorAddr,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "invalid depositor type",
			args:    []interface{}{"not-an-address", validCoins},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidHexAddress, "not-an-address"),
		},
		{
			name:    "empty depositor address",
			args:    []interface{}{common.Address{}, validCoins},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidHexAddress, common.Address{}),
		},
		{
			name:    "invalid coins",
			args:    []interface{}{depositorAddr, "invalid-coins"},
			wantErr: true,
			errMsg:  fmt.Sprintf(ErrInvalidAmount, "amount arg"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgFundCommunityPool(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, depositorAddr, returnAddr)
				require.Equal(t, tt.wantDepositor, msg.Depositor)
				require.NotEmpty(t, msg.Amount)
			}
		})
	}
}

func TestNewMsgDepositValidatorRewardsPool(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	depositorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validCoins := []cmn.Coin{{Denom: "stake", Amount: big.NewInt(1000)}}

	expectedDepositorAddr, err := addrCodec.BytesToString(depositorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name          string
		args          []interface{}
		wantErr       bool
		errMsg        string
		wantDepositor string
		wantValidator string
	}{
		{
			name:          "valid",
			args:          []interface{}{depositorAddr, validatorAddr, validCoins},
			wantErr:       false,
			wantDepositor: expectedDepositorAddr,
			wantValidator: validatorAddr,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 3, 0),
		},
		{
			name:    "invalid depositor type",
			args:    []interface{}{"not-an-address", validatorAddr, validCoins},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidHexAddress, "not-an-address"),
		},
		{
			name:    "empty depositor address",
			args:    []interface{}{common.Address{}, validatorAddr, validCoins},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidHexAddress, common.Address{}),
		},
		{
			name:    "invalid coins",
			args:    []interface{}{depositorAddr, validatorAddr, "invalid-coins"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidAmount, "invalid-coins"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgDepositValidatorRewardsPool(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, depositorAddr, returnAddr)
				require.Equal(t, tt.wantDepositor, msg.Depositor)
				require.Equal(t, tt.wantValidator, msg.ValidatorAddress)
				require.NotEmpty(t, msg.Amount)
			}
		})
	}
}

func TestNewDelegationRewardsRequest(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name          string
		args          []interface{}
		wantErr       bool
		errMsg        string
		wantDelegator string
		wantValidator string
	}{
		{
			name:          "valid",
			args:          []interface{}{delegatorAddr, validatorAddr},
			wantErr:       false,
			wantDelegator: expectedDelegatorAddr,
			wantValidator: validatorAddr,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "invalid delegator type",
			args:    []interface{}{"not-an-address", validatorAddr},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, "not-an-address"),
		},
		{
			name:    "empty delegator address",
			args:    []interface{}{common.Address{}, validatorAddr},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, common.Address{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewDelegationRewardsRequest(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantDelegator, req.DelegatorAddress)
				require.Equal(t, tt.wantValidator, req.ValidatorAddress)
			}
		})
	}
}

func TestNewDelegationTotalRewardsRequest(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name          string
		args          []interface{}
		wantErr       bool
		errMsg        string
		wantDelegator string
	}{
		{
			name:          "valid",
			args:          []interface{}{delegatorAddr},
			wantErr:       false,
			wantDelegator: expectedDelegatorAddr,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 0),
		},
		{
			name:    "invalid delegator type",
			args:    []interface{}{"not-an-address"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, "not-an-address"),
		},
		{
			name:    "empty delegator address",
			args:    []interface{}{common.Address{}},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, common.Address{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewDelegationTotalRewardsRequest(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantDelegator, req.DelegatorAddress)
			}
		})
	}
}

func TestNewDelegatorValidatorsRequest(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name          string
		args          []interface{}
		wantErr       bool
		errMsg        string
		wantDelegator string
	}{
		{
			name:          "valid",
			args:          []interface{}{delegatorAddr},
			wantErr:       false,
			wantDelegator: expectedDelegatorAddr,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 0),
		},
		{
			name:    "invalid delegator type",
			args:    []interface{}{"not-an-address"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, "not-an-address"),
		},
		{
			name:    "empty delegator address",
			args:    []interface{}{common.Address{}},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, common.Address{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewDelegatorValidatorsRequest(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantDelegator, req.DelegatorAddress)
			}
		})
	}
}

func TestNewDelegatorWithdrawAddressRequest(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name          string
		args          []interface{}
		wantErr       bool
		errMsg        string
		wantDelegator string
	}{
		{
			name:          "valid",
			args:          []interface{}{delegatorAddr},
			wantErr:       false,
			wantDelegator: expectedDelegatorAddr,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 1, 0),
		},
		{
			name:    "invalid delegator type",
			args:    []interface{}{"not-an-address"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, "not-an-address"),
		},
		{
			name:    "empty delegator address",
			args:    []interface{}{common.Address{}},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, common.Address{}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewDelegatorWithdrawAddressRequest(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantDelegator, req.DelegatorAddress)
			}
		})
	}
}
