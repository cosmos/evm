package erc20factory

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/erc20factory"
	utiltx "github.com/cosmos/evm/testutil/tx"
)

func (s *PrecompileTestSuite) TestEmitCreateEvent() {
	testcases := []struct {
		testName        string
		tokenAddress    common.Address
		tokenType       uint8
		salt            [32]uint8
		name            string
		symbol          string
		decimals        uint8
		minter          common.Address
		premintedSupply *big.Int
	}{
		{
			testName:        "pass",
			tokenAddress:    utiltx.GenerateAddress(),
			tokenType:       0,
			salt:            [32]uint8{0},
			name:            "Test",
			symbol:          "TEST",
			decimals:        18,
			minter:          utiltx.GenerateAddress(),
			premintedSupply: big.NewInt(1000000),
		},
	}

	for _, tc := range testcases {
		s.Run(tc.testName, func() {
			s.SetupTest()
			stateDB := s.network.GetStateDB()

			err := s.precompile.EmitCreateEvent(s.network.GetContext(), stateDB, tc.tokenAddress, tc.salt, tc.name, tc.symbol, tc.decimals, tc.minter, tc.premintedSupply)
			s.Require().NoError(err, "expected create event to be emitted successfully")

			log := stateDB.Logs()[0]
			s.Require().Equal(log.Address, s.precompile.Address())

			// Check event signature matches the one emitted
			event := s.precompile.ABI.Events[erc20factory.EventTypeCreate]
			s.Require().Equal(crypto.Keccak256Hash([]byte(event.Sig)), common.HexToHash(log.Topics[0].Hex()))
			s.Require().Equal(log.BlockNumber, uint64(s.network.GetContext().BlockHeight())) //nolint:gosec // G115

			// Check event parameters
			var createEvent erc20factory.EventCreate
			err = cmn.UnpackLog(s.precompile.ABI, &createEvent, erc20factory.EventTypeCreate, *log)
			s.Require().NoError(err, "unable to unpack log into create event")

			s.Require().Equal(tc.tokenAddress, createEvent.TokenAddress, "expected different token address")
			s.Require().Equal(tc.tokenType, createEvent.TokenPairType, "expected different token type")
			s.Require().Equal(tc.salt, createEvent.Salt, "expected different salt")
			s.Require().Equal(tc.name, createEvent.Name, "expected different name")
			s.Require().Equal(tc.symbol, createEvent.Symbol, "expected different symbol")
			s.Require().Equal(tc.decimals, createEvent.Decimals, "expected different decimals")
			s.Require().Equal(tc.minter, createEvent.Minter, "expected different minter")
			s.Require().Equal(tc.premintedSupply, createEvent.PremintedSupply, "expected different preminted supply")
		})
	}
}
