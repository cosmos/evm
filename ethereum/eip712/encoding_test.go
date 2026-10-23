package eip712_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/reflect/protoregistry"

	"github.com/cosmos/evm/ethereum/eip712"

	"cosmossdk.io/math"
	txsigning "cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/address"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// buildProtobufSignDocBytes builds a SIGN_MODE_DIRECT SignDoc with the given
// fee granter and payer, encoded the same way clients and the verifier do.
func buildProtobufSignDocBytes(t *testing.T, granter, payer string, body *txtypes.TxBody) []byte {
	t.Helper()

	registry, err := codectypes.NewInterfaceRegistryWithOptions(codectypes.InterfaceRegistryOptions{
		ProtoFiles: protoregistry.GlobalFiles,
		SigningOptions: txsigning.Options{
			AddressCodec:          address.NewBech32Codec(sdk.Bech32PrefixAccAddr),
			ValidatorAddressCodec: address.NewBech32Codec(sdk.Bech32PrefixValAddr),
		},
	})
	require.NoError(t, err)
	cryptocodec.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	eip712.SetEncodingConfig(codec.NewLegacyAmino(), registry, 9000)

	priv := secp256k1.GenPrivKey()
	addr := sdk.AccAddress(priv.PubKey().Address())

	msgSend := banktypes.NewMsgSend(addr, addr, sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(1))))
	msgAny, err := codectypes.NewAnyWithValue(msgSend)
	require.NoError(t, err)

	if body == nil {
		body = &txtypes.TxBody{Messages: []*codectypes.Any{msgAny}}
	}

	bodyBz, err := body.Marshal()
	require.NoError(t, err)

	pubKeyAny, err := codectypes.NewAnyWithValue(priv.PubKey())
	require.NoError(t, err)

	authInfoBz, err := (&txtypes.AuthInfo{
		SignerInfos: []*txtypes.SignerInfo{{
			PublicKey: pubKeyAny,
			ModeInfo: &txtypes.ModeInfo{Sum: &txtypes.ModeInfo_Single_{
				Single: &txtypes.ModeInfo_Single{Mode: 1}, // SIGN_MODE_DIRECT
			}},
		}},
		Fee: &txtypes.Fee{
			Amount:   sdk.NewCoins(sdk.NewCoin(sdk.DefaultBondDenom, math.NewInt(1000))),
			GasLimit: 100000,
			Granter:  granter,
			Payer:    payer,
		},
	}).Marshal()
	require.NoError(t, err)

	signDocBz, err := (&txtypes.SignDoc{
		BodyBytes:     bodyBz,
		AuthInfoBytes: authInfoBz,
		ChainId:       "9001",
	}).Marshal()
	require.NoError(t, err)

	return signDocBz
}

func TestGetEIP712TypedDataForMsgRejectsUnsupportedFields(t *testing.T) {
	now := time.Now().UTC()
	testCases := []struct {
		name    string
		granter string
		payer   string
		body    *txtypes.TxBody
		legacy  bool
	}{
		{"fee granter set", "cosmos1wq1r..." + "granter", "", nil, false},
		{"fee payer set", "", "cosmos1wq1r..." + "payer", nil, false},
		{"timeout timestamp set", "", "", &txtypes.TxBody{TimeoutTimestamp: &now}, false},
		{"unordered", "", "", &txtypes.TxBody{Unordered: true}, false},
		{"legacy fee granter set", "cosmos1v9zc6ggvr9582t7m4sd6rdc7d9tlsx9w76w7kt", "", nil, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			signDocBz := buildProtobufSignDocBytes(t, tc.granter, tc.payer, tc.body)

			var err error
			if tc.legacy {
				_, err = eip712.LegacyGetEIP712TypedDataForMsg(signDocBz, 9000)
			} else {
				_, err = eip712.GetEIP712TypedDataForMsg(signDocBz)
			}
			require.Error(t, err)
			require.Contains(t, err.Error(), "body contains unsupported fields")
		})
	}
}
