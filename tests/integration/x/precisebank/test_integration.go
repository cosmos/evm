package precisebank

import (
	"math/big"
	"math/rand"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/contracts"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	"github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperIntegrationTestSuite) TestMintBurnSendCoinsRandomValueMultiDecimals() {
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
		s.Run(tt.name, func() {
			s.SetupTestWithChainID(tt.chainID)

			moduleName := evmtypes.ModuleName
			sender := sdk.AccAddress([]byte{1})
			recipient := sdk.AccAddress([]byte{2})

			// Mint initial balance to sender
			initialBalance := types.ConversionFactor().MulRaw(100)
			initialCoins := cs(ci(types.ExtendedCoinDenom(), initialBalance))
			s.Require().NoError(s.network.App.GetPreciseBankKeeper().MintCoins(s.network.GetContext(), moduleName, initialCoins))
			s.Require().NoError(s.network.App.GetPreciseBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), moduleName, sender, initialCoins))

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
					if err := s.network.App.GetPreciseBankKeeper().MintCoins(s.network.GetContext(), moduleName, mintCoins); err != nil {
						continue
					}
					if err := s.network.App.GetPreciseBankKeeper().SendCoinsFromModuleToAccount(s.network.GetContext(), moduleName, sender, mintCoins); err != nil {
						continue
					}
					expectedSenderBal = expectedSenderBal.Add(randAmount)
					mintAmount = mintAmount.Add(randAmount)
					mintCount++

				case 1: // Burn from sender via module
					senderBal := s.GetAllBalances(sender).AmountOf(types.ExtendedCoinDenom())
					if senderBal.IsZero() {
						continue
					}
					burnable := sdkmath.MinInt(senderBal, maxUnit)
					randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, burnable.BigInt())).AddRaw(1)
					burnCoins := cs(ci(types.ExtendedCoinDenom(), randAmount))
					if err := s.network.App.GetPreciseBankKeeper().SendCoinsFromAccountToModule(s.network.GetContext(), sender, moduleName, burnCoins); err != nil {
						continue
					}
					if err := s.network.App.GetPreciseBankKeeper().BurnCoins(s.network.GetContext(), moduleName, burnCoins); err != nil {
						continue
					}
					expectedSenderBal = expectedSenderBal.Sub(randAmount)
					burnAmount = burnAmount.Add(randAmount)
					burnCount++

				case 2: // Send from sender to recipient
					senderBal := s.GetAllBalances(sender).AmountOf(types.ExtendedCoinDenom())
					if senderBal.IsZero() {
						continue
					}
					sendable := sdkmath.MinInt(senderBal, maxUnit)
					randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, sendable.BigInt())).AddRaw(1)
					sendCoins := cs(ci(types.ExtendedCoinDenom(), randAmount))
					if err := s.network.App.GetPreciseBankKeeper().SendCoins(s.network.GetContext(), sender, recipient, sendCoins); err != nil {
						continue
					}
					expectedSenderBal = expectedSenderBal.Sub(randAmount)
					expectedRecipientBal = expectedRecipientBal.Add(randAmount)
					sendCount++
				}
			}

			s.T().Logf("Executed operations: %d mints, %d burns, %d sends", mintCount, burnCount, sendCount)

			// Check balances
			actualSenderBal := s.GetAllBalances(sender).AmountOf(types.ExtendedCoinDenom())
			actualRecipientBal := s.GetAllBalances(recipient).AmountOf(types.ExtendedCoinDenom())
			s.Require().Equal(expectedSenderBal.BigInt().Cmp(actualSenderBal.BigInt()), 0, "Sender balance mismatch (expected: %s, actual: %s)", expectedSenderBal, actualSenderBal)
			s.Require().Equal(expectedRecipientBal.BigInt().Cmp(actualRecipientBal.BigInt()), 0, "Recipient balance mismatch (expected: %s, actual: %s)", expectedRecipientBal, actualRecipientBal)

			// Check remainder
			expectedRemainder := burnAmount.Sub(mintAmount).Mod(types.ConversionFactor())
			actualRemainder := s.network.App.GetPreciseBankKeeper().GetRemainderAmount(s.network.GetContext())
			s.Require().Equal(expectedRemainder.BigInt().Cmp(actualRemainder.BigInt()), 0, "Remainder mismatch (expected: %s, actual: %s)", expectedRemainder, actualRemainder)
		})
	}
}

func (s *KeeperIntegrationTestSuite) TestSendEvmTxRandomValueMultiDecimals() {
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
		s.Run(tt.name, func() {
			s.SetupTestWithChainID(tt.chainID)

			sender := s.keyring.GetKey(0)
			recipient := s.keyring.GetKey(1)
			burnerAddr := common.HexToAddress("0x0000000000000000000000000000000000000000")

			baseFeeResp, err := s.network.GetEvmClient().BaseFee(s.network.GetContext(), &evmtypes.QueryBaseFeeRequest{})
			s.Require().NoError(err)
			gasPrice := sdkmath.NewIntFromBigInt(baseFeeResp.BaseFee.BigInt())
			gasFee := gasPrice.Mul(sdkmath.NewInt(defaultEVMCoinTransferGasLimit))

			// Burn balance from sender except for initial balance
			initialBalance := types.ConversionFactor().MulRaw(100)
			senderBal := s.GetAllBalances(sender.AccAddr).AmountOf(types.ExtendedCoinDenom()).Sub(gasFee).Sub(initialBalance)
			_, err = s.factory.ExecuteEthTx(sender.Priv, evmtypes.EvmTxArgs{
				To:       &burnerAddr,
				Amount:   senderBal.BigInt(),
				GasLimit: uint64(defaultEVMCoinTransferGasLimit), //nolint:gosec // G115
				GasPrice: gasPrice.BigInt(),
			})
			s.Require().NoError(err)

			// Burn balance from recipient
			recipientBal := s.GetAllBalances(recipient.AccAddr).AmountOf(types.ExtendedCoinDenom()).Sub(gasFee)
			_, err = s.factory.ExecuteEthTx(recipient.Priv, evmtypes.EvmTxArgs{
				To:       &burnerAddr,
				Amount:   recipientBal.BigInt(),
				GasLimit: uint64(defaultEVMCoinTransferGasLimit), //nolint:gosec // G115
				GasPrice: gasPrice.BigInt(),
			})
			s.Require().NoError(err)

			err = s.network.NextBlock()
			s.Require().NoError(err)

			maxSendUnit := types.ConversionFactor().MulRaw(2).SubRaw(1)
			r := rand.New(rand.NewSource(SEED))

			expectedSenderBal := initialBalance
			expectedRecipientBal := sdkmath.ZeroInt()

			sentCount := 0
			for {
				gasLimit := r.Int63n(maxGasLimit-defaultEVMCoinTransferGasLimit) + defaultEVMCoinTransferGasLimit
				baseFeeResp, err = s.network.GetEvmClient().BaseFee(s.network.GetContext(), &evmtypes.QueryBaseFeeRequest{})
				s.Require().NoError(err)
				gasPrice = sdkmath.NewIntFromBigInt(baseFeeResp.BaseFee.BigInt())

				// Generate random value to send
				randAmount := sdkmath.NewIntFromBigInt(new(big.Int).Rand(r, maxSendUnit.BigInt())).AddRaw(1)

				// Execute EVM coin transfer
				txRes, _ := s.factory.ExecuteEthTx(sender.Priv, evmtypes.EvmTxArgs{
					To:       &recipient.Addr,
					Amount:   randAmount.BigInt(),
					GasLimit: uint64(gasLimit), //nolint:gosec // G115
					GasPrice: gasPrice.BigInt(),
				})
				err = s.network.NextBlock()
				s.Require().NoError(err)

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

			s.T().Logf("Completed %d random evm sends", sentCount)

			// Check sender balance
			actualSenderBal := s.GetAllBalances(sender.AccAddr).AmountOf(types.ExtendedCoinDenom())
			s.Require().Equal(expectedSenderBal.BigInt().Cmp(actualSenderBal.BigInt()), 0,
				"Sender balance mismatch (expected: %s, actual: %s)", expectedSenderBal, actualSenderBal)

			// Check recipient balance
			actualRecipientBal := s.GetAllBalances(recipient.AccAddr).AmountOf(types.ExtendedCoinDenom())
			s.Require().Equal(expectedRecipientBal.BigInt().Cmp(actualRecipientBal.BigInt()), 0,
				"Recipient balance mismatch (expected: %s, actual: %s)", expectedRecipientBal, actualRecipientBal)
		})
	}
}

func (s *KeeperIntegrationTestSuite) TestWATOMWrapUnwrapMultiDecimal() {
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
		s.Run(tt.name, func() {
			s.SetupTestWithChainID(tt.chainID)

			sender := s.keyring.GetKey(0)
			amount := big.NewInt(1)

			// Deploy WATOM contract
			watomAddr, err := s.factory.DeployContract(
				sender.Priv,
				evmtypes.EvmTxArgs{},
				testutiltypes.ContractDeploymentData{
					Contract: contracts.WATOMContract,
				},
			)
			s.Require().NoError(err)

			err = s.network.NextBlock()
			s.Require().NoError(err)

			baseFeeRes, err := s.network.GetEvmClient().BaseFee(s.network.GetContext(), &evmtypes.QueryBaseFeeRequest{})
			s.Require().NoError(err)

			// Call deposit() with msg.value = wrapAmount
			_, err = s.factory.ExecuteContractCall(
				sender.Priv,
				evmtypes.EvmTxArgs{
					To:        &watomAddr,
					Amount:    amount,
					GasLimit:  100_000,
					GasFeeCap: baseFeeRes.BaseFee.BigInt(),
					GasTipCap: big.NewInt(1),
				},
				testutiltypes.CallArgs{
					ContractABI: contracts.WATOMContract.ABI,
					MethodName:  "deposit",
				},
			)
			s.Require().NoError(err)
			err = s.network.NextBlock()
			s.Require().NoError(err)

			// Check WATOM balance == wrapAmount
			bal, err := utils.GetERC20Balance(s.network, watomAddr, sender.Addr)
			s.Require().NoError(err)
			s.Require().Equal(amount.Cmp(bal), 0, "WATOM balance should match deposited amount (expected: %s, actual: %s)", amount, bal)

			baseFeeRes, err = s.network.GetEvmClient().BaseFee(s.network.GetContext(), &evmtypes.QueryBaseFeeRequest{})
			s.Require().NoError(err)

			// Call withdraw(wrapAmount)
			_, err = s.factory.ExecuteContractCall(
				sender.Priv,
				evmtypes.EvmTxArgs{
					To:        &watomAddr,
					GasLimit:  100_000,
					GasFeeCap: baseFeeRes.BaseFee.BigInt(),
					GasTipCap: big.NewInt(1),
				},
				testutiltypes.CallArgs{
					ContractABI: contracts.WATOMContract.ABI,
					MethodName:  "withdraw",
					Args:        []interface{}{amount},
				},
			)
			s.Require().NoError(err)
			s.Require().NoError(s.network.NextBlock())

			// Final WATOM balance should be 0
			bal, err = utils.GetERC20Balance(s.network, watomAddr, sender.Addr)
			s.Require().NoError(err)
			s.Require().Equal("0", bal.String(), "WATOM balance should be zero after withdraw")
		})
	}
}
