package types_test

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/erc20/types"
)

type AllowanceTestSuite struct {
	suite.Suite
}

func (s *AllowanceTestSuite) TestNewAllowance() {
	testCases := []struct {
		msg        string
		erc20Addr  common.Address
		owner      common.Address
		spender    common.Address
		value      *big.Int
		expectPass bool
	}{
		{
			msg:        "invalid erc20 address",
			erc20Addr:  common.Address{},
			owner:      utiltx.GenerateAddress(),
			spender:    utiltx.GenerateAddress(),
			value:      big.NewInt(1),
			expectPass: false,
		},
		{
			msg:        "invalid owner address",
			erc20Addr:  utiltx.GenerateAddress(),
			owner:      common.Address{},
			spender:    utiltx.GenerateAddress(),
			value:      big.NewInt(1),
			expectPass: false,
		},
		{
			msg:        "invalid spender address",
			erc20Addr:  utiltx.GenerateAddress(),
			owner:      utiltx.GenerateAddress(),
			spender:    common.Address{},
			value:      big.NewInt(1),
			expectPass: false,
		},
		{
			msg:        "negative value",
			erc20Addr:  utiltx.GenerateAddress(),
			owner:      utiltx.GenerateAddress(),
			spender:    utiltx.GenerateAddress(),
			value:      big.NewInt(-1),
			expectPass: false,
		},
		{
			msg:        "invalid value",
			erc20Addr:  utiltx.GenerateAddress(),
			owner:      utiltx.GenerateAddress(),
			spender:    utiltx.GenerateAddress(),
			value:      big.NewInt(0),
			expectPass: false,
		},
		{
			msg:        "pass",
			erc20Addr:  utiltx.GenerateAddress(),
			owner:      utiltx.GenerateAddress(),
			spender:    utiltx.GenerateAddress(),
			value:      big.NewInt(1),
			expectPass: true,
		},
	}

	for _, tc := range testCases {
		allowance := types.NewAllowance(tc.erc20Addr, tc.owner, tc.spender, tc.value)
		err := allowance.Validate()

		if tc.expectPass {
			s.Require().NoError(err, "valid test %s failed: %s", tc.msg, err)
		} else {
			s.Require().Error(err, "invalid test %s passed: %s", tc.msg, err)
		}
	}
}
