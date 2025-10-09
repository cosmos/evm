package staking

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

const (
	denom         = "stake"
	validatorAddr = "cosmosvaloper1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5a3kaax"
)

func TestNewMsgCreateValidator(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	validatorHexAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	description := Description{
		Moniker:         "test-validator",
		Identity:        "test-identity",
		Website:         "https://test.com",
		SecurityContact: "test@test.com",
		Details:         "test validator",
	}
	commission := Commission{
		Rate:          big.NewInt(100000000000000000), // 0.1
		MaxRate:       big.NewInt(200000000000000000), // 0.2
		MaxChangeRate: big.NewInt(10000000000000000),  // 0.01
	}
	minSelfDelegation := big.NewInt(1000000)
	pubkey := "rOQZYCGGhzjKUOUlM3MfOWFxGKX8L5z5B+/J9NqfLmw="
	value := big.NewInt(1000000000)

	expectedValidatorAddr, err := addrCodec.BytesToString(validatorHexAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name              string
		args              []interface{}
		wantErr           bool
		errMsg            string
		wantDelegatorAddr string
		wantValidatorAddr string
		wantMinSelfDel    *big.Int
		wantValue         *big.Int
	}{
		{
			name:              "valid",
			args:              []interface{}{description, commission, minSelfDelegation, validatorHexAddr, pubkey, value},
			wantErr:           false,
			wantDelegatorAddr: expectedValidatorAddr,
			wantValidatorAddr: sdk.ValAddress(validatorHexAddr.Bytes()).String(),
			wantMinSelfDel:    minSelfDelegation,
			wantValue:         value,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 6, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{description, commission, minSelfDelegation, validatorHexAddr, pubkey, value, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 6, 7),
		},
		{
			name:    "invalid description type",
			args:    []interface{}{"not-a-description", commission, minSelfDelegation, validatorHexAddr, pubkey, value},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDescription, "not-a-description"),
		},
		{
			name:    "invalid commission type",
			args:    []interface{}{description, "not-a-commission", minSelfDelegation, validatorHexAddr, pubkey, value},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidCommission, "not-a-commission"),
		},
		{
			name:    "invalid min self delegation type",
			args:    []interface{}{description, commission, "not-a-big-int", validatorHexAddr, pubkey, value},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidAmount, "not-a-big-int"),
		},
		{
			name:    "invalid validator address type",
			args:    []interface{}{description, commission, minSelfDelegation, "not-an-address", pubkey, value},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidValidator, "not-an-address"),
		},
		{
			name:    "empty validator address",
			args:    []interface{}{description, commission, minSelfDelegation, common.Address{}, pubkey, value},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidValidator, common.Address{}),
		},
		{
			name:    "invalid pubkey type",
			args:    []interface{}{description, commission, minSelfDelegation, validatorHexAddr, 123, value},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidType, "pubkey", "string", 123),
		},
		{
			name:    "invalid value type",
			args:    []interface{}{description, commission, minSelfDelegation, validatorHexAddr, pubkey, "not-a-big-int"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidAmount, "not-a-big-int"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgCreateValidator(tt.args, denom, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, validatorHexAddr, returnAddr)
				require.Equal(t, tt.wantDelegatorAddr, msg.DelegatorAddress) //nolint:staticcheck // its populated, we'll check it
				require.Equal(t, tt.wantValidatorAddr, msg.ValidatorAddress)
				require.Equal(t, tt.wantMinSelfDel, msg.MinSelfDelegation.BigInt())
				require.Equal(t, tt.wantValue, msg.Value.Amount.BigInt())
				require.Equal(t, denom, msg.Value.Denom)
			}
		})
	}
}

func TestNewMsgDelegate(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	amount := big.NewInt(1000000000)

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name              string
		args              []interface{}
		wantErr           bool
		errMsg            string
		wantDelegatorAddr string
		wantValidatorAddr string
		wantAmount        *big.Int
	}{
		{
			name:              "valid",
			args:              []interface{}{delegatorAddr, validatorAddr, amount},
			wantErr:           false,
			wantDelegatorAddr: expectedDelegatorAddr,
			wantValidatorAddr: validatorAddr,
			wantAmount:        amount,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 3, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{delegatorAddr, validatorAddr, amount, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 3, 4),
		},
		{
			name:    "invalid delegator type",
			args:    []interface{}{"not-an-address", validatorAddr, amount},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, "not-an-address"),
		},
		{
			name:    "empty delegator address",
			args:    []interface{}{common.Address{}, validatorAddr, amount},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, common.Address{}),
		},
		{
			name:    "invalid validator address type",
			args:    []interface{}{delegatorAddr, 123, amount},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidType, "validatorAddress", "string", 123),
		},
		{
			name:    "invalid amount type",
			args:    []interface{}{delegatorAddr, validatorAddr, "not-a-big-int"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidAmount, "not-a-big-int"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgDelegate(tt.args, denom, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, delegatorAddr, returnAddr)
				require.Equal(t, tt.wantDelegatorAddr, msg.DelegatorAddress)
				require.Equal(t, tt.wantValidatorAddr, msg.ValidatorAddress)
				require.Equal(t, tt.wantAmount, msg.Amount.Amount.BigInt())
				require.Equal(t, denom, msg.Amount.Denom)
			}
		})
	}
}

func TestNewMsgUndelegate(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	amount := big.NewInt(1000000000)

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name              string
		args              []interface{}
		wantErr           bool
		errMsg            string
		wantDelegatorAddr string
		wantValidatorAddr string
		wantAmount        *big.Int
	}{
		{
			name:              "valid",
			args:              []interface{}{delegatorAddr, validatorAddr, amount},
			wantErr:           false,
			wantDelegatorAddr: expectedDelegatorAddr,
			wantValidatorAddr: validatorAddr,
			wantAmount:        amount,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 3, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{delegatorAddr, validatorAddr, amount, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 3, 4),
		},
		{
			name:    "invalid delegator type",
			args:    []interface{}{"not-an-address", validatorAddr, amount},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, "not-an-address"),
		},
		{
			name:    "empty delegator address",
			args:    []interface{}{common.Address{}, validatorAddr, amount},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, common.Address{}),
		},
		{
			name:    "invalid validator address type",
			args:    []interface{}{delegatorAddr, 123, amount},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidType, "validatorAddress", "string", 123),
		},
		{
			name:    "invalid amount type",
			args:    []interface{}{delegatorAddr, validatorAddr, "not-a-big-int"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidAmount, "not-a-big-int"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgUndelegate(tt.args, denom, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, delegatorAddr, returnAddr)
				require.Equal(t, tt.wantDelegatorAddr, msg.DelegatorAddress)
				require.Equal(t, tt.wantValidatorAddr, msg.ValidatorAddress)
				require.Equal(t, tt.wantAmount, msg.Amount.Amount.BigInt())
				require.Equal(t, denom, msg.Amount.Denom)
			}
		})
	}
}

func TestNewMsgRedelegate(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	validatorSrcAddr := "cosmosvaloper1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5a3kaax"
	validatorDstAddr := "cosmosvaloper1qypqxpq9qcrsszg2pvxq6rs0zqg3yyc5a3kaay"
	amount := big.NewInt(1000000000)

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name                 string
		args                 []interface{}
		wantErr              bool
		errMsg               string
		wantDelegatorAddr    string
		wantValidatorSrcAddr string
		wantValidatorDstAddr string
		wantAmount           *big.Int
	}{
		{
			name:                 "valid",
			args:                 []interface{}{delegatorAddr, validatorSrcAddr, validatorDstAddr, amount},
			wantErr:              false,
			wantDelegatorAddr:    expectedDelegatorAddr,
			wantValidatorSrcAddr: validatorSrcAddr,
			wantValidatorDstAddr: validatorDstAddr,
			wantAmount:           amount,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 4, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{delegatorAddr, validatorSrcAddr, validatorDstAddr, amount, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 4, 5),
		},
		{
			name:    "invalid delegator type",
			args:    []interface{}{"not-an-address", validatorSrcAddr, validatorDstAddr, amount},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, "not-an-address"),
		},
		{
			name:    "empty delegator address",
			args:    []interface{}{common.Address{}, validatorSrcAddr, validatorDstAddr, amount},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, common.Address{}),
		},
		{
			name:    "invalid validator src address type",
			args:    []interface{}{delegatorAddr, 123, validatorDstAddr, amount},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidType, "validatorSrcAddress", "string", 123),
		},
		{
			name:    "invalid validator dst address type",
			args:    []interface{}{delegatorAddr, validatorSrcAddr, 123, amount},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidType, "validatorDstAddress", "string", 123),
		},
		{
			name:    "invalid amount type",
			args:    []interface{}{delegatorAddr, validatorSrcAddr, validatorDstAddr, "not-a-big-int"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidAmount, "not-a-big-int"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgRedelegate(tt.args, denom, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, delegatorAddr, returnAddr)
				require.Equal(t, tt.wantDelegatorAddr, msg.DelegatorAddress)
				require.Equal(t, tt.wantValidatorSrcAddr, msg.ValidatorSrcAddress)
				require.Equal(t, tt.wantValidatorDstAddr, msg.ValidatorDstAddress)
				require.Equal(t, tt.wantAmount, msg.Amount.Amount.BigInt())
				require.Equal(t, denom, msg.Amount.Denom)
			}
		})
	}
}

func TestNewMsgCancelUnbondingDelegation(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")
	amount := big.NewInt(1000000000)
	creationHeight := big.NewInt(100)

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name               string
		args               []interface{}
		wantErr            bool
		errMsg             string
		wantDelegatorAddr  string
		wantValidatorAddr  string
		wantAmount         *big.Int
		wantCreationHeight int64
	}{
		{
			name:               "valid",
			args:               []interface{}{delegatorAddr, validatorAddr, amount, creationHeight},
			wantErr:            false,
			wantDelegatorAddr:  expectedDelegatorAddr,
			wantValidatorAddr:  validatorAddr,
			wantAmount:         amount,
			wantCreationHeight: creationHeight.Int64(),
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 4, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{delegatorAddr, validatorAddr, amount, creationHeight, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 4, 5),
		},
		{
			name:    "invalid delegator type",
			args:    []interface{}{"not-an-address", validatorAddr, amount, creationHeight},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, "not-an-address"),
		},
		{
			name:    "empty delegator address",
			args:    []interface{}{common.Address{}, validatorAddr, amount, creationHeight},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidDelegator, common.Address{}),
		},
		{
			name:    "invalid validator address type",
			args:    []interface{}{delegatorAddr, 123, amount, creationHeight},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidType, "validatorAddress", "string", 123),
		},
		{
			name:    "invalid amount type",
			args:    []interface{}{delegatorAddr, validatorAddr, "not-a-big-int", creationHeight},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidAmount, "not-a-big-int"),
		},
		{
			name:    "invalid creation height type",
			args:    []interface{}{delegatorAddr, validatorAddr, amount, "not-a-big-int"},
			wantErr: true,
			errMsg:  "invalid creation height",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgCancelUnbondingDelegation(tt.args, denom, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, msg)
			} else {
				require.NoError(t, err)
				require.NotNil(t, msg)
				require.Equal(t, delegatorAddr, returnAddr)
				require.Equal(t, tt.wantDelegatorAddr, msg.DelegatorAddress)
				require.Equal(t, tt.wantValidatorAddr, msg.ValidatorAddress)
				require.Equal(t, tt.wantAmount, msg.Amount.Amount.BigInt())
				require.Equal(t, tt.wantCreationHeight, msg.CreationHeight)
				require.Equal(t, denom, msg.Amount.Denom)
			}
		})
	}
}

func TestNewDelegationRequest(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name              string
		args              []interface{}
		wantErr           bool
		errMsg            string
		wantDelegatorAddr string
		wantValidatorAddr string
	}{
		{
			name:              "valid",
			args:              []interface{}{delegatorAddr, validatorAddr},
			wantErr:           false,
			wantDelegatorAddr: expectedDelegatorAddr,
			wantValidatorAddr: validatorAddr,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{delegatorAddr, validatorAddr, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 3),
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
		{
			name:    "invalid validator address type",
			args:    []interface{}{delegatorAddr, 123},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidType, "validatorAddress", "string", 123),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewDelegationRequest(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantDelegatorAddr, req.DelegatorAddr)
				require.Equal(t, tt.wantValidatorAddr, req.ValidatorAddr)
			}
		})
	}
}

func TestNewUnbondingDelegationRequest(t *testing.T) {
	addrCodec := evmaddress.NewEvmCodec(sdk.GetConfig().GetBech32AccountAddrPrefix())

	delegatorAddr := common.HexToAddress("0x1234567890123456789012345678901234567890")

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name              string
		args              []interface{}
		wantErr           bool
		errMsg            string
		wantDelegatorAddr string
		wantValidatorAddr string
	}{
		{
			name:              "valid",
			args:              []interface{}{delegatorAddr, validatorAddr},
			wantErr:           false,
			wantDelegatorAddr: expectedDelegatorAddr,
			wantValidatorAddr: validatorAddr,
		},
		{
			name:    "no arguments",
			args:    []interface{}{},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 0),
		},
		{
			name:    "too many arguments",
			args:    []interface{}{delegatorAddr, validatorAddr, "extra"},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidNumberOfArgs, 2, 3),
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
		{
			name:    "invalid validator address type",
			args:    []interface{}{delegatorAddr, 123},
			wantErr: true,
			errMsg:  fmt.Sprintf(cmn.ErrInvalidType, "validatorAddress", "string", 123),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewUnbondingDelegationRequest(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Contains(t, err.Error(), tt.errMsg)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantDelegatorAddr, req.DelegatorAddr)
				require.Equal(t, tt.wantValidatorAddr, req.ValidatorAddr)
			}
		})
	}
}
