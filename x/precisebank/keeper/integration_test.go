package keeper_test

import (
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/contracts"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/utils"
	"github.com/cosmos/evm/x/precisebank/keeper"
	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (suite *KeeperIntegrationTestSuite) TestMintBurnSendCoins_RandomValueMultiDecimals() {
	tests := []struct {
		name    string
		chainID testconstants.ChainID
	}{
		{
			name:    "6 decimals",
			chainID: testconstants.SixDecimalsChainID,
		},
		{
			name:    "2 decimals",
			chainID: testconstants.TwoDecimalsChainID,
		},
		{
			name:    "12 decimals",
			chainID: testconstants.TwelveDecimalsChainID,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTest()

			moduleName := evmtypes.ModuleName
			sender := sdk.AccAddress([]byte{1})
			recipient := sdk.AccAddress([]byte{2})

			// Mint initial balance to sender
			initialBalance := types.ConversionFactor().MulRaw(100)
			initialCoins := cs(ci(types.ExtendedCoinDenom(), initialBalance))
			suite.Require().NoError(suite.network.App.PreciseBankKeeper.MintCoins(suite.network.GetContext(), moduleName, initialCoins))
			suite.Require().NoError(suite.network.App.PreciseBankKeeper.SendCoinsFromModuleToAccount(suite.network.GetContext(), moduleName, sender, initialCoins))

			maxUnit := types.ConversionFactor().MulRaw(2).SubRaw(1)
			r := rand.New(rand.NewSource(SEED))

			// Expected balances tracking
			expectedSenderBal := initialBalance
			expectedRecipientBal := sdkmath.ZeroInt()

			mintCount, burnCount, sendCount := 0, 0, 0

			mintAmount := sdkmath.NewInt(0)
			burnAmount := sdkmath.NewInt(0)

			iterations := 1000
			for range iterations {
				op := r.Intn(3)
				switch op {
				case 0: // Mint to sender via module
					randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, maxUnit.BigInt())).AddRaw(1)
					mintCoins := cs(ci(types.ExtendedCoinDenom(), randAmount))
					if err := suite.network.App.PreciseBankKeeper.MintCoins(suite.network.GetContext(), moduleName, mintCoins); err != nil {
						continue
					}
					if err := suite.network.App.PreciseBankKeeper.SendCoinsFromModuleToAccount(suite.network.GetContext(), moduleName, sender, mintCoins); err != nil {
						continue
					}
					expectedSenderBal = expectedSenderBal.Add(randAmount)
					mintAmount = mintAmount.Add(randAmount)
					mintCount++

				case 1: // Burn from sender via module
					senderBal := suite.GetAllBalances(sender).AmountOf(types.ExtendedCoinDenom())
					if senderBal.IsZero() {
						continue
					}
					burnable := sdkmath.MinInt(senderBal, maxUnit)
					randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, burnable.BigInt())).AddRaw(1)
					burnCoins := cs(ci(types.ExtendedCoinDenom(), randAmount))
					if err := suite.network.App.PreciseBankKeeper.SendCoinsFromAccountToModule(suite.network.GetContext(), sender, moduleName, burnCoins); err != nil {
						continue
					}
					if err := suite.network.App.PreciseBankKeeper.BurnCoins(suite.network.GetContext(), moduleName, burnCoins); err != nil {
						continue
					}
					expectedSenderBal = expectedSenderBal.Sub(randAmount)
					burnAmount = burnAmount.Add(randAmount)
					burnCount++

				case 2: // Send from sender to recipient
					senderBal := suite.GetAllBalances(sender).AmountOf(types.ExtendedCoinDenom())
					if senderBal.IsZero() {
						continue
					}
					sendable := sdkmath.MinInt(senderBal, maxUnit)
					randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, sendable.BigInt())).AddRaw(1)
					sendCoins := cs(ci(types.ExtendedCoinDenom(), randAmount))
					if err := suite.network.App.PreciseBankKeeper.SendCoins(suite.network.GetContext(), sender, recipient, sendCoins); err != nil {
						continue
					}
					expectedSenderBal = expectedSenderBal.Sub(randAmount)
					expectedRecipientBal = expectedRecipientBal.Add(randAmount)
					sendCount++
				}
			}

			suite.T().Logf("Executed operations: %d mints, %d burns, %d sends", mintCount, burnCount, sendCount)

			// Check balances
			actualSenderBal := suite.GetAllBalances(sender).AmountOf(types.ExtendedCoinDenom())
			actualRecipientBal := suite.GetAllBalances(recipient).AmountOf(types.ExtendedCoinDenom())
			suite.Require().Equal(expectedSenderBal.BigInt().Cmp(actualSenderBal.BigInt()), 0, "Sender balance mismatch (expected: %s, actual: %s)", expectedSenderBal, actualSenderBal)
			suite.Require().Equal(expectedRecipientBal.BigInt().Cmp(actualRecipientBal.BigInt()), 0, "Recipient balance mismatch (expected: %s, actual: %s)", expectedRecipientBal, actualRecipientBal)

			// Check remainder
			expectedRemainder := burnAmount.Sub(mintAmount).Mod(types.ConversionFactor())
			actualRemainder := suite.network.App.PreciseBankKeeper.GetRemainderAmount(suite.network.GetContext())
			suite.Require().Equal(expectedRemainder.BigInt().Cmp(actualRemainder.BigInt()), 0, "Remainder mismatch (expected: %s, actual: %s)", expectedRemainder, actualRemainder)

			// Invariant check
			inv := keeper.AllInvariants(suite.network.App.PreciseBankKeeper)
			res, stop := inv(suite.network.GetContext())
			suite.Require().False(stop, "Invariant broken")
			suite.Require().Empty(res, "Unexpected invariant violation: %s", res)
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestSendEvmTx_RandomValueMultiDecimals() {
	maxGasLimit := int64(500000)
	defaultEVMCoinTransferGasLimit := int64(21000)

	tests := []struct {
		name    string
		chainID testconstants.ChainID
	}{
		{
			name:    "6 decimals",
			chainID: testconstants.SixDecimalsChainID,
		},
		{
			name:    "12 decimals",
			chainID: testconstants.TwelveDecimalsChainID,
		},
		{
			name:    "2 decimals",
			chainID: testconstants.TwoDecimalsChainID,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTestWithChainID(tt.chainID)

			sender := suite.keyring.GetKey(0)
			recipient := suite.keyring.GetKey(1)
			burnerAddr := common.HexToAddress("0x0000000000000000000000000000000000000000")

			baseFeeResp, err := suite.network.GetEvmClient().BaseFee(suite.network.GetContext(), &evmtypes.QueryBaseFeeRequest{})
			suite.Require().NoError(err)
			gasPrice := sdkmath.NewIntFromBigInt(baseFeeResp.BaseFee.BigInt())
			gasFee := gasPrice.Mul(sdkmath.NewInt(defaultEVMCoinTransferGasLimit))

			// Burn balance from sender except for initial balance
			initialBalance := types.ConversionFactor().MulRaw(100)
			senderBal := suite.GetAllBalances(sender.AccAddr).AmountOf(types.ExtendedCoinDenom()).Sub(gasFee).Sub(initialBalance)
			_, err = suite.factory.ExecuteEthTx(sender.Priv, evmtypes.EvmTxArgs{
				To:       &burnerAddr,
				Amount:   senderBal.BigInt(),
				GasLimit: uint64(defaultEVMCoinTransferGasLimit), //nolint:gosec // G115
				GasPrice: gasPrice.BigInt(),
			})
			suite.Require().NoError(err)

			// Burn balance from recipient
			recipientBal := suite.GetAllBalances(recipient.AccAddr).AmountOf(types.ExtendedCoinDenom()).Sub(gasFee)
			_, err = suite.factory.ExecuteEthTx(recipient.Priv, evmtypes.EvmTxArgs{
				To:       &burnerAddr,
				Amount:   recipientBal.BigInt(),
				GasLimit: uint64(defaultEVMCoinTransferGasLimit), //nolint:gosec // G115
				GasPrice: gasPrice.BigInt(),
			})
			suite.Require().NoError(err)

			err = suite.network.NextBlock()
			suite.Require().NoError(err)

			maxSendUnit := types.ConversionFactor().MulRaw(2).SubRaw(1)
			r := rand.New(rand.NewSource(SEED))

			expectedSenderBal := initialBalance
			expectedRecipientBal := sdkmath.ZeroInt()

			sentCount := 0
			for {
				gasLimit := r.Int63n(maxGasLimit-defaultEVMCoinTransferGasLimit) + defaultEVMCoinTransferGasLimit
				baseFeeResp, err = suite.network.GetEvmClient().BaseFee(suite.network.GetContext(), &evmtypes.QueryBaseFeeRequest{})
				suite.Require().NoError(err)
				gasPrice = sdkmath.NewIntFromBigInt(baseFeeResp.BaseFee.BigInt())

				// Generate random value to send
				randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, maxSendUnit.BigInt())).AddRaw(1)

				// Execute EVM coin transfer
				txRes, _ := suite.factory.ExecuteEthTx(sender.Priv, evmtypes.EvmTxArgs{
					To:       &recipient.Addr,
					Amount:   randAmount.BigInt(),
					GasLimit: uint64(gasLimit), //nolint:gosec // G115
					GasPrice: gasPrice.BigInt(),
				})
				err = suite.network.NextBlock()
				suite.Require().NoError(err)

				// Calculate gas fee used
				gasUsed := txRes.GasUsed
				gasFeeUsed := gasPrice.Mul(sdkmath.NewInt(gasUsed))
				expectedSenderBal = expectedSenderBal.Sub(gasFeeUsed)

				// break, if EVM coin transfer tx is failed
				sentCount++
				if txRes.IsErr() {
					break
				}

				// Update expected balances
				expectedSenderBal = expectedSenderBal.Sub(randAmount)
				expectedRecipientBal = expectedRecipientBal.Add(randAmount)
			}

			suite.T().Logf("Completed %d random evm sends", sentCount)

			// Check sender balance
			actualSenderBal := suite.GetAllBalances(sender.AccAddr).AmountOf(types.ExtendedCoinDenom())
			suite.Require().Equal(expectedSenderBal.BigInt().Cmp(actualSenderBal.BigInt()), 0,
				"Sender balance mismatch (expected: %s, actual: %s)", expectedSenderBal, actualSenderBal)

			// Check recipient balance
			actualRecipientBal := suite.GetAllBalances(recipient.AccAddr).AmountOf(types.ExtendedCoinDenom())
			suite.Require().Equal(expectedRecipientBal.BigInt().Cmp(actualRecipientBal.BigInt()), 0,
				"Recipient balance mismatch (expected: %s, actual: %s)", expectedRecipientBal, actualRecipientBal)

			// Check invariants
			inv := keeper.AllInvariants(suite.network.App.PreciseBankKeeper)
			res, stop := inv(suite.network.GetContext())
			suite.Require().False(stop, "Invariant broken")
			suite.Require().Empty(res, "Unexpected invariant violation: %s", res)
		})
	}
}

func (suite *KeeperIntegrationTestSuite) TestWATOMWrapUnwrap_MultiDecimal() {
	tests := []struct {
		name    string
		chainID testconstants.ChainID
	}{
		{
			name:    "6 decimals",
			chainID: testconstants.SixDecimalsChainID,
		},
		{
			name:    "12 decimals",
			chainID: testconstants.TwelveDecimalsChainID,
		},
		{
			name:    "2 decimals",
			chainID: testconstants.TwoDecimalsChainID,
		},
	}

	for _, tt := range tests {
		suite.Run(tt.name, func() {
			suite.SetupTestWithChainID(tt.chainID)

			sender := suite.keyring.GetKey(0)
			amount := big.NewInt(1)

			// Deploy WATOM contract
			watomAddr, err := suite.factory.DeployContract(
				sender.Priv,
				evmtypes.EvmTxArgs{},
				factory.ContractDeploymentData{
					Contract: contracts.WATOMContract,
				},
			)
			suite.Require().NoError(err)

			err = suite.network.NextBlock()
			suite.Require().NoError(err)

			baseFeeRes, err := suite.network.GetEvmClient().BaseFee(suite.network.GetContext(), &evmtypes.QueryBaseFeeRequest{})
			suite.Require().NoError(err)

			// Call deposit() with msg.value = wrapAmount
			_, err = suite.factory.ExecuteContractCall(
				sender.Priv,
				evmtypes.EvmTxArgs{
					To:        &watomAddr,
					Amount:    amount,
					GasLimit:  100_000,
					GasFeeCap: baseFeeRes.BaseFee.BigInt(),
					GasTipCap: big.NewInt(1),
				},
				factory.CallArgs{
					ContractABI: contracts.WATOMContract.ABI,
					MethodName:  "deposit",
				},
			)
			suite.Require().NoError(err)
			err = suite.network.NextBlock()
			suite.Require().NoError(err)

			// Check WATOM balance == wrapAmount
			bal, err := utils.GetERC20Balance(suite.network, watomAddr, sender.Addr)
			suite.Require().NoError(err)
			suite.Require().Equal(amount.Cmp(bal), 0, "WATOM balance should match deposited amount (expected: %s, actual: %s)", amount, bal)

			baseFeeRes, err = suite.network.GetEvmClient().BaseFee(suite.network.GetContext(), &evmtypes.QueryBaseFeeRequest{})
			suite.Require().NoError(err)

			// Call withdraw(wrapAmount)
			_, err = suite.factory.ExecuteContractCall(
				sender.Priv,
				evmtypes.EvmTxArgs{
					To:        &watomAddr,
					GasLimit:  100_000,
					GasFeeCap: baseFeeRes.BaseFee.BigInt(),
					GasTipCap: big.NewInt(1),
				},
				factory.CallArgs{
					ContractABI: contracts.WATOMContract.ABI,
					MethodName:  "withdraw",
					Args:        []interface{}{amount},
				},
			)
			suite.Require().NoError(err)
			suite.Require().NoError(suite.network.NextBlock())

			// Final WATOM balance should be 0
			bal, err = utils.GetERC20Balance(suite.network, watomAddr, sender.Addr)
			suite.Require().NoError(err)
			suite.Require().Equal("0", bal.String(), "WATOM balance should be zero after withdraw")
		})
	}
}
