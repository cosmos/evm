package erc20_test

import (
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/common/mocks"
	"github.com/cosmos/evm/precompiles/erc20"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (s *PrecompileTestSuite) TestSend() {
	s.SetupTest()

	testcases := []struct {
		name     string
		malleate func() cmn.BankKeeper
		expFail  bool
	}{
		{
			name: "send with BankKeeper",
			malleate: func() cmn.BankKeeper {
				return s.network.App.BankKeeper
			},
			expFail: false,
		},
		{
			name: "send with PreciseBankKeeper",
			malleate: func() cmn.BankKeeper {
				return s.network.App.PreciseBankKeeper
			},
			expFail: false,
		},
		{
			name: "send with MockBankKeeper",
			malleate: func() cmn.BankKeeper {
				return mocks.NewBankKeeper(s.T())
			},
			expFail: true,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			bankKeeper := tc.malleate()
			msgServ := erc20.NewMsgServerImpl(bankKeeper)
			s.Require().NotNil(msgServ)
			err := msgServ.Send(s.network.GetContext(), &types.MsgSend{
				FromAddress: s.keyring.GetAccAddr(0).String(),
				ToAddress:   s.keyring.GetAccAddr(1).String(),
				Amount:      sdk.NewCoins(sdk.NewCoin(vmtypes.GetEVMCoinExtendedDenom(), math.OneInt())),
			})
			if tc.expFail {
				s.Require().ErrorContains(err, "invalid keeper type")
			} else {
				s.Require().NoError(err)
			}
		})
	}
}
