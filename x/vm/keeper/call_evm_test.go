package keeper_test

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/contracts"
	testconstants "github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/keeper/testdata"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func (suite *KeeperTestSuite) TestCallEVM() {
	wcosmosEVMContract := common.HexToAddress(testconstants.WEVMOSContractMainnet)
	testCases := []struct {
		name    string
		method  string
		expPass bool
	}{
		{
			"unknown method",
			"",
			false,
		},
		{
			"pass",
			"balanceOf",
			true,
		},
	}
	for _, tc := range testCases {
		suite.SetupTest() // reset

		erc20 := contracts.ERC20MinterBurnerDecimalsContract.ABI
		account := utiltx.GenerateAddress()
		res, err := suite.network.App.EVMKeeper.CallEVM(suite.network.GetContext(), erc20, types.ModuleAddress, wcosmosEVMContract, false, tc.method, account)
		if tc.expPass {
			suite.Require().IsTypef(&evmtypes.MsgEthereumTxResponse{}, res, tc.name)
			suite.Require().NoError(err)
		} else {
			suite.Require().Error(err)
		}
	}
}

func (suite *KeeperTestSuite) TestCallEVMWithData() {
	erc20 := contracts.ERC20MinterBurnerDecimalsContract.ABI
	wcosmosEVMContract := common.HexToAddress(testconstants.WEVMOSContractMainnet)
	testCases := []struct {
		name     string
		from     common.Address
		malleate func() []byte
		deploy   bool
		expPass  bool
	}{
		{
			"pass with unknown method",
			types.ModuleAddress,
			func() []byte {
				account := utiltx.GenerateAddress()
				data, _ := erc20.Pack("", account)
				return data
			},
			false,
			true,
		},
		{
			"pass",
			types.ModuleAddress,
			func() []byte {
				account := utiltx.GenerateAddress()
				data, _ := erc20.Pack("balanceOf", account)
				return data
			},
			false,
			true,
		},
		{
			"pass with empty data",
			types.ModuleAddress,
			func() []byte {
				return []byte{}
			},
			false,
			true,
		},

		{
			"fail empty sender",
			common.Address{},
			func() []byte {
				return []byte{}
			},
			false,
			false,
		},
		{
			"deploy",
			types.ModuleAddress,
			func() []byte {
				ctorArgs, _ := contracts.ERC20MinterBurnerDecimalsContract.ABI.Pack("", "test", "test", uint8(18))
				data := append(contracts.ERC20MinterBurnerDecimalsContract.Bin, ctorArgs...) //nolint:gocritic
				return data
			},
			true,
			true,
		},
		{
			"fail deploy",
			types.ModuleAddress,
			func() []byte {
				params := suite.network.App.EVMKeeper.GetParams(suite.network.GetContext())
				params.AccessControl.Create = evmtypes.AccessControlType{
					AccessType: evmtypes.AccessTypeRestricted,
				}
				_ = suite.network.App.EVMKeeper.SetParams(suite.network.GetContext(), params)
				ctorArgs, _ := contracts.ERC20MinterBurnerDecimalsContract.ABI.Pack("", "test", "test", uint8(18))
				data := append(contracts.ERC20MinterBurnerDecimalsContract.Bin, ctorArgs...) //nolint:gocritic
				return data
			},
			true,
			false,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest() // reset

			data := tc.malleate()
			var res *evmtypes.MsgEthereumTxResponse
			var err error

			if tc.deploy {
				res, err = suite.network.App.EVMKeeper.CallEVMWithData(suite.network.GetContext(), tc.from, nil, data, true)
			} else {
				res, err = suite.network.App.EVMKeeper.CallEVMWithData(suite.network.GetContext(), tc.from, &wcosmosEVMContract, data, false)
			}

			if tc.expPass {
				suite.Require().IsTypef(&evmtypes.MsgEthereumTxResponse{}, res, tc.name)
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

// TestDerivedEVMCallCommitFlag is a regression test for F-2026-17736: a derived
// call with commit=false must not persist any state changes, while commit=true
// must persist them. It deploys an ERC20, performs a state-changing transfer via
// DerivedEVMCall, and asserts the recipient balance in the underlying store —
// reading it back with the independent CallEVM read path, since the bug was
// invisible on the passed context object and only observable at the store level.
func (suite *KeeperTestSuite) TestDerivedEVMCallCommitFlag() {
	testCases := []struct {
		name      string
		commit    bool
		expChange bool
	}{
		{"commit=false must NOT persist state", false, false},
		{"commit=true must persist state", true, true},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest() // reset

			erc20Contract, err := testdata.LoadERC20Contract()
			suite.Require().NoError(err)
			erc20ABI := erc20Contract.ABI

			owner := suite.keyring.GetAddr(0)
			recipient := utiltx.GenerateAddress()
			amount := big.NewInt(1000)

			contractAddr := suite.DeployTestContract(suite.T(), suite.network.GetContext(), owner, big.NewInt(1_000_000))

			// balanceOf reads through CallEVM (a read-only path independent of the
			// function under test) so the assertion reflects committed store state.
			balanceOf := func(addr common.Address) *big.Int {
				res, err := suite.network.App.EVMKeeper.CallEVM(
					suite.network.GetContext(), erc20ABI, types.ModuleAddress, contractAddr, false, "balanceOf", addr,
				)
				suite.Require().NoError(err)
				out, err := erc20ABI.Unpack("balanceOf", res.Ret)
				suite.Require().NoError(err)
				return out[0].(*big.Int)
			}

			suite.Require().Equal(int64(0), balanceOf(recipient).Int64(), "recipient must start at zero balance")

			_, err = suite.network.App.EVMKeeper.DerivedEVMCall(
				suite.network.GetContext(),
				erc20ABI,
				owner,         // from
				contractAddr,  // contract
				big.NewInt(0), // value
				nil,           // gasLimit
				tc.commit,     // commit (flag under test)
				false,        // gasless
				false,        // isModuleSender
				nil,          // manualNonce
				"transfer",
				recipient, amount,
			)
			suite.Require().NoError(err)

			got := balanceOf(recipient)
			if tc.expChange {
				suite.Require().Equal(amount.Int64(), got.Int64(), "commit=true: transfer must be persisted")
			} else {
				suite.Require().Equal(int64(0), got.Int64(), "commit=false: state must NOT be persisted")
			}
		})
	}
}
