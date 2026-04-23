package eip712_test

import (
	"context"
	"fmt"
	"math/big"
	"strconv"
	"testing"

	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/ethereum/eip712"
	"github.com/cosmos/evm/testutil/constants"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// setupEIP712Test initializes the EIP-712 encoding codec with the given
// evmChainID set as the global eip155ChainID (as SetEncodingConfig does at
// app boot), plus bank/auth types so MsgSend sign docs round-trip.
func setupEIP712Test(t *testing.T, globalEVMChainID uint64) encoding.Config {
	t.Helper()

	cfg := encoding.MakeConfig(globalEVMChainID)
	authtypes.RegisterInterfaces(cfg.InterfaceRegistry)
	banktypes.RegisterInterfaces(cfg.InterfaceRegistry)
	banktypes.RegisterLegacyAminoCodec(cfg.Amino)

	// Re-register so the eip712 globals see the bank/auth interfaces above.
	eip712.SetEncodingConfig(cfg.Amino, cfg.InterfaceRegistry, globalEVMChainID)

	return cfg
}

// generateBankSendSignBytes builds MsgSend sign-doc bytes for the given sign
// mode, using chainIDStr as the signer's --chain-id.
func generateBankSendSignBytes(
	t *testing.T,
	cfg encoding.Config,
	signMode signing.SignMode,
	chainIDStr string,
) []byte {
	t.Helper()

	privKey, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	pubKey := privKey.PubKey()
	senderAddr := sdk.AccAddress(pubKey.Address().Bytes())

	recipientPriv, err := ethsecp256k1.GenerateKey()
	require.NoError(t, err)
	recipientAddr := sdk.AccAddress(recipientPriv.PubKey().Address().Bytes())

	msg := banktypes.NewMsgSend(
		senderAddr,
		recipientAddr,
		sdk.NewCoins(sdk.NewCoin("atest", sdkmath.NewInt(1000))),
	)

	txBuilder := cfg.TxConfig.NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(msg))
	txBuilder.SetGasLimit(200_000)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin("atest", sdkmath.NewInt(100))))

	sigV2 := signing.SignatureV2{
		PubKey: pubKey,
		Data: &signing.SingleSignatureData{
			SignMode:  signMode,
			Signature: nil,
		},
		Sequence: 0,
	}
	require.NoError(t, txBuilder.SetSignatures(sigV2))

	signerData := authsigning.SignerData{
		Address:       senderAddr.String(),
		ChainID:       chainIDStr,
		AccountNumber: 0,
		Sequence:      0,
		PubKey:        pubKey,
	}

	signBytes, err := authsigning.GetSignBytesAdapter(
		context.Background(),
		cfg.TxConfig.SignModeHandler(),
		signMode,
		signerData,
		txBuilder.GetTx(),
	)
	require.NoError(t, err)

	return signBytes
}

// Regression tests for https://github.com/cosmos/evm/pull/918: the EIP-712
// domain chain id must come from the sign doc, not the process-global
// eip155ChainID (which can be stale when CLI flows build a temporary app
// with simtestutil.EmptyAppOptions{}).
func TestGetEIP712TypedData_UsesChainIDFromSignDoc_Amino(t *testing.T) {
	const wrongGlobalChainID uint64 = 262144
	expectedEVMChainID := constants.ExampleChainID.EVMChainID
	cosmosChainID := fmt.Sprintf("%s_%d-1", constants.ExampleChainIDPrefix, expectedEVMChainID)

	cfg := setupEIP712Test(t, wrongGlobalChainID)

	signBytes := generateBankSendSignBytes(
		t, cfg,
		signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		cosmosChainID,
	)

	typedData, err := eip712.GetEIP712TypedDataForMsg(signBytes)
	require.NoError(t, err)

	requireDomainChainID(t, expectedEVMChainID, typedData.Domain.ChainId,
		"amino: domain chain id must come from the sign doc")
}

func TestGetEIP712TypedData_UsesChainIDFromSignDoc_Protobuf(t *testing.T) {
	const wrongGlobalChainID uint64 = 262144
	expectedEVMChainID := constants.ExampleChainID.EVMChainID
	cosmosChainID := fmt.Sprintf("%s_%d-1", constants.ExampleChainIDPrefix, expectedEVMChainID)

	cfg := setupEIP712Test(t, wrongGlobalChainID)

	signBytes := generateBankSendSignBytes(
		t, cfg,
		signing.SignMode_SIGN_MODE_DIRECT,
		cosmosChainID,
	)

	typedData, err := eip712.GetEIP712TypedDataForMsg(signBytes)
	require.NoError(t, err)

	requireDomainChainID(t, expectedEVMChainID, typedData.Domain.ChainId,
		"protobuf: domain chain id must come from the sign doc")
}

// Bare-decimal chain ids (as used in the PR #918 reproduction with intervald)
// must also resolve via the sign doc.
func TestGetEIP712TypedData_UsesChainIDFromSignDoc_BareNumeric(t *testing.T) {
	const wrongGlobalChainID uint64 = 262144
	const signDocChainID uint64 = 1230263908

	cfg := setupEIP712Test(t, wrongGlobalChainID)

	signBytes := generateBankSendSignBytes(
		t, cfg,
		signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		strconv.FormatUint(signDocChainID, 10),
	)

	typedData, err := eip712.GetEIP712TypedDataForMsg(signBytes)
	require.NoError(t, err)

	requireDomainChainID(t, signDocChainID, typedData.Domain.ChainId,
		"bare numeric: domain chain id must come from the sign doc")
}

// Chain ids that don't encode an EVM id (e.g. "cosmos-1", "cosmoshub-4")
// fall back to the global eip155ChainID.
func TestGetEIP712TypedData_FallsBackToGlobal_NonNumericChainID(t *testing.T) {
	const globalEVMChainID uint64 = 9001

	cfg := setupEIP712Test(t, globalEVMChainID)

	for _, cosmosChainID := range []string{
		constants.ExampleChainID.ChainID, // "cosmos-1"
		"cosmoshub-4",
	} {
		t.Run(cosmosChainID, func(t *testing.T) {
			signBytes := generateBankSendSignBytes(
				t, cfg,
				signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
				cosmosChainID,
			)

			typedData, err := eip712.GetEIP712TypedDataForMsg(signBytes)
			require.NoError(t, err)

			requireDomainChainID(t, globalEVMChainID, typedData.Domain.ChainId,
				"should fall back to global eip155ChainID")
		})
	}
}

// requireDomainChainID asserts domain.ChainId equals expected. HexOrDecimal256's
// underlying type is big.Int, so a direct pointer conversion is safe.
func requireDomainChainID(t *testing.T, expected uint64, actual *ethmath.HexOrDecimal256, msg string) {
	t.Helper()
	require.NotNil(t, actual, "%s: domain.ChainId is nil", msg)
	actualBig := (*big.Int)(actual)
	require.Zero(t,
		new(big.Int).SetUint64(expected).Cmp(actualBig),
		"%s: got %s, want %d", msg, actualBig.String(), expected,
	)
}
