package staking

import (
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm/precompiles/staking"
	"github.com/cosmos/evm/precompiles/testutil"
	chainutil "github.com/cosmos/evm/testutil"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/keyring"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

func (s *PrecompileTestSuite) TestIsTransaction() {
	testCases := []struct {
		name   string
		method abi.Method
		isTx   bool
	}{
		{
			staking.CreateValidatorMethod,
			s.precompile.Methods[staking.CreateValidatorMethod],
			true,
		},
		{
			staking.DelegateMethod,
			s.precompile.Methods[staking.DelegateMethod],
			true,
		},
		{
			staking.UndelegateMethod,
			s.precompile.Methods[staking.UndelegateMethod],
			true,
		},
		{
			staking.RedelegateMethod,
			s.precompile.Methods[staking.RedelegateMethod],
			true,
		},
		{
			staking.CancelUnbondingDelegationMethod,
			s.precompile.Methods[staking.CancelUnbondingDelegationMethod],
			true,
		},
		{
			staking.DelegationMethod,
			s.precompile.Methods[staking.DelegationMethod],
			false,
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

func (s *PrecompileTestSuite) TestRequiredGas() {
	testcases := []struct {
		name     string
		malleate func() []byte
		expGas   uint64
	}{
		{
			"success - delegate transaction with correct gas estimation",
			func() []byte {
				input, err := s.precompile.Pack(
					staking.DelegateMethod,
					s.keyring.GetAddr(0),
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(10000000000),
				)
				s.Require().NoError(err)
				return input
			},
			0x1ec8,
		},
		{
			"success - undelegate transaction with correct gas estimation",
			func() []byte {
				input, err := s.precompile.Pack(
					staking.UndelegateMethod,
					s.keyring.GetAddr(0),
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(1),
				)
				s.Require().NoError(err)
				return input
			},
			0x1ec8,
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			s.SetupTest()

			// malleate contract input
			input := tc.malleate()
			gas := s.precompile.RequiredGas(input)

			s.Require().Equal(gas, tc.expGas)
		})
	}
}

// TestRun tests the precompile's Run method.
func (s *PrecompileTestSuite) TestRun() {
	var ctx sdk.Context
	testcases := []struct {
		name        string
		malleate    func(delegator keyring.Key) []byte
		gas         uint64
		readOnly    bool
		expPass     bool
		errContains string
	}{
		{
			"fail - contract gas limit is < gas cost to run a query / tx",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.DelegateMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(1000),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			8000,
			false,
			false,
			"out of gas",
		},
		{
			"pass - delegate transaction",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.DelegateMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(1000),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			false,
			true,
			"",
		},
		{
			"pass - undelegate transaction",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.UndelegateMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(1),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			false,
			true,
			"",
		},
		{
			"pass - redelegate transaction",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.RedelegateMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					s.network.GetValidators()[1].GetOperator(),
					big.NewInt(1),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			false,
			true,
			"failed to redelegate tokens",
		},
		{
			"pass - cancel unbonding delegation transaction",
			func(delegator keyring.Key) []byte {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)
				// add unbonding delegation to staking keeper
				ubd := stakingtypes.NewUnbondingDelegation(
					delegator.AccAddr,
					valAddr,
					ctx.BlockHeight(),
					time.Now().Add(time.Hour),
					math.NewInt(1000),
					0,
					s.network.App.GetStakingKeeper().ValidatorAddressCodec(),
					s.network.App.GetAccountKeeper().AddressCodec(),
				)
				err = s.network.App.GetStakingKeeper().SetUnbondingDelegation(ctx, ubd)
				s.Require().NoError(err, "failed to set unbonding delegation")

				// Needs to be called after setting unbonding delegation
				// In order to mimic the coins being added to the unboding pool
				coin := sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(1000))
				err = s.network.App.GetBankKeeper().SendCoinsFromModuleToModule(ctx, stakingtypes.BondedPoolName, stakingtypes.NotBondedPoolName, sdk.Coins{coin})
				s.Require().NoError(err, "failed to send coins from module to module")

				input, err := s.precompile.Pack(
					staking.CancelUnbondingDelegationMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(1000),
					big.NewInt(ctx.BlockHeight()),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			false,
			true,
			"",
		},
		{
			"pass - delegation query",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.DelegationMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			false,
			true,
			"",
		},
		{
			"pass - validator query",
			func(_ keyring.Key) []byte {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)

				input, err := s.precompile.Pack(
					staking.ValidatorMethod,
					common.BytesToAddress(valAddr.Bytes()),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			false,
			true,
			"",
		},
		{
			"pass - redelgation query",
			func(delegator keyring.Key) []byte {
				valAddr1, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)
				valAddr2, err := sdk.ValAddressFromBech32(s.network.GetValidators()[1].GetOperator())
				s.Require().NoError(err)
				// add redelegation to staking keeper
				redelegation := stakingtypes.NewRedelegation(
					delegator.AccAddr,
					valAddr1,
					valAddr2,
					ctx.BlockHeight(),
					time.Now().Add(time.Hour),
					math.NewInt(1000),
					math.LegacyNewDec(1),
					0,
					s.network.App.GetStakingKeeper().ValidatorAddressCodec(),
					s.network.App.GetAccountKeeper().AddressCodec(),
				)

				err = s.network.App.GetStakingKeeper().SetRedelegation(ctx, redelegation)
				s.Require().NoError(err, "failed to set redelegation")

				input, err := s.precompile.Pack(
					staking.RedelegationMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					s.network.GetValidators()[1].GetOperator(),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			false,
			true,
			"",
		},
		{
			"pass - delegation query - read only",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.DelegationMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"",
		},
		{
			"pass - unbonding delegation query",
			func(delegator keyring.Key) []byte {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)
				// add unbonding delegation to staking keeper
				ubd := stakingtypes.NewUnbondingDelegation(
					delegator.AccAddr,
					valAddr,
					ctx.BlockHeight(),
					time.Now().Add(time.Hour),
					math.NewInt(1000),
					0,
					s.network.App.GetStakingKeeper().ValidatorAddressCodec(),
					s.network.App.GetAccountKeeper().AddressCodec(),
				)
				err = s.network.App.GetStakingKeeper().SetUnbondingDelegation(ctx, ubd)
				s.Require().NoError(err, "failed to set unbonding delegation")

				// Needs to be called after setting unbonding delegation
				// In order to mimic the coins being added to the unboding pool
				coin := sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(1000))
				err = s.network.App.GetBankKeeper().SendCoinsFromModuleToModule(ctx, stakingtypes.BondedPoolName, stakingtypes.NotBondedPoolName, sdk.Coins{coin})
				s.Require().NoError(err, "failed to send coins from module to module")

				input, err := s.precompile.Pack(
					staking.UnbondingDelegationMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"",
		},
		{
			"fail - delegate method - read only",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.DelegateMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(1000),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			21559, // use enough gas to avoid out of gas error
			true,
			false,
			"write protection",
		},
		{
			"fail - invalid method",
			func(_ keyring.Key) []byte {
				return []byte("invalid")
			},
			21559, // use enough gas to avoid out of gas error
			false,
			false,
			"no method with id",
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			// setup basic test suite
			s.SetupTest()
			ctx = s.network.GetContext().WithBlockTime(time.Now())

			baseFee := s.network.App.GetEVMKeeper().GetBaseFee(ctx)

			delegator := s.keyring.GetKey(0)

			contract := vm.NewPrecompile(delegator.Addr, s.precompile.Address(), uint256.NewInt(0), tc.gas)
			contractAddr := contract.Address()

			// malleate testcase
			contract.Input = tc.malleate(delegator)

			// Build and sign Ethereum transaction
			txArgs := evmtypes.EvmTxArgs{
				ChainID:   evmtypes.GetEthChainConfig().ChainID,
				Nonce:     0,
				To:        &contractAddr,
				Amount:    nil,
				GasLimit:  tc.gas,
				GasPrice:  chainutil.ExampleMinGasPrices,
				GasFeeCap: baseFee,
				GasTipCap: big.NewInt(1),
				Accesses:  &ethtypes.AccessList{},
			}

			msg, err := s.factory.GenerateGethCoreMsg(delegator.Priv, txArgs)
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
				s.Require().NotNil(bz, "expected returned bytes to be nil")
				execRevertErr := evmtypes.NewExecErrorWithReason(bz)
				s.Require().ErrorContains(execRevertErr, tc.errContains)
				consumed := ctx.GasMeter().GasConsumed()
				// LessThanOrEqual because the gas is consumed before the error is returned
				s.Require().LessOrEqual(tc.gas, consumed, "expected gas consumed to be equal to gas limit")
			}
		})
	}
}

// TestCMS tests the cache multistore writes.
func (s *PrecompileTestSuite) TestCMS() {
	s.customGenesis = true
	var ctx sdk.Context
	testcases := []struct {
		name          string
		malleate      func(delegator keyring.Key) []byte
		gas           uint64
		expPass       bool
		expKeeperPass bool
		errContains   string
	}{
		{
			"fail - contract gas limit is < gas cost to run a query / tx",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.DelegateMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(1000),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			8000,
			false,
			false,
			"gas too low",
		},
		{
			"pass - delegate transaction",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.DelegateMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(1000),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"",
		},
		{
			"pass - undelegate transaction",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.UndelegateMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(1),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"",
		},
		{
			"pass - redelegate transaction",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.RedelegateMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					s.network.GetValidators()[1].GetOperator(),
					big.NewInt(1),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"failed to redelegate tokens",
		},
		{
			"pass - cancel unbonding delegation transaction",
			func(delegator keyring.Key) []byte {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)
				// add unbonding delegation to staking keeper
				ubd := stakingtypes.NewUnbondingDelegation(
					delegator.AccAddr,
					valAddr,
					ctx.BlockHeight(),
					time.Now().Add(time.Hour),
					math.NewInt(1000),
					0,
					s.network.App.GetStakingKeeper().ValidatorAddressCodec(),
					s.network.App.GetAccountKeeper().AddressCodec(),
				)
				err = s.network.App.GetStakingKeeper().SetUnbondingDelegation(ctx, ubd)
				s.Require().NoError(err, "failed to set unbonding delegation")

				// Needs to be called after setting unbonding delegation
				// In order to mimic the coins being added to the unboding pool
				coin := sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(1000))
				err = s.network.App.GetBankKeeper().SendCoinsFromModuleToModule(ctx, stakingtypes.BondedPoolName, stakingtypes.NotBondedPoolName, sdk.Coins{coin})
				s.Require().NoError(err, "failed to send coins from module to module")

				input, err := s.precompile.Pack(
					staking.CancelUnbondingDelegationMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					big.NewInt(1000),
					big.NewInt(ctx.BlockHeight()),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"",
		},
		{
			"pass - delegation query",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.DelegationMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"",
		},
		{
			"pass - validator query",
			func(_ keyring.Key) []byte {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].OperatorAddress)
				s.Require().NoError(err)

				input, err := s.precompile.Pack(
					staking.ValidatorMethod,
					common.BytesToAddress(valAddr.Bytes()),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"",
		},
		{
			"pass - redelgation query",
			func(delegator keyring.Key) []byte {
				valAddr1, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)
				valAddr2, err := sdk.ValAddressFromBech32(s.network.GetValidators()[1].GetOperator())
				s.Require().NoError(err)
				// add redelegation to staking keeper
				redelegation := stakingtypes.NewRedelegation(
					delegator.AccAddr,
					valAddr1,
					valAddr2,
					ctx.BlockHeight(),
					time.Now().Add(time.Hour),
					math.NewInt(1000),
					math.LegacyNewDec(1),
					0,
					s.network.App.GetStakingKeeper().ValidatorAddressCodec(),
					s.network.App.GetAccountKeeper().AddressCodec(),
				)

				err = s.network.App.GetStakingKeeper().SetRedelegation(ctx, redelegation)
				s.Require().NoError(err, "failed to set redelegation")

				input, err := s.precompile.Pack(
					staking.RedelegationMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
					s.network.GetValidators()[1].GetOperator(),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"",
		},
		{
			"pass - delegation query - read only",
			func(delegator keyring.Key) []byte {
				input, err := s.precompile.Pack(
					staking.DelegationMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"",
		},
		{
			"pass - unbonding delegation query",
			func(delegator keyring.Key) []byte {
				valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
				s.Require().NoError(err)
				// add unbonding delegation to staking keeper
				ubd := stakingtypes.NewUnbondingDelegation(
					delegator.AccAddr,
					valAddr,
					ctx.BlockHeight(),
					time.Now().Add(time.Hour),
					math.NewInt(1000),
					0,
					s.network.App.GetStakingKeeper().ValidatorAddressCodec(),
					s.network.App.GetAccountKeeper().AddressCodec(),
				)
				err = s.network.App.GetStakingKeeper().SetUnbondingDelegation(ctx, ubd)
				s.Require().NoError(err, "failed to set unbonding delegation")

				// Needs to be called after setting unbonding delegation
				// In order to mimic the coins being added to the unboding pool
				coin := sdk.NewCoin(testconstants.ExampleAttoDenom, math.NewInt(1000))
				err = s.network.App.GetBankKeeper().SendCoinsFromModuleToModule(ctx, stakingtypes.BondedPoolName, stakingtypes.NotBondedPoolName, sdk.Coins{coin})
				s.Require().NoError(err, "failed to send coins from module to module")

				input, err := s.precompile.Pack(
					staking.UnbondingDelegationMethod,
					delegator.Addr,
					s.network.GetValidators()[0].GetOperator(),
				)
				s.Require().NoError(err, "failed to pack input")
				return input
			},
			1000000,
			true,
			true,
			"",
		},
		{
			"fail - invalid method",
			func(_ keyring.Key) []byte {
				return []byte("invalid")
			},
			100000, // use gas > 0 to avoid doing gas estimation
			false,
			true,
			"no method with id",
		},
	}

	for _, tc := range testcases {
		s.Run(tc.name, func() {
			// setup basic test suite
			s.SetupTest()
			ctx = s.network.GetContext().WithBlockTime(time.Now())

			cms := &testutil.TrackingMultiStore{
				Store:            s.network.App.GetBaseApp().CommitMultiStore().CacheMultiStore(),
				Writes:           0,
				HistoricalStores: nil,
			}
			ctx = ctx.WithMultiStore(cms)
			baseFee := s.network.App.GetEVMKeeper().GetBaseFee(ctx)

			delegator := s.keyring.GetKey(0)

			contract := vm.NewPrecompile(delegator.Addr, s.precompile.Address(), uint256.NewInt(0), tc.gas)
			contractAddr := contract.Address()

			// malleate testcase
			input := tc.malleate(delegator)

			// Build and sign Ethereum transaction
			txArgs := evmtypes.EvmTxArgs{
				Input:     input,
				ChainID:   evmtypes.GetEthChainConfig().ChainID,
				Nonce:     0,
				To:        &contractAddr,
				Amount:    nil,
				GasLimit:  tc.gas,
				GasPrice:  chainutil.ExampleMinGasPrices,
				GasFeeCap: baseFee,
				GasTipCap: big.NewInt(1),
				Accesses:  &ethtypes.AccessList{},
			}

			msgEthereumTx, err := s.factory.GenerateMsgEthereumTx(s.keyring.GetPrivKey(0), txArgs)
			s.Require().NoError(err, "failed to generate Ethereum message")
			signedMsg, err := s.factory.SignMsgEthereumTx(s.keyring.GetPrivKey(0), msgEthereumTx)
			s.Require().NoError(err, "failed to sign Ethereum message")

			resp, err := s.network.App.GetEVMKeeper().EthereumTx(ctx, &signedMsg)

			// Check results
			if tc.expPass {
				s.Require().NoError(err, "expected no error when running the precompile")
				s.Require().Empty(resp.VmError, "expected returned VmError to be empty string")
				s.Require().NotNil(resp.Ret, "expected returned bytes not to be nil")
				// NOTES: After stack-based snapshot mechanism is added for precompile call,
				// CacheMultiStore.Write() is always called once when tx succeeds.
				// It is because CacheMultiStore() is not called when creating snapshot for MultiStore,
				// Count of Write() is not accumulated.
				testutil.ValidateWrites(s.T(), cms, 1)
			} else {
				if tc.expKeeperPass {
					s.Require().NoError(err, "expected no error when running the precompile")
					s.Require().Contains(resp.VmError, vm.ErrExecutionReverted.Error(),
						"expected error to be returned when running the precompile")
					s.Require().NotNil(resp.Ret, "expected returned bytes to be encoded error reason")
					execRevertErr := evmtypes.NewExecErrorWithReason(resp.Ret)
					s.Require().Contains(execRevertErr.Error(), tc.errContains)

					consumed := ctx.GasMeter().GasConsumed()
					// Because opCall (for calling precompile) return ErrExecutionReverted, leftOverGas is refunded.
					// So, consumed gas is less than gasLimit
					s.Require().LessOrEqual(consumed, tc.gas, "expected gas consumed to be equal to gas limit")
					// NOTES: After stack-based snapshot mechanism is added for precompile call,
					// CacheMultiStore.Write() is not called when tx fails.
					testutil.ValidateWrites(s.T(), cms, 0)
				} else {
					s.Require().Error(err, "expected error to be returned when running the precompile")
					s.Require().Nil(resp, "expected returned response to be nil")
					s.Require().ErrorContains(err, tc.errContains)
					testutil.ValidateWrites(s.T(), cms, 0)

					// If a keeper method fails, the gas in the gasMeter is fully consumed.
					consumed := ctx.GasMeter().GasConsumed()
					s.Require().Equal(consumed, ctx.GasMeter().Limit(), "expected gas consumed to be equal to gas limit")
				}
			}
		})
	}
}
