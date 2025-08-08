package p256

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cometbft/cometbft/crypto"

	"github.com/cosmos/evm/precompiles/p256"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

var trueValue = common.LeftPadBytes(common.Big1.Bytes(), 32)

func (s *PrecompileTestSuite) TestAddress() {
	s.Require().Equal(evmtypes.P256PrecompileAddress, s.precompile.Address().String())
}

func (s *PrecompileTestSuite) TestRequiredGas() {
	s.Require().Equal(p256.VerifyGas, s.precompile.RequiredGas(nil))
}

func (s *PrecompileTestSuite) TestRun() {
	testCases := []struct {
		name    string
		sign    func() []byte
		expPass bool
	}{
		{
			"pass - Sign",
			func() []byte {
				msg := []byte("hello world")
				hash := crypto.Sha256(msg)

				rInt, sInt, err := ecdsa.Sign(rand.Reader, s.p256Priv, hash)
				s.Require().NoError(err)

				input := make([]byte, p256.VerifyInputLength)
				copy(input[0:32], hash)
				rInt.FillBytes(input[32:64])
				sInt.FillBytes(input[64:96])
				s.p256Priv.X.FillBytes(input[96:128])
				s.p256Priv.Y.FillBytes(input[128:160])

				return input
			},
			true,
		},
		{
			"pass - sign ASN.1 encoded signature",
			func() []byte {
				msg := []byte("hello world")
				hash := crypto.Sha256(msg)

				sig, err := ecdsa.SignASN1(rand.Reader, s.p256Priv, hash)
				s.Require().NoError(err)

				rBz, sBz, err := parseSignature(sig)
				rInt, sInt := new(big.Int).SetBytes(rBz), new(big.Int).SetBytes(sBz)
				s.Require().NoError(err)

				input := make([]byte, p256.VerifyInputLength)
				copy(input[0:32], hash)
				rInt.FillBytes(input[32:64])
				sInt.FillBytes(input[64:96])
				s.p256Priv.X.FillBytes(input[96:128])
				s.p256Priv.Y.FillBytes(input[128:160])

				return input
			},
			true,
		},
		{
			"fail - invalid signature",
			func() []byte {
				privB, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
				s.Require().NoError(err)

				bz := elliptic.MarshalCompressed(elliptic.P256(), s.p256Priv.X, s.p256Priv.Y)
				s.Require().NotEmpty(bz)

				msg := []byte("hello world")
				hash := crypto.Sha256(msg)

				rInt, sInt, err := ecdsa.Sign(rand.Reader, s.p256Priv, hash)
				s.Require().NoError(err)

				input := make([]byte, p256.VerifyInputLength)
				copy(input[0:32], hash)
				rInt.FillBytes(input[32:64])
				sInt.FillBytes(input[64:96])
				privB.X.FillBytes(input[96:128])
				privB.Y.FillBytes(input[128:160])

				return input
			},
			false,
		},
		{
			"fail - invalid length",
			func() []byte {
				msg := []byte("hello world")
				hash := crypto.Sha256(msg)

				input := make([]byte, 32)
				copy(input[0:32], hash)

				return input
			},
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			input := tc.sign()
			bz, err := s.precompile.Run(nil, &vm.Contract{Input: input}, false)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(trueValue, bz)
			} else {
				s.Require().NoError(err)
				s.Require().Empty(bz)
			}
		})
	}
}
