package p256

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"errors"

	"github.com/stretchr/testify/suite"
	"golang.org/x/crypto/cryptobyte"
	"golang.org/x/crypto/cryptobyte/asn1"

	"github.com/cometbft/cometbft/crypto"

	"github.com/cosmos/evm/precompiles/p256"
	"github.com/cosmos/evm/testutil/integration/evm/network"
)

type PrecompileTestSuite struct {
	suite.Suite

	create     network.CreateEvmApp
	p256Priv   *ecdsa.PrivateKey
	precompile *p256.Precompile
}

func NewPrecompileTestSuite(create network.CreateEvmApp) *PrecompileTestSuite {
	return &PrecompileTestSuite{
		create:     create,
		precompile: &p256.Precompile{},
	}
}

func (s *PrecompileTestSuite) SetupTest() {
	p256Priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	s.Require().NoError(err)
	s.p256Priv = p256Priv
	s.precompile = &p256.Precompile{}
}

func signMsg(msg []byte, priv *ecdsa.PrivateKey) ([]byte, error) {
	hash := crypto.Sha256(msg)

	rInt, sInt, err := ecdsa.Sign(rand.Reader, priv, hash)
	if err != nil {
		return nil, err
	}

	input := make([]byte, p256.VerifyInputLength)
	copy(input[0:32], hash)
	rInt.FillBytes(input[32:64])
	sInt.FillBytes(input[64:96])
	priv.X.FillBytes(input[96:128])
	priv.Y.FillBytes(input[128:160])

	return input, nil
}

func parseSignature(sig []byte) (r, s []byte, err error) {
	var inner cryptobyte.String
	input := cryptobyte.String(sig)
	if !input.ReadASN1(&inner, asn1.SEQUENCE) ||
		!input.Empty() ||
		!inner.ReadASN1Integer(&r) ||
		!inner.ReadASN1Integer(&s) ||
		!inner.Empty() {
		return nil, nil, errors.New("invalid ASN.1")
	}
	return r, s, nil
}
