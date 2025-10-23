package staking

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	evmaddress "github.com/cosmos/evm/encoding/address"

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
	commission := CommissionRates{
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
		args              CreateValidatorCall
		wantErr           bool
		errMsg            string
		wantDelegatorAddr string
		wantValidatorAddr string
		wantMinSelfDel    *big.Int
		wantValue         *big.Int
	}{
		{
			name: "valid",
			args: CreateValidatorCall{
				Description:       description,
				CommissionRates:   commission,
				MinSelfDelegation: minSelfDelegation,
				ValidatorAddress:  validatorHexAddr,
				Pubkey:            pubkey,
				Value:             value,
			},
			wantErr:           false,
			wantDelegatorAddr: expectedValidatorAddr,
			wantValidatorAddr: sdk.ValAddress(validatorHexAddr.Bytes()).String(),
			wantMinSelfDel:    minSelfDelegation,
			wantValue:         value,
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
		args              DelegateCall
		wantErr           bool
		errMsg            string
		wantDelegatorAddr string
		wantValidatorAddr string
		wantAmount        *big.Int
	}{
		{
			name: "valid",
			args: DelegateCall{
				DelegatorAddress: delegatorAddr,
				ValidatorAddress: validatorAddr,
				Amount:           amount,
			},
			wantErr:           false,
			wantDelegatorAddr: expectedDelegatorAddr,
			wantValidatorAddr: validatorAddr,
			wantAmount:        amount,
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
		args              UndelegateCall
		wantErr           bool
		errMsg            string
		wantDelegatorAddr string
		wantValidatorAddr string
		wantAmount        *big.Int
	}{
		{
			name: "valid",
			args: UndelegateCall{
				DelegatorAddress: delegatorAddr,
				ValidatorAddress: validatorAddr,
				Amount:           amount,
			},
			wantErr:           false,
			wantDelegatorAddr: expectedDelegatorAddr,
			wantValidatorAddr: validatorAddr,
			wantAmount:        amount,
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
		args                 RedelegateCall
		wantErr              bool
		errMsg               string
		wantDelegatorAddr    string
		wantValidatorSrcAddr string
		wantValidatorDstAddr string
		wantAmount           *big.Int
	}{
		{
			name: "valid",
			args: RedelegateCall{
				DelegatorAddress:    delegatorAddr,
				ValidatorSrcAddress: validatorSrcAddr,
				ValidatorDstAddress: validatorDstAddr,
				Amount:              amount,
			},
			wantErr:              false,
			wantDelegatorAddr:    expectedDelegatorAddr,
			wantValidatorSrcAddr: validatorSrcAddr,
			wantValidatorDstAddr: validatorDstAddr,
			wantAmount:           amount,
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
		args               CancelUnbondingDelegationCall
		wantErr            bool
		errMsg             string
		wantDelegatorAddr  string
		wantValidatorAddr  string
		wantAmount         *big.Int
		wantCreationHeight int64
	}{
		{
			name: "valid",
			args: CancelUnbondingDelegationCall{
				DelegatorAddress: delegatorAddr,
				ValidatorAddress: validatorAddr,
				Amount:           amount,
				CreationHeight:   creationHeight,
			},
			wantErr:            false,
			wantDelegatorAddr:  expectedDelegatorAddr,
			wantValidatorAddr:  validatorAddr,
			wantAmount:         amount,
			wantCreationHeight: creationHeight.Int64(),
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
		args              DelegationCall
		wantErr           bool
		errMsg            string
		wantDelegatorAddr string
		wantValidatorAddr string
	}{
		{
			name: "valid",
			args: DelegationCall{
				DelegatorAddress: delegatorAddr,
				ValidatorAddress: validatorAddr,
			},
			wantErr:           false,
			wantDelegatorAddr: expectedDelegatorAddr,
			wantValidatorAddr: validatorAddr,
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
		args              UnbondingDelegationCall
		wantErr           bool
		errMsg            string
		wantDelegatorAddr string
		wantValidatorAddr string
	}{
		{
			name: "valid",
			args: UnbondingDelegationCall{
				DelegatorAddress: delegatorAddr,
				ValidatorAddress: validatorAddr,
			},
			wantErr:           false,
			wantDelegatorAddr: expectedDelegatorAddr,
			wantValidatorAddr: validatorAddr,
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
