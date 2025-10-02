package accountabstraction

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"
)

func TestEIP7702(t *testing.T) {
	const (
		user0 = "acc0"
		user1 = "acc1"
	)

	Describe("test SetCode tx with diverse SetCodeAuthorization", Ordered, func() {
		var (
			s AccountAbstractionTestSuite
		)

		// We intentionally use BeforeAll instead of BeforeAll because,
		// The test takes too much time if we restart network for each test case.
		BeforeAll(func() {
			s = NewTestSuite(t)
			s.SetupTest(t)
		})

		AfterEach(func() {
			// Reset code of EoAs to 0x0 address
			//
			// We set user0's authorization nonce to currentNonce + 1
			// because user0 will also send the SetCode transaction.
			// Since the sender’s nonce is incremented before applying authorization,
			// the SetCodeAuthorization must use currentNonce + 1.
			user0Nonce := s.GetNonce(user0) + 1
			cleanupAuth0 := createSetCodeAuthorization(s.GetChainID(), user0Nonce, common.Address{})
			signedCleanup0, signErr := signSetCodeAuthorization(s.GetPrivKey(user0), cleanupAuth0)
			Expect(signErr).To(BeNil())

			user1Nonce := s.GetNonce(user1)
			cleanupAuth1 := createSetCodeAuthorization(s.GetChainID(), user1Nonce, common.Address{})
			signedCleanup1, signErr := signSetCodeAuthorization(s.GetPrivKey(user1), cleanupAuth1)
			Expect(signErr).To(BeNil())

			_, err := s.SendSetCodeTx(user0, signedCleanup0, signedCleanup1)
			Expect(err).To(BeNil(), "error while clearing SetCode delegation")

			s.AwaitNBlocks(t, 1)
			s.CheckSetCode(user0, common.Address{}, false)
			s.CheckSetCode(user1, common.Address{}, false)
		})

		type testCase struct {
			authChainID   func() uint64
			authNonce     func() uint64
			authAddress   func() common.Address
			authSigner    string
			txSender      string
			expDelegation bool
		}

		DescribeTable("SetCode authorization scenarios", func(tc testCase) {
			authorization := createSetCodeAuthorization(tc.authChainID(), tc.authNonce(), tc.authAddress())
			signedAuthorization, err := signSetCodeAuthorization(s.GetPrivKey(tc.authSigner), authorization)
			Expect(err).To(BeNil())

			_, err = s.SendSetCodeTx(tc.txSender, signedAuthorization)
			Expect(err).To(BeNil(), "error while sending SetCode tx")
			s.AwaitNBlocks(t, 1)

			s.CheckSetCode(tc.authSigner, tc.authAddress(), tc.expDelegation)
		},
			Entry("setCode with invalid chainID should fail", testCase{
				authChainID: func() uint64 { return s.GetChainID() + 1 },
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: func() common.Address {
					return s.GetSmartWalletAddr()
				},
				authSigner:    user0,
				txSender:      user0,
				expDelegation: false,
			}),
			Entry("setCode with empty address should reset delegation", testCase{
				authChainID: func() uint64 { return s.GetChainID() + 1 },
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: func() common.Address {
					return common.HexToAddress("0x0")
				},
				authSigner:    user0,
				txSender:      user0,
				expDelegation: false,
			}),
			Entry("setCode with invalid address should fail", testCase{
				authChainID: func() uint64 { return s.GetChainID() + 1 },
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: func() common.Address {
					return common.BytesToAddress([]byte("invalid"))
				},
				authSigner:    user0,
				txSender:      user0,
				expDelegation: false,
			}),
			Entry("setCode with EoA address should fail", testCase{
				authChainID: func() uint64 { return s.GetChainID() + 1 },
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: func() common.Address {
					return s.GetAddr(user1)
				},
				authSigner:    user0,
				txSender:      user0,
				expDelegation: false,
			}),
			Entry("same signer/sender with matching nonce should fail", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user0)
				},
				authAddress: func() common.Address {
					return s.GetSmartWalletAddr()
				},
				authSigner:    user0,
				txSender:      user0,
				expDelegation: false,
			}),
			Entry("same signer/sender with future nonce sholud succeed", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: func() common.Address {
					return s.GetSmartWalletAddr()
				},
				authSigner:    user0,
				txSender:      user0,
				expDelegation: true,
			}),
			Entry("different signer/sender with current nonce should succeed", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user1)
				},
				authAddress: func() common.Address {
					return s.GetSmartWalletAddr()
				},
				authSigner:    user1,
				txSender:      user0,
				expDelegation: true,
			}),
			Entry("different signer/sender with future nonce should fail", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user1) + 1
				},
				authAddress: func() common.Address {
					return s.GetSmartWalletAddr()
				},
				authSigner:    user1,
				txSender:      user0,
				expDelegation: false,
			}),
		)
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "EIP7702 Integration Test Suite")
}
