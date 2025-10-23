package distribution

import (
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

	expectedDelegatorAddr, err := addrCodec.BytesToString(delegatorAddr.Bytes())
	require.NoError(t, err)

	tests := []struct {
		name           string
		args           SetWithdrawAddressCall
		wantErr        bool
		wantDelegator  string
		wantWithdrawer string
	}{
		{
			name: "valid with bech32 withdrawer",
			args: SetWithdrawAddressCall{
				DelegatorAddress:  delegatorAddr,
				WithdrawerAddress: withdrawerBech32,
			},
			wantErr:        false,
			wantDelegator:  expectedDelegatorAddr,
			wantWithdrawer: withdrawerBech32,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgSetWithdrawAddress(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
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
		args          WithdrawDelegatorRewardsCall
		wantErr       bool
		wantDelegator string
		wantValidator string
	}{
		{
			name: "valid",
			args: WithdrawDelegatorRewardsCall{
				DelegatorAddress: delegatorAddr,
				ValidatorAddress: validatorAddr,
			},
			wantErr:       false,
			wantDelegator: expectedDelegatorAddr,
			wantValidator: validatorAddr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgWithdrawDelegatorReward(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
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
		args          FundCommunityPoolCall
		wantErr       bool
		wantDepositor string
	}{
		{
			name: "valid",
			args: FundCommunityPoolCall{
				Depositor: depositorAddr,
				Amount:    validCoins,
			},
			wantErr:       false,
			wantDepositor: expectedDepositorAddr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgFundCommunityPool(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
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
		args          DepositValidatorRewardsPoolCall
		wantErr       bool
		wantDepositor string
		wantValidator string
	}{
		{
			name: "valid",
			args: DepositValidatorRewardsPoolCall{
				Depositor:        depositorAddr,
				ValidatorAddress: validatorAddr,
				Amount:           validCoins,
			},
			wantErr:       false,
			wantDepositor: expectedDepositorAddr,
			wantValidator: validatorAddr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg, returnAddr, err := NewMsgDepositValidatorRewardsPool(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
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
		args          DelegationRewardsCall
		wantErr       bool
		wantDelegator string
		wantValidator string
	}{
		{
			name: "valid",
			args: DelegationRewardsCall{
				DelegatorAddress: delegatorAddr,
				ValidatorAddress: validatorAddr,
			},
			wantErr:       false,
			wantDelegator: expectedDelegatorAddr,
			wantValidator: validatorAddr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewDelegationRewardsRequest(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
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
		args          DelegationTotalRewardsCall
		wantErr       bool
		wantDelegator string
	}{
		{
			name: "valid",
			args: DelegationTotalRewardsCall{
				DelegatorAddress: delegatorAddr,
			},
			wantErr:       false,
			wantDelegator: expectedDelegatorAddr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewDelegationTotalRewardsRequest(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
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
		args          DelegatorValidatorsCall
		wantErr       bool
		wantDelegator string
	}{
		{
			name: "valid",
			args: DelegatorValidatorsCall{
				DelegatorAddress: delegatorAddr,
			},
			wantErr:       false,
			wantDelegator: expectedDelegatorAddr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewDelegatorValidatorsRequest(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
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
		args          DelegatorWithdrawAddressCall
		wantErr       bool
		wantDelegator string
	}{
		{
			name: "valid",
			args: DelegatorWithdrawAddressCall{
				DelegatorAddress: delegatorAddr,
			},
			wantErr:       false,
			wantDelegator: expectedDelegatorAddr,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := NewDelegatorWithdrawAddressRequest(tt.args, addrCodec)

			if tt.wantErr {
				require.Error(t, err)
				require.Nil(t, req)
			} else {
				require.NoError(t, err)
				require.NotNil(t, req)
				require.Equal(t, tt.wantDelegator, req.DelegatorAddress)
			}
		})
	}
}
