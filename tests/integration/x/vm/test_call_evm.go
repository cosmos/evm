package vm

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/contracts"
	testconstants "github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func (s *KeeperTestSuite) TestCallEVM() {
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
		s.SetupTest() // reset

		erc20 := contracts.ERC20MinterBurnerDecimalsContract.ABI
		account := utiltx.GenerateAddress()
		res, err := s.Network.App.GetEVMKeeper().CallEVM(s.Network.GetContext(), erc20, types.ModuleAddress, wcosmosEVMContract, false, nil, tc.method, account)
		if tc.expPass {
			s.Require().IsTypef(&evmtypes.MsgEthereumTxResponse{}, res, tc.name)
			s.Require().NoError(err)
		} else {
			s.Require().Error(err)
		}
	}
}

func (s *KeeperTestSuite) TestCallEVMWithData() {
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
				params := s.Network.App.GetEVMKeeper().GetParams(s.Network.GetContext())
				params.AccessControl.Create = evmtypes.AccessControlType{
					AccessType: evmtypes.AccessTypeRestricted,
				}
				_ = s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), params)
				ctorArgs, _ := contracts.ERC20MinterBurnerDecimalsContract.ABI.Pack("", "test", "test", uint8(18))
				data := append(contracts.ERC20MinterBurnerDecimalsContract.Bin, ctorArgs...) //nolint:gocritic
				return data
			},
			true,
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset

			data := tc.malleate()
			var res *evmtypes.MsgEthereumTxResponse
			var err error

			if tc.deploy {
				res, err = s.Network.App.GetEVMKeeper().CallEVMWithData(s.Network.GetContext(), tc.from, nil, data, true, nil)
			} else {
				res, err = s.Network.App.GetEVMKeeper().CallEVMWithData(s.Network.GetContext(), tc.from, &wcosmosEVMContract, data, false, nil)
			}

			if tc.expPass {
				s.Require().IsTypef(&evmtypes.MsgEthereumTxResponse{}, res, tc.name)
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}
