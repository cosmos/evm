package accountabstraction

import (
	"testing"

	//nolint:revive // dot imports are fine for Ginkgo
	"github.com/ethereum/go-ethereum/common"
	. "github.com/onsi/ginkgo/v2"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"
	// "github.com/cosmos/evm/precompiles/testutil"
)

// var logCheck testutil.LogCheckArgs

func TestEIP7702(t *testing.T) {
	const (
		validChainID = uint64(4221)

		user0 = "acc0"
		user1 = "acc1"
	)

	Describe("test SetCode tx with diverse SetCodeAuthorization", Ordered, func() {
		var s AccountAbstractionTestSuite
		var smartWalletAddr common.Address

		BeforeAll(func() {
			s = NewTestSuite(t)
			s.SetupTest(t)

			smartWalletAddr = s.GetSmartWalletAddress()
		})

		type testCase struct {
			authChainID uint64
			authNonce   func() uint64
			authAddress common.Address
			authSigner  string
			txSender    string
			expPass     bool
		}

		DescribeTable("SetCode authorization scenarios", func(tc testCase) {
			authorization := createSetCodeAuthorization(tc.authChainID, tc.authNonce(), tc.authAddress)
			signedAuthorization, err := signSetCodeAuthorization(s.GetPrivKey(tc.authSigner), authorization)
			Expect(err).To(BeNil())

			_, err = s.SendSetCodeTx(tc.txSender, signedAuthorization)
			Expect(err).To(BeNil(), "error while sending SetCode tx")
			if tc.expPass {

			} else {

			}

			s.AwaitNBlocks(t, 1)
		},
			Entry("same signer/sender with committed nonce", testCase{
				authChainID: validChainID,
				authNonce: func() uint64 {
					return s.GetNonce(user0)
				},
				authAddress: smartWalletAddr,
				authSigner:  user0,
				txSender:    user0,
				expPass:     false,
			}),
			Entry("same signer/sender with pending nonce", testCase{
				authChainID: validChainID,
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: smartWalletAddr,
				authSigner:  user0,
				txSender:    user0,
				expPass:     true,
			}),
			Entry("authorized by different signer using committed nonce", testCase{
				authChainID: validChainID,
				authNonce: func() uint64 {
					return s.GetNonce(user1)
				},
				authAddress: smartWalletAddr,
				authSigner:  user1,
				txSender:    user0,
				expPass:     true,
			}),
			Entry("authorized by different signer using pending nonce", testCase{
				authChainID: validChainID,
				authNonce: func() uint64 {
					return s.GetNonce(user1) + 1
				},
				authAddress: smartWalletAddr,
				authSigner:  user1,
				txSender:    user0,
				expPass:     false,
			}),
		)

		// When("sender of SetCodeTx is same with signer of SetCodeAuthorization", func() {
		// 	Context("if current nonce is set to SetCodeAuthorization", func() {
		// 		It("should fail", func() {
		// 			authorization := createSetCodeAuthorization(validChainID, s.GetNonce(user0), s.GetSmartWalletAddress())
		// 			signedAuthorization, err := signSetCodeAuthorization(s.GetPrivKey(user0), authorization)
		// 			Expect(err).To(BeNil())

		// 			_, err = s.SendSetCodeTx(user0, signedAuthorization)
		// 			Expect(err).To(BeNil(), "error while sending SetCode tx")
		// 			s.AwaitNBlocks(t, 1)

		// 			// s.checkSetCode(user0, s.GetSmartWalletAddress(), false)
		// 		})
		// 	})

		// 	Context("if current nonce + 1 is set to SetCodeAuthorization", func() {
		// 		It("should succeed", func() {
		// 			authorization := createSetCodeAuthorization(validChainID, s.GetNonce(user0)+1, s.GetSmartWalletAddress())
		// 			signedAuthorization, err := signSetCodeAuthorization(s.GetPrivKey(user0), authorization)
		// 			Expect(err).To(BeNil())

		// 			_, err = s.SendSetCodeTx(user0, signedAuthorization)
		// 			Expect(err).To(BeNil(), "error is expected while sending SetCode tx")
		// 			s.AwaitNBlocks(t, 1)

		// 			// s.checkSetCode(user0, s.GetSmartWalletAddress(), true)
		// 		})
		// 	})
		// })

		// When("sender of SetCodeTx is different with singer of SetCodeAuthorization", func() {
		// 	Context("if current nonce is set to SetCodeAuthorization", func() {
		// 		It("should succeed", func() {
		// 			authorization := createSetCodeAuthorization(validChainID, s.GetNonce(user1), s.GetSmartWalletAddress())
		// 			signedAuthorization, err := signSetCodeAuthorization(s.GetPrivKey(user1), authorization)
		// 			Expect(err).To(BeNil())

		// 			_, err = s.SendSetCodeTx(user0, signedAuthorization)
		// 			Expect(err).To(BeNil(), "error is expected while sending SetCode tx")
		// 			s.AwaitNBlocks(t, 1)

		// 			// s.checkSetCode(user1, s.GetSmartWalletAddress(), true)
		// 		})
		// 	})

		// 	Context("if current nonce + 1 is set to SetCodeAuthorization", func() {
		// 		It("should fail", func() {
		// 			authorization := createSetCodeAuthorization(validChainID, s.GetNonce(user1)+1, s.GetSmartWalletAddress())
		// 			signedAuthorization, err := signSetCodeAuthorization(s.GetPrivKey(user1), authorization)
		// 			Expect(err).To(BeNil())

		// 			_, err = s.SendSetCodeTx(user0, signedAuthorization)
		// 			Expect(err).To(BeNil(), "error is expected while sending SetCode tx")
		// 			s.AwaitNBlocks(t, 1)

		// 			// s.checkSetCode(user1, s.GetSmartWalletAddress(), false)
		// 		})
		// 	})
		// })
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "EIP7702 Integration Test Suite")
}
