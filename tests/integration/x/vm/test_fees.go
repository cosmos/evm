package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/vm/keeper"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperTestSuite) TestCheckSenderBalance() {
	hundredInt := sdkmath.NewInt(100)
	zeroInt := sdkmath.ZeroInt()
	oneInt := sdkmath.OneInt()
	fiveInt := sdkmath.NewInt(5)
	fiftyInt := sdkmath.NewInt(50)
	negInt := sdkmath.NewInt(-10)
	addr := utiltx.GenerateAddress()

	testCases := []struct {
		name            string
		to              string
		gasLimit        uint64
		gasPrice        *sdkmath.Int
		gasFeeCap       *big.Int
		gasTipCap       *big.Int
		cost            *sdkmath.Int
		from            []byte
		accessList      *ethtypes.AccessList
		expectPass      bool
		EnableFeemarket bool
	}{
		{
			name:       "Enough balance",
			to:         s.Keyring.GetAddr(0).String(),
			gasLimit:   10,
			gasPrice:   &oneInt,
			cost:       &oneInt,
			from:       addr.Bytes(),
			accessList: &ethtypes.AccessList{},
			expectPass: true,
		},
		{
			name:       "Equal balance",
			to:         s.Keyring.GetAddr(0).String(),
			gasLimit:   99,
			gasPrice:   &oneInt,
			cost:       &oneInt,
			from:       addr.Bytes(),
			accessList: &ethtypes.AccessList{},
			expectPass: true,
		},
		{
			name:       "negative cost",
			to:         s.Keyring.GetAddr(0).String(),
			gasLimit:   1,
			gasPrice:   &oneInt,
			cost:       &negInt,
			from:       addr.Bytes(),
			accessList: &ethtypes.AccessList{},
			expectPass: false,
		},
		{
			name:       "Higher gas limit, not enough balance",
			to:         s.Keyring.GetAddr(0).String(),
			gasLimit:   100,
			gasPrice:   &oneInt,
			cost:       &oneInt,
			from:       addr.Bytes(),
			accessList: &ethtypes.AccessList{},
			expectPass: false,
		},
		{
			name:       "Higher gas price, enough balance",
			to:         s.Keyring.GetAddr(0).String(),
			gasLimit:   10,
			gasPrice:   &fiveInt,
			cost:       &oneInt,
			from:       addr.Bytes(),
			accessList: &ethtypes.AccessList{},
			expectPass: true,
		},
		{
			name:       "Higher gas price, not enough balance",
			to:         s.Keyring.GetAddr(0).String(),
			gasLimit:   20,
			gasPrice:   &fiveInt,
			cost:       &oneInt,
			from:       addr.Bytes(),
			accessList: &ethtypes.AccessList{},
			expectPass: false,
		},
		{
			name:       "Higher cost, enough balance",
			to:         s.Keyring.GetAddr(0).String(),
			gasLimit:   10,
			gasPrice:   &fiveInt,
			cost:       &fiftyInt,
			from:       addr.Bytes(),
			accessList: &ethtypes.AccessList{},
			expectPass: true,
		},
		{
			name:       "Higher cost, not enough balance",
			to:         s.Keyring.GetAddr(0).String(),
			gasLimit:   10,
			gasPrice:   &fiveInt,
			cost:       &hundredInt,
			from:       addr.Bytes(),
			accessList: &ethtypes.AccessList{},
			expectPass: false,
		},
		{
			name:            "Enough balance w/ EnableFeemarket",
			to:              s.Keyring.GetAddr(0).String(),
			gasLimit:        10,
			gasFeeCap:       big.NewInt(1),
			cost:            &oneInt,
			from:            addr.Bytes(),
			accessList:      &ethtypes.AccessList{},
			expectPass:      true,
			EnableFeemarket: true,
		},
		{
			name:            "Equal balance w/ EnableFeemarket",
			to:              s.Keyring.GetAddr(0).String(),
			gasLimit:        99,
			gasFeeCap:       big.NewInt(1),
			cost:            &oneInt,
			from:            addr.Bytes(),
			accessList:      &ethtypes.AccessList{},
			expectPass:      true,
			EnableFeemarket: true,
		},
		{
			name:            "negative cost w/ EnableFeemarket",
			to:              s.Keyring.GetAddr(0).String(),
			gasLimit:        1,
			gasFeeCap:       big.NewInt(1),
			cost:            &negInt,
			from:            addr.Bytes(),
			accessList:      &ethtypes.AccessList{},
			expectPass:      false,
			EnableFeemarket: true,
		},
		{
			name:            "Higher gas limit, not enough balance w/ EnableFeemarket",
			to:              s.Keyring.GetAddr(0).String(),
			gasLimit:        100,
			gasFeeCap:       big.NewInt(1),
			cost:            &oneInt,
			from:            addr.Bytes(),
			accessList:      &ethtypes.AccessList{},
			expectPass:      false,
			EnableFeemarket: true,
		},
		{
			name:            "Higher gas price, enough balance w/ EnableFeemarket",
			to:              s.Keyring.GetAddr(0).String(),
			gasLimit:        10,
			gasFeeCap:       big.NewInt(5),
			cost:            &oneInt,
			from:            addr.Bytes(),
			accessList:      &ethtypes.AccessList{},
			expectPass:      true,
			EnableFeemarket: true,
		},
		{
			name:            "Higher gas price, not enough balance w/ EnableFeemarket",
			to:              s.Keyring.GetAddr(0).String(),
			gasLimit:        20,
			gasFeeCap:       big.NewInt(5),
			cost:            &oneInt,
			from:            addr.Bytes(),
			accessList:      &ethtypes.AccessList{},
			expectPass:      false,
			EnableFeemarket: true,
		},
		{
			name:            "Higher cost, enough balance w/ EnableFeemarket",
			to:              s.Keyring.GetAddr(0).String(),
			gasLimit:        10,
			gasFeeCap:       big.NewInt(5),
			cost:            &fiftyInt,
			from:            addr.Bytes(),
			accessList:      &ethtypes.AccessList{},
			expectPass:      true,
			EnableFeemarket: true,
		},
		{
			name:            "Higher cost, not enough balance w/ EnableFeemarket",
			to:              s.Keyring.GetAddr(0).String(),
			gasLimit:        10,
			gasFeeCap:       big.NewInt(5),
			cost:            &hundredInt,
			from:            addr.Bytes(),
			accessList:      &ethtypes.AccessList{},
			expectPass:      false,
			EnableFeemarket: true,
		},
	}

	vmdb := s.StateDB()
	vmdb.AddBalance(addr, uint256.MustFromBig(hundredInt.BigInt()), tracing.BalanceChangeUnspecified)
	balance := vmdb.GetBalance(addr)
	s.Require().Equal(balance.ToBig(), hundredInt.BigInt())
	err := vmdb.Commit()
	s.Require().NoError(err, "Unexpected error while committing to vmdb: %d", err)

	for i, tc := range testCases {
		s.Run(tc.name, func() {
			to := common.HexToAddress(tc.to)

			var amount, gasPrice, gasFeeCap, gasTipCap *big.Int
			if tc.cost != nil {
				amount = tc.cost.BigInt()
			}

			if tc.EnableFeemarket {
				gasFeeCap = tc.gasFeeCap
				if tc.gasTipCap == nil {
					gasTipCap = oneInt.BigInt()
				} else {
					gasTipCap = tc.gasTipCap
				}
			} else if tc.gasPrice != nil {
				gasPrice = tc.gasPrice.BigInt()
			}

			ethTxParams := &evmtypes.EvmTxArgs{
				ChainID:   zeroInt.BigInt(),
				Nonce:     1,
				To:        &to,
				Amount:    amount,
				GasLimit:  tc.gasLimit,
				GasPrice:  gasPrice,
				GasFeeCap: gasFeeCap,
				GasTipCap: gasTipCap,
				Accesses:  tc.accessList,
			}
			tx := evmtypes.NewTx(ethTxParams)
			tx.From = to.Bytes()

			acct := s.Network.App.GetEVMKeeper().GetAccountOrEmpty(s.Network.GetContext(), addr)
			err := keeper.CheckSenderBalance(
				sdkmath.NewIntFromBigInt(acct.Balance.ToBig()),
				tx.AsTransaction(),
			)

			if tc.expectPass {
				s.Require().NoError(err, "valid test %d failed", i)
			} else {
				s.Require().Error(err, "invalid test %d passed", i)
			}
		})
	}
}

// TestVerifyFeeAndDeductTxCostsFromUserBalance is a test method for both the VerifyFee
// function and the DeductTxCostsFromUserBalance method.
//
// NOTE: This method combines testing for both functions, because these used to be
// in one function and share a lot of the same setup.
// In practice, the two tested functions will also be sequentially executed.
func (s *KeeperTestSuite) TestVerifyFeeAndDeductTxCostsFromUserBalance() {
	hundredInt := sdkmath.NewInt(100)
	zeroInt := sdkmath.ZeroInt()
	oneInt := sdkmath.NewInt(1)
	fiveInt := sdkmath.NewInt(5)
	fiftyInt := sdkmath.NewInt(50)
	addr, _ := utiltx.NewAddrKey()

	// should be enough to cover all test cases
	initBalance := sdkmath.NewInt((ethparams.InitialBaseFee + 10) * 105)

	testCases := []struct {
		name             string
		gasLimit         uint64
		gasPrice         *sdkmath.Int
		gasFeeCap        *big.Int
		gasTipCap        *big.Int
		cost             *sdkmath.Int
		accessList       *ethtypes.AccessList
		expectPassVerify bool
		expectPassDeduct bool
		EnableFeemarket  bool
		from             []byte
		malleate         func()
	}{
		{
			name:             "Enough balance",
			gasLimit:         10,
			gasPrice:         &oneInt,
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: true,
			expectPassDeduct: true,
			from:             addr.Bytes(),
		},
		{
			name:             "Equal balance",
			gasLimit:         100,
			gasPrice:         &oneInt,
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: true,
			expectPassDeduct: true,
			from:             addr.Bytes(),
		},
		{
			name:             "Higher gas limit, not enough balance",
			gasLimit:         105,
			gasPrice:         &oneInt,
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: true,
			expectPassDeduct: false,
			from:             addr.Bytes(),
		},
		{
			name:             "Higher gas price, enough balance",
			gasLimit:         20,
			gasPrice:         &fiveInt,
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: true,
			expectPassDeduct: true,
			from:             addr.Bytes(),
		},
		{
			name:             "Higher gas price, not enough balance",
			gasLimit:         20,
			gasPrice:         &fiftyInt,
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: true,
			expectPassDeduct: false,
			from:             addr.Bytes(),
		},
		// This case is expected to be true because the fees can be deducted, but the tx
		// execution is going to fail because there is no more balance to pay the cost
		{
			name:             "Higher cost, enough balance",
			gasLimit:         100,
			gasPrice:         &oneInt,
			cost:             &fiftyInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: true,
			expectPassDeduct: true,
			from:             addr.Bytes(),
		},
		//  testcases with EnableFeemarket enabled.
		{
			name:             "Invalid gasFeeCap w/ EnableFeemarket",
			gasLimit:         10,
			gasFeeCap:        big.NewInt(1),
			gasTipCap:        big.NewInt(1),
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: false,
			expectPassDeduct: true,
			EnableFeemarket:  true,
			from:             addr.Bytes(),
		},
		{
			name:             "empty tip fee is valid to deduct",
			gasLimit:         10,
			gasFeeCap:        big.NewInt(ethparams.InitialBaseFee),
			gasTipCap:        big.NewInt(1),
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: true,
			expectPassDeduct: true,
			EnableFeemarket:  true,
			from:             addr.Bytes(),
		},
		{
			name:             "effectiveTip equal to gasTipCap",
			gasLimit:         100,
			gasFeeCap:        big.NewInt(ethparams.InitialBaseFee + 2),
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: true,
			expectPassDeduct: true,
			EnableFeemarket:  true,
			from:             addr.Bytes(),
		},
		{
			name:             "effectiveTip equal to (gasFeeCap - baseFee)",
			gasLimit:         105,
			gasFeeCap:        big.NewInt(ethparams.InitialBaseFee + 1),
			gasTipCap:        big.NewInt(2),
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: true,
			expectPassDeduct: true,
			EnableFeemarket:  true,
			from:             addr.Bytes(),
		},
		{
			name:             "Invalid from address",
			gasLimit:         10,
			gasPrice:         &oneInt,
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: true,
			expectPassDeduct: false,
			from:             []byte("abcdef"),
		},
		{
			name:     "Enough balance - with access list",
			gasLimit: 10,
			gasPrice: &oneInt,
			cost:     &oneInt,
			accessList: &ethtypes.AccessList{
				ethtypes.AccessTuple{
					Address:     s.Keyring.GetAddr(0),
					StorageKeys: []common.Hash{},
				},
			},
			expectPassVerify: true,
			expectPassDeduct: true,
			from:             addr.Bytes(),
		},
		{
			name:             "gasLimit < intrinsicGas during IsCheckTx",
			gasLimit:         1,
			gasPrice:         &oneInt,
			cost:             &oneInt,
			accessList:       &ethtypes.AccessList{},
			expectPassVerify: false,
			expectPassDeduct: true,
			from:             addr.Bytes(),
			malleate: func() {
				s.Network.WithIsCheckTxCtx(true)
			},
		},
	}

	for i, tc := range testCases {
		s.Run(tc.name, func() {
			s.EnableFeemarket = tc.EnableFeemarket
			s.SetupTest()
			vmdb := s.StateDB()

			if tc.malleate != nil {
				tc.malleate()
			}
			var amount, gasPrice, gasFeeCap, gasTipCap *big.Int
			if tc.cost != nil {
				amount = tc.cost.BigInt()
			}

			if s.EnableFeemarket {
				if tc.gasFeeCap != nil {
					gasFeeCap = tc.gasFeeCap
				}
				if tc.gasTipCap == nil {
					gasTipCap = oneInt.BigInt()
				} else {
					gasTipCap = tc.gasTipCap
				}
				vmdb.AddBalance(addr, uint256.MustFromBig(initBalance.BigInt()), tracing.BalanceChangeUnspecified)
				balance := vmdb.GetBalance(addr)
				s.Require().Equal(balance.ToBig(), initBalance.BigInt())
			} else {
				if tc.gasPrice != nil {
					gasPrice = tc.gasPrice.BigInt()
				}

				vmdb.AddBalance(addr, uint256.MustFromBig(hundredInt.BigInt()), tracing.BalanceChangeUnspecified)
				balance := vmdb.GetBalance(addr)
				s.Require().Equal(balance.ToBig(), hundredInt.BigInt())
			}
			err := vmdb.Commit()
			s.Require().NoError(err, "Unexpected error while committing to vmdb: %d", err)

			toAddr := s.Keyring.GetAddr(0)
			ethTxParams := &evmtypes.EvmTxArgs{
				ChainID:   zeroInt.BigInt(),
				Nonce:     1,
				To:        &toAddr,
				Amount:    amount,
				GasLimit:  tc.gasLimit,
				GasPrice:  gasPrice,
				GasFeeCap: gasFeeCap,
				GasTipCap: gasTipCap,
				Accesses:  tc.accessList,
			}
			tx := evmtypes.NewTx(ethTxParams)
			tx.From = tc.from

			ethTx := tx.AsTransaction()

			baseFee := s.Network.App.GetEVMKeeper().GetBaseFee(s.Network.GetContext())
			priority := evmtypes.GetTxPriority(ethTx, baseFee)

			baseDenom := evmtypes.GetEVMCoinDenom()

			fees, err := keeper.VerifyFee(ethTx, baseDenom, baseFee, false, false, false, s.Network.GetContext().IsCheckTx())
			if tc.expectPassVerify {
				s.Require().NoError(err, "valid test %d failed - '%s'", i, tc.name)
				if tc.EnableFeemarket {
					baseFee := s.Network.App.GetFeeMarketKeeper().GetBaseFee(s.Network.GetContext())
					s.Require().Equal(
						fees,
						sdk.NewCoins(
							sdk.NewCoin(baseDenom, sdkmath.NewIntFromBigInt(tx.GetEffectiveFee(baseFee.TruncateInt().BigInt()))),
						),
						"valid test %d failed, fee value is wrong  - '%s'", i, tc.name,
					)
					s.Require().Equal(int64(0), priority)
				} else {
					s.Require().Equal(
						fees,
						sdk.NewCoins(
							sdk.NewCoin(baseDenom, tc.gasPrice.Mul(sdkmath.NewIntFromUint64(tc.gasLimit))),
						),
						"valid test %d failed, fee value is wrong  - '%s'", i, tc.name,
					)
				}
			} else {
				s.Require().Error(err, "invalid test %d passed - '%s'", i, tc.name)
				s.Require().Nil(fees, "invalid test %d passed. fees value must be nil - '%s'", i, tc.name)
			}

			err = s.Network.App.GetEVMKeeper().DeductTxCostsFromUserBalance(s.Network.GetContext(), fees, common.BytesToAddress(tx.From))
			if tc.expectPassDeduct {
				s.Require().NoError(err, "valid test %d failed - '%s'", i, tc.name)
			} else {
				s.Require().Error(err, "invalid test %d passed - '%s'", i, tc.name)
			}
		})
	}
	s.EnableFeemarket = false // reset flag
}
