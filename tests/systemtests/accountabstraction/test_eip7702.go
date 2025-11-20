//go:build system_test

package accountabstraction

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	suite "github.com/cosmos/evm/tests/systemtests/suite"
)

func RunEIP7702(t *testing.T, base *suite.BaseTestSuite) {
	Describe("test EIP-7702 scenorios", Ordered, func() {
		var (
			s        *TestSuite
			user0Acc *suite.TestAccount
			user1Acc *suite.TestAccount
			user0    string
			user1    string
		)

		// We intentionally use BeforeAll instead of BeforeAll because,
		// The test takes too much time if we restart network for each test case.
		BeforeAll(func() {
			s = NewTestSuite(base)
			user0Acc = s.BaseTestSuite.Acc(0)
			user1Acc = s.BaseTestSuite.Acc(1)
			user0 = user0Acc.ID
			user1 = user1Acc.ID
			s.SetPrimaryAccount(user0Acc)
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

			txHash, err := s.SendSetCodeTx(user0, signedCleanup0, signedCleanup1)
			Expect(err).To(BeNil(), "error while clearing SetCode delegation")

			s.WaitForCommit(txHash)
			s.CheckSetCode(user0, common.Address{}, false)
			s.CheckSetCode(user1, common.Address{}, false)
		})

		type testCase struct {
			authChainID   func() uint64
			authNonce     func() uint64
			authAddress   func() common.Address
			authSigner    func() string
			txSender      func() string
			expDelegation bool
		}

		DescribeTable("SetCode authorization scenarios", func(tc testCase) {
			authorization := createSetCodeAuthorization(tc.authChainID(), tc.authNonce(), tc.authAddress())
			signedAuthorization, err := signSetCodeAuthorization(s.GetPrivKey(tc.authSigner()), authorization)
			Expect(err).To(BeNil())

			txHash, err := s.SendSetCodeTx(tc.txSender(), signedAuthorization)
			Expect(err).To(BeNil(), "error while sending SetCode tx")
			s.WaitForCommit(txHash)
			s.CheckSetCode(tc.authSigner(), tc.authAddress(), tc.expDelegation)
		},
			Entry("setCode with invalid chainID should fail", testCase{
				authChainID: func() uint64 { return s.GetChainID() + 1 },
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: func() common.Address {
					return s.GetCounterAddr()
				},
				authSigner:    func() string { return user0 },
				txSender:      func() string { return user0 },
				expDelegation: false,
			}),
			Entry("setCode with empty address should reset delegation", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: func() common.Address {
					return common.HexToAddress("0x0")
				},
				authSigner:    func() string { return user0 },
				txSender:      func() string { return user0 },
				expDelegation: false,
			}),
			Entry("setCode with invalid address should fail", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: func() common.Address {
					return common.BytesToAddress([]byte("invalid"))
				},
				authSigner:    func() string { return user0 },
				txSender:      func() string { return user0 },
				expDelegation: true,
			}),
			Entry("setCode with EoA address should fail", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: func() common.Address {
					return s.GetAddr(user1)
				},
				authSigner:    func() string { return user0 },
				txSender:      func() string { return user0 },
				expDelegation: true,
			}),
			Entry("same signer/sender with matching nonce should fail", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user0)
				},
				authAddress: func() common.Address {
					return s.GetCounterAddr()
				},
				authSigner:    func() string { return user0 },
				txSender:      func() string { return user0 },
				expDelegation: false,
			}),
			Entry("same signer/sender with future nonce sholud succeed", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user0) + 1
				},
				authAddress: func() common.Address {
					return s.GetCounterAddr()
				},
				authSigner:    func() string { return user0 },
				txSender:      func() string { return user0 },
				expDelegation: true,
			}),
			Entry("different signer/sender with current nonce should succeed", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user1)
				},
				authAddress: func() common.Address {
					return s.GetCounterAddr()
				},
				authSigner:    func() string { return user1 },
				txSender:      func() string { return user0 },
				expDelegation: true,
			}),
			Entry("different signer/sender with future nonce should fail", testCase{
				authChainID: func() uint64 { return s.GetChainID() },
				authNonce: func() uint64 {
					return s.GetNonce(user1) + 1
				},
				authAddress: func() common.Address {
					return s.GetCounterAddr()
				},
				authSigner:    func() string { return user1 },
				txSender:      func() string { return user0 },
				expDelegation: false,
			}),
		)

		Describe("executes counter contract methods via delegated account", func() {
			Context("when delegation is active", func() {
				It("should succeed", func() {
					counterAddr := s.GetCounterAddr()

					chainID := s.GetChainID()
					authorization := createSetCodeAuthorization(chainID, s.GetNonce(user0)+1, counterAddr)
					signedAuthorization, err := signSetCodeAuthorization(s.GetPrivKey(user0), authorization)
					Expect(err).To(BeNil())

					txHash, err := s.SendSetCodeTx(user0, signedAuthorization)
					Expect(err).To(BeNil(), "error while sending SetCode tx")
					s.WaitForCommit(txHash)
					s.CheckSetCode(user0, counterAddr, true)

					txHash, err = s.InvokeCounter(user0, "setNumber", big.NewInt(0))
					Expect(err).To(BeNil(), "failed to reset counter")
					s.WaitForCommit(txHash)

					txHash, err = s.InvokeCounter(user0, "increment")
					Expect(err).To(BeNil(), "failed to increment counter")
					s.WaitForCommit(txHash)

					value, err := s.QueryCounterNumber(user0)
					Expect(err).To(BeNil(), "failed to query counter value")
					Expect(value.Uint64()).To(Equal(uint64(1)))
				})
			})

			Context("after delegation has been revoked", func() {
				It("should no longer execute counter methods", func() {
					counterAddr := s.GetCounterAddr()
					chainID := s.GetChainID()

					authorization := createSetCodeAuthorization(chainID, s.GetNonce(user0)+1, counterAddr)
					signedAuthorization, err := signSetCodeAuthorization(s.GetPrivKey(user0), authorization)
					Expect(err).To(BeNil())

					txHash, err := s.SendSetCodeTx(user0, signedAuthorization)
					Expect(err).To(BeNil(), "error while sending SetCode tx")
					s.WaitForCommit(txHash)
					s.CheckSetCode(user0, counterAddr, true)

					cleanup := createSetCodeAuthorization(chainID, s.GetNonce(user0)+1, common.Address{})
					signedCleanup, err := signSetCodeAuthorization(s.GetPrivKey(user0), cleanup)
					Expect(err).To(BeNil())

					txHash, err = s.SendSetCodeTx(user0, signedCleanup)
					Expect(err).To(BeNil(), "error while clearing SetCode delegation")
					s.WaitForCommit(txHash)
					s.CheckSetCode(user0, common.Address{}, false)

					txHash, err = s.InvokeCounter(user0, "increment")
					Expect(err).To(BeNil(), "counter invocation tx should be accepted but do nothing")
					s.WaitForCommit(txHash)

					value, err := s.QueryCounterNumber(user0)
					Expect(err).To(BeNil(), "failed to query counter value after revocation")
					Expect(value.Uint64()).To(Equal(uint64(0)), "counter value should remain unchanged without delegation")
				})
			})
		})
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "EIP7702 Integration Test Suite")
}
