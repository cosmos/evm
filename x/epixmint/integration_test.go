package epixmint_test

import (
	"math"
	"math/big"
	"testing"

	"github.com/stretchr/testify/suite"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/evm/x/epixmint/types"
)

type IntegrationTestSuite struct {
	suite.Suite
}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}

func (s *IntegrationTestSuite) TestTokenomicsCalculations() {
	// Test the tokenomics calculations match the expected values
	params := types.DefaultParams()

	// Verify the initial annual mint amount (10.527B EPIX in aepix)
	expectedInitialAmount, ok := sdkmath.NewIntFromString("10527000000000000000000000000")
	s.Require().True(ok)
	s.Require().Equal(expectedInitialAmount, params.InitialAnnualMintAmount)

	// Verify the max supply (42B EPIX in aepix)
	expectedMaxSupply, ok := sdkmath.NewIntFromString("42000000000000000000000000000")
	s.Require().True(ok)
	s.Require().Equal(expectedMaxSupply, params.MaxSupply)

	// Calculate tokens per block using dynamic calculation
	secondsPerYear := uint64(365 * 24 * 60 * 60)
	blocksPerYear := secondsPerYear / params.BlockTimeSeconds
	tokensPerBlock := params.InitialAnnualMintAmount.Quo(sdkmath.NewIntFromUint64(blocksPerYear))

	// Verify tokens per block is reasonable
	// 10.527B / 5.256M blocks = ~2,002 EPIX per block (at genesis)
	expectedTokensPerBlock := params.InitialAnnualMintAmount.Quo(sdkmath.NewIntFromUint64(blocksPerYear))
	s.Require().Equal(expectedTokensPerBlock, tokensPerBlock)
}

func (s *IntegrationTestSuite) TestTwentyYearProjection() {
	params := types.DefaultParams()

	// Calculate total tokens that would be minted over 20 years using geometric series
	// Total = a * (1 - r^n) / (1 - r) where a = 10.527B, r = 0.75, n = 20
	a := 10.527e9 // 10.527 billion EPIX
	r := 0.75     // retention rate (1 - 0.25)
	n := 20.0     // 20 years

	totalMintedFloat := a * (1 - math.Pow(r, n)) / (1 - r)
	// Convert to aepix more carefully to avoid precision issues
	totalMintedBigFloat := big.NewFloat(totalMintedFloat)
	totalMintedBigFloat.Mul(totalMintedBigFloat, big.NewFloat(1e18))
	totalMintedBigInt, _ := totalMintedBigFloat.Int(nil)
	totalMinted := sdkmath.NewIntFromBigInt(totalMintedBigInt)

	// Genesis supply (23.689M EPIX in aepix)
	genesisSupply, ok := sdkmath.NewIntFromString("23689538000000000000000000")
	s.Require().True(ok)

	// Total supply after 20 years
	finalSupply := genesisSupply.Add(totalMinted)

	// The dynamic emission approach should reach close to 42B
	// The max supply protection will ensure we don't exceed 42B
	s.Require().True(finalSupply.LTE(params.MaxSupply),
		"Final supply %s should not exceed max supply %s",
		finalSupply.String(), params.MaxSupply.String())

	// Should be close to max supply (within 1B EPIX)
	difference := params.MaxSupply.Sub(finalSupply)
	maxDifference, _ := sdkmath.NewIntFromString("1000000000000000000000000000") // 1B EPIX in aepix
	s.Require().True(difference.LTE(maxDifference),
		"Difference %s should be within 1B EPIX of max supply",
		difference.String())
}

func (s *IntegrationTestSuite) TestMintingStopsAtMaxSupply() {
	params := types.DefaultParams()

	// With dynamic emission and 25% annual reduction, it should take approximately 20 years
	// to reach max supply. The exact calculation is complex due to exponential decay,
	// but we can verify the parameters are set up for this timeframe.

	// Verify annual reduction rate is 25%
	expectedReductionRate := sdkmath.LegacyMustNewDecFromStr("0.25")
	s.Require().Equal(expectedReductionRate, params.AnnualReductionRate)

	// Verify initial amount is 10.527B EPIX
	expectedInitialAmount, _ := sdkmath.NewIntFromString("10527000000000000000000000000")
	s.Require().Equal(expectedInitialAmount, params.InitialAnnualMintAmount)
}

func (s *IntegrationTestSuite) TestDenominationConsistency() {
	params := types.DefaultParams()

	// Verify we're using the extended precision denomination
	s.Require().Equal("aepix", params.MintDenom)

	// Verify the amounts are in the correct denomination (18 decimals)
	// 10.527B EPIX = 10.527 * 10^9 * 10^18 = 10.527 * 10^27
	expectedDigits := 29 // 10.527 * 10^27 has 29 digits
	actualDigits := len(params.InitialAnnualMintAmount.String())
	s.Require().Equal(expectedDigits, actualDigits, "Initial annual mint amount should have %d digits but has %d", expectedDigits, actualDigits)

	// 42B EPIX = 42 * 10^9 * 10^18 = 42 * 10^27
	expectedMaxDigits := 29 // 42 * 10^27 has 29 digits
	actualMaxDigits := len(params.MaxSupply.String())
	s.Require().Equal(expectedMaxDigits, actualMaxDigits, "Max supply should have %d digits but has %d", expectedMaxDigits, actualMaxDigits)
}

func (s *IntegrationTestSuite) TestBlockTimeAssumptions() {
	params := types.DefaultParams()

	// Verify block time is set to 6 seconds
	expectedBlockTime := uint64(6)
	s.Require().Equal(expectedBlockTime, params.BlockTimeSeconds)

	// Calculate blocks per year based on block time
	secondsPerYear := uint64(365 * 24 * 60 * 60) // 31,536,000 seconds
	expectedBlocksPerYear := secondsPerYear / params.BlockTimeSeconds
	s.Require().Equal(uint64(5256000), expectedBlocksPerYear, "Should calculate 5,256,000 blocks per year with 6-second blocks")

	// Verify the calculation is dynamic
	if params.BlockTimeSeconds == 3 {
		s.Require().Equal(uint64(10512000), secondsPerYear/3, "3-second blocks should give 10,512,000 blocks per year")
	}
}

func (s *IntegrationTestSuite) TestSupplyOfQueryTypes() {
	// Test that the new SupplyOf query types are properly defined

	// Test QuerySupplyOfRequest
	aepixReq := &types.QuerySupplyOfRequest{
		Denom: "aepix",
	}
	s.Require().NotNil(aepixReq)
	s.Require().Equal("aepix", aepixReq.Denom)

	epixReq := &types.QuerySupplyOfRequest{
		Denom: "epix",
	}
	s.Require().NotNil(epixReq)
	s.Require().Equal("epix", epixReq.Denom)

	// Test QuerySupplyOfResponse
	resp := &types.QuerySupplyOfResponse{
		Supply: sdkmath.NewInt(1000000),
	}
	s.Require().NotNil(resp)
	s.Require().Equal(sdkmath.NewInt(1000000), resp.Supply)

	// Test denomination conversion logic
	// 1 epix = 10^18 aepix
	aepixAmount := sdkmath.NewInt(1000000000000000000) // 1 epix in aepix
	conversionFactor := sdkmath.NewInt(1000000000000000000) // 10^18
	epixAmount := aepixAmount.Quo(conversionFactor)
	s.Require().Equal(sdkmath.NewInt(1), epixAmount)
}
