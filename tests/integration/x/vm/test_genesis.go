package vm

import (
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	"github.com/cosmos/evm/x/vm"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestInitGenesis runs various scenarios against InitGenesis
func (s *GenesisTestSuite) TestInitGenesis() {
	// prepare a key and address for storage tests
	privkey, err := ethsecp256k1.GenerateKey()
	s.Require().NoError(err)
	address := common.HexToAddress(privkey.PubKey().Address().String())

	params := types.Params{
		EvmDenom:                "aatom",
		ExtraEIPs:               types.DefaultExtraEIPs,
		EVMChannels:             types.DefaultEVMChannels,
		AccessControl:           types.DefaultAccessControl,
		ActiveStaticPrecompiles: types.DefaultStaticPrecompiles,
		HistoryServeWindow:      types.DefaultHistoryServeWindow,
		ExtendedDenomOptions:    nil,
	}

	var (
		vmdb *statedb.StateDB
		ctx  sdk.Context
	)

	// table-driven cases
	testCases := []struct {
		name     string
		malleate func(*network.UnitTestNetwork)
		genState *types.GenesisState
		code     common.Hash
		expPanic bool
	}{
		{
			name:     "pass - default",
			malleate: func(_ *network.UnitTestNetwork) {},
			genState: &types.GenesisState{
				Params:   params,
				Accounts: []types.GenesisAccount{},
			},
			expPanic: false,
		},
		{
			name: "valid account",
			malleate: func(_ *network.UnitTestNetwork) {
				vmdb.AddBalance(address, uint256.NewInt(1), tracing.BalanceChangeUnspecified)
			},
			genState: &types.GenesisState{
				Params: params,
				Accounts: []types.GenesisAccount{
					{
						Address: address.String(),
						Storage: types.Storage{
							{Key: common.BytesToHash([]byte("key")).String(), Value: common.BytesToHash([]byte("value")).String()},
						},
					},
				},
			},
			expPanic: false,
		},
		{
			name:     "account not found",
			malleate: func(_ *network.UnitTestNetwork) {},
			genState: &types.GenesisState{
				Params: params,
				Accounts: []types.GenesisAccount{
					{
						Address: address.String(),
					},
				},
			},
			expPanic: true,
		},
		{
			name: "ignore empty account code checking",
			malleate: func(network *network.UnitTestNetwork) {
				acc := network.App.GetAccountKeeper().NewAccountWithAddress(ctx, address.Bytes())
				network.App.GetAccountKeeper().SetAccount(ctx, acc)
			},
			genState: &types.GenesisState{
				Params: params,
				Accounts: []types.GenesisAccount{
					{
						Address: address.String(),
						Code:    "",
					},
				},
			},
			expPanic: false,
		},
		{
			name: "valid account with code",
			malleate: func(network *network.UnitTestNetwork) {
				acc := network.App.GetAccountKeeper().NewAccountWithAddress(ctx, address.Bytes())
				network.App.GetAccountKeeper().SetAccount(ctx, acc)
			},
			genState: &types.GenesisState{
				Params: params,
				Accounts: []types.GenesisAccount{
					{
						Address: address.String(),
						Code:    "1234",
					},
				},
			},
			expPanic: false,
		},
	}

	for _, tc := range testCases {
		s.T().Run(tc.name, func(t *testing.T) {
			// reinitialize suite state for each subtest
			s.SetupTest()
			ctx = s.network.GetContext()
			vmdb = statedb.New(
				ctx, s.network.App.GetEVMKeeper(),
				statedb.NewEmptyTxConfig())

			tc.malleate(s.network)
			err := vmdb.Commit()
			s.Require().NoError(err)

			configurator := types.NewEVMConfigurator()
			configurator.ResetTestConfig()

			if tc.expPanic {
				s.Require().Panics(func() {
					_ = vm.InitGenesis(
						s.network.GetContext(),
						s.network.App.GetEVMKeeper(),
						s.network.App.GetAccountKeeper(),
						s.network.App.GetBankKeeper(),
						*tc.genState,
						&sync.Once{},
					)
				})
			} else {
				s.Require().NotPanics(func() {
					_ = vm.InitGenesis(
						ctx,
						s.network.App.GetEVMKeeper(),
						s.network.App.GetAccountKeeper(),
						s.network.App.GetBankKeeper(),
						*tc.genState,
						&sync.Once{},
					)
				})
				// verify state for each account
				for _, acct := range tc.genState.Accounts {
					s.Require().NotNil(
						s.network.App.GetAccountKeeper().GetAccount(ctx, common.HexToAddress(acct.Address).Bytes()),
					)
					expHash := crypto.Keccak256Hash(common.Hex2Bytes(acct.Code))
					if acct.Code == "" {
						expHash = common.BytesToHash(types.EmptyCodeHash)
					}

					s.Require().Equal(
						expHash.String(),
						s.network.App.GetEVMKeeper().GetCodeHash(ctx, common.HexToAddress(acct.Address)).String(),
					)
					s.Require().Equal(
						acct.Code,
						common.Bytes2Hex(
							s.network.App.GetEVMKeeper().GetCode(ctx, expHash),
						),
					)

					for _, storage := range acct.Storage {
						s.Require().Equal(
							common.HexToHash(storage.Value),
							vmdb.GetState(common.HexToAddress(acct.Address), common.HexToHash(storage.Key)),
						)
					}
				}

				// verify preinstalls
				for _, preinstall := range tc.genState.Preinstalls {
					preinstallAddr := common.HexToAddress(preinstall.Address)
					accAddress := sdk.AccAddress(preinstallAddr.Bytes())
					s.Require().NotNil(
						s.network.App.GetAccountKeeper().GetAccount(ctx, accAddress),
					)
					preinstallCode := common.Hex2Bytes(preinstall.Code)
					expectedCodeHash := crypto.Keccak256Hash(preinstallCode)
					s.Require().Equal(
						preinstallCode,
						s.network.App.GetEVMKeeper().GetCode(ctx, expectedCodeHash),
					)

					s.Require().Equal(
						expectedCodeHash,
						s.network.App.GetEVMKeeper().GetCodeHash(ctx, preinstallAddr),
					)
				}
			}
		})
	}
}

// TestExportGenesis verifies ExportGenesis output
func (s *GenesisTestSuite) TestExportGenesis() {
	contractAddr, err := s.factory.DeployContract(
		s.keyring.GetPrivKey(0),
		types.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        contracts.ERC20MinterBurnerDecimalsContract,
			ConstructorArgs: []interface{}{"TestToken", "TTK", uint8(18)},
		},
	)
	s.Require().NoError(err)
	s.Require().NoError(s.network.NextBlock())

	contractAddr2, err := s.factory.DeployContract(
		s.keyring.GetPrivKey(0),
		types.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        contracts.ERC20MinterBurnerDecimalsContract,
			ConstructorArgs: []interface{}{"AnotherToken", "ATK", uint8(18)},
		},
	)
	s.Require().NoError(err)
	s.Require().NoError(s.network.NextBlock())

	genState := vm.ExportGenesis(s.network.GetContext(), s.network.App.GetEVMKeeper())
	// Exported accounts 4 default preinstalls
	s.Require().Len(genState.Accounts, 8)

	addrs := make([]string, len(genState.Accounts))
	for i, acct := range genState.Accounts {
		addrs[i] = acct.Address
	}
	s.Require().Contains(addrs, contractAddr.Hex())
	s.Require().Contains(addrs, contractAddr2.Hex())

	// Since preinstalls gets exported as normal contracts, it should be empty on export genesis
	s.Require().Empty(genState.Preinstalls)
}
