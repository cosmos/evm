package eip712

import (
	"bytes"
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/stretchr/testify/suite"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/ethereum/eip712"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distributiontypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// Unit tests for single-signer EIP-712 signature verification. Multi-signature key verification tests are out-of-scope
// here and included with the ante_tests.

const (
	msgsFieldName = "msgs"
)

type TestSuite struct {
	suite.Suite

	create                   network.CreateEvmApp
	options                  []network.ConfigOption
	config                   sdktestutil.TestEncodingConfig
	clientCtx                client.Context
	useLegacyEIP712TypedData bool
	denom                    string
}

func NewTestSuite(create network.CreateEvmApp, useLegacyEIP712TypedData bool, options ...network.ConfigOption) *TestSuite {
	return &TestSuite{
		create:                   create,
		useLegacyEIP712TypedData: useLegacyEIP712TypedData,
		options:                  options,
	}
}

type TestParams struct {
	fee           txtypes.Fee
	address       sdk.AccAddress
	accountNumber uint64
	sequence      uint64
	memo          string
}

func (s *TestSuite) SetupTest() {
	nw := network.New(s.create, s.options...)
	s.config = nw.GetEncodingConfig()
	s.clientCtx = client.Context{}.WithTxConfig(s.config.TxConfig)
	s.denom = evmtypes.GetEVMCoinDenom()

	sdk.GetConfig().SetBech32PrefixForAccount("cosmos", "")
}

// createTestAddress creates random test addresses for messages
func (s *TestSuite) createTestAddress() sdk.AccAddress {
	privkey, _ := ethsecp256k1.GenerateKey()
	key, err := privkey.ToECDSA()
	s.Require().NoError(err)

	addr := crypto.PubkeyToAddress(key.PublicKey)

	return addr.Bytes()
}

// createTestKeyPair creates a random keypair for signing and verification
func (s *TestSuite) createTestKeyPair() (*ethsecp256k1.PrivKey, *ethsecp256k1.PubKey) {
	privKey, err := ethsecp256k1.GenerateKey()
	s.Require().NoError(err)

	pubKey := &ethsecp256k1.PubKey{
		Key: privKey.PubKey().Bytes(),
	}
	s.Require().Implements((*cryptotypes.PubKey)(nil), pubKey)

	return privKey, pubKey
}

// makeCoins helps create an instance of sdk.Coins[] with single coin
func (s *TestSuite) makeCoins(denom string, amount math.Int) sdk.Coins {
	return sdk.NewCoins(
		sdk.NewCoin(
			denom,
			amount,
		),
	)
}

func (s *TestSuite) TestEIP712() {
	s.SetupTest()

	signModes := []signing.SignMode{
		signing.SignMode_SIGN_MODE_DIRECT,
		signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
	}

	params := TestParams{
		fee: txtypes.Fee{
			Amount:   s.makeCoins(s.denom, math.NewInt(2000)),
			GasLimit: 20000,
		},
		address:       s.createTestAddress(),
		accountNumber: 25,
		sequence:      78,
		memo:          "",
	}

	testCases := []struct {
		title         string
		chainID       string
		msgs          []sdk.Msg
		timeoutHeight uint64
		expectSuccess bool
	}{
		{
			title: "Succeeds - Standard MsgSend",
			msgs: []sdk.Msg{
				banktypes.NewMsgSend(
					s.createTestAddress(),
					s.createTestAddress(),
					s.makeCoins(s.denom, math.NewInt(1)),
				),
			},
			expectSuccess: true,
		},
		{
			title: "Succeeds - Standard MsgVote",
			msgs: []sdk.Msg{
				govtypes.NewMsgVote(
					s.createTestAddress(),
					5,
					govtypes.OptionNo,
				),
			},
			expectSuccess: true,
		},
		{
			title: "Succeeds - Standard MsgDelegate",
			msgs: []sdk.Msg{
				stakingtypes.NewMsgDelegate(
					s.createTestAddress().String(),
					sdk.ValAddress(s.createTestAddress()).String(),
					s.makeCoins(s.denom, math.NewInt(1))[0],
				),
			},
			expectSuccess: true,
		},
		{
			title: "Succeeds - Standard MsgWithdrawDelegationReward",
			msgs: []sdk.Msg{
				distributiontypes.NewMsgWithdrawDelegatorReward(
					s.createTestAddress().String(),
					sdk.ValAddress(s.createTestAddress()).String(),
				),
			},
			expectSuccess: true,
		},
		{
			title: "Succeeds - Two Single-Signer MsgDelegate",
			msgs: []sdk.Msg{
				stakingtypes.NewMsgDelegate(
					params.address.String(),
					sdk.ValAddress(s.createTestAddress()).String(),
					s.makeCoins(s.denom, math.NewInt(1))[0],
				),
				stakingtypes.NewMsgDelegate(
					params.address.String(),
					sdk.ValAddress(s.createTestAddress()).String(),
					s.makeCoins(s.denom, math.NewInt(5))[0],
				),
			},
			expectSuccess: true,
		},
		{
			title: "Succeeds - Single-Signer MsgVote V1 with Omitted Value",
			msgs: []sdk.Msg{
				govtypesv1.NewMsgVote(
					params.address,
					5,
					govtypesv1.VoteOption_VOTE_OPTION_NO,
					"",
				),
			},
			expectSuccess: true,
		},
		{
			title: "Succeeds - Single-Signer MsgSend + MsgVote",
			msgs: []sdk.Msg{
				govtypes.NewMsgVote(
					params.address,
					5,
					govtypes.OptionNo,
				),
				banktypes.NewMsgSend(
					params.address,
					s.createTestAddress(),
					s.makeCoins(s.denom, math.NewInt(50)),
				),
			},
			expectSuccess: !s.useLegacyEIP712TypedData,
		},
		{
			title: "Succeeds - Single-Signer 2x MsgVoteV1 with Different Schemas",
			msgs: []sdk.Msg{
				govtypesv1.NewMsgVote(
					params.address,
					5,
					govtypesv1.VoteOption_VOTE_OPTION_NO,
					"",
				),
				govtypesv1.NewMsgVote(
					params.address,
					10,
					govtypesv1.VoteOption_VOTE_OPTION_YES,
					"Has Metadata",
				),
			},
			expectSuccess: !s.useLegacyEIP712TypedData,
		},
		{
			title: "Fails - Two MsgVotes with Different Signers",
			msgs: []sdk.Msg{
				govtypes.NewMsgVote(
					s.createTestAddress(),
					5,
					govtypes.OptionNo,
				),
				govtypes.NewMsgVote(
					s.createTestAddress(),
					25,
					govtypes.OptionAbstain,
				),
			},
			expectSuccess: false,
		},
		{
			title:         "Fails - Empty Transaction",
			msgs:          []sdk.Msg{},
			expectSuccess: false,
		},
		{
			title: "Fails - Includes TimeoutHeight",
			msgs: []sdk.Msg{
				govtypes.NewMsgVote(
					s.createTestAddress(),
					5,
					govtypes.OptionNo,
				),
			},
			timeoutHeight: 1000,
			expectSuccess: false,
		},
		{
			title: "Fails - Single Message / Multi-Signer",
			msgs: []sdk.Msg{
				&banktypes.MsgMultiSend{
					Inputs: []banktypes.Input{
						banktypes.NewInput(
							s.createTestAddress(),
							s.makeCoins(s.denom, math.NewInt(50)),
						),
						banktypes.NewInput(
							s.createTestAddress(),
							s.makeCoins(s.denom, math.NewInt(50)),
						),
					},
					Outputs: []banktypes.Output{
						banktypes.NewOutput(
							s.createTestAddress(),
							s.makeCoins(s.denom, math.NewInt(50)),
						),
						banktypes.NewOutput(
							s.createTestAddress(),
							s.makeCoins(s.denom, math.NewInt(50)),
						),
					},
				},
			},
			expectSuccess: false,
		},
	}

	for _, tc := range testCases {
		for _, signMode := range signModes {
			s.Run(tc.title, func() {
				privKey, pubKey := s.createTestKeyPair()

				txBuilder := s.clientCtx.TxConfig.NewTxBuilder()

				txBuilder.SetGasLimit(params.fee.GasLimit)
				txBuilder.SetFeeAmount(params.fee.Amount)

				err := txBuilder.SetMsgs(tc.msgs...)
				s.Require().NoError(err)

				txBuilder.SetMemo(params.memo)

				// Prepare signature field with empty signatures
				txSigData := signing.SingleSignatureData{
					SignMode:  signMode,
					Signature: nil,
				}
				txSig := signing.SignatureV2{
					PubKey:   pubKey,
					Data:     &txSigData,
					Sequence: params.sequence,
				}

				err = txBuilder.SetSignatures([]signing.SignatureV2{txSig}...)
				s.Require().NoError(err)

				chainID := constants.ExampleChainID.ChainID
				if tc.chainID != "" {
					chainID = tc.chainID
				}

				if tc.timeoutHeight != 0 {
					txBuilder.SetTimeoutHeight(tc.timeoutHeight)
				}

				signerData := authsigning.SignerData{
					ChainID:       chainID,
					AccountNumber: params.accountNumber,
					Sequence:      params.sequence,
					PubKey:        pubKey,
					Address:       sdk.MustBech32ifyAddressBytes(constants.ExampleBech32Prefix, pubKey.Bytes()),
				}

				bz, err := authsigning.GetSignBytesAdapter(
					s.clientCtx.CmdContext,
					s.clientCtx.TxConfig.SignModeHandler(),
					signMode,
					signerData,
					txBuilder.GetTx(),
				)
				s.Require().NoError(err)

				s.verifyEIP712SignatureVerification(tc.expectSuccess, *privKey, *pubKey, bz)

				// Verify payload flattening only if the payload is in valid JSON format
				if signMode == signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON {
					s.verifySignDocFlattening(bz)

					if tc.expectSuccess {
						s.verifyBasicTypedData(bz)
					}
				}
			})
		}
	}
}

// verifyEIP712SignatureVerification verifies that the payload passes signature verification if signed as its EIP-712 representation.
func (s *TestSuite) verifyEIP712SignatureVerification(expectedSuccess bool, privKey ethsecp256k1.PrivKey, pubKey ethsecp256k1.PubKey, signBytes []byte) {
	eip712Bytes, err := eip712.GetEIP712BytesForMsg(signBytes)

	if s.useLegacyEIP712TypedData {
		eip712Bytes, err = eip712.LegacyGetEIP712BytesForMsg(signBytes)
	}

	if !expectedSuccess {
		s.Require().Error(err)
		return
	}

	s.Require().NoError(err)

	sig, err := privKey.Sign(eip712Bytes)
	s.Require().NoError(err)

	// Verify against original payload bytes. This should pass, even though it is not
	// the original message that was signed.
	res := pubKey.VerifySignature(signBytes, sig)
	s.Require().True(res)

	// Verify against the signed EIP-712 bytes. This should pass, since it is the message signed.
	res = pubKey.VerifySignature(eip712Bytes, sig)
	s.Require().True(res)

	// Verify against random bytes to ensure it does not pass unexpectedly (sanity check).
	randBytes := make([]byte, len(signBytes))
	copy(randBytes, signBytes)
	// Change the first element of signBytes to a different value
	randBytes[0] = (signBytes[0] + 10) % 255
	res = pubKey.VerifySignature(randBytes, sig)
	s.Require().False(res)
}

// verifySignDocFlattening tests the flattening algorithm against the sign doc's JSON payload,
// using verifyPayloadAgainstFlattened.
func (s *TestSuite) verifySignDocFlattening(signDoc []byte) {
	payload := gjson.ParseBytes(signDoc)
	s.Require().True(payload.IsObject())

	flattened, _, err := eip712.FlattenPayloadMessages(payload)
	s.Require().NoError(err)

	s.verifyPayloadAgainstFlattened(payload, flattened)
}

// verifyPayloadAgainstFlattened compares a payload against its flattened counterpart to ensure that
// the flattening algorithm behaved as expected.
func (s *TestSuite) verifyPayloadAgainstFlattened(payload gjson.Result, flattened gjson.Result) {
	payloadMap, ok := payload.Value().(map[string]interface{})
	s.Require().True(ok)
	flattenedMap, ok := flattened.Value().(map[string]interface{})
	s.Require().True(ok)

	s.verifyPayloadMapAgainstFlattenedMap(payloadMap, flattenedMap)
}

// verifyPayloadMapAgainstFlattenedMap directly compares two JSON maps in Go representations to
// test flattening.
func (s *TestSuite) verifyPayloadMapAgainstFlattenedMap(original map[string]interface{}, flattened map[string]interface{}) {
	interfaceMessages, ok := original[msgsFieldName]
	s.Require().True(ok)

	messages, ok := interfaceMessages.([]interface{})
	// If passing an empty msgs array
	// the interfaceMessages is nil
	// in that case, don't try to iterate the messages
	if ok {
		// Verify message contents
		for i, msg := range messages {
			flattenedMsg, ok := flattened[fmt.Sprintf("msg%d", i)]
			s.Require().True(ok)

			flattenedMsgJSON, ok := flattenedMsg.(map[string]interface{})
			s.Require().True(ok)

			s.Require().Equal(flattenedMsgJSON, msg)
		}
	}

	// Verify new payload does not have msgs field
	_, ok = flattened[msgsFieldName]
	s.Require().False(ok)

	// Verify number of total keys
	numKeysOriginal := len(original)
	numKeysFlattened := len(flattened)
	numMessages := len(messages)

	// + N keys, then -1 for msgs
	s.Require().Equal(numKeysFlattened, numKeysOriginal+numMessages-1)

	// Verify contents of remaining keys
	for k, obj := range original {
		if k == msgsFieldName {
			continue
		}

		flattenedObj, ok := flattened[k]
		s.Require().True(ok)

		s.Require().Equal(obj, flattenedObj)
	}
}

// verifyBasicTypedData performs basic verification on the TypedData generation.
func (s *TestSuite) verifyBasicTypedData(signDoc []byte) {
	typedData, err := eip712.GetEIP712TypedDataForMsg(signDoc)

	s.Require().NoError(err)

	jsonPayload := gjson.ParseBytes(signDoc)
	s.Require().True(jsonPayload.IsObject())

	flattened, _, err := eip712.FlattenPayloadMessages(jsonPayload)
	s.Require().NoError(err)
	s.Require().True(flattened.IsObject())

	flattenedMsgMap, ok := flattened.Value().(map[string]interface{})
	s.Require().True(ok)

	s.Require().Equal(typedData.Message, flattenedMsgMap)
}

// TestFlattenPayloadErrorHandling tests error handling in TypedData generation,
// specifically regarding the payload.
func (s *TestSuite) TestFlattenPayloadErrorHandling() {
	// No msgs
	_, _, err := eip712.FlattenPayloadMessages(gjson.Parse(""))
	s.Require().ErrorContains(err, "no messages found")

	// Non-array Msgs
	_, _, err = eip712.FlattenPayloadMessages(gjson.Parse(`{"msgs": 10}`))
	s.Require().ErrorContains(err, "array of messages")

	// Array with non-object items
	_, _, err = eip712.FlattenPayloadMessages(gjson.Parse(`{"msgs": [10, 20]}`))
	s.Require().ErrorContains(err, "not valid JSON")

	// Malformed payload
	malformed, err := sjson.Set(s.generateRandomPayload(2).Raw, "msg0", 20)
	s.Require().NoError(err)
	_, _, err = eip712.FlattenPayloadMessages(gjson.Parse(malformed))
	s.Require().ErrorContains(err, "malformed payload")
}

// TestTypedDataErrorHandling tests error handling for TypedData generation
// in the main algorithm.
func (s *TestSuite) TestTypedDataErrorHandling() {
	// Empty JSON
	_, err := eip712.WrapTxToTypedData(0, make([]byte, 0))
	s.Require().ErrorContains(err, "invalid JSON")

	_, err = eip712.WrapTxToTypedData(0, []byte(gjson.Parse(`{"msgs": 10}`).Raw))
	s.Require().ErrorContains(err, "array of messages")

	// Invalid message 'type'
	_, err = eip712.WrapTxToTypedData(0, []byte(gjson.Parse(`{"msgs": [{ "type": 10 }] }`).Raw))
	s.Require().ErrorContains(err, "message type value")

	// Max duplicate type recursion depth
	messagesArr := new(bytes.Buffer)
	maxRecursionDepth := 1001

	messagesArr.WriteString("[")
	for i := 0; i < maxRecursionDepth; i++ {
		fmt.Fprintf(messagesArr, `{ "type": "msgType", "value": { "field%v": 10 } }`, i)
		if i != maxRecursionDepth-1 {
			messagesArr.WriteString(",")
		}
	}
	messagesArr.WriteString("]")

	_, err = eip712.WrapTxToTypedData(0, []byte(fmt.Sprintf(`{ "msgs": %v }`, messagesArr)))
	s.Require().ErrorContains(err, "maximum number of duplicates")
}

// TestTypedDataEdgeCases tests certain interesting edge cases to ensure that they work
// (or don't work) as expected.
func (s *TestSuite) TestTypedDataEdgeCases() {
	// Type without '/' separator
	typedData, err := eip712.WrapTxToTypedData(0, []byte(gjson.Parse(`{"msgs": [{ "type": "MsgSend", "value": { "field": 10 } }] }`).Raw))
	s.Require().NoError(err)
	types := typedData.Types["TypeMsgSend0"]
	s.Require().Greater(len(types), 0)

	// Null value
	typedData, err = eip712.WrapTxToTypedData(0, []byte(gjson.Parse(`{"msgs": [{ "type": "MsgSend", "value": { "field": null } }] }`).Raw))
	s.Require().NoError(err)
	types = typedData.Types["TypeValue0"]
	// Skip null type, since we don't expect any in the payload
	s.Require().Equal(len(types), 0)

	// Boolean value
	typedData, err = eip712.WrapTxToTypedData(0, []byte(gjson.Parse(`{"msgs": [{ "type": "MsgSend", "value": { "field": true } }] }`).Raw))
	s.Require().NoError(err)
	types = typedData.Types["TypeValue0"]
	s.Require().Equal(len(types), 1)
	s.Require().Equal(types[0], apitypes.Type{
		Name: "field",
		Type: "bool",
	})

	// Empty array
	typedData, err = eip712.WrapTxToTypedData(0, []byte(gjson.Parse(`{"msgs": [{ "type": "MsgSend", "value": { "field": [] } }] }`).Raw))
	s.Require().NoError(err)
	types = typedData.Types["TypeValue0"]
	s.Require().Equal(types[0], apitypes.Type{
		Name: "field",
		Type: "string[]",
	})

	// Simple arrays
	typedData, err = eip712.WrapTxToTypedData(0, []byte(gjson.Parse(`{"msgs": [{ "type": "MsgSend", "value": { "array": [1, 2, 3] } }] }`).Raw))
	s.Require().NoError(err)
	types = typedData.Types["TypeValue0"]
	s.Require().Equal(len(types), 1)
	s.Require().Equal(types[0], apitypes.Type{
		Name: "array",
		Type: "int64[]",
	})

	// Nested arrays (EIP-712 does not support nested arrays)
	typedData, err = eip712.WrapTxToTypedData(0, []byte(gjson.Parse(`{"msgs": [{ "type": "MsgSend", "value": { "array": [[1, 2, 3], [1, 2]] } }] }`).Raw))
	s.Require().NoError(err)
	types = typedData.Types["TypeValue0"]
	s.Require().Equal(len(types), 0)
}

// TestTypedDataGeneration tests certain qualities about the output Types representation.
func (s *TestSuite) TestTypedDataGeneration() {
	// Multiple messages with the same schema should share one type
	payloadRaw := `{ "msgs": [{ "type": "msgType", "value": { "field1": 10 }}, { "type": "msgType", "value": { "field1": 20 }}] }`

	typedData, err := eip712.WrapTxToTypedData(0, []byte(payloadRaw))
	s.Require().NoError(err)
	s.Require().True(typedData.Types["TypemsgType1"] == nil)

	// Multiple messages with different schemas should have different types
	payloadRaw = `{ "msgs": [{ "type": "msgType", "value": { "field1": 10 }}, { "type": "msgType", "value": { "field2": 20 }}] }`

	typedData, err = eip712.WrapTxToTypedData(0, []byte(payloadRaw))
	s.Require().NoError(err)
	s.Require().False(typedData.Types["TypemsgType1"] == nil)
}
