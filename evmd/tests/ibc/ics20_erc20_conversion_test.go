package ibc

import (
	"fmt"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/tests/integration"
	"github.com/cosmos/evm/precompiles/ics20"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	transfertypes "github.com/cosmos/ibc-go/v11/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v11/modules/core/02-client/types"

	sdkmath "cosmossdk.io/math"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

type ICS20ERC20ConversionTestSuite struct {
	suite.Suite

	coordinator *evmibctesting.Coordinator

	chainA           *evmibctesting.TestChain
	chainAPrecompile *ics20.Precompile
	chainB           *evmibctesting.TestChain
	chainBPrecompile *ics20.Precompile
}

func (suite *ICS20ERC20ConversionTestSuite) SetupTest() {
	suite.coordinator = evmibctesting.NewCoordinator(suite.T(), 2, 0, integration.SetupEvmd)
	suite.chainA = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(1))
	suite.chainB = suite.coordinator.GetChain(evmibctesting.GetEvmChainID(2))

	evmAppA := suite.chainA.App.(*evmd.EVMD)
	suite.chainAPrecompile = ics20.NewPrecompile(
		evmAppA.BankKeeper,
		*evmAppA.StakingKeeper,
		evmAppA.TransferKeeper,
		evmAppA.IBCKeeper.ChannelKeeper,
		evmAppA.Erc20Keeper,
	)
	evmAppB := suite.chainB.App.(*evmd.EVMD)
	suite.chainBPrecompile = ics20.NewPrecompile(
		evmAppB.BankKeeper,
		*evmAppB.StakingKeeper,
		evmAppB.TransferKeeper,
		evmAppB.IBCKeeper.ChannelKeeper,
		evmAppB.Erc20Keeper,
	)
}

func TestICS20ERC20ConversionTestSuite(t *testing.T) {
	suite.Run(t, new(ICS20ERC20ConversionTestSuite))
}

// TestTransferWithERC20Conversion tests IBC transfers with ERC20 token conversion
func (suite *ICS20ERC20ConversionTestSuite) TestTransferWithERC20Conversion() {
	var (
		denom       string
		amount      sdkmath.Int
		sender      common.Address
		nativeErc20 *NativeErc20Info
		path        *evmibctesting.Path
	)

	receiver := suite.chainB.SenderAccount.GetAddress().String()
	timeoutHeight := clienttypes.NewHeight(1, 110)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"pass - no token pair",
			func() {
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				var err error
				denom, err = evmAppA.StakingKeeper.BondDenom(suite.chainA.GetContext())
				suite.Require().NoError(err)
				amount = sdkmath.NewInt(10)
				sender = common.BytesToAddress(suite.chainA.SenderAccount.GetAddress().Bytes())
			},
			true,
		},
		{
			"no-op - disabled erc20 by params - sufficient sdk.Coins balance",
			func() {
				// Deploy and mint ERC20
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA, suite.chainA.SenderAccounts[0])
				denom = nativeErc20.Denom
				amount = sdkmath.NewInt(nativeErc20.InitialBal.Int64())

				// Convert ERC20 to coins
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				_, err := evmAppA.Erc20Keeper.ConvertERC20(suite.chainA.GetContext(), &erc20types.MsgConvertERC20{
					ContractAddress: nativeErc20.ContractAddr.Hex(),
					Amount:          amount,
					Receiver:        suite.chainA.SenderAccount.GetAddress().String(),
					Sender:          nativeErc20.Account.Hex(),
				})
				suite.Require().NoError(err)
				suite.chainA.NextBlock()

				// Disable ERC20
				params := evmAppA.Erc20Keeper.GetParams(suite.chainA.GetContext())
				params.EnableErc20 = false
				err = evmAppA.Erc20Keeper.SetParams(suite.chainA.GetContext(), params)
				suite.Require().NoError(err)
				suite.chainA.NextBlock()

				sender = nativeErc20.Account
			},
			true,
		},
		{
			"error - disabled erc20 by params - insufficient sdk.Coins balance",
			func() {
				// Deploy and mint ERC20 but don't convert
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA, suite.chainA.SenderAccounts[0])
				denom = nativeErc20.Denom
				amount = sdkmath.NewInt(nativeErc20.InitialBal.Int64())

				evmAppA := suite.chainA.App.(*evmd.EVMD)
				ctx := suite.chainA.GetContext()

				// No conversion to IBC coin, so the balance is insufficient
				suite.Require().EqualValues(
					evmAppA.BankKeeper.GetBalance(ctx, suite.chainA.SenderAccount.GetAddress(), denom).Amount,
					sdkmath.ZeroInt(),
					"Bank balance should be zero since we didn't convert",
				)

				// Disable ERC20 without converting
				params := evmAppA.Erc20Keeper.GetParams(ctx)
				params.EnableErc20 = false
				err := evmAppA.Erc20Keeper.SetParams(ctx, params)
				suite.Require().NoError(err)
				suite.chainA.NextBlock()

				sender = nativeErc20.Account
			},
			false,
		},
		{
			"error - pair not registered",
			func() {
				denom = "unregistered"
				amount = sdkmath.NewInt(10)
				sender = common.BytesToAddress(suite.chainA.SenderAccount.GetAddress().Bytes())
			},
			false,
		},
		{
			"no-op - pair is disabled",
			func() {
				// Deploy and mint ERC20
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA, suite.chainA.SenderAccounts[0])
				denom = nativeErc20.Denom
				amount = sdkmath.NewInt(nativeErc20.InitialBal.Int64())

				// Convert to coins first
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				_, err := evmAppA.Erc20Keeper.ConvertERC20(suite.chainA.GetContext(), &erc20types.MsgConvertERC20{
					ContractAddress: nativeErc20.ContractAddr.Hex(),
					Amount:          amount,
					Receiver:        suite.chainA.SenderAccount.GetAddress().String(),
					Sender:          nativeErc20.Account.Hex(),
				})
				suite.Require().NoError(err)
				suite.chainA.NextBlock()

				// Disable the token pair
				govAddr := authtypes.NewModuleAddress(govtypes.ModuleName).String()
				_, err = evmAppA.Erc20Keeper.ToggleConversion(suite.chainA.GetContext(), &erc20types.MsgToggleConversion{
					Token:     denom,
					Authority: govAddr,
				})
				suite.Require().NoError(err)
				suite.chainA.NextBlock()

				sender = nativeErc20.Account
			},
			true,
		},
		{
			"pass - has enough balance in erc20 - need to convert",
			func() {
				// Deploy and mint ERC20 but don't convert - transfer should auto-convert
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA, suite.chainA.SenderAccounts[0])
				denom = nativeErc20.Denom
				amount = sdkmath.NewInt(nativeErc20.InitialBal.Int64())

				// Verify denom format is correct
				suite.Require().Equal(
					erc20types.CreateDenom(nativeErc20.ContractAddr.String()),
					denom,
					"Denom should match the ERC20 contract address format",
				)

				sender = nativeErc20.Account
			},
			true,
		},
		{
			"pass - has enough balance in coins",
			func() {
				// Deploy and mint ERC20
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA, suite.chainA.SenderAccounts[0])
				denom = nativeErc20.Denom
				amount = sdkmath.NewInt(nativeErc20.InitialBal.Int64())

				// Convert to coins
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				_, err := evmAppA.Erc20Keeper.ConvertERC20(suite.chainA.GetContext(), &erc20types.MsgConvertERC20{
					ContractAddress: nativeErc20.ContractAddr.Hex(),
					Amount:          amount,
					Receiver:        suite.chainA.SenderAccount.GetAddress().String(),
					Sender:          nativeErc20.Account.Hex(),
				})
				suite.Require().NoError(err)
				suite.chainA.NextBlock()

				sender = nativeErc20.Account
			},
			true,
		},
		{
			"error - fail conversion - no balance in erc20",
			func() {
				// Deploy and register ERC20 but don't mint
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA, suite.chainA.SenderAccounts[0])
				denom = nativeErc20.Denom
				// Request more than minted
				amount = sdkmath.NewInt(nativeErc20.InitialBal.Int64() * 2)
				sender = nativeErc20.Account
			},
			false,
		},
		{
			"pass - verify correct prefix trimming for ERC20 native tokens",
			func() {
				// Deploy and mint ERC20
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA, suite.chainA.SenderAccounts[0])
				denom = nativeErc20.Denom
				amount = sdkmath.NewInt(nativeErc20.InitialBal.Int64())

				evmAppA := suite.chainA.App.(*evmd.EVMD)
				ctx := suite.chainA.GetContext()

				// Create a denom with erc20: prefix
				erc20Denom := erc20types.CreateDenom(nativeErc20.ContractAddr.String())
				suite.Require().Equal(erc20types.Erc20NativeCoinDenomPrefix+nativeErc20.ContractAddr.String(), erc20Denom)

				// Verify that GetTokenPairID works correctly with the contract address (hex string)
				pairIDFromAddress := evmAppA.Erc20Keeper.GetTokenPairID(ctx, nativeErc20.ContractAddr.String())
				suite.Require().NotEmpty(pairIDFromAddress)

				// Verify that GetTokenPairID works correctly with the full denom
				pairIDFromDenom := evmAppA.Erc20Keeper.GetTokenPairID(ctx, erc20Denom)
				suite.Require().NotEmpty(pairIDFromDenom)

				// Both should return the same pair ID
				suite.Require().Equal(pairIDFromAddress, pairIDFromDenom)

				sender = nativeErc20.Account
			},
			true,
		},
		{
			"no-op - fail transfer",
			func() {
				evmAppA := suite.chainA.App.(*evmd.EVMD)
				ctx := suite.chainA.GetContext()
				senderAcc := suite.chainA.SenderAccount.GetAddress()

				// Create a fake IBC voucher denom (IBC-transferred token)
				denom = "ibc/DF63978F803A2E27CA5CC9B7631654CCF0BBC788B3B7F0A10200508E37C70992"

				// Register it as an ERC20 extension
				_, err := evmAppA.Erc20Keeper.RegisterERC20Extension(ctx, denom)
				suite.Require().NoError(err)
				suite.chainA.NextBlock()

				// Verify the pair exists
				pairID := evmAppA.Erc20Keeper.GetTokenPairID(ctx, denom)
				suite.Require().NotEmpty(pairID, "Token pair should be registered")

				pair, found := evmAppA.Erc20Keeper.GetTokenPair(ctx, pairID)
				suite.Require().True(found)
				suite.Require().Equal(pair.Denom, denom)

				// Try to transfer without having any balance (should fail)
				amount = sdkmath.NewInt(10)
				sender = common.BytesToAddress(senderAcc)
			},
			false,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()

			// Setup IBC path
			path = evmibctesting.NewTransferPath(suite.chainA, suite.chainB)
			path.Setup()

			// Run test-specific setup
			tc.malleate()

			// Call precompile transfer
			data, err := suite.chainAPrecompile.ABI.Pack(
				"transfer",
				transfertypes.PortID,
				path.EndpointA.ChannelID,
				denom,
				amount.BigInt(),
				sender,
				receiver,
				timeoutHeight,
				uint64(0),
				"",
			)
			suite.Require().NoError(err)

			res, _, _, err := suite.chainA.SendEvmTx(
				suite.chainA.SenderAccounts[0],
				0,
				suite.chainAPrecompile.Address(),
				big.NewInt(0),
				data,
				0,
			)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(uint32(0), res.Code, res.Log)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}

// TestPrefixTrimming specifically tests that the erc20: prefix is correctly handled
func (suite *ICS20ERC20ConversionTestSuite) TestPrefixTrimming() {
	var (
		denom       string
		amount      sdkmath.Int
		sender      common.Address
		nativeErc20 *NativeErc20Info
		path        *evmibctesting.Path
	)

	receiver := suite.chainB.SenderAccount.GetAddress().String()
	timeoutHeight := clienttypes.NewHeight(1, 110)

	testCases := []struct {
		name     string
		malleate func()
		expPass  bool
	}{
		{
			"pass - correct prefix trimming erc20:",
			func() {
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA, suite.chainA.SenderAccounts[0])
				denom = nativeErc20.Denom
				amount = sdkmath.NewInt(nativeErc20.InitialBal.Int64())

				// Verify the denom has the correct prefix
				suite.Require().Contains(denom, erc20types.Erc20NativeCoinDenomPrefix)

				evmAppA := suite.chainA.App.(*evmd.EVMD)
				ctx := suite.chainA.GetContext()

				// TEST: Verify that the prefix trimming works correctly
				// The Transfer method should trim "erc20:" prefix to get the hex address
				expectedTrimmed := strings.TrimPrefix(denom, erc20types.Erc20NativeCoinDenomPrefix)
				suite.Require().Equal(nativeErc20.ContractAddr.String(), expectedTrimmed,
					"Correct: Trimming 'erc20:' yields the contract address")

				// TEST: Verify that incorrect prefix trimming would fail
				// If we incorrectly trim "erc20/" instead of "erc20:", we'd get the wrong string
				incorrectTrimmed := strings.TrimPrefix(denom, erc20types.ModuleName+"/")
				suite.Require().NotEqual(nativeErc20.ContractAddr.String(), incorrectTrimmed,
					"Bug: Trimming 'erc20/' does not yield the contract address")
				suite.Require().Equal(denom, incorrectTrimmed,
					"Since 'erc20/' is not in the string, TrimPrefix returns it unchanged")

				// Verify that GetTokenPairID works correctly with the contract address (hex string)
				pairIDFromAddress := evmAppA.Erc20Keeper.GetTokenPairID(ctx, nativeErc20.ContractAddr.String())
				suite.Require().NotEmpty(pairIDFromAddress)

				// Verify that GetTokenPairID works correctly with the full denom
				pairIDFromDenom := evmAppA.Erc20Keeper.GetTokenPairID(ctx, denom)
				suite.Require().NotEmpty(pairIDFromDenom)

				// Both should return the same pair ID
				suite.Require().Equal(pairIDFromAddress, pairIDFromDenom)

				sender = nativeErc20.Account
			},
			true,
		},
		{
			"pass - demonstrate bug impact",
			func() {
				nativeErc20 = SetupNativeErc20(suite.T(), suite.chainA, suite.chainA.SenderAccounts[0])
				denom = nativeErc20.Denom
				amount = sdkmath.NewInt(nativeErc20.InitialBal.Int64())

				evmAppA := suite.chainA.App.(*evmd.EVMD)
				ctx := suite.chainA.GetContext()

				// Demonstrate the bug's impact: incorrect vs correct prefix trimming
				// The denom format is "erc20:0x1234..." where "erc20:" is the prefix

				// CORRECT trimming: trim "erc20:" to get the hex address
				correctTrimmed := strings.TrimPrefix(denom, erc20types.Erc20NativeCoinDenomPrefix)
				suite.Require().Equal(nativeErc20.ContractAddr.String(), correctTrimmed,
					"Trimming 'erc20:' should yield the contract address")

				// INCORRECT trimming: trim "erc20/" instead (the bug)
				// This doesn't match the actual prefix, so TrimPrefix returns the string unchanged
				incorrectTrimmed := strings.TrimPrefix(denom, erc20types.ModuleName+"/")
				suite.Require().Equal(denom, incorrectTrimmed,
					"Trimming 'erc20/' should not change the string since prefix is 'erc20:'")
				suite.Require().NotEqual(nativeErc20.ContractAddr.String(), incorrectTrimmed,
					"Incorrect trimming does not yield the contract address")

				// Demonstrate why the bug wasn't caught earlier:
				// Both lookups work due to dual mapping in the keeper
				// The keeper maps both "0x1234..." and "erc20:0x1234..." to the same pair
				pairIDFromCorrect := evmAppA.Erc20Keeper.GetTokenPairID(ctx, correctTrimmed)     // "0x1234..."
				pairIDFromIncorrect := evmAppA.Erc20Keeper.GetTokenPairID(ctx, incorrectTrimmed) // "erc20:0x1234..."

				suite.Require().NotEmpty(pairIDFromCorrect)
				suite.Require().NotEmpty(pairIDFromIncorrect)
				suite.Require().Equal(pairIDFromCorrect, pairIDFromIncorrect,
					"Both lookups succeed due to dual mapping, masking the prefix bug")

				sender = nativeErc20.Account
			},
			true,
		},
	}

	for _, tc := range testCases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()

			// Setup IBC path
			path = evmibctesting.NewTransferPath(suite.chainA, suite.chainB)
			path.Setup()

			// Run test-specific setup
			tc.malleate()

			// Call precompile transfer
			data, err := suite.chainAPrecompile.ABI.Pack(
				"transfer",
				transfertypes.PortID,
				path.EndpointA.ChannelID,
				denom,
				amount.BigInt(),
				sender,
				receiver,
				timeoutHeight,
				uint64(0),
				"",
			)
			suite.Require().NoError(err)

			res, _, _, err := suite.chainA.SendEvmTx(
				suite.chainA.SenderAccounts[0],
				0,
				suite.chainAPrecompile.Address(),
				big.NewInt(0),
				data,
				0,
			)

			if tc.expPass {
				suite.Require().NoError(err)
				suite.Require().Equal(uint32(0), res.Code, res.Log)
			} else {
				suite.Require().Error(err)
			}
		})
	}
}
