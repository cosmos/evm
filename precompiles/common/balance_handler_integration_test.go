package common_test

import (
	"math/big"
	"testing"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestPrecompileIntegrationTestSuite(t *testing.T) {
	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "Common Precompile Integration Tests")
}

var _ = Describe("Testing recursive precompile calls with debug precompile", func() {
	var s *PrecompileTestSuite

	BeforeEach(func() {
		s = new(PrecompileTestSuite)
		s.SetupTest()
	})

	Describe("Recursive precompile calls", func() {
		It("should handle balance correctly during recursive calls", func() {
			deployer := s.keyring.GetKey(0)

			// Verify the debug precompile is active
			evmParams, err := s.grpcHandler.GetEvmParams()
			Expect(err).To(BeNil(), "error getting EVM params")

			debugAddr := s.debugPrecompile.Address().String()
			isActive := false
			for _, addr := range evmParams.Params.ActiveStaticPrecompiles {
				if addr == debugAddr {
					isActive = true
					break
				}
			}
			Expect(isActive).To(BeTrue(), "debug precompile %s should be active", debugAddr)

			GinkgoWriter.Printf("Verified debug precompile %s is active\n", debugAddr)

			// Deploy Greeter contract
			greeterContract, err := contracts.LoadDebugPrecompileCaller()
			Expect(err).To(BeNil(), "error loading Greeter contract")

			greeterAddr, err := s.factory.DeployContract(
				deployer.Priv,
				evmtypes.EvmTxArgs{},
				factory.ContractDeploymentData{
					Contract: greeterContract,
				},
			)
			Expect(err).To(BeNil(), "error deploying Greeter contract")
			Expect(s.network.NextBlock()).To(BeNil())

			GinkgoWriter.Printf("Deployed Greeter contract at %s\n", greeterAddr.Hex())
			GinkgoWriter.Printf("Debug precompile at %s\n", s.debugPrecompile.Address().Hex())

			// Fund the contract
			ctx := s.network.GetContext()
			err = s.network.App.BankKeeper.SendCoins(
				ctx,
				deployer.AccAddr,
				greeterAddr.Bytes(),
				sdk.NewCoins(sdk.NewCoin(s.bondDenom, sdkmath.NewInt(10000000))),
			)
			Expect(err).To(BeNil(), "error funding contract")

			// We must directly commit keeper calls to state, otherwise they get
			// fully wiped when the next block finalizes.
			store := s.network.GetContext().MultiStore()
			if cms, ok := store.(storetypes.CacheMultiStore); ok {
				cms.Write()
			} else {
				panic("store is not a CacheMultiStore")
			}

			err = s.network.NextBlock()
			Expect(err).To(BeNil(), "next block")

			// Prepare call to callback(0)
			callArgs := factory.CallArgs{
				ContractABI: greeterContract.ABI,
				MethodName:  "callback",
				Args:        []interface{}{big.NewInt(0)},
			}

			txArgs := evmtypes.EvmTxArgs{
				To:       &greeterAddr,
				GasLimit: 10_000_000, // Explicit gas limit to skip estimation
			}

			// Call the contract
			res, ethRes, err := s.factory.CallContractAndCheckLogs(
				deployer.Priv,
				txArgs,
				callArgs,
				testutil.LogCheckArgs{}.WithExpPass(true),
			)
			Expect(err).To(BeNil(), "error while calling callback")
			Expect(ethRes.Failed()).To(BeFalse(), "callback should not fail")

			//// Commit the transaction
			// Expect(s.network.NextBlock()).To(BeNil())

			// Count debug precompile events and print all event types
			debugCount := 0
			for _, event := range res.Events {
				if event.Type == "debug_precompile" {
					debugCount++
				}
			}
			// Total event count may vary based on test framework and setup.
			// Our fork emits extra bank events from base-fee burning (BurnAmountFromAccount),
			// which accounts for the difference from upstream's baseline of 30.
			Expect(len(res.Events)).To(Equal(41))
			Expect(debugCount).To(Equal(10), "callback should have 10 debug precompile events")
			Expect(len(res.Events)).To(BeNumerically(">=", 10), "should have at least 10 events")

			// Advance to next block to finalize state
			Expect(s.network.NextBlock()).To(BeNil())
		})
	})
})
