package gov

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/gov"
	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/precompiles/testutil/contracts"
	commonfactory "github.com/cosmos/evm/testutil/integration/base/factory"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testutiltx "github.com/cosmos/evm/testutil/tx"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
	govv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	"github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

// General variables used for integration tests
var (
	// differentAddr is an address generated for testing purposes that e.g. raises the different origin error
	differentAddr = testutiltx.GenerateAddress()
	// defaultCallArgs  are the default arguments for calling the smart contract
	//
	// NOTE: this has to be populated in a BeforeEach block because the contractAddr would otherwise be a nil address.
	callArgs testutiltypes.CallArgs
	// txArgs are the EVM transaction arguments to use in the transactions
	txArgs evmtypes.EvmTxArgs
	// defaultLogCheck instantiates a log check arguments struct with the precompile ABI events populated.
	defaultLogCheck testutil.LogCheckArgs
	// passCheck defines the arguments to check if the precompile returns no error
	passCheck testutil.LogCheckArgs
	// outOfGasCheck defines the arguments to check if the precompile returns out of gas error
	outOfGasCheck testutil.LogCheckArgs
	// govModuleAddr is the address of the gov module account
	govModuleAddr sdk.AccAddress
)

const (
	testDepositFromContract        = "testDepositFromContract"
	testSubmitProposalFromContract = "testSubmitProposalFromContract"
)

func TestPrecompileIntegrationTestSuite(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	_ = Describe("Calling governance precompile from EOA", func() {
		var (
			s               *PrecompileTestSuite
			proposerKey     types.PrivKey
			proposerAddr    common.Address
			proposerAccAddr sdk.AccAddress
		)
		const (
			proposalID uint64 = 1
			option     uint8  = 1
			metadata          = "metadata"
		)
		BeforeEach(func() {
			s = NewPrecompileTestSuite(create, options...)
			s.SetupTest()

			// set the default call arguments
			callArgs = testutiltypes.CallArgs{
				ContractABI: s.precompile.ABI,
			}
			defaultLogCheck = testutil.LogCheckArgs{
				ABIEvents: s.precompile.Events,
			}
			passCheck = defaultLogCheck.WithExpPass(true)
			outOfGasCheck = defaultLogCheck.WithErrContains(vm.ErrOutOfGas.Error())

			// reset tx args each test to avoid keeping custom
			// values of previous tests (e.g. gasLimit)
			precompileAddr := s.precompile.Address()
			txArgs = evmtypes.EvmTxArgs{
				To: &precompileAddr,
			}
			txArgs.GasLimit = 200_000

			proposerKey = s.keyring.GetPrivKey(0)
			proposerAddr = s.keyring.GetAddr(0)
			proposerAccAddr = sdk.AccAddress(proposerAddr.Bytes())
			govModuleAddr = authtypes.NewModuleAddress(govtypes.ModuleName)
		})

		// =====================================
		// 				TRANSACTIONS
		// =====================================
		Describe("Execute SubmitProposal transaction", func() {
			const method = gov.SubmitProposalMethod

			BeforeEach(func() { callArgs.MethodName = method })

			It("fails with low gas", func() {
				txArgs.GasLimit = 37_790 // meed the requirement of floor data gas cost
				jsonBlob := minimalBankSendProposalJSON(proposerAccAddr, s.network.GetBaseDenom(), "50")
				callArgs.Args = []interface{}{proposerAddr, jsonBlob, minimalDeposit(s.network.GetBaseDenom(), big.NewInt(1))}

				_, _, err := s.factory.CallContractAndCheckLogs(proposerKey, txArgs, callArgs, outOfGasCheck)
				Expect(err).To(BeNil())
			})

			It("creates a proposal and emits event", func() {
				jsonBlob := minimalBankSendProposalJSON(proposerAccAddr, s.network.GetBaseDenom(), "1")
				callArgs.Args = []interface{}{proposerAddr, jsonBlob, minimalDeposit(s.network.GetBaseDenom(), big.NewInt(1))}
				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)

				_, ethRes, err := s.factory.CallContractAndCheckLogs(proposerKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())

				// unpack return → proposalId
				var out uint64
				err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
				Expect(err).To(BeNil())
				Expect(out).To(BeNumerically(">", 0))

				// ensure proposal exists on-chain
				prop, err := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), out)
				Expect(err).To(BeNil())
				Expect(prop.Proposer).To(Equal(sdk.AccAddress(proposerAddr.Bytes()).String()))
			})

			It("fails with invalid JSON", func() {
				callArgs.Args = []interface{}{proposerAddr, []byte("{invalid}"), minimalDeposit(s.network.GetBaseDenom(), big.NewInt(1))}
				errCheck := defaultLogCheck.WithErrContains("invalid proposal JSON")
				_, _, err := s.factory.CallContractAndCheckLogs(
					proposerKey, txArgs, callArgs, errCheck)
				Expect(err).To(BeNil())
			})

			It("fails with invalid deposit denom", func() {
				jsonBlob := minimalBankSendProposalJSON(proposerAccAddr, s.network.GetBaseDenom(), "1")
				invalidDep := []cmn.Coin{{Denom: "bad", Amount: big.NewInt(1)}}
				callArgs.Args = []interface{}{proposerAddr, jsonBlob, invalidDep}
				errCheck := defaultLogCheck.WithErrContains("invalid deposit denom")
				_, _, err := s.factory.CallContractAndCheckLogs(
					proposerKey, txArgs, callArgs, errCheck)
				Expect(err).To(BeNil())
			})
		})

		Describe("Execute Deposit transaction", func() {
			const method = gov.DepositMethod

			BeforeEach(func() { callArgs.MethodName = method })

			It("fails with wrong proposal id", func() {
				callArgs.Args = []interface{}{proposerAddr, uint64(999), minimalDeposit(s.network.GetBaseDenom(), big.NewInt(1))}
				errCheck := defaultLogCheck.WithErrContains("not found")
				_, _, err := s.factory.CallContractAndCheckLogs(proposerKey, txArgs, callArgs, errCheck)
				Expect(err).To(BeNil())
			})

			It("deposits successfully and emits event", func() {
				jsonBlob := minimalBankSendProposalJSON(proposerAccAddr, s.network.GetBaseDenom(), "1")
				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
				callArgs.MethodName = gov.SubmitProposalMethod
				minDeposit := minimalDeposit(s.network.GetBaseDenom(), big.NewInt(1))
				callArgs.Args = []interface{}{proposerAddr, jsonBlob, minDeposit}
				_, evmRes, err := s.factory.CallContractAndCheckLogs(proposerKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				var propID uint64
				err = s.precompile.UnpackIntoInterface(&propID, gov.SubmitProposalMethod, evmRes.Ret)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				// get proposal by propID
				prop, err := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), propID)
				Expect(err).To(BeNil())
				Expect(prop.Status).To(Equal(govv1.StatusDepositPeriod))
				Expect(prop.Proposer).To(Equal(sdk.AccAddress(proposerAddr.Bytes()).String()))
				minDepositCoins, err := cmn.NewSdkCoinsFromCoins(minDeposit)
				Expect(err).To(BeNil())
				td := prop.GetTotalDeposit()
				Expect(td).To(HaveLen(1))
				Expect(td[0].Denom).To(Equal(minDepositCoins[0].Denom))
				Expect(td[0].Amount.String()).To(Equal(minDepositCoins[0].Amount.String()))

				callArgs.MethodName = gov.DepositMethod
				callArgs.Args = []interface{}{proposerAddr, propID, minimalDeposit(s.network.GetBaseDenom(), big.NewInt(1))}
				eventCheck = passCheck.WithExpEvents(gov.EventTypeDeposit)
				_, _, err = s.factory.CallContractAndCheckLogs(proposerKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())
				// Update expected total deposit
				td[0].Amount = td[0].Amount.Add(minDepositCoins[0].Amount)

				// verify via query
				callArgs.MethodName = gov.GetProposalMethod
				callArgs.Args = []interface{}{propID}
				_, ethRes, err := s.factory.CallContractAndCheckLogs(proposerKey, txArgs, callArgs, passCheck)
				Expect(err).To(BeNil())

				var out gov.ProposalOutput
				err = s.precompile.UnpackIntoInterface(&out, gov.GetProposalMethod, ethRes.Ret)
				Expect(err).To(BeNil())
				Expect(out.Proposal.Id).To(Equal(propID))
				Expect(out.Proposal.Status).To(Equal(uint32(govv1.StatusDepositPeriod)))
				newTd := out.Proposal.TotalDeposit
				Expect(newTd).To(HaveLen(1))
				Expect(newTd[0].Denom).To(Equal(minDepositCoins[0].Denom))
				Expect(newTd[0].Amount.String()).To(Equal(td[0].Amount.String()))
			})
		})

		Describe("Execute CancelProposal transaction", func() {
			const method = gov.CancelProposalMethod

			BeforeEach(func() {
				callArgs.MethodName = method
			})

			It("fails when called by a non-proposer", func() {
				callArgs.Args = []interface{}{proposerAddr, proposalID}
				notProposerKey := s.keyring.GetPrivKey(1)
				notProposerAddr := s.keyring.GetAddr(1)
				errCheck := defaultLogCheck.WithErrContains(
					cmn.ErrRequesterIsNotMsgSender,
					notProposerAddr.String(),
					proposerAddr.String(),
				)

				_, _, err := s.factory.CallContractAndCheckLogs(notProposerKey, txArgs, callArgs, errCheck)
				Expect(err).To(BeNil())
			})

			It("cancels a live proposal and emits event", func() {
				proposal, err := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposalID)
				Expect(err).To(BeNil())

				// Cancel proposal
				callArgs.Args = []interface{}{proposerAddr, proposal.Id}
				eventCheck := passCheck.WithExpEvents(gov.EventTypeCancelProposal)
				_, evmRes, err := s.factory.CallContractAndCheckLogs(proposerKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())
				var succeeded bool
				err = s.precompile.UnpackIntoInterface(&succeeded, gov.CancelProposalMethod, evmRes.Ret)
				Expect(err).To(BeNil())
				Expect(succeeded).To(BeTrue())

				// 3. Check that the proposal is not found
				_, err = s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposal.Id)
				Expect(err.Error()).To(ContainSubstring("not found"))
			})

			It("cancels a proposal and see cancellation fee charged", func() {
				// Fix the gas limit and gas price for predictable gas usage.
				// This is for calculating expected cancellation fee.
				baseFee := s.network.App.GetFeeMarketKeeper().GetBaseFee(s.network.GetContext())
				baseFeeInt := baseFee.TruncateInt64()
				txArgs.GasPrice = new(big.Int).SetInt64(baseFeeInt)
				txArgs.GasLimit = 500_000

				// Get the prposal for cancellation
				proposal, err := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), 1)
				Expect(err).To(BeNil())

				// Calc cancellation fee
				proposalDeposits, err := s.network.App.GetGovKeeper().GetDeposits(s.network.GetContext(), proposal.Id)
				Expect(err).To(BeNil())
				proposalDepositAmt := proposalDeposits[0].Amount[0].Amount
				params, err := s.network.App.GetGovKeeper().Params.Get(s.network.GetContext())
				Expect(err).To(BeNil())
				rate := math.LegacyMustNewDecFromStr(params.ProposalCancelRatio)
				cancelFee := proposalDepositAmt.ToLegacyDec().Mul(rate).TruncateInt()
				remaining := proposalDepositAmt.Sub(cancelFee)

				// Cancel it
				callArgs.Args = []interface{}{proposerAddr, proposal.Id}
				eventCheck := passCheck.WithExpEvents(gov.EventTypeCancelProposal)
				// Balance of proposer
				proposalBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), proposerAccAddr, s.network.GetBaseDenom())
				res, _, err := s.factory.CallContractAndCheckLogs(proposerKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())
				gasCost := math.NewInt(res.GasUsed).Mul(math.NewInt(txArgs.GasPrice.Int64()))

				// 6. Check that the cancellation fee is charged, diff should be less than the deposit amount
				afterCancelBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), proposerAccAddr, s.network.GetBaseDenom())
				Expect(afterCancelBal.Amount).To(Equal(
					proposalBal.Amount.
						Sub(gasCost).
						Add(remaining),
				),
					"expected cancellation fee to be deducted from proposer balance")

				// 7. Check that the proposal is not found
				_, err = s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposal.Id)
				Expect(err.Error()).To(ContainSubstring("not found"))
			})
		})

		Describe("Execute Vote transaction", func() {
			const method = gov.VoteMethod

			BeforeEach(func() {
				// set the default call arguments
				callArgs.MethodName = method
			})

			It("should return error if the provided gasLimit is too low", func() {
				txArgs.GasLimit = 30000
				callArgs.Args = []interface{}{
					s.keyring.GetAddr(0), proposalID, option, metadata,
				}

				_, _, err := s.factory.CallContractAndCheckLogs(s.keyring.GetPrivKey(0), txArgs, callArgs, outOfGasCheck)
				Expect(err).To(BeNil())

				// tally result yes count should remain unchanged
				proposal, _ := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposalID)
				_, _, tallyResult, err := s.network.App.GetGovKeeper().Tally(s.network.GetContext(), proposal)
				Expect(err).To(BeNil())
				Expect(tallyResult.YesCount).To(Equal("0"), "expected tally result yes count to remain unchanged")
			})

			It("should return error if the origin is different than the voter", func() {
				callArgs.Args = []interface{}{
					differentAddr, proposalID, option, metadata,
				}

				voterSetCheck := defaultLogCheck.WithErrContains(cmn.ErrRequesterIsNotMsgSender, s.keyring.GetAddr(0).String(), differentAddr.String())

				_, _, err := s.factory.CallContractAndCheckLogs(s.keyring.GetPrivKey(0), txArgs, callArgs, voterSetCheck)
				Expect(err).To(BeNil())
			})

			It("should vote success", func() {
				callArgs.Args = []interface{}{
					s.keyring.GetAddr(0), proposalID, option, metadata,
				}

				voterSetCheck := passCheck.WithExpEvents(gov.EventTypeVote)

				_, _, err := s.factory.CallContractAndCheckLogs(s.keyring.GetPrivKey(0), txArgs, callArgs, voterSetCheck)
				Expect(err).To(BeNil(), "error while calling the precompile")

				// tally result yes count should updated
				proposal, _ := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposalID)
				_, _, tallyResult, err := s.network.App.GetGovKeeper().Tally(s.network.GetContext(), proposal)
				Expect(err).To(BeNil())

				Expect(tallyResult.YesCount).To(Equal(math.NewInt(3e18).String()), "expected tally result yes count updated")
			})
		})

		Describe("Execute VoteWeighted transaction", func() {
			const method = gov.VoteWeightedMethod

			BeforeEach(func() {
				callArgs.MethodName = method
			})

			It("should return error if the provided gasLimit is too low", func() {
				txArgs.GasLimit = 30000
				callArgs.Args = []interface{}{
					s.keyring.GetAddr(0),
					proposalID,
					[]gov.WeightedVoteOption{
						{Option: 1, Weight: "0.5"},
						{Option: 2, Weight: "0.5"},
					},
					metadata,
				}

				_, _, err := s.factory.CallContractAndCheckLogs(s.keyring.GetPrivKey(0), txArgs, callArgs, outOfGasCheck)
				Expect(err).To(BeNil())

				// tally result should remain unchanged
				proposal, _ := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposalID)
				_, _, tallyResult, err := s.network.App.GetGovKeeper().Tally(s.network.GetContext(), proposal)
				Expect(err).To(BeNil())
				Expect(tallyResult.YesCount).To(Equal("0"), "expected tally result to remain unchanged")
			})

			It("should return error if the origin is different than the voter", func() {
				callArgs.Args = []interface{}{
					differentAddr,
					proposalID,
					[]gov.WeightedVoteOption{
						{Option: 1, Weight: "0.5"},
						{Option: 2, Weight: "0.5"},
					},
					metadata,
				}

				voterSetCheck := defaultLogCheck.WithErrContains(cmn.ErrRequesterIsNotMsgSender, s.keyring.GetAddr(0).String(), differentAddr.String())

				_, _, err := s.factory.CallContractAndCheckLogs(s.keyring.GetPrivKey(0), txArgs, callArgs, voterSetCheck)
				Expect(err).To(BeNil())
			})

			It("should vote weighted success", func() {
				callArgs.Args = []interface{}{
					s.keyring.GetAddr(0),
					proposalID,
					[]gov.WeightedVoteOption{
						{Option: 1, Weight: "0.7"},
						{Option: 2, Weight: "0.3"},
					},
					metadata,
				}

				voterSetCheck := passCheck.WithExpEvents(gov.EventTypeVoteWeighted)

				_, _, err := s.factory.CallContractAndCheckLogs(s.keyring.GetPrivKey(0), txArgs, callArgs, voterSetCheck)
				Expect(err).To(BeNil(), "error while calling the precompile")

				// tally result should be updated
				proposal, _ := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposalID)
				_, _, tallyResult, err := s.network.App.GetGovKeeper().Tally(s.network.GetContext(), proposal)
				Expect(err).To(BeNil())

				expectedYesCount := math.NewInt(21e17) // 70% of 3e18
				Expect(tallyResult.YesCount).To(Equal(expectedYesCount.String()), "expected tally result yes count updated")

				expectedAbstainCount := math.NewInt(9e17) // 30% of 3e18
				Expect(tallyResult.AbstainCount).To(Equal(expectedAbstainCount.String()), "expected tally result no count updated")
			})
		})

		// =====================================
		// 				QUERIES
		// =====================================
		Describe("Execute queries", func() {
			Context("vote query", func() {
				method := gov.GetVoteMethod
				BeforeEach(func() {
					// submit a vote
					voteArgs := testutiltypes.CallArgs{
						ContractABI: s.precompile.ABI,
						MethodName:  gov.VoteMethod,
						Args: []interface{}{
							s.keyring.GetAddr(0), proposalID, option, metadata,
						},
					}

					voterSetCheck := passCheck.WithExpEvents(gov.EventTypeVote)

					_, _, err := s.factory.CallContractAndCheckLogs(s.keyring.GetPrivKey(0), txArgs, voteArgs, voterSetCheck)
					Expect(err).To(BeNil(), "error while calling the precompile")
					Expect(s.network.NextBlock()).To(BeNil())
				})
				It("should return a vote", func() {
					callArgs.MethodName = method
					callArgs.Args = []interface{}{proposalID, s.keyring.GetAddr(0)}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

					var out gov.VoteOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					Expect(out.Vote.Voter).To(Equal(s.keyring.GetAddr(0)))
					Expect(out.Vote.ProposalId).To(Equal(proposalID))
					Expect(out.Vote.Metadata).To(Equal(metadata))
					Expect(out.Vote.Options).To(HaveLen(1))
					Expect(out.Vote.Options[0].Option).To(Equal(option))
					Expect(out.Vote.Options[0].Weight).To(Equal(math.LegacyOneDec().String()))
				})
			})

			Context("weighted vote query", func() {
				method := gov.GetVoteMethod
				BeforeEach(func() {
					// submit a weighted vote
					voteArgs := testutiltypes.CallArgs{
						ContractABI: s.precompile.ABI,
						MethodName:  gov.VoteWeightedMethod,
						Args: []interface{}{
							s.keyring.GetAddr(0),
							proposalID,
							[]gov.WeightedVoteOption{
								{Option: 1, Weight: "0.7"},
								{Option: 2, Weight: "0.3"},
							},
							metadata,
						},
					}

					voterSetCheck := passCheck.WithExpEvents(gov.EventTypeVoteWeighted)

					_, _, err := s.factory.CallContractAndCheckLogs(s.keyring.GetPrivKey(0), txArgs, voteArgs, voterSetCheck)
					Expect(err).To(BeNil(), "error while calling the precompile")
					Expect(s.network.NextBlock()).To(BeNil())
				})

				It("should return a weighted vote", func() {
					callArgs.MethodName = method
					callArgs.Args = []interface{}{proposalID, s.keyring.GetAddr(0)}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

					var out gov.VoteOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					Expect(out.Vote.Voter).To(Equal(s.keyring.GetAddr(0)))
					Expect(out.Vote.ProposalId).To(Equal(proposalID))
					Expect(out.Vote.Metadata).To(Equal(metadata))
					Expect(out.Vote.Options).To(HaveLen(2))
					Expect(out.Vote.Options[0].Option).To(Equal(uint8(1)))
					Expect(out.Vote.Options[0].Weight).To(Equal("0.7"))
					Expect(out.Vote.Options[1].Option).To(Equal(uint8(2)))
					Expect(out.Vote.Options[1].Weight).To(Equal("0.3"))
				})
			})

			Context("votes query", func() {
				method := gov.GetVotesMethod
				BeforeEach(func() {
					// submit votes
					for _, key := range s.keyring.GetKeys() {
						voteArgs := testutiltypes.CallArgs{
							ContractABI: s.precompile.ABI,
							MethodName:  gov.VoteMethod,
							Args: []interface{}{
								key.Addr, proposalID, option, metadata,
							},
						}

						voterSetCheck := passCheck.WithExpEvents(gov.EventTypeVote)

						_, _, err := s.factory.CallContractAndCheckLogs(key.Priv, txArgs, voteArgs, voterSetCheck)
						Expect(err).To(BeNil(), "error while calling the precompile")
						Expect(s.network.NextBlock()).To(BeNil())
					}
				})
				It("should return all votes", func() {
					callArgs.MethodName = method
					callArgs.Args = []interface{}{
						proposalID,
						query.PageRequest{
							CountTotal: true,
						},
					}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

					var out gov.VotesOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					votersCount := len(s.keyring.GetKeys())
					Expect(out.PageResponse.Total).To(Equal(uint64(votersCount)))
					Expect(out.PageResponse.NextKey).To(Equal([]byte{}))
					Expect(out.Votes).To(HaveLen(votersCount))
					for _, v := range out.Votes {
						Expect(v.ProposalId).To(Equal(proposalID))
						Expect(v.Metadata).To(Equal(metadata))
						Expect(v.Options).To(HaveLen(1))
						Expect(v.Options[0].Option).To(Equal(option))
						Expect(v.Options[0].Weight).To(Equal(math.LegacyOneDec().String()))
					}
				})
			})

			Context("deposit query", func() {
				method := gov.GetDepositMethod
				BeforeEach(func() {
					callArgs.MethodName = method
				})

				It("should return a deposit", func() {
					callArgs.Args = []interface{}{proposalID, s.keyring.GetAddr(0)}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

					var out gov.DepositOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					Expect(out.Deposit.ProposalId).To(Equal(proposalID))
					Expect(out.Deposit.Depositor).To(Equal(s.keyring.GetAddr(0)))
					Expect(out.Deposit.Amount).To(HaveLen(1))
					Expect(out.Deposit.Amount[0].Denom).To(Equal(s.network.GetBaseDenom()))
					Expect(out.Deposit.Amount[0].Amount.Cmp(big.NewInt(100))).To(Equal(0))
				})
			})

			Context("deposits query", func() {
				method := gov.GetDepositsMethod
				BeforeEach(func() {
					callArgs.MethodName = method
				})

				It("should return all deposits", func() {
					callArgs.Args = []interface{}{
						proposalID,
						query.PageRequest{
							CountTotal: true,
						},
					}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

					var out gov.DepositsOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					Expect(out.PageResponse.Total).To(Equal(uint64(1)))
					Expect(out.PageResponse.NextKey).To(Equal([]byte{}))
					Expect(out.Deposits).To(HaveLen(1))
					for _, d := range out.Deposits {
						Expect(d.ProposalId).To(Equal(proposalID))
						Expect(d.Amount).To(HaveLen(1))
						Expect(d.Amount[0].Denom).To(Equal(s.network.GetBaseDenom()))
						Expect(d.Amount[0].Amount.Cmp(big.NewInt(100))).To(Equal(0))
					}
				})
			})

			Context("tally result query", func() {
				method := gov.GetTallyResultMethod
				BeforeEach(func() {
					callArgs.MethodName = method
					voteArgs := testutiltypes.CallArgs{
						ContractABI: s.precompile.ABI,
						MethodName:  gov.VoteMethod,
						Args: []interface{}{
							s.keyring.GetAddr(0), proposalID, option, metadata,
						},
					}

					voterSetCheck := passCheck.WithExpEvents(gov.EventTypeVote)

					_, _, err := s.factory.CallContractAndCheckLogs(s.keyring.GetPrivKey(0), txArgs, voteArgs, voterSetCheck)
					Expect(err).To(BeNil(), "error while calling the precompile")
					Expect(s.network.NextBlock()).To(BeNil())
				})

				It("should return the tally result", func() {
					callArgs.Args = []interface{}{proposalID}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

					var out gov.TallyResultOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					Expect(out.TallyResult.Yes).To(Equal("3000000000000000000"))
					Expect(out.TallyResult.Abstain).To(Equal("0"))
					Expect(out.TallyResult.No).To(Equal("0"))
					Expect(out.TallyResult.NoWithVeto).To(Equal("0"))
				})
			})

			Context("proposal query", func() {
				method := gov.GetProposalMethod
				BeforeEach(func() {
					callArgs.MethodName = method
				})

				It("should return a proposal", func() {
					callArgs.Args = []interface{}{uint64(1)}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

					var out gov.ProposalOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					// Check proposal details
					Expect(out.Proposal.Id).To(Equal(uint64(1)))
					Expect(out.Proposal.Status).To(Equal(uint32(govv1.StatusVotingPeriod)))
					Expect(out.Proposal.Proposer).To(Equal(s.keyring.GetAddr(0)))
					Expect(out.Proposal.Metadata).To(Equal("ipfs://CID"))
					Expect(out.Proposal.Title).To(Equal("test prop"))
					Expect(out.Proposal.Summary).To(Equal("test prop"))
					Expect(out.Proposal.Messages).To(HaveLen(1))
					Expect(out.Proposal.Messages[0]).To(Equal("/cosmos.bank.v1beta1.MsgSend"))

					// Check tally result
					Expect(out.Proposal.FinalTallyResult.Yes).To(Equal("0"))
					Expect(out.Proposal.FinalTallyResult.Abstain).To(Equal("0"))
					Expect(out.Proposal.FinalTallyResult.No).To(Equal("0"))
					Expect(out.Proposal.FinalTallyResult.NoWithVeto).To(Equal("0"))
				})

				It("should fail when proposal doesn't exist", func() {
					callArgs.Args = []interface{}{uint64(999)}

					_, _, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						defaultLogCheck.WithErrContains("proposal 999 doesn't exist"),
					)
					Expect(err).To(BeNil())
				})
			})

			Context("proposals query", func() {
				method := gov.GetProposalsMethod
				BeforeEach(func() {
					callArgs.MethodName = method
				})

				It("should return all proposals", func() {
					callArgs.Args = []interface{}{
						uint32(0), // StatusNil to get all proposals
						common.Address{},
						common.Address{},
						query.PageRequest{
							CountTotal: true,
						},
					}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

					var out gov.ProposalsOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					Expect(out.Proposals).To(HaveLen(2))
					Expect(out.PageResponse.Total).To(Equal(uint64(2)))

					proposal := out.Proposals[0]
					Expect(proposal.Id).To(Equal(uint64(1)))
					Expect(proposal.Status).To(Equal(uint32(govv1.StatusVotingPeriod)))
					Expect(proposal.Proposer).To(Equal(s.keyring.GetAddr(0)))
					Expect(proposal.Messages).To(HaveLen(1))
					Expect(proposal.Messages[0]).To(Equal("/cosmos.bank.v1beta1.MsgSend"))
				})

				It("should filter proposals by status", func() {
					callArgs.Args = []interface{}{
						uint32(govv1.StatusVotingPeriod),
						common.Address{},
						common.Address{},
						query.PageRequest{
							CountTotal: true,
						},
					}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil())

					var out gov.ProposalsOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					Expect(out.Proposals).To(HaveLen(2))
					Expect(out.Proposals[0].Status).To(Equal(uint32(govv1.StatusVotingPeriod)))
					Expect(out.Proposals[1].Status).To(Equal(uint32(govv1.StatusVotingPeriod)))
				})

				It("should filter proposals by voter", func() {
					// First add a vote
					voteArgs := testutiltypes.CallArgs{
						ContractABI: s.precompile.ABI,
						MethodName:  gov.VoteMethod,
						Args: []interface{}{
							s.keyring.GetAddr(0), uint64(1), uint8(govv1.OptionYes), "",
						},
					}
					_, _, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						voteArgs,
						passCheck.WithExpEvents(gov.EventTypeVote),
					)
					Expect(err).To(BeNil())

					// Wait for the vote to be included in the block
					Expect(s.network.NextBlock()).To(BeNil())

					// Query proposals filtered by voter
					callArgs.Args = []interface{}{
						uint32(0), // StatusNil
						s.keyring.GetAddr(0),
						common.Address{},
						query.PageRequest{
							CountTotal: true,
						},
					}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil())

					var out gov.ProposalsOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					Expect(out.Proposals).To(HaveLen(1))
				})

				It("should filter proposals by depositor", func() {
					callArgs.Args = []interface{}{
						uint32(0), // StatusNil
						common.Address{},
						s.keyring.GetAddr(0),
						query.PageRequest{
							CountTotal: true,
						},
					}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil())

					var out gov.ProposalsOutput
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())

					Expect(out.Proposals).To(HaveLen(1))
				})
			})

			Context("params query", func() {
				var (
					err                   error
					callsData             CallsData
					govCallerContractAddr common.Address
					govCallerContract     evmtypes.CompiledContract
				)

				BeforeEach(func() {
					// Setting gas tip cap to zero to have zero gas price.
					txArgs.GasTipCap = new(big.Int).SetInt64(0)

					govCallerContract, err = contracts.LoadGovCallerContract()
					Expect(err).ToNot(HaveOccurred(), "failed to load GovCaller contract")

					govCallerContractAddr, err = s.factory.DeployContract(
						s.keyring.GetPrivKey(0),
						evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
						testutiltypes.ContractDeploymentData{
							Contract: govCallerContract,
						},
					)
					Expect(err).ToNot(HaveOccurred(), "failed to deploy gov caller contract")
					Expect(s.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")

					callsData = CallsData{
						precompileAddr: s.precompile.Address(),
						precompileABI:  s.precompile.ABI,

						precompileCallerAddr: govCallerContractAddr,
						precompileCallerABI:  govCallerContract.ABI,
					}
				})

				DescribeTable("should return all params", func(callType callType) {
					txArgs, callArgs = callsData.getTxAndCallArgs(callArgs, txArgs, callType)

					switch callType {
					case directCall:
						callArgs.MethodName = gov.GetParamsMethod
					case contractCall:
						callArgs.MethodName = "getParams"
					}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil())

					var output struct {
						Params gov.ParamsOutput `json:"params"`
					}
					err = s.precompile.UnpackIntoInterface(&output, gov.GetParamsMethod, ethRes.Ret)
					Expect(err).To(BeNil())

					params, err := s.network.GetGovClient().Params(s.network.GetContext(), &govv1.QueryParamsRequest{})
					Expect(err).To(BeNil())

					Expect(output.Params.MinDeposit).To(HaveLen(len(params.Params.MinDeposit)), "expected min deposit to have same amount of token")
					Expect(output.Params.MinDeposit[0].Denom).To(Equal(params.Params.MinDeposit[0].Denom), "expected min deposit to have same denom")
					Expect(output.Params.MinDeposit[0].Amount.String()).To(Equal(params.Params.MinDeposit[0].Amount.String()), "expected min deposit to have same amount")
					Expect(output.Params.MaxDepositPeriod).To(Equal(int64(*params.Params.MaxDepositPeriod)), "expected max deposit period to be equal")
					Expect(output.Params.VotingPeriod).To(Equal(int64(*params.Params.VotingPeriod)), "expected voting period to be equal")
					Expect(output.Params.Quorum).To(Equal(params.Params.Quorum), "expected quorum to be equal")
					Expect(output.Params.Threshold).To(Equal(params.Params.Threshold), "expected threshold to be equal")
					Expect(output.Params.VetoThreshold).To(Equal(params.Params.VetoThreshold), "expected veto threshold to be equal")
					Expect(output.Params.MinDepositRatio).To(Equal(params.Params.MinDepositRatio), "expected min deposit ratio to be equal")
					Expect(output.Params.ProposalCancelRatio).To(Equal(params.Params.ProposalCancelRatio), "expected proposal cancel ratio to be equal")
					Expect(output.Params.ProposalCancelDest).To(Equal(params.Params.ProposalCancelDest), "expected proposal cancel dest to be equal")
					Expect(output.Params.ExpeditedVotingPeriod).To(Equal(int64(*params.Params.ExpeditedVotingPeriod)), "expected expedited voting period to be equal")
					Expect(output.Params.ExpeditedThreshold).To(Equal(params.Params.ExpeditedThreshold), "expected expedited threshold to be equal")
					Expect(output.Params.ExpeditedMinDeposit).To(HaveLen(len(params.Params.ExpeditedMinDeposit)), "expected expedited min deposit to have same amount of token")
					Expect(output.Params.ExpeditedMinDeposit[0].Denom).To(Equal(params.Params.ExpeditedMinDeposit[0].Denom), "expected expedited min deposit to have same denom")
					Expect(output.Params.ExpeditedMinDeposit[0].Amount.String()).To(Equal(params.Params.ExpeditedMinDeposit[0].Amount.String()), "expected expedited min deposit to have same amount")
					Expect(output.Params.BurnVoteQuorum).To(Equal(params.Params.BurnVoteQuorum), "expected burn vote quorum to be equal")
					Expect(output.Params.BurnProposalDepositPrevote).To(Equal(params.Params.BurnProposalDepositPrevote), "expected burn proposal deposit prevote to be equal")
					Expect(output.Params.BurnVoteVeto).To(Equal(params.Params.BurnVoteVeto), "expected burn vote veto to be equal")
					Expect(output.Params.MinDepositRatio).To(Equal(params.Params.MinDepositRatio), "expected min deposit ratio to be equal")
				},
					Entry("directly calling the precompile", directCall),
					Entry("through a caller contract", contractCall),
				)
			})

			Context("constitution query", func() {
				method := gov.GetConstitutionMethod
				BeforeEach(func() {
					callArgs.MethodName = method
				})

				It("should return a constitution", func() {
					callArgs.Args = []interface{}{}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(proposerKey, txArgs, callArgs, passCheck)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

					var out string
					err = s.precompile.UnpackIntoInterface(&out, method, ethRes.Ret)
					Expect(err).To(BeNil())
				})
			})
		})
	})
	_ = Describe("Calling governance precompile from contract", Ordered, func() {
		s := NewPrecompileTestSuite(create, options...)
		// testCase is a struct used for cases of contracts calls that have some operation
		// performed before and/or after the precompile call
		type testCase struct {
			before bool
			after  bool
		}

		var (
			govCallerContract   evmtypes.CompiledContract
			contractAddr        common.Address
			contractAccAddr     sdk.AccAddress
			contractAddrDupe    common.Address
			contractAccAddrDupe sdk.AccAddress
			txSenderKey         types.PrivKey
			txSenderAddr        common.Address
			err                 error

			proposalID         uint64 // proposal id submitted by eoa
			contractProposalID uint64 // proposal id submitted by contract account

			cancelFee math.Int
			remaining math.Int

			depositor1    sdk.AccAddress
			depositorKey1 types.PrivKey

			// The following variables are used to check the cancellation fees and
			// remaining fess for multiple deposits case
			// key: acc address
			// value: fee amount
			cancelFees    map[string]math.Int
			remainingFees map[string]math.Int
		)

		BeforeAll(func() {
			govCallerContract, err = contracts.LoadGovCallerContract()
			Expect(err).ToNot(HaveOccurred(), "failed to load GovCaller contract")
		})

		BeforeEach(func() {
			s.SetupTest()

			txSenderKey = s.keyring.GetPrivKey(0)
			txSenderAddr = s.keyring.GetAddr(0)
			contractAddr, err = s.factory.DeployContract(
				txSenderKey,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: govCallerContract,
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to deploy gov caller contract")
			Expect(s.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
			contractAccAddr = sdk.AccAddress(contractAddr.Bytes())

			cAcc := s.network.App.GetEVMKeeper().GetAccount(s.network.GetContext(), contractAddr)
			Expect(cAcc).ToNot(BeNil(), "failed to get contract account")
			isContract := s.network.App.GetEVMKeeper().IsContract(s.network.GetContext(), contractAddr)
			Expect(isContract).To(BeTrue(), "expected contract account")

			contractAddrDupe, err = s.factory.DeployContract(
				txSenderKey,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: govCallerContract,
				},
			)
			Expect(err).ToNot(HaveOccurred(), "failed to deploy dupe gov caller contract")
			Expect(s.network.NextBlock()).ToNot(HaveOccurred(), "error on NextBlock")
			contractAccAddrDupe = sdk.AccAddress(contractAddrDupe.Bytes())

			cAccDupe := s.network.App.GetEVMKeeper().GetAccount(s.network.GetContext(), contractAddrDupe)
			Expect(cAccDupe).ToNot(BeNil(), "failed to get dupe contract account")
			isContract = s.network.App.GetEVMKeeper().IsContract(s.network.GetContext(), contractAddrDupe)
			Expect(isContract).To(BeTrue(), "expected dupe contract account")

			callArgs = testutiltypes.CallArgs{
				ContractABI: govCallerContract.ABI,
			}

			txArgs = evmtypes.EvmTxArgs{
				To:       &contractAddr,
				GasLimit: 200_000,
			}
			govModuleAddr = authtypes.NewModuleAddress(govtypes.ModuleName)

			defaultLogCheck = testutil.LogCheckArgs{ABIEvents: s.precompile.Events}
			passCheck = defaultLogCheck.WithExpPass(true)
		})

		// =====================================
		// 				TRANSACTIONS
		// =====================================
		Context("submitProposal as a contract proposer", func() {
			BeforeEach(func() { callArgs.MethodName = testSubmitProposalFromContract })
			It("should submit proposal successfully", func() {
				// Prepare the proposal
				toAddr := s.keyring.GetAccAddr(1)
				denom := s.network.GetBaseDenom()
				amount := "100"
				jsonBlob := minimalBankSendProposalJSON(toAddr, denom, amount)
				callArgs.Args = []interface{}{
					jsonBlob,
					minimalDeposit(s.network.GetBaseDenom(), big.NewInt(100)),
				}

				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)

				txArgs := evmtypes.EvmTxArgs{
					To:       &contractAddr,
					GasLimit: 500_000,
					Amount:   big.NewInt(1000),
				}
				_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				var proposalID uint64
				err = s.precompile.UnpackIntoInterface(&proposalID, gov.SubmitProposalMethod, evmRes.Ret)
				Expect(err).To(BeNil())
				// Expect ProposalID greater than 0
				Expect(proposalID).To(BeNumerically(">", 0))

				contractProposer := sdk.AccAddress(contractAddr.Bytes()).String()
				// ensure proposal exists on-chain
				prop, err := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposalID)
				Expect(err).To(BeNil())
				Expect(prop.Id).To(Equal(proposalID))
				Expect(prop.Proposer).To(Equal(contractProposer), "expected contract proposer to be equal")
			})
		})

		Context("cancelProposal as contract proposer", func() {
			BeforeEach(func() { callArgs.MethodName = "testCancelProposalFromContract" })
			It("should cancel proposal successfully", func() {
				// submit a proposal
				toAddr := s.keyring.GetAccAddr(1)
				denom := s.network.GetBaseDenom()
				jsonBlob := minimalBankSendProposalJSON(toAddr, denom, "100")
				callArgs.MethodName = testSubmitProposalFromContract
				minDepositAmt := math.NewInt(100)
				callArgs.Args = []interface{}{
					jsonBlob,
					minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
				}

				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)

				txArgs := evmtypes.EvmTxArgs{
					To:       &contractAddr,
					GasLimit: 500_000,
					Amount:   minDepositAmt.BigInt(),
				}
				_, evmRes, _ := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(s.network.NextBlock()).To(BeNil())

				var proposalID uint64
				Expect(s.precompile.UnpackIntoInterface(&proposalID, gov.SubmitProposalMethod, evmRes.Ret)).To(BeNil())

				// Get the proposal for cancellation
				proposal, err := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposalID)
				Expect(err).To(BeNil())

				// Calc cancellation fee
				proposalDeposits, err := s.network.App.GetGovKeeper().GetDeposits(s.network.GetContext(), proposal.Id)
				Expect(err).To(BeNil())
				proposalDepositAmt := proposalDeposits[0].Amount[0].Amount
				params, err := s.network.App.GetGovKeeper().Params.Get(s.network.GetContext())
				Expect(err).To(BeNil())
				rate := math.LegacyMustNewDecFromStr(params.ProposalCancelRatio)
				cancelFee := proposalDepositAmt.ToLegacyDec().Mul(rate).TruncateInt()

				// Cancel it
				callArgs.MethodName = "testCancelProposalFromContract"
				callArgs.Args = []interface{}{proposal.Id}
				eventCheck = passCheck.WithExpEvents(gov.EventTypeCancelProposal)
				// Balance of contract proposer
				proposerBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, s.network.GetBaseDenom())
				txArgs.Amount = common.Big0
				_, _, err = s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				// 6. Check that the cancellation fee is charged, diff should be less than the deposit amount
				afterCancelBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, s.network.GetBaseDenom())
				Expect(afterCancelBal.Amount).To(Equal(
					proposerBal.Amount.
						Sub(cancelFee).
						Add(proposalDepositAmt)),
					"expected cancellation fee to be deducted from proposer balance")

				// 7. Check that the proposal is not found
				_, err = s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposal.Id)
				Expect(err.Error()).To(ContainSubstring("not found"))
			})
		})

		Context("deposit as contract proposer", func() {
			BeforeEach(func() { callArgs.MethodName = testDepositFromContract })
			It("should deposit successfully", func() {
				// submit a proposal
				toAddr := s.keyring.GetAccAddr(1)
				denom := s.network.GetBaseDenom()
				jsonBlob := minimalBankSendProposalJSON(toAddr, denom, "100")
				callArgs.MethodName = testSubmitProposalFromContract
				minDepositAmt := math.NewInt(100)
				callArgs.Args = []interface{}{
					jsonBlob,
					minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
				}

				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
				txArgs := evmtypes.EvmTxArgs{
					To:       &contractAddr,
					GasLimit: 500_000,
					Amount:   minDepositAmt.BigInt(),
				}
				_, evmRes, _ := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(s.network.NextBlock()).To(BeNil())

				var proposalID uint64
				Expect(s.precompile.UnpackIntoInterface(&proposalID, gov.SubmitProposalMethod, evmRes.Ret)).To(BeNil())

				// Get the proposal for deposit
				proposal, err := s.network.App.GetGovKeeper().Proposals.Get(s.network.GetContext(), proposalID)
				Expect(err).To(BeNil())

				// Deposit it
				callArgs.MethodName = "testDepositFromContract"
				callArgs.Args = []interface{}{
					proposal.Id,
					minimalDeposit(s.network.GetBaseDenom(), big.NewInt(100)),
				}
				eventCheck = passCheck.WithExpEvents(gov.EventTypeDeposit)
				_, _, err = s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				// Check that the deposit is found
				deposits, err := s.network.App.GetGovKeeper().GetDeposits(s.network.GetContext(), proposal.Id)
				Expect(err).To(BeNil())
				Expect(deposits).To(HaveLen(1))
				Expect(deposits[0].Amount[0].Amount).To(Equal(math.NewInt(200)))
			})
		})

		Context("testSubmitProposal with transfer", func() {
			BeforeEach(func() { callArgs.MethodName = "testSubmitProposalWithTransfer" })

			DescribeTable("contract proposer should submit proposal with transfer",
				func(tc testCase) {
					// Fix the gas limit and gas price for predictable gas usage.
					// This is for calculating expected cancellation fee.
					baseFee := s.network.App.GetFeeMarketKeeper().GetBaseFee(s.network.GetContext())
					baseFeeInt := baseFee.TruncateInt64()
					txArgs.GasPrice = new(big.Int).SetInt64(baseFeeInt)
					txArgs.GasLimit = 500_000

					// Prepare the proposal
					toAddr := s.keyring.GetAccAddr(1)
					denom := s.network.GetBaseDenom()
					amount := "100"
					jsonBlob := minimalBankSendProposalJSON(toAddr, denom, amount)
					minDepositAmt := math.NewInt(100)
					callArgs.Args = []interface{}{
						jsonBlob, minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
						tc.before, tc.after,
					}
					txArgs.Amount = minDepositAmt.Mul(math.NewInt(2)).BigInt()
					eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
					txArgs.To = &contractAddr
					baseDenom := s.network.GetBaseDenom()
					txSender := s.keyring.GetAccAddr(0)
					txSenderKey := s.keyring.GetPrivKey(0)
					txSenderBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), txSender, baseDenom)
					contractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)
					res, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
					Expect(err).To(BeNil())
					Expect(s.network.NextBlock()).To(BeNil())

					fees := math.NewInt(res.GasUsed).Mul(math.NewInt(txArgs.GasPrice.Int64()))

					// check submitted proposal
					var proposalID uint64
					err = s.precompile.UnpackIntoInterface(&proposalID, gov.SubmitProposalMethod, evmRes.Ret)
					Expect(err).To(BeNil())
					Expect(proposalID).To(BeNumerically(">", 0))

					afterSubmitTxSenderBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), txSender, baseDenom)
					afterSubmitContractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)

					amtFromContract := math.ZeroInt()
					for _, transferred := range []bool{tc.before, tc.after} {
						if transferred {
							amtFromContract = amtFromContract.AddRaw(15)
						}
					}
					Expect(afterSubmitTxSenderBal.Amount).To(Equal(
						txSenderBal.Amount.Sub(math.NewIntFromBigInt(txArgs.Amount)).
							Sub(fees).Add(amtFromContract)))
					Expect(afterSubmitContractBal.Amount).To(Equal(
						contractBal.Amount.
							Add(math.NewIntFromBigInt(txArgs.Amount).
								Sub(amtFromContract)).Sub(minDepositAmt),
					))
				},
				Entry("with internal transfers before and after precompile call", testCase{
					before: true,
					after:  true,
				}),
				Entry("with internal transfers before precompile call", testCase{
					before: true,
					after:  false,
				}),
				Entry("with internal transfers after precompile call", testCase{
					before: false,
					after:  true,
				}),
			)
		})

		Context("testRefunds security issue", func() {
			var minDepositAmt math.Int

			BeforeEach(func() {
				toAddr := s.keyring.GetAccAddr(1)
				denom := s.network.GetBaseDenom()
				amount := "100"
				jsonBlob := minimalBankSendProposalJSON(toAddr, denom, amount)
				minDepositAmt = math.NewInt(100)
				callArgs.MethodName = testSubmitProposalFromContract
				callArgs.Args = []interface{}{
					jsonBlob,
					minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
				}
				txArgs.Amount = minDepositAmt.BigInt()
				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
				txArgs.To = &contractAddr

				// 1. Submit gov prop for contract 1
				_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				err = s.precompile.UnpackIntoInterface(&contractProposalID, gov.SubmitProposalMethod, evmRes.Ret)
				Expect(err).To(BeNil())

				// 2. Deposit to gov prop from contract 2
				txArgs.To = &contractAddrDupe
				txArgs.GasLimit = 1_000_000_000
				callArgs.MethodName = testDepositFromContract
				callArgs.Args = []interface{}{
					contractProposalID,
					minimalDeposit(s.network.GetBaseDenom(), big.NewInt(100)),
				}
				eventCheck = passCheck.WithExpEvents(gov.EventTypeDeposit)
				_, _, err = s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				// Check that the deposit is found
				deposits, err := s.network.App.GetGovKeeper().GetDeposits(s.network.GetContext(), contractProposalID)
				Expect(err).To(BeNil())
				Expect(deposits).To(HaveLen(2))
				Expect(deposits[0].Amount[0].Amount).To(Equal(math.NewInt(100)))
				Expect(deposits[1].Amount[0].Amount).To(Equal(math.NewInt(100)))
			})

			Describe("test transferCancelFund", func() {
				It("should cancel proposal and fund to communityPool", func() {
					baseDenom := s.network.GetBaseDenom()
					txArgs.To = &contractAddr
					txArgs.GasLimit = 1_000_000_000
					callArgs.MethodName = "testTransferCancelFund"
					callArgs.Args = []interface{}{
						contractAddrDupe,
						contractProposalID,
						[]byte(baseDenom),
						s.network.GetValidators()[0].OperatorAddress,
					}
					// Call the contract
					_, err := s.factory.ExecuteContractCall(txSenderKey, txArgs, callArgs)
					Expect(err).To(BeNil())
					Expect(s.network.NextBlock()).To(BeNil())

					params, err := s.network.App.GetGovKeeper().Params.Get(s.network.GetContext())
					Expect(err).To(BeNil())

					cancelRatio := math.LegacyMustNewDecFromStr(params.ProposalCancelRatio)
					cancelFee := minDepositAmt.ToLegacyDec().Mul(cancelRatio).TruncateInt()
					transferAmount := math.NewInt(1)
					fundCommunityPoolAmount := math.NewInt(2)
					expectedDepositorBal := minDepositAmt.
						Sub(cancelFee).
						Add(transferAmount).
						Sub(fundCommunityPoolAmount)

					afterDepositorBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddrDupe, baseDenom)
					Expect(afterDepositorBal.Amount).To(Equal(expectedDepositorBal))
				})
			},
			)
		})

		Context("testSubmitProposalFromContract with transfer", func() {
			BeforeEach(func() { callArgs.MethodName = "testSubmitProposalFromContractWithTransfer" })

			DescribeTable("contract proposer should submit proposal with transfer",
				func(tc testCase) {
					// Fix the gas limit and gas price for predictable gas usage.
					// This is for calculating expected cancellation fee.
					baseFee := s.network.App.GetFeeMarketKeeper().GetBaseFee(s.network.GetContext())
					baseFeeInt := baseFee.TruncateInt64()
					txArgs.GasPrice = new(big.Int).SetInt64(baseFeeInt)
					txArgs.GasLimit = 500_000

					// Prepare the proposal
					toAddr := s.keyring.GetAccAddr(1)
					denom := s.network.GetBaseDenom()
					amount := "100"
					jsonBlob := minimalBankSendProposalJSON(toAddr, denom, amount)
					minDepositAmt := math.NewInt(100)
					randomAddr := testutiltx.GenerateAddress()
					callArgs.Args = []interface{}{
						randomAddr, jsonBlob,
						minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
						tc.before, tc.after,
					}
					extraContractFundinAmt := math.NewInt(100)
					txArgs.Amount = minDepositAmt.Add(extraContractFundinAmt).BigInt()
					eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
					txArgs.To = &contractAddr
					baseDenom := s.network.GetBaseDenom()
					contractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)
					randomAddrBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), randomAddr.Bytes(), baseDenom)
					_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
					Expect(err).To(BeNil())
					Expect(s.network.NextBlock()).To(BeNil())

					// check submitted proposal
					var proposalID uint64
					err = s.precompile.UnpackIntoInterface(&proposalID, gov.SubmitProposalMethod, evmRes.Ret)
					Expect(err).To(BeNil())
					Expect(proposalID).To(BeNumerically(">", 0))

					afterSubmitRandomAddrBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), randomAddr.Bytes(), baseDenom)
					afterSubmitContractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)

					amtFromContract := math.ZeroInt()
					for _, transferred := range []bool{tc.before, tc.after} {
						if transferred {
							amtFromContract = amtFromContract.AddRaw(15)
						}
					}
					Expect(afterSubmitRandomAddrBal.Amount).To(Equal(
						randomAddrBal.Amount.
							Add(amtFromContract),
					))

					Expect(afterSubmitContractBal.Amount).To(Equal(
						contractBal.Amount.Add(math.NewIntFromBigInt(txArgs.Amount).Sub(minDepositAmt).Sub(amtFromContract)),
					))
				},
				Entry("with internal transfers before and after precompile call", testCase{
					before: true,
					after:  true,
				}),
				Entry("with internal transfers before precompile call", testCase{
					before: true,
					after:  false,
				}),
				Entry("with internal transfers after precompile call", testCase{
					before: false,
					after:  true,
				}),
			)
		})

		Context("testDeposit with transfer", func() {
			BeforeEach(func() {
				toAddr := s.keyring.GetAccAddr(1)
				denom := s.network.GetBaseDenom()
				amount := "100"
				jsonBlob := minimalBankSendProposalJSON(toAddr, denom, amount)
				minDepositAmt := math.NewInt(100)
				callArgs.MethodName = testSubmitProposalFromContract
				callArgs.Args = []interface{}{
					jsonBlob,
					minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
				}
				txArgs.Amount = minDepositAmt.BigInt()
				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
				txArgs.To = &contractAddr
				_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				err = s.precompile.UnpackIntoInterface(&contractProposalID, gov.SubmitProposalMethod, evmRes.Ret)
				Expect(err).To(BeNil())

				callArgs.MethodName = "testDepositWithTransfer"
			})

			DescribeTable("all balance changes should be correct",
				func(tc testCase) {
					// Fix the gas limit and gas price for predictable gas usage.
					// This is for calculating expected cancellation fee.
					baseFee := s.network.App.GetFeeMarketKeeper().GetBaseFee(s.network.GetContext())
					baseFeeInt := baseFee.TruncateInt64()
					txArgs.GasPrice = new(big.Int).SetInt64(baseFeeInt)
					txArgs.GasLimit = 500_000
					txArgs.Amount = big.NewInt(300)

					minDepositAmt := math.NewInt(100)
					callArgs.Args = []interface{}{
						contractProposalID,
						minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
						tc.before, tc.after,
					}
					eventCheck := passCheck.WithExpEvents(gov.EventTypeDeposit)

					baseDenom := s.network.GetBaseDenom()
					contractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)
					txSenderBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), txSenderAddr.Bytes(), baseDenom)
					res, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
					Expect(err).To(BeNil())
					Expect(s.network.NextBlock()).To(BeNil())
					gasCost := math.NewInt(res.GasUsed).Mul(math.NewInt(txArgs.GasPrice.Int64()))

					var success bool
					err = s.precompile.UnpackIntoInterface(&success, gov.DepositMethod, evmRes.Ret)
					Expect(err).To(BeNil())
					Expect(success).To(BeTrue())

					afterTxSenderBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), txSenderAddr.Bytes(), baseDenom)
					afterContractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)

					amtFromContract := math.ZeroInt()
					for _, transferred := range []bool{tc.before, tc.after} {
						if transferred {
							amtFromContract = amtFromContract.AddRaw(15)
						}
					}
					Expect(afterTxSenderBal.Amount).To(Equal(
						txSenderBal.Amount.
							Sub(gasCost).
							Sub(math.NewIntFromBigInt(txArgs.Amount)).
							Add(amtFromContract),
					))

					Expect(afterContractBal.Amount).To(Equal(
						contractBal.Amount.
							Add(math.NewIntFromBigInt(txArgs.Amount).
								Sub(amtFromContract).
								Sub(minDepositAmt)),
					))
				},
				Entry("with internal transfers before and after precompile call", testCase{
					before: true,
					after:  true,
				}),
				Entry("with internal transfers before precompile call", testCase{
					before: true,
					after:  false,
				}),
				Entry("with internal transfers after precompile call", testCase{
					before: false,
					after:  true,
				}),
			)
		})

		Context("testDepositFromContract with transfer", func() {
			BeforeEach(func() {
				toAddr := s.keyring.GetAccAddr(1)
				denom := s.network.GetBaseDenom()
				amount := "100"
				jsonBlob := minimalBankSendProposalJSON(toAddr, denom, amount)
				minDepositAmt := math.NewInt(100)
				callArgs.MethodName = testSubmitProposalFromContract
				callArgs.Args = []interface{}{
					jsonBlob,
					minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
				}
				txArgs.Amount = minDepositAmt.BigInt()
				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
				txArgs.To = &contractAddr
				_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				err = s.precompile.UnpackIntoInterface(&contractProposalID, gov.SubmitProposalMethod, evmRes.Ret)
				Expect(err).To(BeNil())

				callArgs.MethodName = "testDepositFromContractWithTransfer"
			})

			DescribeTable("all balance changes should be correct",
				func(tc testCase) {
					minDepositAmt := math.NewInt(100)
					randomAddr := testutiltx.GenerateAddress()
					callArgs.Args = []interface{}{
						randomAddr, contractProposalID,
						minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
						tc.before, tc.after,
					}
					extraContractFundinAmt := math.NewInt(100)
					txArgs.Amount = minDepositAmt.Add(extraContractFundinAmt).BigInt()
					eventCheck := passCheck.WithExpEvents(gov.EventTypeDeposit)

					baseDenom := s.network.GetBaseDenom()
					randomAddrBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), randomAddr.Bytes(), baseDenom)
					contractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)

					_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
					Expect(err).To(BeNil())
					Expect(s.network.NextBlock()).To(BeNil())
					var success bool
					err = s.precompile.UnpackIntoInterface(&success, gov.DepositMethod, evmRes.Ret)
					Expect(err).To(BeNil())
					Expect(success).To(BeTrue())

					afterRandomAddrBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), randomAddr.Bytes(), baseDenom)
					afterContractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)

					amtFromContract := math.ZeroInt()
					for _, transferred := range []bool{tc.before, tc.after} {
						if transferred {
							amtFromContract = amtFromContract.AddRaw(15)
						}
					}

					Expect(afterRandomAddrBal.Amount).To(Equal(
						randomAddrBal.Amount.
							Add(amtFromContract),
					))
					Expect(afterContractBal.Amount).To(Equal(
						contractBal.Amount.
							Add(math.NewIntFromBigInt(txArgs.Amount).
								Sub(minDepositAmt).
								Sub(amtFromContract)),
					))
				},
				Entry("with internal transfers before and after precompile call", testCase{
					before: true,
					after:  true,
				}),
				Entry("with internal transfers before precompile call", testCase{
					before: true,
					after:  false,
				}),
				Entry("with internal transfers after precompile call", testCase{
					before: false,
					after:  true,
				}),
			)
		})

		Context("testCancel with transfer", func() {
			BeforeEach(func() {
				toAddr := s.keyring.GetAccAddr(1)
				denom := s.network.GetBaseDenom()
				amount := "100"
				jsonBlob := minimalBankSendProposalJSON(toAddr, denom, amount)
				minDepositAmt := math.NewInt(100)
				callArgs.MethodName = testSubmitProposalFromContract
				callArgs.Args = []interface{}{
					jsonBlob,
					minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
				}
				txArgs.Amount = minDepositAmt.BigInt()
				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
				txArgs.To = &contractAddr
				_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				err = s.precompile.UnpackIntoInterface(&proposalID, gov.SubmitProposalMethod, evmRes.Ret)
				Expect(err).To(BeNil())

				// Calc cancellation fee
				proposalDeposits, err := s.network.App.GetGovKeeper().GetDeposits(s.network.GetContext(), proposalID)
				Expect(err).To(BeNil())
				proposalDepositAmt := proposalDeposits[0].Amount[0].Amount
				params, err := s.network.App.GetGovKeeper().Params.Get(s.network.GetContext())
				Expect(err).To(BeNil())
				rate := math.LegacyMustNewDecFromStr(params.ProposalCancelRatio)
				cancelFee = proposalDepositAmt.ToLegacyDec().Mul(rate).TruncateInt()
				remaining = proposalDepositAmt.Sub(cancelFee)

				callArgs.MethodName = "testCancelWithTransfer"
			})

			DescribeTable("eoa proposer should cancel proposal with transfer",
				func(tc testCase) {
					// Fix the gas limit and gas ice for predictable gas usage.
					// This is for calculating expected cancellation fee.
					baseFee := s.network.App.GetFeeMarketKeeper().GetBaseFee(s.network.GetContext())
					baseFeeInt := baseFee.TruncateInt64()
					txArgs.GasPrice = new(big.Int).SetInt64(baseFeeInt)
					txArgs.GasLimit = 500_000
					txArgs.Amount = big.NewInt(100)

					callArgs.Args = []interface{}{
						proposalID,
						tc.before, tc.after,
					}
					eventCheck := passCheck.WithExpEvents(gov.EventTypeCancelProposal)

					baseDenom := s.network.GetBaseDenom()
					txSenderBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), txSenderAddr.Bytes(), baseDenom)
					contractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)

					res, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
					Expect(err).To(BeNil())
					Expect(s.network.NextBlock()).To(BeNil())
					var success bool
					err = s.precompile.UnpackIntoInterface(&success, gov.CancelProposalMethod, evmRes.Ret)
					Expect(err).To(BeNil())
					Expect(success).To(BeTrue())

					afterTxSenderBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), txSenderAddr.Bytes(), baseDenom)
					afterContractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)
					gasCost := math.NewInt(res.GasUsed).Mul(math.NewInt(txArgs.GasPrice.Int64()))
					amtFromContract := math.ZeroInt()
					for _, transferred := range []bool{tc.before, tc.after} {
						if transferred {
							amtFromContract = amtFromContract.AddRaw(15)
						}
					}

					Expect(afterTxSenderBal.Amount).To(Equal(
						txSenderBal.Amount.
							Sub(gasCost).
							Sub(math.NewIntFromBigInt(txArgs.Amount)).
							Add(amtFromContract),
					))
					Expect(afterContractBal.Amount).To(Equal(
						contractBal.Amount.
							Add(remaining).
							Add(math.NewIntFromBigInt(txArgs.Amount).
								Sub(amtFromContract)),
					))
				},
				Entry("with internal transfers before and after precompile call", testCase{
					before: true,
					after:  true,
				}),
				Entry("with internal transfers before precompile call", testCase{
					before: true,
					after:  false,
				}),
				Entry("with internal transfers after precompile call", testCase{
					before: false,
					after:  true,
				}),
			)
		})

		Context("testCancelFromContract with transfer", func() {
			BeforeEach(func() {
				toAddr := s.keyring.GetAccAddr(1)
				denom := s.network.GetBaseDenom()
				amount := "100"
				jsonBlob := minimalBankSendProposalJSON(toAddr, denom, amount)
				minDepositAmt := math.NewInt(100)
				callArgs.MethodName = testSubmitProposalFromContract
				callArgs.Args = []interface{}{
					jsonBlob,
					minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt()),
				}
				txArgs.Amount = minDepositAmt.BigInt()
				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
				txArgs.To = &contractAddr
				_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				err = s.precompile.UnpackIntoInterface(&contractProposalID, gov.SubmitProposalMethod, evmRes.Ret)
				Expect(err).To(BeNil())

				// Calc cancellation fee
				proposalDeposits, err := s.network.App.GetGovKeeper().GetDeposits(s.network.GetContext(), contractProposalID)
				Expect(err).To(BeNil())
				proposalDepositAmt := proposalDeposits[0].Amount[0].Amount
				params, err := s.network.App.GetGovKeeper().Params.Get(s.network.GetContext())
				Expect(err).To(BeNil())
				rate := math.LegacyMustNewDecFromStr(params.ProposalCancelRatio)
				cancelFee = proposalDepositAmt.ToLegacyDec().Mul(rate).TruncateInt()
				remaining = proposalDepositAmt.Sub(cancelFee)

				callArgs.MethodName = "testCancelFromContractWithTransfer"
			})

			DescribeTable("contract proposer should cancel proposal with transfer",
				func(tc testCase) {
					randomAddr := testutiltx.GenerateAddress()
					callArgs.Args = []interface{}{
						randomAddr,
						contractProposalID,
						tc.before, tc.after,
					}
					eventCheck := passCheck.WithExpEvents(gov.EventTypeCancelProposal)

					baseDenom := s.network.GetBaseDenom()
					cancellerBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)
					randomAddrBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), randomAddr.Bytes(), baseDenom)

					_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
					Expect(err).To(BeNil())
					Expect(s.network.NextBlock()).To(BeNil())
					var success bool
					err = s.precompile.UnpackIntoInterface(&success, gov.CancelProposalMethod, evmRes.Ret)
					Expect(err).To(BeNil())
					Expect(success).To(BeTrue())

					afterCancellerBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)
					afterRandomAddrBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), randomAddr.Bytes(), baseDenom)
					amtFromContract := math.ZeroInt()
					for _, transferred := range []bool{tc.before, tc.after} {
						if transferred {
							amtFromContract = amtFromContract.AddRaw(15)
						}
					}

					Expect(afterCancellerBal.Amount).To(Equal(
						cancellerBal.Amount.
							Add(remaining).
							Add(math.NewIntFromBigInt(txArgs.Amount).
								Sub(amtFromContract)),
					))
					Expect(afterRandomAddrBal.Amount).To(Equal(
						randomAddrBal.Amount.
							Add(amtFromContract),
					))
				},
				Entry("with internal transfers before and after precompile call", testCase{
					before: true,
					after:  true,
				}),
				Entry("with internal transfers before precompile call", testCase{
					before: true,
					after:  false,
				}),
				Entry("with internal transfers after precompile call", testCase{
					before: false,
					after:  true,
				}),
			)
		})

		Context("testCancel with transfer (multiple deposits & refund)", func() {
			var cancelDest sdk.AccAddress

			BeforeEach(func() {
				// Submit a proposal with deposit from depositor0
				denom := s.network.GetBaseDenom()
				amount := "100"
				randomRecipient := sdk.AccAddress(testutiltx.GenerateAddress().Bytes())
				jsonBlob := minimalBankSendProposalJSON(randomRecipient, denom, amount)
				minDepositAmt := math.NewInt(100)
				callArgs.MethodName = testSubmitProposalFromContract
				callArgs.Args = []interface{}{
					jsonBlob,
					minimalDeposit(denom, minDepositAmt.BigInt()),
				}
				txArgs.Amount = minDepositAmt.BigInt()
				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
				txArgs.To = &contractAddr
				_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				err = s.precompile.UnpackIntoInterface(&proposalID, gov.SubmitProposalMethod, evmRes.Ret)
				Expect(err).To(BeNil())

				// Deposit from depositor1
				minDeposits := minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt())
				minDepositCoins, err := cmn.NewSdkCoinsFromCoins(minDeposits)
				Expect(err).To(BeNil())

				depositor1 = s.keyring.GetAccAddr(1)
				depositorKey1 = s.keyring.GetPrivKey(1)

				msg := &v1beta1.MsgDeposit{
					ProposalId: proposalID,
					Depositor:  depositor1.String(),
					Amount:     minDepositCoins,
				}
				var gas uint64 = 500_000
				res, err := s.factory.ExecuteCosmosTx(depositorKey1, commonfactory.CosmosTxArgs{
					Gas:  &gas,
					Msgs: []sdk.Msg{msg},
				})
				Expect(err).To(BeNil())
				Expect(res.Code).To(BeZero(), "expected no error code in response")

				// Calc cancellation fees for both deposits
				params, err := s.network.App.GetGovKeeper().Params.Get(s.network.GetContext())
				Expect(err).To(BeNil())
				rate := math.LegacyMustNewDecFromStr(params.ProposalCancelRatio)
				cancelDest = sdk.MustAccAddressFromBech32(params.ProposalCancelDest)
				proposalDeposits, err := s.network.App.GetGovKeeper().GetDeposits(s.network.GetContext(), proposalID)
				Expect(err).To(BeNil())
				Expect(proposalDeposits).To(HaveLen(2))

				cancelFees = make(map[string]math.Int)
				remainingFees = make(map[string]math.Int)

				for _, deposit := range proposalDeposits {
					for _, amount := range deposit.Amount {
						if amount.Denom == s.network.GetBaseDenom() {
							proposalDepositAmt := amount.Amount
							cancelFee = proposalDepositAmt.ToLegacyDec().Mul(rate).TruncateInt()
							cancelFees[deposit.Depositor] = cancelFee
							remaining = proposalDepositAmt.Sub(cancelFee)
							remainingFees[deposit.Depositor] = remaining
						}
					}
				}
				Expect(cancelFees).To(HaveLen(2))
				Expect(remainingFees).To(HaveLen(2))

				callArgs.MethodName = "testCancelWithTransfer"
			})

			DescribeTable("contract proposer should cancel proposal with transfer",
				func(tc testCase) {
					// Fix the gas limit and gas ice for predictable gas usage.
					// This is for calculating expected cancellation fee.
					baseFee := s.network.App.GetFeeMarketKeeper().GetBaseFee(s.network.GetContext())
					baseFeeInt := baseFee.TruncateInt64()
					txArgs.GasPrice = new(big.Int).SetInt64(baseFeeInt)
					txArgs.GasLimit = 500_000
					txArgs.Amount = big.NewInt(100)

					callArgs.Args = []interface{}{
						proposalID,
						tc.before, tc.after,
					}
					eventCheck := passCheck.WithExpEvents(gov.EventTypeCancelProposal)

					baseDenom := s.network.GetBaseDenom()
					contractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)
					depositor1Bal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), depositor1, baseDenom)
					txSenderBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), txSenderAddr.Bytes(), baseDenom)
					cancelDestBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), cancelDest, baseDenom)

					res, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
					Expect(err).To(BeNil())
					Expect(s.network.NextBlock()).To(BeNil())
					var success bool
					err = s.precompile.UnpackIntoInterface(&success, gov.CancelProposalMethod, evmRes.Ret)
					Expect(err).To(BeNil())
					Expect(success).To(BeTrue())
					gasCost := math.NewInt(res.GasUsed).Mul(math.NewInt(txArgs.GasPrice.Int64()))

					afterContractBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)
					afterDepositor1Bal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), depositor1, baseDenom)
					afterTxSenderBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), txSenderAddr.Bytes(), baseDenom)
					afterCancelDestBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), cancelDest, baseDenom)
					amtFromContract := math.ZeroInt()
					for _, transferred := range []bool{tc.before, tc.after} {
						if transferred {
							amtFromContract = amtFromContract.AddRaw(15)
						}
					}

					Expect(afterTxSenderBal.Amount).To(Equal(
						txSenderBal.Amount.
							Sub(gasCost).
							Sub(math.NewIntFromBigInt(txArgs.Amount)).
							Add(amtFromContract),
					))
					Expect(afterDepositor1Bal.Amount).To(Equal(
						depositor1Bal.Amount.
							Add(remainingFees[depositor1.String()]),
					))
					Expect(afterCancelDestBal.Amount).To(Equal(
						cancelDestBal.Amount.
							Add(cancelFees[depositor1.String()]).
							Add(cancelFees[contractAccAddr.String()]),
					))
					Expect(afterContractBal.Amount).To(Equal(
						contractBal.Amount.
							Add(remainingFees[contractAccAddr.String()]).
							Add(math.NewIntFromBigInt(txArgs.Amount).
								Sub(amtFromContract)),
					))
				},
				Entry("with internal transfers before and after precompile call", testCase{
					before: true,
					after:  true,
				}),
				Entry("with internal transfers before precompile call", testCase{
					before: true,
					after:  false,
				}),
				Entry("with internal transfers after precompile call", testCase{
					before: false,
					after:  true,
				}),
			)
		})

		Context("testCancelFromContract with transfer (multiple deposits & refund)", func() {
			BeforeEach(func() {
				// Submit a proposal with deposit from depositor0
				denom := s.network.GetBaseDenom()
				amount := "100"
				randomRecipient := sdk.AccAddress(testutiltx.GenerateAddress().Bytes())
				jsonBlob := minimalBankSendProposalJSON(randomRecipient, denom, amount)
				minDepositAmt := math.NewInt(100)
				callArgs.MethodName = testSubmitProposalFromContract
				callArgs.Args = []interface{}{
					jsonBlob,
					minimalDeposit(denom, minDepositAmt.BigInt()),
				}
				txArgs.Amount = minDepositAmt.BigInt()
				eventCheck := passCheck.WithExpEvents(gov.EventTypeSubmitProposal)
				txArgs.To = &contractAddr
				_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())

				err = s.precompile.UnpackIntoInterface(&proposalID, gov.SubmitProposalMethod, evmRes.Ret)
				Expect(err).To(BeNil())

				// Deposit from depositor1
				minDeposits := minimalDeposit(s.network.GetBaseDenom(), minDepositAmt.BigInt())
				minDepositCoins, err := cmn.NewSdkCoinsFromCoins(minDeposits)
				Expect(err).To(BeNil())

				depositor1 = s.keyring.GetAccAddr(1)
				depositorKey1 = s.keyring.GetPrivKey(1)

				msg := &v1beta1.MsgDeposit{
					ProposalId: proposalID,
					Depositor:  depositor1.String(),
					Amount:     minDepositCoins,
				}
				var gas uint64 = 500_000
				res, err := s.factory.ExecuteCosmosTx(depositorKey1, commonfactory.CosmosTxArgs{
					Gas:  &gas,
					Msgs: []sdk.Msg{msg},
				})
				Expect(err).To(BeNil())
				Expect(res.Code).To(BeZero(), "expected no error code in response")

				// Calc cancellation fees for both deposits
				params, err := s.network.App.GetGovKeeper().Params.Get(s.network.GetContext())
				Expect(err).To(BeNil())
				rate := math.LegacyMustNewDecFromStr(params.ProposalCancelRatio)
				proposalDeposits, err := s.network.App.GetGovKeeper().GetDeposits(s.network.GetContext(), proposalID)
				Expect(err).To(BeNil())
				Expect(proposalDeposits).To(HaveLen(2))

				cancelFees = make(map[string]math.Int)
				remainingFees = make(map[string]math.Int)

				for _, deposit := range proposalDeposits {
					for _, amount := range deposit.Amount {
						if amount.Denom == s.network.GetBaseDenom() {
							proposalDepositAmt := amount.Amount
							cancelFee = proposalDepositAmt.ToLegacyDec().Mul(rate).TruncateInt()
							cancelFees[deposit.Depositor] = cancelFee
							remaining = proposalDepositAmt.Sub(cancelFee)
							remainingFees[deposit.Depositor] = remaining
						}
					}
				}
				Expect(cancelFees).To(HaveLen(2))
				Expect(remainingFees).To(HaveLen(2))

				callArgs.MethodName = "testCancelFromContractWithTransfer"
			})

			DescribeTable("contract proposer should cancel proposal with transfer",
				func(tc testCase) {
					// Fix the gas limit and gas price for predictable gas usage.
					// This is for calculating expected cancellation fee.
					baseFee := s.network.App.GetFeeMarketKeeper().GetBaseFee(s.network.GetContext())
					baseFeeInt := baseFee.TruncateInt64()
					txArgs.GasPrice = new(big.Int).SetInt64(baseFeeInt)
					txArgs.GasLimit = 500_000
					txArgs.Amount = big.NewInt(100)
					randomAddr := testutiltx.GenerateAddress()
					callArgs.Args = []interface{}{
						randomAddr,
						contractProposalID,
						tc.before, tc.after,
					}
					eventCheck := passCheck.WithExpEvents(gov.EventTypeCancelProposal)

					baseDenom := s.network.GetBaseDenom()
					cancellerBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)
					depositor1Bal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), depositor1, baseDenom)
					randomAccBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), randomAddr.Bytes(), baseDenom)

					txSenderKey := s.keyring.GetPrivKey(0)
					_, evmRes, err := s.factory.CallContractAndCheckLogs(txSenderKey, txArgs, callArgs, eventCheck)
					Expect(err).To(BeNil())
					Expect(s.network.NextBlock()).To(BeNil())
					var success bool
					err = s.precompile.UnpackIntoInterface(&success, gov.CancelProposalMethod, evmRes.Ret)
					Expect(err).To(BeNil())
					Expect(success).To(BeTrue())

					afterCancellerBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), contractAccAddr, baseDenom)
					afterDepositor1Bal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), depositor1, baseDenom)
					afterRandomAccBal := s.network.App.GetBankKeeper().GetBalance(s.network.GetContext(), randomAddr.Bytes(), baseDenom)
					amtFromContract := math.ZeroInt()
					for _, transferred := range []bool{tc.before, tc.after} {
						if transferred {
							amtFromContract = amtFromContract.AddRaw(15)
						}
					}

					Expect(afterCancellerBal.Amount).To(Equal(
						cancellerBal.Amount.
							Add(remaining).
							Add(math.NewIntFromBigInt(txArgs.Amount).
								Sub(amtFromContract)),
					))
					Expect(afterRandomAccBal.Amount).To(Equal(
						randomAccBal.Amount.
							Add(amtFromContract),
					))
					Expect(afterDepositor1Bal.Amount).To(Equal(
						depositor1Bal.Amount.
							Add(remainingFees[depositor1.String()]),
					))
				},
				Entry("with internal transfers before and after precompile call", testCase{
					before: true,
					after:  true,
				}),
				Entry("with internal transfers before precompile call", testCase{
					before: true,
					after:  false,
				}),
				Entry("with internal transfers after precompile call", testCase{
					before: false,
					after:  true,
				}),
			)
		})
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "Keeper Suite")
}

// -----------------------------------------------------------------------------
// Helper functions (test‑only)
// -----------------------------------------------------------------------------

func minimalDeposit(denom string, amount *big.Int) []cmn.Coin {
	return []cmn.Coin{{Denom: denom, Amount: amount}}
}

// minimalBankSendProposalJSON returns a valid governance proposal encoded as UTF‑8 bytes.
func minimalBankSendProposalJSON(to sdk.AccAddress, denom, amount string) []byte {
	// proto‑JSON marshal via std JSON since test helpers don’t expose codec here.
	// We craft by hand for brevity.
	msgJSON, _ := json.Marshal(map[string]interface{}{
		"@type": "/cosmos.bank.v1beta1.MsgSend",
		// from_address must be gov module account
		"from_address": govModuleAddr.String(),
		"to_address":   to.String(),
		"amount":       []map[string]string{{"denom": denom, "amount": amount}},
	})

	prop := map[string]interface{}{
		"messages":  []json.RawMessage{msgJSON},
		"metadata":  "ipfs://CID",
		"title":     "test prop",
		"summary":   "test prop",
		"expedited": false,
	}
	blob, _ := json.Marshal(prop)
	return blob
}
