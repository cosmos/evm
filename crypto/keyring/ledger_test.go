package keyring

import (
	"crypto/sha256"
	"testing"

	secp "github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/decred/dcrd/dcrec/secp256k1/v4/ecdsa"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/crypto/ethsecp256k1"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cosmosLedger "github.com/cosmos/cosmos-sdk/crypto/ledger"
	"github.com/cosmos/cosmos-sdk/crypto/types"
)

// fakeApp is a Ledger app that signs with an in-memory key.
type fakeApp struct {
	key  *secp.PrivateKey
	sign func(key *secp.PrivateKey, msg []byte) []byte
}

func (a *fakeApp) Close() error { return nil }

func (a *fakeApp) GetPublicKeySECP256K1([]uint32) ([]byte, error) {
	return a.key.PubKey().SerializeUncompressed(), nil
}

func (a *fakeApp) GetAddressPubKeySECP256K1(hdPath []uint32, _ string) ([]byte, string, error) {
	pubKey, err := a.GetPublicKeySECP256K1(hdPath)
	return pubKey, "", err
}

func (a *fakeApp) SignSECP256K1(_ []uint32, msg []byte, _ byte) ([]byte, error) {
	return a.sign(a.key, msg), nil
}

func TestLedgerRouter(t *testing.T) {
	key, err := secp.GeneratePrivateKey()
	require.NoError(t, err)
	ethApp := &fakeApp{key: key, sign: func(key *secp.PrivateKey, msg []byte) []byte {
		// R || S || V over the keccak hash
		sig, err := crypto.Sign(crypto.Keccak256(msg), key.ToECDSA())
		require.NoError(t, err)
		return sig
	}}
	cosmosApp := &fakeApp{key: key, sign: func(key *secp.PrivateKey, msg []byte) []byte {
		// DER over the sha256 hash
		hash := sha256.Sum256(msg)
		return ecdsa.Sign(key, hash[:]).Serialize()
	}}

	ethDerivation, cosmosDerivation := LedgerDerivation, cosmosLedgerDerivation
	t.Cleanup(func() { LedgerDerivation, cosmosLedgerDerivation = ethDerivation, cosmosDerivation })
	LedgerDerivation = func() (cosmosLedger.SECP256K1, error) { return ethApp, nil }
	cosmosLedgerDerivation = func() (cosmosLedger.SECP256K1, error) { return cosmosApp, nil }
	cosmosLedger.SetDiscoverLedger(DiscoverLedger)

	// switch apps back and forth in one process, like signing with keys of both
	for _, tc := range []struct {
		coinType uint32
		pubKey   types.PubKey
	}{
		{60, &ethsecp256k1.PubKey{}},
		{118, &secp256k1.PubKey{}},
		{60, &ethsecp256k1.PubKey{}},
	} {
		priv, err := cosmosLedger.NewPrivKeySecp256k1Unsafe(*hd.NewFundraiserParams(0, tc.coinType, 0))
		require.NoError(t, err)
		require.IsType(t, tc.pubKey, priv.PubKey())

		msg := []byte("sign doc")
		sig, err := priv.SignLedgerAminoJSON(msg)
		require.NoError(t, err)
		require.True(t, priv.PubKey().VerifySignature(msg, sig), "coin type %d", tc.coinType)
	}

	_, err = cosmosLedger.NewPrivKeySecp256k1Unsafe(*hd.NewFundraiserParams(0, 529, 0))
	require.ErrorContains(t, err, "unsupported coin type 529")

	// a hardened path reaches the app of its coin type too
	const hardened = 0x80000000
	cosmosApp.sign = func(*secp.PrivateKey, []byte) []byte { return []byte("cosmos") }
	sig, err := (&ledgerRouter{}).SignSECP256K1([]uint32{44 | hardened, 118 | hardened, hardened, 0, 0}, nil, 0)
	require.NoError(t, err)
	require.Equal(t, []byte("cosmos"), sig)
}
