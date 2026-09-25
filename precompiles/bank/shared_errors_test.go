package bank

import (
	"errors"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"

	errorsmod "cosmossdk.io/errors"

	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
)

var errSyntheticBankDrift = errorsmod.Register("bank-g008-drift", 77, "unstable reason")

func TestBankInheritsCanonicalSharedErrorABI(t *testing.T) {
	require.NoError(t, cmn.ValidateSharedErrorABI(ABI))
	require.NoError(t, cmn.ValidateCosmosErrorRegistry(ABI, nil, cmn.SharedSDKErrorMappings(), nil))
}

func TestBankSharedErrorRepresentativeMatrix(t *testing.T) {
	tests := []struct {
		name          string
		sentinel      error
		solidityError string
	}{
		{"unauthorized", sdkerrors.ErrUnauthorized, cmn.SolidityErrSDKUnauthorized},
		{"insufficient funds", sdkerrors.ErrInsufficientFunds, cmn.SolidityErrSDKInsufficientFunds},
		{"invalid address", sdkerrors.ErrInvalidAddress, cmn.SolidityErrSDKInvalidAddress},
		{"invalid coins", sdkerrors.ErrInvalidCoins, cmn.SolidityErrSDKInvalidCoins},
		{"invalid request", sdkerrors.ErrInvalidRequest, cmn.SolidityErrSDKInvalidRequest},
		{"invalid type", sdkerrors.ErrInvalidType, cmn.SolidityErrSDKInvalidType},
		{"not found", sdkerrors.ErrNotFound, cmn.SolidityErrSDKNotFound},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			for _, returned := range []error{
				tc.sentinel,
				errorsmod.Wrap(tc.sentinel, "changed Cosmos wrapper text"),
				fmt.Errorf("changed standard wrapper text: %w", tc.sentinel),
			} {
				translation := cmn.TranslateCosmosError(ABI, cosmosErrorRegistry, returned)
				require.Equal(t, cmn.MappingKindSharedSDK, translation.Kind)
				expectedKey, ok := cmn.ExtractCosmosErrorKey(tc.sentinel)
				require.True(t, ok)
				require.Equal(t, expectedKey, translation.Key)
				require.False(t, translation.IsUnmapped)

				data := translation.Revert.(cmn.RevertDataCarrier).RevertData()
				expected := cmn.NewRevertWithSolidityError(ABI, tc.solidityError).(cmn.RevertDataCarrier).RevertData()
				require.Equal(t, expected, data)
				assertBankParsedError(t, data, tc.solidityError)
			}
		})
	}
}

func TestBankProtocolInfrastructureAndDriftBoundaries(t *testing.T) {
	_, protocolErr := ParseBalancesArgs(nil)
	require.Error(t, protocolErr)
	protocolData := protocolErr.(cmn.RevertDataCarrier).RevertData()
	assertBankParsedError(t, protocolData, cmn.SolidityErrInvalidNumberOfArgs)
	protocolDefinition := ABI.Errors[cmn.SolidityErrInvalidNumberOfArgs]
	protocolArgs, err := protocolDefinition.Unpack(protocolData)
	require.NoError(t, err)
	protocolValues := protocolArgs.([]interface{})
	require.Len(t, protocolValues, 2)
	require.Zero(t, protocolValues[0].(*big.Int).Cmp(big.NewInt(1)))
	require.Zero(t, protocolValues[1].(*big.Int).Cmp(big.NewInt(0)))

	infrastructureErr := errors.New("bank infrastructure failure")
	infrastructure := cmn.TranslateCosmosError(ABI, cosmosErrorRegistry, infrastructureErr)
	require.Equal(t, cmn.MappingKindInternal, infrastructure.Kind)
	require.ErrorIs(t, infrastructure.Revert, infrastructureErr)

	drift := cmn.TranslateCosmosError(ABI, cosmosErrorRegistry, errSyntheticBankDrift)
	require.Equal(t, cmn.MappingKindUnmapped, drift.Kind)
	require.True(t, drift.IsUnmapped)
	driftData := drift.Revert.(cmn.RevertDataCarrier).RevertData()
	assertBankParsedError(t, driftData, cmn.SolidityErrUnmappedCosmosError)
	driftDefinition := ABI.Errors[cmn.SolidityErrUnmappedCosmosError]
	driftArgs, err := driftDefinition.Unpack(driftData)
	require.NoError(t, err)
	require.Equal(t, []interface{}{errSyntheticBankDrift.Codespace(), errSyntheticBankDrift.ABCICode()}, driftArgs)
}

func assertBankParsedError(t *testing.T, data []byte, expectedName string) {
	t.Helper()
	require.GreaterOrEqual(t, len(data), 4)
	var selector [4]byte
	copy(selector[:], data[:4])
	definition, err := ABI.ErrorByID(selector)
	require.NoError(t, err)
	require.Equal(t, expectedName, definition.Name)
	_, err = definition.Unpack(data)
	require.NoError(t, err)
}
