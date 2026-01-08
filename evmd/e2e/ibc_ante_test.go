package e2e

import (
	"testing"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	eapp "github.com/cosmos/evm/evmd/app"
	"github.com/cosmos/evm/evmd/e2e/testharness"
	"github.com/cosmos/evm/evmd/e2e/utils"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
)

func TestIBCAntePanicsWithoutKeeper(t *testing.T) {
	t.Parallel()
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	tempApp := eapp.New(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		sims.EmptyAppOptions{},
	)
	txConfig := tempApp.GetTxConfig()
	ifaceRegistry := tempApp.InterfaceRegistry()

	senderBech32 := sdk.MustBech32ifyAddressBytes(utils.TestBech32Prefix, harness.SenderAddr.Bytes())

	authClient := authtypes.NewQueryClient(chain.GrpcClient)
	accountResp, err := authClient.Account(harness.Ctx, &authtypes.QueryAccountRequest{Address: senderBech32})
	require.NoError(t, err)

	var account sdk.AccountI
	require.NoError(t, ifaceRegistry.UnpackAny(accountResp.Account, &account))

	accountNumber := account.GetAccountNumber()
	sequence := account.GetSequence()

	packet := channeltypes.NewPacket(
		[]byte{0x01},
		1,
		"transfer",
		"channel-0",
		"transfer",
		"channel-1",
		clienttypes.NewHeight(1, 1),
		1,
	)
	msg := channeltypes.NewMsgTimeout(
		packet,
		1,
		[]byte{0x01},
		clienttypes.NewHeight(1, 1),
		senderBech32,
	)

	txBuilder := txConfig.NewTxBuilder()
	require.NoError(t, txBuilder.SetMsgs(msg))
	txBuilder.SetGasLimit(200_000)
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(utils.TestDenom, sdkmath.NewInt(1_000_000_000_000_000))))

	privKey := &ethsecp256k1.PrivKey{Key: crypto.FromECDSA(harness.SenderKey)}
	apiMode := txConfig.SignModeHandler().DefaultMode()
	mode, err := authsigning.APISignModeToInternal(apiMode)
	require.NoError(t, err)
	sigData := &signingtypes.SingleSignatureData{SignMode: mode}
	require.NoError(t, txBuilder.SetSignatures(signingtypes.SignatureV2{
		PubKey:   privKey.PubKey(),
		Data:     sigData,
		Sequence: sequence,
	}))

	signerData := authsigning.SignerData{
		ChainID:       utils.TestChainID,
		AccountNumber: accountNumber,
		Sequence:      sequence,
	}
	signBytes, err := authsigning.GetSignBytesAdapter(harness.Ctx, txConfig.SignModeHandler(), mode, signerData, txBuilder.GetTx())
	require.NoError(t, err)

	signature, err := privKey.Sign(signBytes)
	require.NoError(t, err)
	sigData.Signature = signature
	require.NoError(t, txBuilder.SetSignatures(signingtypes.SignatureV2{
		PubKey:   privKey.PubKey(),
		Data:     sigData,
		Sequence: sequence,
	}))

	txBytes, err := txConfig.TxEncoder()(txBuilder.GetTx())
	require.NoError(t, err)

	txClient := txtypes.NewServiceClient(chain.GrpcClient)
	res, err := txClient.BroadcastTx(harness.Ctx, &txtypes.BroadcastTxRequest{
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
		TxBytes: txBytes,
	})
	require.NoError(t, err)
	require.NotNil(t, res)
	require.NotNil(t, res.TxResponse)
	require.NotEqual(t, uint32(0), res.TxResponse.Code)

	// Make sure the chain is still producing blocks
	err = utils.WaitForBlocks(harness.Ctx, harness.Chain.EthClient, 5)
	require.NoError(t, err)
}
