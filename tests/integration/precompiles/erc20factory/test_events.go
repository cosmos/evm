package erc20factory

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/erc20factory"
	utiltx "github.com/cosmos/evm/testutil/tx"
)

func (s *PrecompileTestSuite) TestEmitCreateEvent() {
	testcases := []struct {
		testName     string
		tokenAddress common.Address
		tokenType    uint8
		salt         [32]uint8
		name         string
		symbol       string
		decimals     uint8
	}{
		{
			testName:     "pass",
			tokenAddress: utiltx.GenerateAddress(),
			tokenType:    0,
			salt:         [32]uint8{0},
			name:         "Test",
			symbol:       "TEST",
			decimals:     18,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.testName, func() {
			s.SetupTest()
			stateDB := s.network.GetStateDB()

			err := s.precompile.EmitCreateEvent(s.network.GetContext(), stateDB, tc.tokenAddress, tc.tokenType, tc.salt, tc.name, tc.symbol, tc.decimals)
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
		})
	}
}
