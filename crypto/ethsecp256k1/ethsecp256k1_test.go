package ethsecp256k1

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/crypto/secp256k1"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

func TestPrivKey(t *testing.T) {
	// validate type and equality
	privKey, err := GenerateKey()
	require.NoError(t, err)
	require.Implements(t, (*cryptotypes.PrivKey)(nil), privKey)

	// validate inequality
	privKey2, err := GenerateKey()
	require.NoError(t, err)
	require.False(t, privKey.Equals(privKey2))

	// validate Ethereum address equality
	addr := privKey.PubKey().Address()
	key, err := privKey.ToECDSA()
	require.NoError(t, err)
	expectedAddr := crypto.PubkeyToAddress(key.PublicKey)
	require.Equal(t, expectedAddr.Bytes(), addr.Bytes())

	// validate we can sign some bytes
	msg := []byte("hello world")
	sigHash := crypto.Keccak256Hash(msg)
	expectedSig, err := secp256k1.Sign(sigHash.Bytes(), privKey.Bytes())
	require.NoError(t, err)

	sig, err := privKey.Sign(sigHash.Bytes())
	require.NoError(t, err)
	require.Equal(t, expectedSig, sig)
}

func TestPrivKey_PubKey(t *testing.T) {
	privKey, err := GenerateKey()
	require.NoError(t, err)

	// validate type and equality
	pubKey := &PubKey{
		Key: privKey.PubKey().Bytes(),
	}
	require.Implements(t, (*cryptotypes.PubKey)(nil), pubKey)

	// validate inequality
	privKey2, err := GenerateKey()
	require.NoError(t, err)
	require.False(t, pubKey.Equals(privKey2.PubKey()))

	// validate signature
	msg := []byte("hello world")
	sigHash := crypto.Keccak256Hash(msg)
	sig, err := privKey.Sign(sigHash.Bytes())
	require.NoError(t, err)

	res := pubKey.VerifySignature(msg, sig)
	require.True(t, res)
}

func TestMarshalAmino(t *testing.T) {
	aminoCdc := codec.NewLegacyAmino()
	privKey, err := GenerateKey()
	require.NoError(t, err)

	pubKey := privKey.PubKey().(*PubKey)

	testCases := []struct {
		desc      string
		msg       codec.AminoMarshaler
		typ       interface{}
		expBinary []byte
		expJSON   string
	}{
		{
			"ethsecp256k1 private key",
			privKey,
			&PrivKey{},
			append([]byte{32}, privKey.Bytes()...), // Length-prefixed.
			"\"" + base64.StdEncoding.EncodeToString(privKey.Bytes()) + "\"",
		},
		{
			"ethsecp256k1 public key",
			pubKey,
			&PubKey{},
			append([]byte{33}, pubKey.Bytes()...), // Length-prefixed.
			"\"" + base64.StdEncoding.EncodeToString(pubKey.Bytes()) + "\"",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.desc, func(t *testing.T) {
			// Do a round trip of encoding/decoding binary.
			bz, err := aminoCdc.Marshal(tc.msg)
			require.NoError(t, err)
			require.Equal(t, tc.expBinary, bz)

			err = aminoCdc.Unmarshal(bz, tc.typ)
			require.NoError(t, err)

			require.Equal(t, tc.msg, tc.typ)

			// Do a round trip of encoding/decoding JSON.
			bz, err = aminoCdc.MarshalJSON(tc.msg)
			require.NoError(t, err)
			require.Equal(t, tc.expJSON, string(bz))

			err = aminoCdc.UnmarshalJSON(bz, tc.typ)
			require.NoError(t, err)

			require.Equal(t, tc.msg, tc.typ)
		})
	}
}

func TestStripExtraAminoFields(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected interface{} // nil if we expect nil return, otherwise the parsed JSON to compare
	}{
		{
			name:     "not JSON returns nil",
			input:    "not json at all",
			expected: nil,
		},
		{
			name:     "no msgs field returns nil",
			input:    `{"account_number":"220","chain_id":"epix_1916-1","fee":{"amount":[],"gas":"200000"}}`,
			expected: nil,
		},
		{
			name:     "empty msgs array returns nil",
			input:    `{"msgs":[],"fee":{}}`,
			expected: nil,
		},
		{
			name:     "msgs without extra fields returns nil",
			input:    `{"msgs":[{"type":"cosmos-sdk/MsgSend","value":{"from":"addr1","to":"addr2","amount":"100"}}]}`,
			expected: nil,
		},
		{
			name: "strips use_aliasing from IBC transfer msg",
			input: `{"account_number":"220","chain_id":"epix_1916-1","fee":{"amount":[{"amount":"2500","denom":"aepix"}],"gas":"200000"},"memo":"","msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"source_port":"transfer","source_channel":"channel-0","token":{"denom":"aepix","amount":"5000000000000000000000"},"sender":"epix133...","receiver":"osmo1w5g...","timeout_height":{"revision_number":"1","revision_height":"99999999"},"use_aliasing":false,"encoding":""}}],"sequence":"21"}`,
			expected: `{"account_number":"220","chain_id":"epix_1916-1","fee":{"amount":[{"amount":"2500","denom":"aepix"}],"gas":"200000"},"memo":"","msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"receiver":"osmo1w5g...","sender":"epix133...","source_channel":"channel-0","source_port":"transfer","timeout_height":{"revision_height":"99999999","revision_number":"1"},"token":{"amount":"5000000000000000000000","denom":"aepix"}}}],"sequence":"21"}`,
		},
		{
			name: "strips only use_aliasing when encoding is absent",
			input: `{"msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"source_port":"transfer","use_aliasing":false}}]}`,
			expected: `{"msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"source_port":"transfer"}}]}`,
		},
		{
			name: "strips only encoding when use_aliasing is absent",
			input: `{"msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"source_port":"transfer","encoding":""}}]}`,
			expected: `{"msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"source_port":"transfer"}}]}`,
		},
		{
			name: "strips fields from multiple messages",
			input: `{"msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"source_port":"transfer","use_aliasing":false}},{"type":"cosmos-sdk/MsgTransfer","value":{"source_port":"transfer","encoding":"","use_aliasing":true}}]}`,
			expected: `{"msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"source_port":"transfer"}},{"type":"cosmos-sdk/MsgTransfer","value":{"source_port":"transfer"}}]}`,
		},
		{
			name:     "msg without value field is skipped",
			input:    `{"msgs":[{"type":"cosmos-sdk/MsgTransfer"}]}`,
			expected: nil,
		},
		{
			name: "preserves numeric precision",
			input: `{"account_number":"220","msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"amount":"99999999999999999999","use_aliasing":false}}],"sequence":"21"}`,
			expected: `{"account_number":"220","msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"amount":"99999999999999999999"}}],"sequence":"21"}`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := stripExtraAminoFields([]byte(tc.input))
			if tc.expected == nil {
				require.Nil(t, result)
			} else {
				require.NotNil(t, result)
				// Compare as parsed JSON to avoid whitespace/ordering issues
				var got, want interface{}
				require.NoError(t, json.Unmarshal(result, &got))
				require.NoError(t, json.Unmarshal([]byte(tc.expected.(string)), &want))
				require.Equal(t, want, got)
			}
		})
	}
}

func TestVerifySignature_WithStrippedIBCFields(t *testing.T) {
	privKey, err := GenerateKey()
	require.NoError(t, err)
	pubKey := &PubKey{Key: privKey.PubKey().Bytes()}

	// Simulate what Keplr signs: an amino sign doc WITHOUT the extra IBC fields
	keplrSignDoc := `{"account_number":"220","chain_id":"epix_1916-1","fee":{"amount":[{"amount":"2500","denom":"aepix"}],"gas":"200000"},"memo":"","msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"receiver":"osmo1w5gvwq6s8mycgc80h5urhlqs5esjq3n7tdgyk4","sender":"epix133geakf970neekfr8j6h68gaf08fj3srwl04lk","source_channel":"channel-0","source_port":"transfer","timeout_height":{"revision_height":"99999999","revision_number":"1"},"token":{"amount":"5000000000000000000000","denom":"aepix"}}}],"sequence":"21"}`

	// Sign the Keplr version (without extra fields)
	sigHash := crypto.Keccak256Hash([]byte(keplrSignDoc))
	sig, err := privKey.Sign(sigHash.Bytes())
	require.NoError(t, err)

	// Chain constructs sign doc WITH extra IBC fields (use_aliasing, encoding)
	chainSignDoc := `{"account_number":"220","chain_id":"epix_1916-1","fee":{"amount":[{"amount":"2500","denom":"aepix"}],"gas":"200000"},"memo":"","msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"encoding":"","receiver":"osmo1w5gvwq6s8mycgc80h5urhlqs5esjq3n7tdgyk4","sender":"epix133geakf970neekfr8j6h68gaf08fj3srwl04lk","source_channel":"channel-0","source_port":"transfer","timeout_height":{"revision_height":"99999999","revision_number":"1"},"token":{"amount":"5000000000000000000000","denom":"aepix"},"use_aliasing":false}}],"sequence":"21"}`

	// Verification against the chain's sign doc (with extra fields) should succeed
	// because VerifySignature will strip the extra fields and retry
	require.True(t, pubKey.VerifySignature([]byte(chainSignDoc), sig),
		"signature signed by Keplr (without extra IBC fields) should verify against chain sign doc (with extra IBC fields)")

	// Verification against the original Keplr sign doc should also succeed directly
	require.True(t, pubKey.VerifySignature([]byte(keplrSignDoc), sig),
		"signature should verify directly against the original Keplr sign doc")
}

func TestVerifySignature_NonIBCUnaffected(t *testing.T) {
	privKey, err := GenerateKey()
	require.NoError(t, err)
	pubKey := &PubKey{Key: privKey.PubKey().Bytes()}

	// A standard non-IBC message (no extra fields to strip)
	msg := []byte(`{"account_number":"1","chain_id":"epix_1916-1","fee":{"amount":[],"gas":"200000"},"memo":"","msgs":[{"type":"cosmos-sdk/MsgSend","value":{"from_address":"epix1...","to_address":"epix2...","amount":[{"denom":"aepix","amount":"1000"}]}}],"sequence":"0"}`)

	sigHash := crypto.Keccak256Hash(msg)
	sig, err := privKey.Sign(sigHash.Bytes())
	require.NoError(t, err)

	require.True(t, pubKey.VerifySignature(msg, sig),
		"non-IBC messages should continue to verify normally")
}

func TestVerifySignature_InvalidSigStillFails(t *testing.T) {
	privKey, err := GenerateKey()
	require.NoError(t, err)
	pubKey := &PubKey{Key: privKey.PubKey().Bytes()}

	// Sign doc with extra IBC fields
	chainSignDoc := `{"account_number":"220","chain_id":"epix_1916-1","fee":{"amount":[],"gas":"200000"},"memo":"","msgs":[{"type":"cosmos-sdk/MsgTransfer","value":{"source_port":"transfer","use_aliasing":false,"encoding":""}}],"sequence":"21"}`

	// Sign a completely different message
	differentMsg := []byte("completely different message")
	sigHash := crypto.Keccak256Hash(differentMsg)
	sig, err := privKey.Sign(sigHash.Bytes())
	require.NoError(t, err)

	// Should fail even with field stripping, because the signature doesn't match
	require.False(t, pubKey.VerifySignature([]byte(chainSignDoc), sig),
		"invalid signature should still fail verification even with field stripping")
}
