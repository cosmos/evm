package gov

import (
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/precompiles/gov"
	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func (s *PrecompileTestSuite) TestIsTransaction() {
	testCases := []struct {
		name   string
		method abi.Method
		isTx   bool
	}{
		{
			gov.VoteMethod,
			s.precompile.Methods[gov.VoteMethod],
			true,
		},
		{
			"invalid",
			abi.Method{},
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.Require().Equal(s.precompile.IsTransaction(&tc.method), tc.isTx)
		})
	}
}

// TestRun tests the precompile's Run method.
func (s *PrecompileTestSuite) TestRun() {
	testcases := []struct {
		name        string
		malleate    func() (common.Address, []byte)
		readOnly    bool
		expPass     bool
		errContains string
	}{
		{
			name: "pass - vote transaction",
			malleate: func() (common.Address, []byte) {
				const proposalID uint64 = 1
				const option uint8 = 1
				const metadata = "metadata"

				input, err := s.precompile.Pack(
					gov.VoteMethod,
					s.keyring.GetAddr(0),
					proposalID,
					option,
					metadata,
				)
				s.Require().NoError(err, "failed to pack input")
				return s.keyring.GetAddr(0), input
			},
			readOnly: false,
			expPass:  true,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			// setup basic test suite
			s.SetupTest()
			ctx := s.network.GetContext()

			baseFee := s.network.App.GetEVMKeeper().GetBaseFee(ctx)

			// malleate testcase
			caller, input := tc.malleate()

			contract := vm.NewPrecompile(caller, s.precompile.Address(), uint256.NewInt(0), uint64(1e6))
			contract.Input = input

			contractAddr := contract.Address()
			// Build and sign Ethereum transaction
			txArgs := evmtypes.EvmTxArgs{
				ChainID:   evmtypes.GetEthChainConfig().ChainID,
				Nonce:     0,
				To:        &contractAddr,
				Amount:    nil,
				GasLimit:  100000,
				GasPrice:  testutil.ExampleMinGasPrices,
				GasFeeCap: baseFee,
				GasTipCap: big.NewInt(1),
				Accesses:  &ethtypes.AccessList{},
			}
			msg, err := s.factory.GenerateGethCoreMsg(s.keyring.GetPrivKey(0), txArgs)
			s.Require().NoError(err)

			// Instantiate config
			proposerAddress := ctx.BlockHeader().ProposerAddress
			cfg, err := s.network.App.GetEVMKeeper().EVMConfig(ctx, proposerAddress)
			s.Require().NoError(err, "failed to instantiate EVM config")

			// Instantiate EVM
			stDB := statedb.New(
				ctx,
				s.network.App.GetEVMKeeper(),
				statedb.NewEmptyTxConfig(),
			)
			evm := s.network.App.GetEVMKeeper().NewEVM(
				ctx, *msg, cfg, nil, stDB,
			)

			precompiles, found, err := s.network.App.GetEVMKeeper().GetPrecompileInstance(ctx, contractAddr)
			s.Require().NoError(err, "failed to instantiate precompile")
			s.Require().True(found, "not found precompile")
			evm.WithPrecompiles(precompiles.Map)

			// Run precompiled contract
			bz, err := s.precompile.Run(evm, contract, tc.readOnly)

			// Check results
			if tc.expPass {
				s.Require().NoError(err, "expected no error when running the precompile")
				s.Require().NotNil(bz, "expected returned bytes not to be nil")
			} else {
				s.Require().Error(err, "expected error to be returned when running the precompile")
				s.Require().Nil(bz, "expected returned bytes to be nil")
				s.Require().ErrorContains(err, tc.errContains)
			}
		})
	}
}
