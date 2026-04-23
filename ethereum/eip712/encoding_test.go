package eip712_test

import (
	"context"
	"math/big"
	"strconv"
	"testing"

	ethmath "github.com/ethereum/go-ethereum/common/math"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/ethereum/eip712"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// setupEIP712Test initializes the EIP-712 encoding codec with the given
// evmChainID registered globally (mirroring what `SetEncodingConfig` does at
// app boot). Bank and auth types are registered so we can sign `MsgSend`
// payloads.
func setupEIP712Test(t *testing.T, globalEVMChainID uint64) encoding.Config {
	t.Helper()

	cfg := encoding.MakeConfig(globalEVMChainID)
	authtypes.RegisterInterfaces(cfg.InterfaceRegistry)
	banktypes.RegisterInterfaces(cfg.InterfaceRegistry)
	banktypes.RegisterLegacyAminoCodec(cfg.Amino)

	// Re-point the eip712 globals to the freshly populated codec / registry so
	// any interface registrations above are visible to the encoding layer.
	eip712.SetEncodingConfig(cfg.Amino, cfg.InterfaceRegistry, globalEVMChainID)

	return cfg
}

// generateBankSendSignBytes builds sign-doc bytes for a MsgSend transaction
// using the provided sign mode and Cosmos chain-id string. The chain-id is the
// value a signer would see through their `--chain-id` flag at sign time.
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

// Regression test for https://github.com/cosmos/evm/pull/918.
//
// Ledger multisig (and `tx validate-signatures` / `tx multi-sign`) flows create
// a temporary app with `simtestutil.EmptyAppOptions{}`, which leaves the global
// `eip155ChainID` at its default (262144) instead of the chain's actual EVM
// chain id. That meant EIP-712 reconstruction during verification used a
// different domain chain id than the one used at sign time, producing the
// "signature verification failed: unauthorized" error.
//
// The fix extracts the chain id straight from the sign doc so domain hashes
// match regardless of the process-global EVM chain id. These tests lock in
// that contract for both amino and protobuf sign docs.
func TestGetEIP712TypedData_UsesChainIDFromSignDoc_Amino(t *testing.T) {
	const wrongGlobalChainID uint64 = 262144 // default evm chain id
	const signDocChainID uint64 = 1230263908 // what the signer's --chain-id was

	cfg := setupEIP712Test(t, wrongGlobalChainID)

	signBytes := generateBankSendSignBytes(
		t, cfg,
		signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		strconv.FormatUint(signDocChainID, 10),
	)

	typedData, err := eip712.GetEIP712TypedDataForMsg(signBytes)
	require.NoError(t, err)

	requireDomainChainID(t, signDocChainID, typedData.Domain.ChainId,
		"amino decode path: EIP-712 domain chain id must come from the sign doc, not the global eip155ChainID")
}

func TestGetEIP712TypedData_UsesChainIDFromSignDoc_Protobuf(t *testing.T) {
	const wrongGlobalChainID uint64 = 262144
	const signDocChainID uint64 = 1230263908

	cfg := setupEIP712Test(t, wrongGlobalChainID)

	signBytes := generateBankSendSignBytes(
		t, cfg,
		signing.SignMode_SIGN_MODE_DIRECT,
		strconv.FormatUint(signDocChainID, 10),
	)

	typedData, err := eip712.GetEIP712TypedDataForMsg(signBytes)
	require.NoError(t, err)

	requireDomainChainID(t, signDocChainID, typedData.Domain.ChainId,
		"protobuf decode path: EIP-712 domain chain id must come from the sign doc, not the global eip155ChainID")
}

// When the sign doc's chain id is not a bare numeric string (e.g. a typical
// cosmos "name-revision" chain id like "cosmoshub-4"), `parseChainID` fails
// and we fall back to the configured global `eip155ChainID`. This preserves
// the pre-fix behaviour for chains that don't use a numeric chain id.
func TestGetEIP712TypedData_FallsBackToGlobal_NonNumericChainID(t *testing.T) {
	const globalEVMChainID uint64 = 9001

	cfg := setupEIP712Test(t, globalEVMChainID)

	signBytes := generateBankSendSignBytes(
		t, cfg,
		signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
		"cosmoshub-4",
	)

	typedData, err := eip712.GetEIP712TypedDataForMsg(signBytes)
	require.NoError(t, err)

	requireDomainChainID(t, globalEVMChainID, typedData.Domain.ChainId,
		"non-numeric cosmos chain id should fall back to global eip155ChainID")
}

// requireDomainChainID asserts that the EIP-712 domain's ChainId matches the
// expected uint64. `apitypes.TypedDataDomain.ChainId` is a
// `*math.HexOrDecimal256`, whose underlying type is `big.Int`, so we can
// convert directly for comparison.
func requireDomainChainID(t *testing.T, expected uint64, actual *ethmath.HexOrDecimal256, msg string) {
	t.Helper()
	require.NotNil(t, actual, "%s: domain.ChainId is nil", msg)
	actualBig := (*big.Int)(actual)
	require.Zero(t,
		new(big.Int).SetUint64(expected).Cmp(actualBig),
		"%s: got %s, want %d", msg, actualBig.String(), expected,
	)
}
