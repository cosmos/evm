package ics20

import (
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm"
	"github.com/cosmos/evm/precompiles/ics20"
	"github.com/cosmos/evm/precompiles/testutil/contracts"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/tx"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestPrecompileIntegrationTestSuite(t *testing.T, evmAppCreator ibctesting.AppCreator) {
	isContractDeployed := func(ctx sdk.Context, evmApp evm.EvmApp, contractAddr common.Address) bool {
		codeHash := evmApp.GetEVMKeeper().GetCodeHash(ctx, contractAddr)
		code := evmApp.GetEVMKeeper().GetCode(ctx, codeHash)
		return len(code) > 0
	}

	_ = Describe("Calling ICS20 precompile from callerContract", func() {
		s := new(PrecompileTestSuite)
		// testCase is a struct used for cases of contracts calls that have some operation
		// performed before and/or after the precompile call
		type testCase struct {
			before bool
			after  bool
		}

		var (
			ics20CallerContract evmtypes.CompiledContract
			ics20CallerAddr     common.Address
			randomAddr          common.Address
			randomAccAddr       sdk.AccAddress
			err                 error
		)

		BeforeEach(func() {
			ics20CallerContract, err = contracts.LoadIcs20CallerContract()
			Expect(err).To(BeNil())

			s.internalT = t
			s.create = evmAppCreator
			s.SetupTest()

			sender := s.chainA.SenderAccount.GetAddress()
			res, sentEthTx, _, err := s.chainA.SendEvmTx(
				s.chainA.SenderAccounts[0],
				0,
				common.Address{},
				big.NewInt(0),
				ics20CallerContract.Bin,
				0,
			)
			Expect(err).To(BeNil())
			Expect(res.Code).To(BeZero(), "Failed to deploy ICS20 caller contract: %s", res.Log)

			ics20CallerAddr = crypto.CreateAddress(common.BytesToAddress(sender), sentEthTx.AsTransaction().Nonce())
			evmAppA := s.chainA.App.(evm.EvmApp)
			Expect(isContractDeployed(s.chainA.GetContext(), evmAppA, ics20CallerAddr)).To(BeTrue(), "Contract was not deployed successfully")

			randomAddr = tx.GenerateAddress()
			randomAccAddr = sdk.AccAddress(randomAddr.Bytes())
		})

		It("should fail if the provided gas limit is too low", func() {
			path := evmibctesting.NewTransferPath(s.chainA, s.chainB)
			path.Setup()

			sourcePortID := path.EndpointA.ChannelConfig.PortID
			sourceChannelID := path.EndpointA.ChannelID
			sourceBondDenom := s.chainABondDenom

			callArgs := testutiltypes.CallArgs{
				ContractABI: ics20CallerContract.ABI,
				MethodName:  "testIbcTransfer",
				Args: []interface{}{
					sourcePortID,
					sourceChannelID,
					sourceBondDenom,
					big.NewInt(1),
					ics20CallerAddr,
					randomAccAddr.String(),
					ics20.DefaultTimeoutHeight,
					uint64(time.Now().Add(time.Minute).Unix()), //#nosec G115 -- int overflow is not a concern here
					"",
				},
			}
			input, err := factory.GenerateContractCallArgs(callArgs)
			Expect(err).To(BeNil(), "Failed to generate contract call args")
			_, _, _, err = s.chainA.SendEvmTx(
				s.chainA.SenderAccounts[0],
				0,
				ics20CallerAddr,
				big.NewInt(0),
				input,
				30000, // intentionally low gas limit
			)
			Expect(err).NotTo(BeNil(), "Failed to testTransfer: %s", err.Error())
		})

		It("should fail if send is different from msg.sender (only direct call is allowed, not for proxy)", func() {
			path := evmibctesting.NewTransferPath(s.chainA, s.chainB)
			path.Setup()

			sourcePortID := path.EndpointA.ChannelConfig.PortID
			sourceChannelID := path.EndpointA.ChannelID
			sourceBondDenom := s.chainABondDenom
			sender := common.BytesToAddress(s.chainA.SenderAccount.GetAddress())

			callArgs := testutiltypes.CallArgs{
				ContractABI: ics20CallerContract.ABI,
				MethodName:  "testIbcTransfer",
				Args: []interface{}{
					sourcePortID,
					sourceChannelID,
					sourceBondDenom,
					big.NewInt(1),
					sender,
					randomAccAddr.String(),
					ics20.DefaultTimeoutHeight,
					uint64(time.Now().Add(time.Minute).Unix()), //#nosec G115 -- int overflow is not a concern here
					"",
				},
			}
			input, err := factory.GenerateContractCallArgs(callArgs)
			Expect(err).To(BeNil(), "Failed to generate contract call args")
			_, _, _, err = s.chainA.SendEvmTx(
				s.chainA.SenderAccounts[0],
				0,
				ics20CallerAddr,
				big.NewInt(0),
				input,
				0,
			)
			Expect(err).NotTo(BeNil(), "Failed to testTransfer: %s", err.Error())
		})

		It("should fail if the v1 channel is not found", func() {
			path := evmibctesting.NewTransferPath(s.chainA, s.chainB)
			path.Setup()

			sourcePortID := path.EndpointA.ChannelConfig.PortID
			nonExistentChannelID := "channel-100"
			sourceBondDenom := s.chainABondDenom

			callArgs := testutiltypes.CallArgs{
				ContractABI: ics20CallerContract.ABI,
				MethodName:  "testIbcTransfer",
				Args: []interface{}{
					sourcePortID,
					nonExistentChannelID,
					sourceBondDenom,
					big.NewInt(1),
					ics20CallerAddr,
					randomAccAddr.String(),
					ics20.DefaultTimeoutHeight,
					uint64(time.Now().Add(time.Minute).Unix()), //#nosec G115 -- int overflow is not a concern here
					"",
				},
			}
			input, err := factory.GenerateContractCallArgs(callArgs)
			Expect(err).To(BeNil(), "Failed to generate contract call args")
			_, _, _, err = s.chainA.SendEvmTx(
				s.chainA.SenderAccounts[0],
				0,
				ics20CallerAddr,
				big.NewInt(0),
				input,
				0,
			)
			Expect(err).NotTo(BeNil(), "Failed to testTransfer: %s", err.Error())
		})

		It("should fail if the v2 client id format is invalid", func() {
			path := evmibctesting.NewTransferPath(s.chainA, s.chainB)
			path.Setup()

			sourcePortID := path.EndpointA.ChannelConfig.PortID
			invalidV2ClientID := "v2"
			sourceBondDenom := s.chainABondDenom

			callArgs := testutiltypes.CallArgs{
				ContractABI: ics20CallerContract.ABI,
				MethodName:  "testIbcTransfer",
				Args: []interface{}{
					sourcePortID,
					invalidV2ClientID,
					sourceBondDenom,
					big.NewInt(1),
					ics20CallerAddr,
					randomAccAddr.String(),
					ics20.DefaultTimeoutHeight,
					uint64(time.Now().Add(time.Minute).Unix()), //#nosec G115 -- int overflow is not a concern here
					"",
				},
			}
			input, err := factory.GenerateContractCallArgs(callArgs)
			Expect(err).To(BeNil(), "Failed to generate contract call args")
			_, _, _, err = s.chainA.SendEvmTx(
				s.chainA.SenderAccounts[0],
				0,
				ics20CallerAddr,
				big.NewInt(0),
				input,
				0,
			)
			Expect(err).NotTo(BeNil(), "Failed to testTransfer: %s", err.Error())
		})

		It("should successfully call the ICS20 precompile to transfer tokens", func() {
			path := evmibctesting.NewTransferPath(s.chainA, s.chainB)
			path.Setup()
			evmAppA := s.chainA.App.(evm.EvmApp)

			sourcePortID := path.EndpointA.ChannelConfig.PortID
			sourceChannelID := path.EndpointA.ChannelID
			sourceBondDenom := s.chainABondDenom
			escrowAddr := types.GetEscrowAddress(sourcePortID, sourceChannelID)
			escrowBalance := evmAppA.GetBankKeeper().GetBalance(
				s.chainA.GetContext(),
				escrowAddr,
				sourceBondDenom,
			)
			Expect(escrowBalance.Amount).To(Equal(math.ZeroInt()), "Escrow balance should be 0 before transfer")

			// send some tokens to the contract address
			sendAmt := math.NewInt(1)
			err = evmAppA.GetBankKeeper().SendCoins(
				s.chainA.GetContext(),
				s.chainA.SenderAccount.GetAddress(),
				(ics20CallerAddr.Bytes()),
				sdk.NewCoins(sdk.NewCoin(sourceBondDenom, sendAmt)),
			)
			Expect(err).To(BeNil(), "Failed to send tokens to contract address")

			callArgs := testutiltypes.CallArgs{
				ContractABI: ics20CallerContract.ABI,
				MethodName:  "testIbcTransfer",
				Args: []interface{}{
					sourcePortID,
					sourceChannelID,
					sourceBondDenom,
					sendAmt.BigInt(),
					ics20CallerAddr,
					randomAccAddr.String(),
					ics20.DefaultTimeoutHeight,
					uint64(time.Now().UTC().UnixNano()), //#nosec G115 -- int overflow is not a concern here
					"",
				},
			}
			input, err := factory.GenerateContractCallArgs(callArgs)
			Expect(err).To(BeNil(), "Failed to generate contract call args")
			_, _, _, err = s.chainA.SendEvmTx(
				s.chainA.SenderAccounts[0],
				0,
				ics20CallerAddr,
				big.NewInt(0),
				input,
				0,
			)
			Expect(err).To(BeNil(), "Failed to testTransfer")
			// balance after transfer should be 0
			contractBalance := evmAppA.GetBankKeeper().GetBalance(
				s.chainA.GetContext(),
				ics20CallerAddr.Bytes(),
				sourceBondDenom,
			)
			Expect(contractBalance.Amount).To(Equal(math.ZeroInt()), "Contract balance should be 0 after transfer")
			escrowBalance = evmAppA.GetBankKeeper().GetBalance(
				s.chainA.GetContext(),
				escrowAddr,
				sourceBondDenom,
			)
			Expect(escrowBalance.Amount).To(Equal(sendAmt), "Escrow balance should be equal to the sent amount after transfer")
		})

		DescribeTable("ICS20 transfer with transfer", func(tc testCase) {
			path := evmibctesting.NewTransferPath(s.chainA, s.chainB)
			path.Setup()
			evmAppA := s.chainA.App.(evm.EvmApp)

			sourcePortID := path.EndpointA.ChannelConfig.PortID
			sourceChannelID := path.EndpointA.ChannelID
			sourceBondDenom := s.chainABondDenom
			escrowAddr := types.GetEscrowAddress(sourcePortID, sourceChannelID)
			escrowBalance := evmAppA.GetBankKeeper().GetBalance(
				s.chainA.GetContext(),
				escrowAddr,
				sourceBondDenom,
			)
			Expect(escrowBalance.Amount).To(Equal(math.ZeroInt()), "Escrow balance should be 0 before transfer")

			// send some tokens to the contract address
			fundAmt := math.NewInt(100)
			fundAmtConverted := fundAmt.Mul(math.NewInt(1e12))
			err = evmAppA.GetBankKeeper().SendCoins(
				s.chainA.GetContext(),
				s.chainA.SenderAccount.GetAddress(),
				ics20CallerAddr.Bytes(),
				sdk.NewCoins(sdk.NewCoin(sourceBondDenom, fundAmt)),
			)
			Expect(err).To(BeNil(), "Failed to send tokens to contract address")
			// check contract balance
			contractBalance := evmAppA.GetEVMKeeper().GetBalance(
				s.chainA.GetContext(),
				ics20CallerAddr,
			)
			Expect(contractBalance.ToBig()).To(Equal(fundAmtConverted.BigInt()), "Contract balance should be equal to the fund amount")

			sendAmt := math.NewInt(1)
			sendAmtConverted := sendAmt.Mul(math.NewInt(1e12))
			callArgs := testutiltypes.CallArgs{
				ContractABI: ics20CallerContract.ABI,
				MethodName:  "testIbcTransferWithTransfer",
				Args: []interface{}{
					sourcePortID,
					sourceChannelID,
					sourceBondDenom,
					sendAmt.BigInt(),
					ics20CallerAddr,
					randomAccAddr.String(),
					ics20.DefaultTimeoutHeight,
					uint64(time.Now().UTC().UnixNano()), //#nosec G115 -- int overflow is not a concern here
					"",
					tc.before,
					tc.after,
				},
			}
			input, err := factory.GenerateContractCallArgs(callArgs)
			Expect(err).To(BeNil(), "Failed to generate contract call args")
			_, _, _, err = s.chainA.SendEvmTx(
				s.chainA.SenderAccounts[0],
				0,
				ics20CallerAddr,
				big.NewInt(0),
				input,
				0,
			)
			Expect(err).To(BeNil(), "Failed to testTransfer")
			expectedContractBalance := fundAmtConverted.Sub(sendAmtConverted)
			if tc.before {
				expectedContractBalance = expectedContractBalance.Sub(math.NewInt(15))
			}
			if tc.after {
				expectedContractBalance = expectedContractBalance.Sub(math.NewInt(15))
			}
			// balance after transfer should be 0
			contractBalance = evmAppA.GetEVMKeeper().GetBalance(
				s.chainA.GetContext(),
				ics20CallerAddr,
			)
			Expect(contractBalance.ToBig()).To(Equal(expectedContractBalance.BigInt()), "Contract balance should be equal to the expected amount after transfer")
			escrowBalance = evmAppA.GetBankKeeper().GetBalance(
				s.chainA.GetContext(),
				escrowAddr,
				sourceBondDenom,
			)
			Expect(escrowBalance.Amount).To(Equal(sendAmt), "Escrow balance should be equal to the sent amount after transfer")
		},
			Entry("before transfer", testCase{
				before: true,
				after:  false,
			}),
			Entry("after transfer", testCase{
				before: false,
				after:  true,
			}),
			Entry("before and after transfer", testCase{
				before: true,
				after:  true,
			}),
		)

		It("should revert the transfer but continue execution after try catch", func() {
			path := evmibctesting.NewTransferPath(s.chainA, s.chainB)
			path.Setup()
			evmAppA := s.chainA.App.(evm.EvmApp)

			sourcePortID := path.EndpointA.ChannelConfig.PortID
			sourceChannelID := path.EndpointA.ChannelID
			sourceBondDenom := s.chainABondDenom
			escrowAddr := types.GetEscrowAddress(sourcePortID, sourceChannelID)
			escrowBalance := evmAppA.GetEVMKeeper().GetBalance(
				s.chainA.GetContext(),
				common.BytesToAddress(escrowAddr.Bytes()),
			)
			Expect(escrowBalance.ToBig().Uint64()).To(Equal(math.ZeroInt().Uint64()), "Escrow balance should be 0 before transfer")

			// send some tokens to the contract address
			fundAmt := math.NewInt(100)
			fundAmtConverted := fundAmt.Mul(math.NewInt(1e12))
			err = evmAppA.GetBankKeeper().SendCoins(
				s.chainA.GetContext(),
				s.chainA.SenderAccount.GetAddress(),
				ics20CallerAddr.Bytes(),
				sdk.NewCoins(sdk.NewCoin(sourceBondDenom, fundAmt)),
			)
			Expect(err).To(BeNil(), "Failed to send tokens to contract address")
			contractBalance := evmAppA.GetEVMKeeper().GetBalance(
				s.chainA.GetContext(),
				common.BytesToAddress(ics20CallerAddr.Bytes()),
			)
			// check contract balance
			Expect(contractBalance.ToBig()).To(Equal(fundAmtConverted.BigInt()), "Contract balance should be equal to the fund amount")

			sendAmt := math.NewInt(1)
			callArgs := testutiltypes.CallArgs{
				ContractABI: ics20CallerContract.ABI,
				MethodName:  "testRevertIbcTransfer",
				Args: []interface{}{
					sourcePortID,
					sourceChannelID,
					sourceBondDenom,
					sendAmt.BigInt(),
					ics20CallerAddr,
					randomAccAddr.String(),
					common.BytesToAddress(randomAccAddr.Bytes()),
					ics20.DefaultTimeoutHeight,
					uint64(time.Now().UTC().UnixNano()), //#nosec G115 -- int overflow is not a concern here
					"",
					true,
				},
			}
			input, err := factory.GenerateContractCallArgs(callArgs)
			Expect(err).To(BeNil(), "Failed to generate contract call args")
			_, _, _, err = s.chainA.SendEvmTx(
				s.chainA.SenderAccounts[0],
				0,
				ics20CallerAddr,
				big.NewInt(0),
				input,
				0,
			)
			Expect(err).To(BeNil(), "Failed to testTransfer")
			contractBalanceAfter := evmAppA.GetEVMKeeper().GetBalance(
				s.chainA.GetContext(),
				common.BytesToAddress(ics20CallerAddr.Bytes()),
			)
			Expect(contractBalanceAfter.ToBig()).To(Equal(math.NewIntFromBigInt(contractBalance.ToBig()).Sub(math.NewInt(15)).BigInt()), "Contract balance should be equal to the expected amount after transfer")
			escrowBalance = evmAppA.GetEVMKeeper().GetBalance(
				s.chainA.GetContext(),
				common.BytesToAddress(escrowAddr.Bytes()),
			)
			Expect(escrowBalance.ToBig().Uint64()).To(Equal(math.ZeroInt().BigInt().Uint64()))
			randomAccBalance := evmAppA.GetEVMKeeper().GetBalance(
				s.chainA.GetContext(),
				common.BytesToAddress(randomAccAddr.Bytes()),
			)
			Expect(randomAccBalance.ToBig()).To(Equal(math.NewInt(15).BigInt()))
		})
	})

	// TODO: Add tests for calling ICS20 precompile from EoA

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "ICS20 Precompile Test Suite")
}
