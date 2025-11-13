package staking

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"google.golang.org/grpc/codes"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	compiledcontracts "github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/staking"
	"github.com/cosmos/evm/precompiles/staking/testdata"
	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/precompiles/testutil/contracts"
	cosmosevmutil "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	testutiltx "github.com/cosmos/evm/testutil/tx"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	"github.com/cosmos/cosmos-sdk/types/query"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

var (
	// valAddr and valAddr2 are the two validator addresses used for testing
	valAddr, valAddr2 sdk.ValAddress

	// callArgs is the default arguments for calling the smart contract.
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
)

func TestPrecompileIntegrationTestSuite(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	_ = Describe("Calling staking precompile directly", func() {
		// s is the precompile test suite to use for the tests
		var s *PrecompileTestSuite

		BeforeEach(func() {
			var err error
			s = NewPrecompileTestSuite(create, options...)
			s.SetupTest()

			valAddr, err = sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
			Expect(err).To(BeNil())
			valAddr2, err = sdk.ValAddressFromBech32(s.network.GetValidators()[1].GetOperator())
			Expect(err).To(BeNil())

			callArgs = testutiltypes.CallArgs{
				ContractABI: s.precompile.ABI,
			}

			precompileAddr := s.precompile.Address()
			txArgs = evmtypes.EvmTxArgs{
				To: &precompileAddr,
			}

			defaultLogCheck = testutil.LogCheckArgs{ABIEvents: s.precompile.Events}
			passCheck = defaultLogCheck.WithExpPass(true)
			outOfGasCheck = defaultLogCheck.WithErrContains(vm.ErrOutOfGas.Error())
		})

		Describe("when the precompile is not enabled in the EVM params", func() {
			It("should succeed but not perform delegation", func() {
				delegator := s.keyring.GetKey(0)
				// disable the precompile
				res, err := s.grpcHandler.GetEvmParams()
				Expect(err).To(BeNil())

				var activePrecompiles []string
				for _, precompile := range res.Params.ActiveStaticPrecompiles {
					if precompile != s.precompile.Address().String() {
						activePrecompiles = append(activePrecompiles, precompile)
					}
				}
				res.Params.ActiveStaticPrecompiles = activePrecompiles

				err = utils.UpdateEvmParams(utils.UpdateParamsInput{
					Tf:      s.factory,
					Network: s.network,
					Pk:      delegator.Priv,
					Params:  res.Params,
				})
				Expect(err).To(BeNil(), "error while setting params")

				// get the delegation that is available prior to the test
				qRes, err := s.grpcHandler.GetDelegation(delegator.AccAddr.String(), valAddr.String())
				Expect(err).To(BeNil())
				prevDelegation := qRes.DelegationResponse.Balance
				// try to call the precompile
				callArgs.MethodName = staking.DelegateMethod
				callArgs.Args = []interface{}{delegator.Addr, valAddr.String(), big.NewInt(2e18)}

				// Contract should not be called but the transaction should be successful
				// This is the expected behavior in Ethereum where there is a contract call
				// to a non existing contract
				expectedCheck := defaultLogCheck.
					WithExpEvents([]string{}...).
					WithExpPass(true)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					expectedCheck,
				)
				Expect(err).To(BeNil(), "error while calling the contract and checking logs")
				qRes, err = s.grpcHandler.GetDelegation(delegator.AccAddr.String(), valAddr.String())
				Expect(err).To(BeNil())
				postDelegation := qRes.DelegationResponse.Balance
				Expect(postDelegation).To(Equal(prevDelegation), "expected delegation to not change")
			})
		})

		Describe("Revert transaction", func() {
			It("should run out of gas if the gas limit is too low", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.MethodName = staking.DelegateMethod
				callArgs.Args = []interface{}{
					delegator.Addr,
					valAddr.String(),
					big.NewInt(2e18),
				}
				txArgs.GasLimit = 30000

				_, _, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					outOfGasCheck,
				)
				Expect(err).To(BeNil(), "error while calling precompile")
			})
		})

		Describe("to create validator", func() {
			var (
				defaultDescription = staking.Description{
					Moniker:         "new node",
					Identity:        "",
					Website:         "",
					SecurityContact: "",
					Details:         "",
				}
				defaultCommission = staking.Commission{
					Rate:          big.NewInt(100000000000000000),
					MaxRate:       big.NewInt(100000000000000000),
					MaxChangeRate: big.NewInt(100000000000000000),
				}
				defaultMinSelfDelegation = big.NewInt(1)
				defaultPubkeyBase64Str   = GenerateBase64PubKey()
				defaultValue             = big.NewInt(1)
			)

			BeforeEach(func() {
				// populate the default createValidator args
				callArgs.MethodName = staking.CreateValidatorMethod
			})

			Context("when validator address is the msg.sender & EoA", func() {
				It("should succeed", func() {
					callArgs.Args = []interface{}{
						defaultDescription, defaultCommission, defaultMinSelfDelegation, s.keyring.GetAddr(0), defaultPubkeyBase64Str, defaultValue,
					}
					// NOTE: increase gas limit here
					txArgs.GasLimit = 2e5

					logCheckArgs := passCheck.WithExpEvents(staking.EventTypeCreateValidator)

					_, _, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the contract and checking logs")
					Expect(s.network.NextBlock()).To(BeNil())

					valOperAddr := sdk.ValAddress(s.keyring.GetAccAddr(0)).String()
					qc := s.network.GetStakingClient()
					res, err := qc.Validator(s.network.GetContext(), &stakingtypes.QueryValidatorRequest{ValidatorAddr: valOperAddr})
					Expect(err).To(BeNil())
					Expect(res).NotTo(BeNil())
					Expect(res.Validator.OperatorAddress).To(Equal(valOperAddr))
				})
			})

			Context("when validator address is not the msg.sender", func() {
				It("should fail", func() {
					differentAddr := testutiltx.GenerateAddress()

					callArgs.Args = []interface{}{
						defaultDescription, defaultCommission, defaultMinSelfDelegation, differentAddr, defaultPubkeyBase64Str, defaultValue,
					}

					logCheckArgs := defaultLogCheck.WithErrContains(
						fmt.Sprintf(cmn.ErrRequesterIsNotMsgSender, s.keyring.GetAddr(0), differentAddr),
					)

					_, _, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(0),
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the contract and checking logs")
				})
			})
		})

		Describe("to edit validator", func() {
			var (
				defaultDescription = staking.Description{
					Moniker:         "edit node",
					Identity:        "[do-not-modify]",
					Website:         "[do-not-modify]",
					SecurityContact: "[do-not-modify]",
					Details:         "[do-not-modify]",
				}
				defaultCommissionRate    = big.NewInt(staking.DoNotModifyCommissionRate)
				defaultMinSelfDelegation = big.NewInt(staking.DoNotModifyMinSelfDelegation)
			)

			BeforeEach(func() {
				// populate the default editValidator args
				callArgs.MethodName = staking.EditValidatorMethod
			})

			Context("when msg.sender is equal to validator address", func() {
				It("should succeed", func() {
					// create a new validator
					newAddr, newPriv := testutiltx.NewAccAddressAndKey()
					hexAddr := common.BytesToAddress(newAddr.Bytes())

					err := utils.FundAccountWithBaseDenom(s.factory, s.network, s.keyring.GetKey(0), newAddr, math.NewInt(2e18))
					Expect(err).To(BeNil(), "error while sending coins")
					Expect(s.network.NextBlock()).To(BeNil())

					description := staking.Description{
						Moniker:         "new node",
						Identity:        "",
						Website:         "",
						SecurityContact: "",
						Details:         "",
					}
					commission := staking.Commission{
						Rate:          big.NewInt(100000000000000000),
						MaxRate:       big.NewInt(100000000000000000),
						MaxChangeRate: big.NewInt(100000000000000000),
					}
					minSelfDelegation := big.NewInt(1)
					pubkeyBase64Str := "UuhHQmkUh2cPBA6Rg4ei0M2B04cVYGNn/F8SAUsYIb4="
					value := big.NewInt(1e18)

					createValidatorArgs := testutiltypes.CallArgs{
						ContractABI: s.precompile.ABI,
						MethodName:  staking.CreateValidatorMethod,
						Args:        []interface{}{description, commission, minSelfDelegation, hexAddr, pubkeyBase64Str, value},
					}

					logCheckArgs := passCheck.WithExpEvents(staking.EventTypeCreateValidator)
					_, _, err = s.factory.CallContractAndCheckLogs(
						newPriv,
						txArgs, createValidatorArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the contract and checking logs")
					Expect(s.network.NextBlock()).To(BeNil())

					// edit validator
					callArgs.Args = []interface{}{defaultDescription, hexAddr, defaultCommissionRate, defaultMinSelfDelegation}

					logCheckArgs = passCheck.WithExpEvents(staking.EventTypeEditValidator)
					_, _, err = s.factory.CallContractAndCheckLogs(
						newPriv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the contract and checking logs")
					Expect(s.network.NextBlock()).To(BeNil())

					valOperAddr := sdk.ValAddress(newAddr.Bytes()).String()
					qc := s.network.GetStakingClient()
					res, err := qc.Validator(s.network.GetContext(), &stakingtypes.QueryValidatorRequest{ValidatorAddr: valOperAddr})
					Expect(err).To(BeNil())
					Expect(res).NotTo(BeNil())
					validator := res.Validator
					Expect(validator.OperatorAddress).To(Equal(valOperAddr))
					Expect(validator.Description.Moniker).To(Equal(defaultDescription.Moniker), "expected validator moniker is updated")
					// Other fields should not be modified due to the value "[do-not-modify]".
					Expect(validator.Description.Identity).To(Equal(description.Identity), "expected validator identity not to be updated")
					Expect(validator.Description.Website).To(Equal(description.Website), "expected validator website not to be updated")
					Expect(validator.Description.SecurityContact).To(Equal(description.SecurityContact), "expected validator security contact not to be updated")
					Expect(validator.Description.Details).To(Equal(description.Details), "expected validator details not to be updated")

					Expect(validator.Commission.Rate.BigInt().String()).To(Equal(commission.Rate.String()), "expected validator commission rate remain unchanged")
					Expect(validator.Commission.MaxRate.BigInt().String()).To(Equal(commission.MaxRate.String()), "expected validator max commission rate remain unchanged")
					Expect(validator.Commission.MaxChangeRate.BigInt().String()).To(Equal(commission.MaxChangeRate.String()), "expected validator max change rate remain unchanged")
					Expect(validator.MinSelfDelegation.String()).To(Equal(minSelfDelegation.String()), "expected validator min self delegation remain unchanged")
				})
			})

			Context("with msg.sender different than validator address", func() {
				It("should fail", func() {
					valHexAddr := common.BytesToAddress(valAddr.Bytes())
					callArgs.Args = []interface{}{
						defaultDescription, valHexAddr, defaultCommissionRate, defaultMinSelfDelegation,
					}

					logCheckArgs := passCheck.WithExpEvents(staking.EventTypeEditValidator)
					_, _, err := s.factory.CallContractAndCheckLogs(
						s.keyring.GetPrivKey(1),
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).NotTo(BeNil(), "error while calling the contract and checking logs")
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("msg.sender address %s does not match the requester address %s", s.keyring.GetAddr(1), valHexAddr)))
				})
			})
		})
		Describe("to delegate", func() {
			// prevDelegation is the delegation that is available prior to the test (an initial delegation is
			// added in the test suite setup).
			var prevDelegation stakingtypes.Delegation

			BeforeEach(func() {
				delegator := s.keyring.GetKey(0)

				// get the delegation that is available prior to the test
				res, err := s.grpcHandler.GetDelegation(delegator.AccAddr.String(), valAddr.String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponse).NotTo(BeNil())

				prevDelegation = res.DelegationResponse.Delegation
				// populate the default delegate args
				callArgs.MethodName = staking.DelegateMethod
			})

			Context("as the token owner", func() {
				It("should delegate", func() {
					delegator := s.keyring.GetKey(0)

					callArgs.Args = []interface{}{
						delegator.Addr, valAddr.String(), big.NewInt(2e18),
					}

					logCheckArgs := passCheck.WithExpEvents(staking.EventTypeDelegate)

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
					Expect(s.network.NextBlock()).To(BeNil())

					res, err := s.grpcHandler.GetDelegation(delegator.AccAddr.String(), valAddr.String())
					Expect(err).To(BeNil())
					Expect(res.DelegationResponse).NotTo(BeNil())
					expShares := prevDelegation.GetShares().Add(math.LegacyNewDec(2))
					Expect(res.DelegationResponse.Delegation.GetShares()).To(Equal(expShares), "expected different delegation shares")
				})

				It("should not delegate if the account has no sufficient balance", func() {
					newAddr, newAddrPriv := testutiltx.NewAccAddressAndKey()
					err := utils.FundAccountWithBaseDenom(s.factory, s.network, s.keyring.GetKey(0), newAddr, math.NewInt(1e17))
					Expect(err).To(BeNil(), "error while sending coins")
					Expect(s.network.NextBlock()).To(BeNil())

					// try to delegate more than left in account
					callArgs.Args = []interface{}{
						common.BytesToAddress(newAddr), valAddr.String(), big.NewInt(1e18),
					}

					logCheckArgs := defaultLogCheck.WithErrContains("insufficient funds")

					_, _, err = s.factory.CallContractAndCheckLogs(
						newAddrPriv,
						txArgs,
						callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				})

				It("should not delegate if the validator does not exist", func() {
					nonExistingAddr := testutiltx.GenerateAddress()
					nonExistingValAddr := sdk.ValAddress(nonExistingAddr.Bytes())
					delegator := s.keyring.GetKey(0)

					callArgs.Args = []interface{}{
						delegator.Addr, nonExistingValAddr.String(), big.NewInt(2e18),
					}

					logCheckArgs := defaultLogCheck.WithErrContains("validator does not exist")

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs,
						callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				})
			})

			Context("on behalf of another account", func() {
				It("should not delegate if delegator address is not the msg.sender", func() {
					delegator := s.keyring.GetKey(0)
					differentAddr := testutiltx.GenerateAddress()

					callArgs.Args = []interface{}{
						differentAddr, valAddr.String(), big.NewInt(2e18),
					}

					logCheckArgs := defaultLogCheck.WithErrContains(
						fmt.Sprintf(cmn.ErrRequesterIsNotMsgSender, delegator.Addr, differentAddr),
					)

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs,
						callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				})
			})
		})

		Describe("to undelegate", func() {
			BeforeEach(func() {
				callArgs.MethodName = staking.UndelegateMethod
			})

			Context("as the token owner", func() {
				It("should undelegate", func() {
					delegator := s.keyring.GetKey(0)

					valAddr, err := sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
					Expect(err).To(BeNil())

					res, err := s.grpcHandler.GetValidatorUnbondingDelegations(valAddr.String())
					Expect(err).To(BeNil())
					Expect(res.UnbondingResponses).To(HaveLen(0), "expected no unbonding delegations before test")

					callArgs.Args = []interface{}{
						delegator.Addr, valAddr.String(), big.NewInt(1e18),
					}

					logCheckArgs := passCheck.WithExpEvents(staking.EventTypeUnbond)

					_, _, err = s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

					delUbdRes, err := s.grpcHandler.GetDelegatorUnbondingDelegations(delegator.AccAddr.String())
					Expect(err).To(BeNil())
					Expect(delUbdRes.UnbondingResponses).To(HaveLen(1), "expected one undelegation")
					Expect(delUbdRes.UnbondingResponses[0].ValidatorAddress).To(Equal(valAddr.String()), "expected validator address to be %s", valAddr)
				})

				It("should not undelegate if the amount exceeds the delegation", func() {
					delegator := s.keyring.GetKey(0)

					callArgs.Args = []interface{}{
						delegator.Addr, valAddr.String(), big.NewInt(2e18),
					}

					logCheckArgs := defaultLogCheck.WithErrContains("invalid shares amount")

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				})

				It("should not undelegate if the validator does not exist", func() {
					delegator := s.keyring.GetKey(0)
					nonExistingAddr := testutiltx.GenerateAddress()
					nonExistingValAddr := sdk.ValAddress(nonExistingAddr.Bytes())

					callArgs.Args = []interface{}{
						delegator.Addr, nonExistingValAddr.String(), big.NewInt(1e18),
					}

					logCheckArgs := defaultLogCheck.WithErrContains("validator does not exist")

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				})
			})

			Context("on behalf of another account", func() {
				It("should not undelegate if delegator address is not the msg.sender", func() {
					differentAddr := testutiltx.GenerateAddress()
					delegator := s.keyring.GetKey(0)

					callArgs.Args = []interface{}{
						differentAddr, valAddr.String(), big.NewInt(1e18),
					}

					logCheckArgs := defaultLogCheck.WithErrContains(
						fmt.Sprintf(cmn.ErrRequesterIsNotMsgSender, delegator.Addr, differentAddr),
					)

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				})
			})
		})

		Describe("to redelegate", func() {
			BeforeEach(func() {
				callArgs.MethodName = staking.RedelegateMethod
			})

			Context("as the token owner", func() {
				It("should redelegate", func() {
					delegator := s.keyring.GetKey(0)

					callArgs.Args = []interface{}{
						delegator.Addr, valAddr.String(), valAddr2.String(), big.NewInt(1e18),
					}

					logCheckArgs := passCheck.
						WithExpEvents(staking.EventTypeRedelegate)

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
					Expect(s.network.NextBlock()).To(BeNil())

					res, err := s.grpcHandler.GetRedelegations(delegator.AccAddr.String(), valAddr.String(), valAddr2.String())
					Expect(err).To(BeNil())
					Expect(res.RedelegationResponses).To(HaveLen(1), "expected one redelegation to be found")
					bech32Addr := delegator.AccAddr
					Expect(res.RedelegationResponses[0].Redelegation.DelegatorAddress).To(Equal(bech32Addr.String()), "expected delegator address to be %s", delegator.Addr)
					Expect(res.RedelegationResponses[0].Redelegation.ValidatorSrcAddress).To(Equal(valAddr.String()), "expected source validator address to be %s", valAddr)
					Expect(res.RedelegationResponses[0].Redelegation.ValidatorDstAddress).To(Equal(valAddr2.String()), "expected destination validator address to be %s", valAddr2)
				})

				It("should not redelegate if the amount exceeds the delegation", func() {
					delegator := s.keyring.GetKey(0)

					callArgs.Args = []interface{}{
						delegator.Addr, valAddr.String(), valAddr2.String(), big.NewInt(2e18),
					}

					logCheckArgs := defaultLogCheck.WithErrContains("invalid shares amount")

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				})

				It("should not redelegate if the validator does not exist", func() {
					nonExistingAddr := testutiltx.GenerateAddress()
					nonExistingValAddr := sdk.ValAddress(nonExistingAddr.Bytes())
					delegator := s.keyring.GetKey(0)

					callArgs.Args = []interface{}{
						delegator.Addr, valAddr.String(), nonExistingValAddr.String(), big.NewInt(1e18),
					}

					logCheckArgs := defaultLogCheck.WithErrContains("redelegation destination validator not found")

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				})
			})

			Context("on behalf of another account", func() {
				It("should not redelegate if delegator address is not the msg.sender", func() {
					differentAddr := testutiltx.GenerateAddress()
					delegator := s.keyring.GetKey(0)

					callArgs.Args = []interface{}{
						differentAddr, valAddr.String(), valAddr2.String(), big.NewInt(1e18),
					}

					logCheckArgs := defaultLogCheck.WithErrContains(
						fmt.Sprintf(cmn.ErrRequesterIsNotMsgSender, delegator.Addr, differentAddr),
					)

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				})
			})
		})

		Describe("to cancel an unbonding delegation", func() {
			BeforeEach(func() {
				callArgs.MethodName = staking.CancelUnbondingDelegationMethod
				delegator := s.keyring.GetKey(0)

				// Set up an unbonding delegation
				undelegateArgs := testutiltypes.CallArgs{
					ContractABI: s.precompile.ABI,
					MethodName:  staking.UndelegateMethod,
					Args: []interface{}{
						delegator.Addr, valAddr.String(), big.NewInt(1e18),
					},
				}

				logCheckArgs := passCheck.
					WithExpEvents(staking.EventTypeUnbond)

				_, _, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					undelegateArgs,
					logCheckArgs,
				)
				Expect(err).To(BeNil(), "error while setting up an unbonding delegation: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				creationHeight := s.network.GetContext().BlockHeight()

				// Check that the unbonding delegation was created
				res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(delegator.AccAddr.String())
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(HaveLen(1), "expected one unbonding delegation to be found")
				Expect(res.UnbondingResponses[0].DelegatorAddress).To(Equal(delegator.AccAddr.String()), "expected delegator address to be %s", delegator.Addr)
				Expect(res.UnbondingResponses[0].ValidatorAddress).To(Equal(valAddr.String()), "expected validator address to be %s", valAddr)
				Expect(res.UnbondingResponses[0].Entries).To(HaveLen(1), "expected one unbonding delegation entry to be found")
				Expect(res.UnbondingResponses[0].Entries[0].CreationHeight).To(Equal(creationHeight), "expected different creation height")
				Expect(res.UnbondingResponses[0].Entries[0].Balance).To(Equal(math.NewInt(1e18)), "expected different balance")
			})

			Context("as the token owner", func() {
				It("should cancel unbonding delegation", func() {
					delegator := s.keyring.GetKey(0)

					valDelRes, err := s.grpcHandler.GetValidatorDelegations(s.network.GetValidators()[0].GetOperator())
					Expect(err).To(BeNil())
					Expect(valDelRes.DelegationResponses).To(HaveLen(0))

					creationHeight := s.network.GetContext().BlockHeight()
					callArgs.Args = []interface{}{
						delegator.Addr, valAddr.String(), big.NewInt(1e18), big.NewInt(creationHeight),
					}

					logCheckArgs := passCheck.
						WithExpEvents(staking.EventTypeCancelUnbondingDelegation)

					_, _, err = s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs,
						callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
					Expect(s.network.NextBlock()).To(BeNil())

					res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(delegator.AccAddr.String())
					Expect(err).To(BeNil())
					Expect(res.UnbondingResponses).To(HaveLen(0), "expected unbonding delegation to be canceled")

					valDelRes, err = s.grpcHandler.GetValidatorDelegations(s.network.GetValidators()[0].GetOperator())
					Expect(err).To(BeNil())
					Expect(valDelRes.DelegationResponses).To(HaveLen(1), "expected one delegation to be found")
				})

				It("should not cancel an unbonding delegation if the amount is not correct", func() {
					delegator := s.keyring.GetKey(0)

					creationHeight := s.network.GetContext().BlockHeight()
					callArgs.Args = []interface{}{
						delegator.Addr, valAddr.String(), big.NewInt(2e18), big.NewInt(creationHeight),
					}

					logCheckArgs := defaultLogCheck.WithErrContains("amount is greater than the unbonding delegation entry balance")

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
					Expect(s.network.NextBlock()).To(BeNil())

					res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(delegator.AccAddr.String())
					Expect(err).To(BeNil())
					Expect(res.UnbondingResponses).To(HaveLen(1), "expected unbonding delegation not to have been canceled")
				})

				It("should not cancel an unbonding delegation if the creation height is not correct", func() {
					delegator := s.keyring.GetKey(0)

					creationHeight := s.network.GetContext().BlockHeight()
					callArgs.Args = []interface{}{
						delegator.Addr, valAddr.String(), big.NewInt(1e18), big.NewInt(creationHeight + 1),
					}

					logCheckArgs := defaultLogCheck.WithErrContains("unbonding delegation entry is not found at block height")

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
					Expect(s.network.NextBlock()).To(BeNil())

					res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(delegator.AccAddr.String())
					Expect(err).To(BeNil())
					Expect(res.UnbondingResponses).To(HaveLen(1), "expected unbonding delegation not to have been canceled")
				})
			})
		})

		Describe("Validator queries", func() {
			BeforeEach(func() {
				callArgs.MethodName = staking.ValidatorMethod
			})

			It("should return validator", func() {
				delegator := s.keyring.GetKey(0)

				varHexAddr := common.BytesToAddress(valAddr.Bytes())
				callArgs.Args = []interface{}{varHexAddr}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)
				Expect(valOut.Validator.OperatorAddress).To(Equal(varHexAddr.String()), "expected validator address to match")
				Expect(valOut.Validator.DelegatorShares).To(Equal(big.NewInt(1e18)), "expected different delegator shares")
			})

			It("should return an empty validator if the validator is not found", func() {
				delegator := s.keyring.GetKey(0)

				newValHexAddr := testutiltx.GenerateAddress()
				callArgs.Args = []interface{}{newValHexAddr}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)
				Expect(valOut.Validator.OperatorAddress).To(Equal(""), "expected validator address to be empty")
				Expect(valOut.Validator.Status).To(BeZero(), "expected unspecified bonding status")
			})
		})

		Describe("Validators queries", func() {
			BeforeEach(func() {
				callArgs.MethodName = staking.ValidatorsMethod
			})

			It("should return validators (default pagination)", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					stakingtypes.Bonded.String(),
					query.PageRequest{},
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorsOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorsMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)

				Expect(valOut.PageResponse.NextKey).To(BeEmpty())
				Expect(valOut.PageResponse.Total).To(Equal(uint64(len(s.network.GetValidators()))))

				Expect(valOut.Validators).To(HaveLen(len(s.network.GetValidators())), "expected two validators to be returned")
				// return order can change, that's why each validator is checked individually
				for _, val := range valOut.Validators {
					s.CheckValidatorOutput(val)
				}
			})

			//nolint:dupl // this is a duplicate of the test for smart contract calls to the precompile
			It("should return validators w/pagination limit = 1", func() {
				const limit uint64 = 1
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					stakingtypes.Bonded.String(),
					query.PageRequest{
						Limit:      limit,
						CountTotal: true,
					},
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorsOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorsMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)

				// no pagination, should return default values
				Expect(valOut.PageResponse.NextKey).NotTo(BeEmpty())
				Expect(valOut.PageResponse.Total).To(Equal(uint64(len(s.network.GetValidators()))))

				Expect(valOut.Validators).To(HaveLen(int(limit)), "expected one validator to be returned")

				// return order can change, that's why each validator is checked individually
				for _, val := range valOut.Validators {
					s.CheckValidatorOutput(val)
				}
			})

			It("should return an error if the bonding type is not known", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					"15", // invalid bonding type
					query.PageRequest{},
				}

				invalidStatusCheck := defaultLogCheck.WithErrContains("invalid validator status 15")

				_, _, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					invalidStatusCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
			})

			It("should return an empty array if there are no validators with the given bonding type", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					stakingtypes.Unbonded.String(),
					query.PageRequest{},
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorsOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorsMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)

				Expect(valOut.PageResponse.NextKey).To(BeEmpty())
				Expect(valOut.PageResponse.Total).To(Equal(uint64(0)))
				Expect(valOut.Validators).To(HaveLen(0), "expected no validators to be returned")
			})
		})

		Describe("Delegation queries", func() {
			BeforeEach(func() {
				callArgs.MethodName = staking.DelegationMethod
			})

			It("should return a delegation if it is found", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					delegator.Addr,
					valAddr.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var delOut staking.DelegationOutput
				err = s.precompile.UnpackIntoInterface(&delOut, staking.DelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the delegation output: %v", err)
				Expect(delOut.Shares).To(Equal(big.NewInt(1e18)), "expected different shares")
				Expect(delOut.Balance).To(Equal(cmn.Coin{Denom: s.bondDenom, Amount: big.NewInt(1e18)}), "expected different shares")
			})

			It("should return an empty delegation if it is not found", func() {
				delegator := s.keyring.GetKey(0)

				newValAddr := sdk.ValAddress(testutiltx.GenerateAddress().Bytes())
				callArgs.Args = []interface{}{
					delegator.Addr,
					newValAddr.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var delOut staking.DelegationOutput
				err = s.precompile.UnpackIntoInterface(&delOut, staking.DelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the delegation output: %v", err)
				Expect(delOut.Shares.Int64()).To(BeZero(), "expected no shares")
				Expect(delOut.Balance.Denom).To(Equal(s.bondDenom), "expected different denomination")
				Expect(delOut.Balance.Amount.Int64()).To(BeZero(), "expected a zero amount")
			})
		})

		Describe("UnbondingDelegation queries", func() {
			// undelAmount is the amount of tokens to be unbonded
			undelAmount := big.NewInt(1e17)

			BeforeEach(func() {
				callArgs.MethodName = staking.UnbondingDelegationMethod

				delegator := s.keyring.GetKey(0)

				undelegateArgs := testutiltypes.CallArgs{
					ContractABI: s.precompile.ABI,
					MethodName:  staking.UndelegateMethod,
					Args: []interface{}{
						delegator.Addr, valAddr.String(), undelAmount,
					},
				}

				unbondCheck := passCheck.WithExpEvents(staking.EventTypeUnbond)
				_, _, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, undelegateArgs,
					unbondCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				// check that the unbonding delegation exists
				res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(delegator.AccAddr.String())
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(HaveLen(1), "expected one unbonding delegation")
			})

			It("should return an unbonding delegation if it is found", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					delegator.Addr,
					valAddr.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var unbondingDelegationOutput staking.UnbondingDelegationOutput
				err = s.precompile.UnpackIntoInterface(&unbondingDelegationOutput, staking.UnbondingDelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the unbonding delegation output: %v", err)
				Expect(unbondingDelegationOutput.UnbondingDelegation.Entries).To(HaveLen(1), "expected one unbonding delegation entry")
				// TODO: why are initial balance and balance the same always?
				Expect(unbondingDelegationOutput.UnbondingDelegation.Entries[0].InitialBalance).To(Equal(undelAmount), "expected different initial balance")
				Expect(unbondingDelegationOutput.UnbondingDelegation.Entries[0].Balance).To(Equal(undelAmount), "expected different balance")
			})

			It("should return an empty slice if the unbonding delegation is not found", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					delegator.Addr,
					valAddr2.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var unbondingDelegationOutput staking.UnbondingDelegationOutput
				err = s.precompile.UnpackIntoInterface(&unbondingDelegationOutput, staking.UnbondingDelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the unbonding delegation output: %v", err)
				Expect(unbondingDelegationOutput.UnbondingDelegation.Entries).To(HaveLen(0), "expected one unbonding delegation entry")
			})
		})

		Describe("to query a redelegation", func() {
			BeforeEach(func() {
				callArgs.MethodName = staking.RedelegationMethod
			})

			It("should return the redelegation if it exists", func() {
				delegator := s.keyring.GetKey(0)

				// create a redelegation
				redelegateArgs := testutiltypes.CallArgs{
					ContractABI: s.precompile.ABI,
					MethodName:  staking.RedelegateMethod,
					Args: []interface{}{
						delegator.Addr, valAddr.String(), valAddr2.String(), big.NewInt(1e17),
					},
				}

				redelegateCheck := passCheck.WithExpEvents(staking.EventTypeRedelegate)

				_, _, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, redelegateArgs,
					redelegateCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				// query the redelegation
				callArgs.Args = []interface{}{
					delegator.Addr,
					valAddr.String(),
					valAddr2.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var redelegationOutput staking.RedelegationOutput
				err = s.precompile.UnpackIntoInterface(&redelegationOutput, staking.RedelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the redelegation output: %v", err)
				Expect(redelegationOutput.Redelegation.Entries).To(HaveLen(1), "expected one redelegation entry")
				Expect(redelegationOutput.Redelegation.Entries[0].InitialBalance).To(Equal(big.NewInt(1e17)), "expected different initial balance")
				Expect(redelegationOutput.Redelegation.Entries[0].SharesDst).To(Equal(big.NewInt(1e17)), "expected different balance")
			})

			It("should return an empty output if the redelegation is not found", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					delegator.Addr,
					valAddr.String(),
					valAddr2.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var redelegationOutput staking.RedelegationOutput
				err = s.precompile.UnpackIntoInterface(&redelegationOutput, staking.RedelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the redelegation output: %v", err)
				Expect(redelegationOutput.Redelegation.Entries).To(HaveLen(0), "expected no redelegation entries")
			})
		})

		Describe("Redelegations queries", func() {
			var (
				// delAmt is the amount of tokens to be delegated
				delAmt = big.NewInt(3e17)
				// redelTotalCount is the total number of redelegations
				redelTotalCount uint64 = 1
			)

			BeforeEach(func() {
				delegator := s.keyring.GetKey(0)

				callArgs.MethodName = staking.RedelegationsMethod
				// create some redelegations
				redelegationsArgs := []testutiltypes.CallArgs{
					{
						ContractABI: s.precompile.ABI,
						MethodName:  staking.RedelegateMethod,
						Args: []interface{}{
							delegator.Addr, valAddr.String(), valAddr2.String(), delAmt,
						},
					},
					{
						ContractABI: s.precompile.ABI,
						MethodName:  staking.RedelegateMethod,
						Args: []interface{}{
							delegator.Addr, valAddr.String(), valAddr2.String(), delAmt,
						},
					},
				}

				logCheckArgs := passCheck.
					WithExpEvents(staking.EventTypeRedelegate)

				txArgs.GasLimit = 500_000
				for _, args := range redelegationsArgs {
					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, args,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while creating redelegation: %v", err)
					Expect(s.network.NextBlock()).To(BeNil())
				}
			})

			It("should return all redelegations for delegator (default pagination)", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					delegator.Addr,
					"",
					"",
					query.PageRequest{},
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var redelOut staking.RedelegationsOutput
				err = s.precompile.UnpackIntoInterface(&redelOut, staking.RedelegationsMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)

				Expect(redelOut.PageResponse.NextKey).To(BeEmpty())
				Expect(redelOut.PageResponse.Total).To(Equal(redelTotalCount))

				Expect(redelOut.Response).To(HaveLen(int(redelTotalCount)), "expected two redelegations to be returned") //#nosec G115 -- int overflow is not a concern here
				// return order can change
				redOrder := []int{0, 1}
				if len(redelOut.Response[0].Entries) == 2 {
					redOrder = []int{1, 0}
				}

				for i, r := range redelOut.Response {
					Expect(r.Entries).To(HaveLen(redOrder[i] + 1))
				}
			})

			It("should return all redelegations for delegator w/pagination", func() {
				delegator := s.keyring.GetKey(0)

				// make 2 queries
				// 1st one with pagination limit = 1
				// 2nd using the next page key
				var nextPageKey []byte
				for i := 0; i < 2; i++ {
					var pagination query.PageRequest
					if nextPageKey == nil {
						pagination.Limit = 1
						pagination.CountTotal = true
					} else {
						pagination.Key = nextPageKey
					}
					callArgs.Args = []interface{}{
						delegator.Addr,
						"",
						"",
						pagination,
					}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs,
						callArgs,
						passCheck,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
					Expect(s.network.NextBlock()).To(BeNil())

					var redelOut staking.RedelegationsOutput
					err = s.precompile.UnpackIntoInterface(&redelOut, staking.RedelegationsMethod, ethRes.Ret)
					Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)

					if nextPageKey == nil {
						nextPageKey = redelOut.PageResponse.NextKey
						Expect(redelOut.PageResponse.Total).To(Equal(redelTotalCount))
					} else {
						Expect(redelOut.PageResponse.NextKey).To(BeEmpty())
						Expect(redelOut.PageResponse.Total).To(Equal(uint64(1)))
					}

					Expect(redelOut.Response).To(HaveLen(1), "expected two redelegations to be returned")
					// return order can change
					redOrder := []int{0, 1}
					if len(redelOut.Response[0].Entries) == 2 {
						redOrder = []int{1, 0}
					}

					for i, r := range redelOut.Response {
						Expect(r.Entries).To(HaveLen(redOrder[i] + 1))
					}
				}
			})

			It("should return an empty array if no redelegation is found for the given source validator", func() {
				// NOTE: the way that the functionality is implemented in the Cosmos SDK, the following combinations are
				// possible (see https://github.com/evmos/cosmos-sdk/blob/e773cf768844c87245d0c737cda1893a2819dd89/x/staking/keeper/querier.go#L361-L373):
				//
				// - delegator is NOT empty, source validator is empty, destination validator is empty
				//   --> filtering for all redelegations of the given delegator
				// - delegator is empty, source validator is NOT empty, destination validator is empty
				//   --> filtering for all redelegations with the given source validator
				// - delegator is NOT empty, source validator is NOT empty, destination validator is NOT empty
				//   --> filtering for all redelegations with the given combination of delegator, source and destination validator
				callArgs.Args = []interface{}{
					common.Address{}, // passing in an empty address to filter for all redelegations from valAddr2
					valAddr2.String(),
					"",
					query.PageRequest{},
				}

				sender := s.keyring.GetKey(0)
				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					sender.Priv,
					txArgs,
					callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "expected error while calling the smart contract")

				var redelOut staking.RedelegationsOutput
				err = s.precompile.UnpackIntoInterface(&redelOut, staking.RedelegationsMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)

				Expect(redelOut.PageResponse.NextKey).To(BeEmpty())
				Expect(redelOut.PageResponse.Total).To(BeZero(), "expected no redelegations to be returned")

				Expect(redelOut.Response).To(HaveLen(0), "expected no redelegations to be returned")
			})
		})

		It("Should refund leftover gas", func() {
			delegator := s.keyring.GetKey(0)

			resBal, err := s.grpcHandler.GetBalanceFromBank(delegator.AccAddr, s.bondDenom)
			Expect(err).To(BeNil(), "error while getting balance")
			balancePre := resBal.Balance
			gasPrice := big.NewInt(1e9)
			delAmt := big.NewInt(1e18)

			// Call the precompile with a lot of gas
			callArgs.MethodName = staking.DelegateMethod
			callArgs.Args = []interface{}{
				delegator.Addr,
				valAddr.String(),
				delAmt,
			}

			txArgs.GasPrice = gasPrice

			logCheckArgs := passCheck.
				WithExpEvents(staking.EventTypeDelegate)

			res, _, err := s.factory.CallContractAndCheckLogs(
				delegator.Priv,
				txArgs, callArgs,
				logCheckArgs,
			)
			Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
			Expect(s.network.NextBlock()).To(BeNil())

			resBal, err = s.grpcHandler.GetBalanceFromBank(delegator.AccAddr, s.bondDenom)
			Expect(err).To(BeNil(), "error while getting balance")
			balancePost := resBal.Balance
			difference := balancePre.Sub(*balancePost)

			// NOTE: the expected difference is the delegate amount plus the gas price multiplied by the gas used, because the rest should be refunded
			expDifference := delAmt.Int64() + gasPrice.Int64()*res.GasUsed
			Expect(difference.Amount.Int64()).To(Equal(expDifference), "expected different total transaction cost")
		})
	})
	_ = Describe("Calling staking precompile via Solidity", Ordered, func() {
		// We cannot check staking precompile returns appropriate error in precompile call via caller contract.
		// It is because, caller contract call precompile with its own address for delegatorAddr
		// So, many expected error is filtered by `require` statement of caller contract that returns err message below.
		const (
			CallerErrDelegationNotExist          = "Delegation does not exist or insufficient delegation amount"
			CallerErrUnbondingDelegationNotExist = "Unbonding delegation does not exist"
		)

		var (
			// s is the precompile test suite to use for the tests
			s *PrecompileTestSuite
			// contractAddr is the address of the smart contract that will be deployed
			contractAddr    common.Address
			contractTwoAddr common.Address
			stkReverterAddr common.Address

			// stakingCallerContract is the contract instance calling into the staking precompile
			stakingCallerContract    evmtypes.CompiledContract
			stakingCallerTwoContract evmtypes.CompiledContract
			stakingReverterContract  evmtypes.CompiledContract

			// execRevertedCheck defines the default log checking arguments which include the
			// standard revert message
			execRevertedCheck testutil.LogCheckArgs
			// err is a basic error type
			err error

			// nonExistingAddr is an address that does not exist in the state of the test suite
			nonExistingAddr = testutiltx.GenerateAddress()
			// nonExistingVal is a validator address that does not exist in the state of the test suite
			nonExistingVal             = sdk.ValAddress(nonExistingAddr.Bytes())
			testContractInitialBalance = math.NewInt(1e18)
		)

		BeforeAll(func() {
			stakingCallerContract, err = testdata.LoadStakingCallerContract()
			Expect(err).To(BeNil())
			stakingCallerTwoContract, err = testdata.LoadStakingCallerTwoContract()
			Expect(err).To(BeNil(), "error while loading the StakingCallerTwo contract")
			stakingReverterContract, err = contracts.LoadStakingReverterContract()
			Expect(err).To(BeNil(), "error while loading the StakingReverter contract")
		})

		BeforeEach(func() {
			s = NewPrecompileTestSuite(create, options...)
			s.SetupTest()
			delegator := s.keyring.GetKey(0)

			contractAddr, err = s.factory.DeployContract(
				delegator.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: stakingCallerContract,
				},
			)
			Expect(err).To(BeNil(), "error while deploying the smart contract: %v", err)
			valAddr, err = sdk.ValAddressFromBech32(s.network.GetValidators()[0].GetOperator())
			Expect(err).To(BeNil())
			valAddr2, err = sdk.ValAddressFromBech32(s.network.GetValidators()[1].GetOperator())
			Expect(err).To(BeNil())

			Expect(s.network.NextBlock()).To(BeNil())

			// Deploy StakingCallerTwo contract
			contractTwoAddr, err = s.factory.DeployContract(
				delegator.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: stakingCallerTwoContract,
				},
			)
			Expect(err).To(BeNil(), "error while deploying the StakingCallerTwo contract")
			Expect(s.network.NextBlock()).To(BeNil())

			// Deploy StakingReverter contract
			stkReverterAddr, err = s.factory.DeployContract(
				delegator.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: stakingReverterContract,
				},
			)
			Expect(err).To(BeNil(), "error while deploying the StakingReverter contract")
			Expect(s.network.NextBlock()).To(BeNil())

			// send some funds to the StakingCallerTwo & StakingReverter contracts to transfer to the
			// delegator during the tx
			err := utils.FundAccountWithBaseDenom(s.factory, s.network, s.keyring.GetKey(0), contractTwoAddr.Bytes(), testContractInitialBalance)
			Expect(err).To(BeNil(), "error while funding the smart contract: %v", err)
			Expect(s.network.NextBlock()).To(BeNil())
			err = utils.FundAccountWithBaseDenom(s.factory, s.network, s.keyring.GetKey(0), stkReverterAddr.Bytes(), testContractInitialBalance)
			Expect(err).To(BeNil(), "error while funding the smart contract: %v", err)
			Expect(s.network.NextBlock()).To(BeNil())

			// check contract was correctly deployed
			cAcc := s.network.App.GetEVMKeeper().GetAccount(s.network.GetContext(), contractAddr)
			Expect(cAcc).ToNot(BeNil(), "contract account should exist")
			isContract := s.network.App.GetEVMKeeper().IsContract(s.network.GetContext(), contractAddr)
			Expect(isContract).To(BeTrue(), "account should be a contract")

			// populate default TxArgs
			txArgs.To = &contractAddr
			// populate default call args
			callArgs = testutiltypes.CallArgs{
				ContractABI: stakingCallerContract.ABI,
			}
			// populate default log check args
			defaultLogCheck = testutil.LogCheckArgs{
				ABIEvents: s.precompile.Events,
			}
			execRevertedCheck = defaultLogCheck.WithErrContains(vm.ErrExecutionReverted.Error())
			passCheck = defaultLogCheck.WithExpPass(true)
		})

		Describe("when the precompile is not enabled in the EVM params", func() {
			It("should return an error", func() {
				delegator := s.keyring.GetKey(0)

				// disable the precompile
				res, err := s.grpcHandler.GetEvmParams()
				Expect(err).To(BeNil(), "error while setting params")
				params := res.Params
				var activePrecompiles []string
				for _, precompile := range params.ActiveStaticPrecompiles {
					if precompile != s.precompile.Address().String() {
						activePrecompiles = append(activePrecompiles, precompile)
					}
				}
				params.ActiveStaticPrecompiles = activePrecompiles

				err = utils.UpdateEvmParams(utils.UpdateParamsInput{
					Tf:      s.factory,
					Network: s.network,
					Pk:      delegator.Priv,
					Params:  params,
				})
				Expect(err).To(BeNil(), "error while setting params")

				// try to call the precompile
				callArgs.MethodName = "testDelegate"
				callArgs.Args = []interface{}{
					valAddr.String(),
				}

				txArgs.Amount = big.NewInt(1e9)
				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					execRevertedCheck,
				)
				Expect(err).To(BeNil(), "error while funding the smart contract: %v", err)
			})
		})

		Context("create a validator", func() {
			var (
				valPriv    *ethsecp256k1.PrivKey
				valAddr    sdk.AccAddress
				valHexAddr common.Address

				defaultDescription = staking.Description{
					Moniker:         "new node",
					Identity:        "",
					Website:         "",
					SecurityContact: "",
					Details:         "",
				}
				defaultCommission = staking.Commission{
					Rate:          big.NewInt(100000000000000000),
					MaxRate:       big.NewInt(100000000000000000),
					MaxChangeRate: big.NewInt(100000000000000000),
				}
				defaultMinSelfDelegation = big.NewInt(1)
				defaultPubkeyBase64Str   = GenerateBase64PubKey()
				defaultValue             = big.NewInt(1e8)
			)

			BeforeEach(func() {
				callArgs.MethodName = "testCreateValidator"
				valAddr, valPriv = testutiltx.NewAccAddressAndKey()
				valHexAddr = common.BytesToAddress(valAddr.Bytes())
				err = utils.FundAccountWithBaseDenom(s.factory, s.network, s.keyring.GetKey(0), valAddr.Bytes(), math.NewInt(1e18))
				Expect(err).To(BeNil(), "error while funding account: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())
			})

			It("tx from validator operator - should NOT create a validator", func() {
				callArgs.Args = []interface{}{
					defaultDescription, defaultCommission, defaultMinSelfDelegation, valHexAddr, defaultPubkeyBase64Str, defaultValue,
				}

				_, _, err = s.factory.CallContractAndCheckLogs(
					valPriv,
					txArgs, callArgs,
					execRevertedCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())

				qc := s.network.GetStakingClient()
				_, err := qc.Validator(s.network.GetContext(), &stakingtypes.QueryValidatorRequest{ValidatorAddr: sdk.ValAddress(valAddr).String()})
				Expect(err).NotTo(BeNil(), "expected validator NOT to be found")
				Expect(err.Error()).To(ContainSubstring("not found"), "expected validator NOT to be found")
			})

			It("tx from another EOA - should create a validator fail", func() {
				callArgs.Args = []interface{}{
					defaultDescription, defaultCommission, defaultMinSelfDelegation, valHexAddr, defaultPubkeyBase64Str, defaultValue,
				}

				_, _, err = s.factory.CallContractAndCheckLogs(
					s.keyring.GetPrivKey(0),
					txArgs, callArgs,
					execRevertedCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())

				qc := s.network.GetStakingClient()
				_, err := qc.Validator(s.network.GetContext(), &stakingtypes.QueryValidatorRequest{ValidatorAddr: sdk.ValAddress(valAddr).String()})
				Expect(err).NotTo(BeNil(), "expected validator NOT to be found")
				Expect(err.Error()).To(ContainSubstring("not found"), "expected validator NOT to be found")
			})

			It("tx from validator operator with delegated code - should create a validator", func() {
				s.delegateAccountToContract(valPriv, valHexAddr, contractTwoAddr)

				callArgs = testutiltypes.CallArgs{
					ContractABI: stakingCallerTwoContract.ABI,
					MethodName:  "testCreateValidatorWithTransfer",
					Args: []interface{}{
						defaultDescription, defaultCommission, defaultMinSelfDelegation, valHexAddr, defaultPubkeyBase64Str, false, false,
					},
				}

				txArgs = evmtypes.EvmTxArgs{
					To:       &valHexAddr,
					GasLimit: 500_000,
					Amount:   new(big.Int).Set(defaultValue),
				}

				_, _, err = s.factory.CallContractAndCheckLogs(
					valPriv,
					txArgs, callArgs,
					passCheck.WithExpEvents(staking.EventTypeCreateValidator),
				)
				Expect(err).To(BeNil(), "error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())

				qc := s.network.GetStakingClient()
				_, err := qc.Validator(s.network.GetContext(), &stakingtypes.QueryValidatorRequest{ValidatorAddr: sdk.ValAddress(valAddr).String()})
				Expect(err).To(BeNil(), "expected validator NOT to be found")
			})
		})

		Context("to edit a validator", func() {
			var (
				valPriv    *ethsecp256k1.PrivKey
				valAddr    sdk.AccAddress
				valHexAddr common.Address

				defaultDescription = staking.Description{
					Moniker:         "edit node",
					Identity:        "[do-not-modify]",
					Website:         "[do-not-modify]",
					SecurityContact: "[do-not-modify]",
					Details:         "[do-not-modify]",
				}
				defaultCommissionRate    = big.NewInt(staking.DoNotModifyCommissionRate)
				defaultMinSelfDelegation = big.NewInt(staking.DoNotModifyMinSelfDelegation)

				minSelfDelegation = big.NewInt(1)

				description = staking.Description{}
				commission  = staking.Commission{}
			)

			BeforeEach(func() {
				callArgs.MethodName = "testEditValidator"

				// create a new validator
				valAddr, valPriv = testutiltx.NewAccAddressAndKey()
				valHexAddr = common.BytesToAddress(valAddr.Bytes())
				err = utils.FundAccountWithBaseDenom(s.factory, s.network, s.keyring.GetKey(0), valAddr.Bytes(), math.NewInt(2e18))
				Expect(err).To(BeNil(), "error while funding account: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				description = staking.Description{
					Moniker:         "original moniker",
					Identity:        "",
					Website:         "",
					SecurityContact: "",
					Details:         "",
				}
				commission = staking.Commission{
					Rate:          big.NewInt(100000000000000000),
					MaxRate:       big.NewInt(100000000000000000),
					MaxChangeRate: big.NewInt(100000000000000000),
				}
				pubkeyBase64Str := "UuhHQmkUh2cPBA6Rg4ei0M2B04cVYGNn/F8SAUsYIb4="
				value := big.NewInt(1e18)

				createValidatorArgs := testutiltypes.CallArgs{
					ContractABI: s.precompile.ABI,
					MethodName:  staking.CreateValidatorMethod,
					Args:        []interface{}{description, commission, minSelfDelegation, valHexAddr, pubkeyBase64Str, value},
				}

				logCheckArgs := passCheck.WithExpEvents(staking.EventTypeCreateValidator)

				toAddr := s.precompile.Address()
				_, _, err = s.factory.CallContractAndCheckLogs(
					valPriv,
					evmtypes.EvmTxArgs{
						To: &toAddr,
					},
					createValidatorArgs,
					logCheckArgs,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())
			})

			It("with tx from validator operator - should NOT edit a validator", func() {
				callArgs.Args = []interface{}{
					defaultDescription, valHexAddr,
					defaultCommissionRate, defaultMinSelfDelegation,
				}

				_, _, err = s.factory.CallContractAndCheckLogs(
					valPriv,
					txArgs,
					callArgs,
					execRevertedCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())

				qc := s.network.GetStakingClient()
				qRes, err := qc.Validator(s.network.GetContext(), &stakingtypes.QueryValidatorRequest{ValidatorAddr: sdk.ValAddress(valAddr).String()})
				Expect(err).To(BeNil())
				Expect(qRes).NotTo(BeNil())
				validator := qRes.Validator
				Expect(validator.Description.Moniker).NotTo(Equal(defaultDescription.Moniker), "expected validator moniker NOT to be updated")
			})

			It("with tx from another EOA - should fail", func() {
				callArgs.Args = []interface{}{
					defaultDescription, valHexAddr,
					defaultCommissionRate, defaultMinSelfDelegation,
				}

				_, _, err = s.factory.CallContractAndCheckLogs(
					s.keyring.GetPrivKey(0),
					txArgs,
					callArgs,
					execRevertedCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())

				// validator should remain unchanged
				qc := s.network.GetStakingClient()
				qRes, err := qc.Validator(s.network.GetContext(), &stakingtypes.QueryValidatorRequest{ValidatorAddr: sdk.ValAddress(valAddr).String()})
				Expect(err).To(BeNil())
				Expect(qRes).NotTo(BeNil())

				validator := qRes.Validator
				Expect(validator.Description.Moniker).To(Equal("original moniker"), "expected validator moniker is updated")
				Expect(validator.Commission.Rate.BigInt().String()).To(Equal("100000000000000000"), "expected validator commission rate remain unchanged")
			})

			It("with tx from validator operator using delegated code - should NOT edit a validator", func() {
				s.delegateAccountToContract(valPriv, valHexAddr, contractAddr)
				callArgs.Args = []interface{}{
					defaultDescription, valHexAddr,
					defaultCommissionRate, defaultMinSelfDelegation,
				}

				txArgs.To = &valHexAddr

				_, _, err = s.factory.CallContractAndCheckLogs(
					valPriv,
					txArgs,
					callArgs,
					execRevertedCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())

				qc := s.network.GetStakingClient()
				qRes, err := qc.Validator(s.network.GetContext(), &stakingtypes.QueryValidatorRequest{ValidatorAddr: sdk.ValAddress(valAddr).String()})
				Expect(err).To(BeNil())
				Expect(qRes).NotTo(BeNil())

				validator := qRes.Validator
				Expect(validator.Description.Moniker).NotTo(Equal(defaultDescription.Moniker), "expected validator moniker NOT to be updated")
			})
		})

		Context("delegating", func() {
			// prevDelegation is the delegation that is available prior to the test (an initial delegation is
			// added in the test suite setup).
			var prevDelegation stakingtypes.Delegation

			BeforeEach(func() {
				delegator := s.keyring.GetKey(0)

				txArgs.Amount = big.NewInt(1e18)
				txArgs.GasLimit = 500_000

				// initial delegation via contract
				callArgs.MethodName = "testDelegate"
				callArgs.Args = []interface{}{
					valAddr.String(),
				}

				logCheckArgs := passCheck.
					WithExpEvents(staking.EventTypeDelegate)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					logCheckArgs,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())

				// get the delegation that is available prior to the test
				contractAccAddr := sdk.AccAddress(contractAddr.Bytes())
				Expect(err).To(BeNil())
				res, err := s.grpcHandler.GetDelegation(contractAccAddr.String(), valAddr.String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponse).NotTo(BeNil())

				prevDelegation = res.DelegationResponse.Delegation
			})

			Context("with native coin transfer", func() {
				It("should delegate", func() {
					delegator := s.keyring.GetKey(0)

					txArgs.Amount = big.NewInt(1e18)

					callArgs.Args = []interface{}{
						valAddr.String(),
					}

					logCheckArgs := passCheck.
						WithExpEvents(staking.EventTypeDelegate)

					_, _, err = s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						logCheckArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
					Expect(s.network.NextBlock()).To(BeNil())

					contractAccAddr := sdk.AccAddress(contractAddr.Bytes())
					res, err := s.grpcHandler.GetDelegation(contractAccAddr.String(), valAddr.String())
					Expect(err).To(BeNil())
					Expect(res.DelegationResponse).NotTo(BeNil())
					delegation := res.DelegationResponse.Delegation

					expShares := prevDelegation.GetShares().Add(math.LegacyNewDec(1))
					Expect(delegation.GetShares()).To(Equal(expShares), "expected delegation shares to be 2")
				})

				Context("Calling the precompile from the StakingReverter contract", func() {
					var (
						txSenderInitialBal     *sdk.Coin
						contractInitialBalance *sdk.Coin
						gasPrice               = math.NewInt(1e9)
					)

					BeforeEach(func() {
						balRes, err := s.grpcHandler.GetBalanceFromBank(s.keyring.GetAccAddr(0), s.bondDenom)
						Expect(err).To(BeNil())
						txSenderInitialBal = balRes.Balance
						balRes, err = s.grpcHandler.GetBalanceFromBank(stkReverterAddr.Bytes(), s.bondDenom)
						Expect(err).To(BeNil())
						contractInitialBalance = balRes.Balance
					})

					It("should revert the changes and NOT delegate - successful tx", func() {
						callArgs := testutiltypes.CallArgs{
							ContractABI: stakingReverterContract.ABI,
							MethodName:  "run",
							Args: []interface{}{
								big.NewInt(5), s.network.GetValidators()[0].OperatorAddress,
							},
						}

						// Tx should be successful, but no state changes happened
						res, _, err := s.factory.CallContractAndCheckLogs(
							s.keyring.GetPrivKey(0),
							evmtypes.EvmTxArgs{
								To:       &stkReverterAddr,
								GasPrice: gasPrice.BigInt(),
							},
							callArgs,
							passCheck,
						)
						Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
						Expect(s.network.NextBlock()).To(BeNil())

						fees := gasPrice.MulRaw(res.GasUsed)

						// contract balance should remain unchanged
						balRes, err := s.grpcHandler.GetBalanceFromBank(stkReverterAddr.Bytes(), s.bondDenom)
						Expect(err).To(BeNil())
						contractFinalBalance := balRes.Balance
						Expect(contractFinalBalance.Amount).To(Equal(contractInitialBalance.Amount))

						// No delegation should be created
						_, err = s.grpcHandler.GetDelegation(sdk.AccAddress(stkReverterAddr.Bytes()).String(), s.network.GetValidators()[0].OperatorAddress)
						Expect(err).NotTo(BeNil())
						Expect(err.Error()).To(ContainSubstring("not found"), "expected NO delegation created")

						// Only fees deducted on tx sender
						balRes, err = s.grpcHandler.GetBalanceFromBank(s.keyring.GetAccAddr(0), s.bondDenom)
						Expect(err).To(BeNil())
						txSenderFinalBal := balRes.Balance
						Expect(txSenderFinalBal.Amount).To(Equal(txSenderInitialBal.Amount.Sub(fees)))
					})

					It("should revert the changes and NOT delegate - failed tx - max precompile calls reached", func() {
						callArgs := testutiltypes.CallArgs{
							ContractABI: stakingReverterContract.ABI,
							MethodName:  "multipleDelegations",
							Args: []interface{}{
								big.NewInt(int64(evmtypes.MaxPrecompileCalls + 2)), s.network.GetValidators()[0].OperatorAddress,
							},
						}

						// Tx should fail due to MaxPrecompileCalls
						_, _, err := s.factory.CallContractAndCheckLogs(
							s.keyring.GetPrivKey(0),
							evmtypes.EvmTxArgs{
								To:       &stkReverterAddr,
								GasPrice: gasPrice.BigInt(),
							},
							callArgs,
							execRevertedCheck,
						)
						Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

						// contract balance should remain unchanged
						balRes, err := s.grpcHandler.GetBalanceFromBank(stkReverterAddr.Bytes(), s.bondDenom)
						Expect(err).To(BeNil())
						contractFinalBalance := balRes.Balance
						Expect(contractFinalBalance.Amount).To(Equal(contractInitialBalance.Amount))

						// No delegation should be created
						_, err = s.grpcHandler.GetDelegation(sdk.AccAddress(stkReverterAddr.Bytes()).String(), s.network.GetValidators()[0].OperatorAddress)
						Expect(err).NotTo(BeNil())
						Expect(err.Error()).To(ContainSubstring("not found"), "expected NO delegation created")
					})

					It("should delegate before and after intentionaly ignored delegation revert - successful tx", func() {
						delegationAmount := math.NewInt(10)
						expectedDelegationAmount := delegationAmount.Add(delegationAmount)

						callArgs := testutiltypes.CallArgs{
							ContractABI: stakingReverterContract.ABI,
							MethodName:  "callPrecompileBeforeAndAfterRevert",
							Args: []interface{}{
								big.NewInt(5), s.network.GetValidators()[0].OperatorAddress,
							},
						}

						delegateCheck := passCheck.WithExpEvents(staking.EventTypeDelegate, staking.EventTypeDelegate)

						// The transaction should succeed with delegations occurring both before and after the intended revert.
						// The revert itself is not propagated because it occurs within the scope of a try-catch statement,
						// but is not caught by the catch block.
						res, _, err := s.factory.CallContractAndCheckLogs(
							s.keyring.GetPrivKey(0),
							evmtypes.EvmTxArgs{
								To:       &stkReverterAddr,
								GasPrice: gasPrice.BigInt(),
							},
							callArgs,
							delegateCheck,
						)
						Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
						Expect(s.network.NextBlock()).To(BeNil())

						fees := gasPrice.MulRaw(res.GasUsed)

						// delegation should have been created
						qRes, err := s.grpcHandler.GetDelegation(sdk.AccAddress(stkReverterAddr.Bytes()).String(), s.network.GetValidators()[0].OperatorAddress)
						Expect(err).To(BeNil())
						Expect(qRes.DelegationResponse.Delegation.GetDelegatorAddr()).To(Equal(sdk.AccAddress(stkReverterAddr.Bytes()).String()), "expected delegator address is equal to contract address")
						Expect(qRes.DelegationResponse.Delegation.GetShares().BigInt()).To(Equal(expectedDelegationAmount.BigInt()), "expected different delegation shares")

						// contract balance should be deducted by delegation amount
						balRes, err := s.grpcHandler.GetBalanceFromBank(stkReverterAddr.Bytes(), s.bondDenom)
						Expect(err).To(BeNil())
						contractFinalBalance := balRes.Balance
						Expect(contractFinalBalance.Amount).To(Equal(contractInitialBalance.Amount.Sub(expectedDelegationAmount)))

						// fees deducted on tx sender.
						// delegation amount is deducted on contract balance that is previously funded.
						balRes, err = s.grpcHandler.GetBalanceFromBank(s.keyring.GetAccAddr(0), s.bondDenom)
						Expect(err).To(BeNil())
						txSenderFinalBal := balRes.Balance
						Expect(txSenderFinalBal.Amount).To(Equal(txSenderInitialBal.Amount.Sub(fees)), "expected tx sender balance to be deducted by fees")
					})
					It("should test nested precompile calls with revert", func() {
						outerTimes := int64(3)
						innerTimes := int64(2)
						expectedDelegations := outerTimes + 2 // 1 before + outerTimes after catches + 1 after loop
						expectedDelegationAmount := math.NewInt(10).MulRaw(expectedDelegations)

						callArgs := testutiltypes.CallArgs{
							ContractABI: stakingReverterContract.ABI,
							MethodName:  "nestedTryCatchDelegations",
							Args: []interface{}{
								big.NewInt(outerTimes), big.NewInt(innerTimes), s.network.GetValidators()[0].OperatorAddress,
							},
						}

						expEvents := make([]string, 0, expectedDelegations)
						for range expectedDelegations {
							expEvents = append(expEvents, staking.EventTypeDelegate)
						}
						delegateCheck := passCheck.WithExpEvents(expEvents...)

						res, _, err := s.factory.CallContractAndCheckLogs(
							s.keyring.GetPrivKey(0),
							evmtypes.EvmTxArgs{
								To:       &stkReverterAddr,
								GasPrice: gasPrice.BigInt(),
							},
							callArgs,
							delegateCheck,
						)
						Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
						Expect(s.network.NextBlock()).To(BeNil())

						fees := gasPrice.MulRaw(res.GasUsed)

						// delegation should have been created with expected shares
						qRes, err := s.grpcHandler.GetDelegation(sdk.AccAddress(stkReverterAddr.Bytes()).String(), s.network.GetValidators()[0].OperatorAddress)
						Expect(err).To(BeNil())
						Expect(qRes.DelegationResponse.Delegation.GetDelegatorAddr()).To(Equal(sdk.AccAddress(stkReverterAddr.Bytes()).String()))
						Expect(qRes.DelegationResponse.Delegation.GetShares().BigInt()).To(Equal(expectedDelegationAmount.BigInt()))

						// contract balance should be deducted by total delegation amount
						balRes, err := s.grpcHandler.GetBalanceFromBank(stkReverterAddr.Bytes(), s.bondDenom)
						Expect(err).To(BeNil())
						contractFinalBalance := balRes.Balance
						Expect(contractFinalBalance.Amount).To(Equal(contractInitialBalance.Amount.Sub(expectedDelegationAmount)))

						// fees deducted on tx sender only
						balRes, err = s.grpcHandler.GetBalanceFromBank(s.keyring.GetAccAddr(0), s.bondDenom)
						Expect(err).To(BeNil())
						txSenderFinalBal := balRes.Balance
						Expect(txSenderFinalBal.Amount).To(Equal(txSenderInitialBal.Amount.Sub(fees)))
					})
				})

				Context("Table-driven tests for Delegate method", func() {
					// testCase is a struct used for cases of contracts calls that have some operation
					// performed before and/or after the precompile call
					type testCase struct {
						before bool
						after  bool
					}

					var (
						args                           testutiltypes.CallArgs
						delegatorInitialBal            *sdk.Coin
						contractInitialBalance         *sdk.Coin
						bondedTokensPoolInitialBalance *sdk.Coin
						delAmt                         = math.NewInt(1e18)
						gasPrice                       = math.NewInt(1e9)
						bondedTokensPoolAccAddr        = authtypes.NewModuleAddress("bonded_tokens_pool")
					)

					BeforeEach(func() {
						balRes, err := s.grpcHandler.GetBalanceFromBank(s.keyring.GetAccAddr(0), s.bondDenom)
						Expect(err).To(BeNil())
						delegatorInitialBal = balRes.Balance
						balRes, err = s.grpcHandler.GetBalanceFromBank(contractTwoAddr.Bytes(), s.bondDenom)
						Expect(err).To(BeNil())
						contractInitialBalance = balRes.Balance
						balRes, err = s.grpcHandler.GetBalanceFromBank(bondedTokensPoolAccAddr, s.bondDenom)
						Expect(err).To(BeNil())
						bondedTokensPoolInitialBalance = balRes.Balance

						args.ContractABI = stakingCallerTwoContract.ABI
						args.MethodName = "testDelegateWithCounterAndTransfer"
					})

					DescribeTable("should delegate and update balances accordingly", func(tc testCase) {
						args.Args = []interface{}{
							valAddr.String(), tc.before, tc.after,
						}

						// This is the amount of tokens transferred from the contract to the delegator
						// during the contract call
						transferToDelAmt := math.ZeroInt()
						for _, transferred := range []bool{tc.before, tc.after} {
							if transferred {
								transferToDelAmt = transferToDelAmt.AddRaw(15)
							}
						}

						logCheckArgs := passCheck.
							WithExpEvents(staking.EventTypeDelegate)

						txArgs := evmtypes.EvmTxArgs{
							To:       &contractTwoAddr,
							GasPrice: gasPrice.BigInt(),
							GasLimit: 500_000,
							Amount:   delAmt.BigInt(),
						}

						res, _, err := s.factory.CallContractAndCheckLogs(
							s.keyring.GetPrivKey(0),
							txArgs,
							args,
							logCheckArgs,
						)
						Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
						Expect(s.network.NextBlock()).To(BeNil())

						fees := gasPrice.MulRaw(res.GasUsed)

						// check the contract's balance was deducted to fund the vesting account
						balRes, err := s.grpcHandler.GetBalanceFromBank(contractTwoAddr.Bytes(), s.bondDenom)
						contractFinalBal := balRes.Balance
						Expect(err).To(BeNil())
						Expect(contractFinalBal.Amount).To(Equal(contractInitialBalance.Amount.Sub(transferToDelAmt)))

						contractTwoAccAddr := sdk.AccAddress(contractTwoAddr.Bytes())
						qRes, err := s.grpcHandler.GetDelegation(contractTwoAccAddr.String(), valAddr.String())
						Expect(err).To(BeNil())
						Expect(qRes).NotTo(BeNil(), "expected delegation to be found")
						delegation := qRes.DelegationResponse.Delegation
						expShares := math.LegacyZeroDec().Add(math.LegacyNewDec(1))
						Expect(delegation.GetShares()).To(Equal(expShares), "expected delegation shares to be 2")

						balRes, err = s.grpcHandler.GetBalanceFromBank(s.keyring.GetAccAddr(0), s.bondDenom)
						Expect(err).To(BeNil())
						delegatorFinalBal := balRes.Balance
						Expect(delegatorFinalBal.Amount).To(Equal(delegatorInitialBal.Amount.Sub(fees).Sub(delAmt).Add(transferToDelAmt)))

						// check the bondedTokenPool is updated with the delegated tokens
						balRes, err = s.grpcHandler.GetBalanceFromBank(bondedTokensPoolAccAddr, s.bondDenom)
						bondedTokensPoolFinalBalance := balRes.Balance
						Expect(err).To(BeNil())
						Expect(bondedTokensPoolFinalBalance.Amount).To(Equal(bondedTokensPoolInitialBalance.Amount.Add(delAmt)))
					},
						Entry("contract tx with transfer to delegator before and after precompile call ", testCase{
							before: true,
							after:  true,
						}),
						Entry("contract tx with transfer to delegator before precompile call ", testCase{
							before: true,
							after:  false,
						}),
						Entry("contract tx with transfer to delegator after precompile call ", testCase{
							before: false,
							after:  true,
						}),
					)

					It("should NOT delegate and update balances accordingly - internal transfer to tokens pool", func() {
						args.MethodName = "testDelegateWithTransfer"
						args.Args = []interface{}{
							common.BytesToAddress(bondedTokensPoolAccAddr),
							s.keyring.GetAddr(0), valAddr.String(), true, true,
						}

						txArgs.To = &contractTwoAddr

						reverReasonCheck := execRevertedCheck.WithErrContains(
							errorsmod.Wrapf(
								sdkerrors.ErrUnauthorized, "%s is not allowed to receive funds", bondedTokensPoolAccAddr.String(),
							).Error(),
						)

						_, _, err := s.factory.CallContractAndCheckLogs(
							s.keyring.GetPrivKey(0),
							txArgs,
							args,
							reverReasonCheck,
						)
						Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
						Expect(s.network.NextBlock()).To(BeNil())

						// contract balance should remain unchanged
						balRes, err := s.grpcHandler.GetBalanceFromBank(contractTwoAddr.Bytes(), s.bondDenom)
						Expect(err).To(BeNil())
						contractFinalBal := balRes.Balance
						Expect(contractFinalBal.Amount).To(Equal(contractInitialBalance.Amount))

						// check the bondedTokenPool should remain unchanged
						balRes, err = s.grpcHandler.GetBalanceFromBank(bondedTokensPoolAccAddr, s.bondDenom)
						Expect(err).To(BeNil())
						bondedTokensPoolFinalBalance := balRes.Balance
						Expect(bondedTokensPoolFinalBalance.Amount).To(Equal(bondedTokensPoolInitialBalance.Amount))
					})
				})

				It("should not delegate when validator does not exist", func() {
					delegator := s.keyring.GetKey(0)

					txArgs.Amount = big.NewInt(1e18)

					callArgs.Args = []interface{}{
						nonExistingVal.String(),
					}

					reverReasonCheck := execRevertedCheck.WithErrContains(
						stakingtypes.ErrNoValidatorFound.Error(),
					)

					_, _, err = s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						reverReasonCheck,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
					Expect(s.network.NextBlock()).To(BeNil())

					contractAccAddr := sdk.AccAddress(contractAddr.Bytes())
					res, err := s.grpcHandler.GetDelegation(contractAccAddr.String(), nonExistingVal.String())
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("delegation with delegator %s not found for validator %s", contractAccAddr.String(), nonExistingVal.String())))
					Expect(res).To(BeNil())
				})
			})
		})

		Context("unbonding", func() {
			var contractAccAddr sdk.AccAddress

			BeforeEach(func() {
				contractAccAddr = sdk.AccAddress(contractAddr.Bytes())

				callArgs.MethodName = "testUndelegate"

				// delegate to undelegate
				_, _, err = s.factory.CallContractAndCheckLogs(
					s.keyring.GetPrivKey(0),
					evmtypes.EvmTxArgs{
						To:       &contractAddr,
						Amount:   big.NewInt(1e18),
						GasPrice: big.NewInt(1e9),
						GasLimit: 500_000,
					},
					testutiltypes.CallArgs{
						ContractABI: stakingCallerContract.ABI,
						MethodName:  "testDelegate",
						Args: []interface{}{
							valAddr.String(),
						},
					},
					passCheck.WithExpEvents(staking.EventTypeDelegate),
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				txArgs.GasLimit = 500_000
				txArgs.Amount = big.NewInt(0)
			})

			It("should undelegate", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					valAddr.String(), big.NewInt(1e18),
				}

				logCheckArgs := defaultLogCheck.
					WithExpEvents(staking.EventTypeUnbond).
					WithExpPass(true)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					logCheckArgs)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(contractAccAddr.String())
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(HaveLen(1), "expected one undelegation")
				Expect(res.UnbondingResponses[0].ValidatorAddress).To(Equal(valAddr.String()), "expected validator address to be %s", valAddr)
			})

			It("should not undelegate if the delegation does not exist", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					nonExistingVal.String(), big.NewInt(1e18),
				}

				revertReasonCheck := execRevertedCheck.WithErrNested(CallerErrDelegationNotExist)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					revertReasonCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(contractAccAddr.String())
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(BeEmpty())
			})

			It("should not undelegate when called from a different address", func() {
				delegator := s.keyring.GetKey(0)
				differentSender := s.keyring.GetKey(1)

				callArgs.Args = []interface{}{
					valAddr.String(), big.NewInt(1e18),
				}

				revertReasonCheck := execRevertedCheck.WithErrNested(CallerErrDelegationNotExist)

				_, _, err := s.factory.CallContractAndCheckLogs(
					differentSender.Priv,
					txArgs, callArgs,
					revertReasonCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(delegator.AccAddr.String())
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(BeEmpty())
			})
		})

		Context("redelegating", func() {
			var contractAccAddr sdk.AccAddress

			BeforeEach(func() {
				contractAccAddr = sdk.AccAddress(contractAddr.Bytes())

				callArgs.MethodName = "testRedelegate"

				// delegate to redelegate
				_, _, err = s.factory.CallContractAndCheckLogs(
					s.keyring.GetPrivKey(0),
					evmtypes.EvmTxArgs{
						To:       &contractAddr,
						Amount:   big.NewInt(1e18),
						GasPrice: big.NewInt(1e9),
						GasLimit: 500_000,
					},
					testutiltypes.CallArgs{
						ContractABI: stakingCallerContract.ABI,
						MethodName:  "testDelegate",
						Args: []interface{}{
							valAddr.String(),
						},
					},
					passCheck.WithExpEvents(staking.EventTypeDelegate),
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				txArgs.GasLimit = 500_000
				txArgs.Amount = big.NewInt(0)
			})

			It("should redelegate", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					valAddr.String(), valAddr2.String(), big.NewInt(1e18),
				}

				logCheckArgs := defaultLogCheck.
					WithExpEvents(staking.EventTypeRedelegate).
					WithExpPass(true)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					logCheckArgs,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				res, err := s.grpcHandler.GetRedelegations(contractAccAddr.String(), valAddr.String(), valAddr2.String())
				Expect(err).To(BeNil())
				Expect(res.RedelegationResponses).To(HaveLen(1), "expected one redelegation to be found")
				Expect(res.RedelegationResponses[0].Redelegation.DelegatorAddress).To(Equal(contractAccAddr.String()), "expected delegator address to be %s", contractAccAddr)
				Expect(res.RedelegationResponses[0].Redelegation.ValidatorSrcAddress).To(Equal(valAddr.String()), "expected source validator address to be %s", valAddr)
				Expect(res.RedelegationResponses[0].Redelegation.ValidatorDstAddress).To(Equal(valAddr2.String()), "expected destination validator address to be %s", valAddr2)
			})

			It("should not redelegate if the delegation does not exist", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					nonExistingVal.String(), valAddr2.String(), big.NewInt(1e18),
				}

				revertReasonCheck := execRevertedCheck.WithErrNested(CallerErrDelegationNotExist)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					revertReasonCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				res, err := s.grpcHandler.GetRedelegations(contractAccAddr.String(), nonExistingVal.String(), valAddr2.String())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("redelegation not found for delegator address %s from validator address %s", contractAccAddr, nonExistingVal)))
				Expect(res).To(BeNil(), "expected no redelegations to be found")
			})

			It("should not redelegate when calling from a different address", func() {
				differentSender := s.keyring.GetKey(1)

				callArgs.Args = []interface{}{
					valAddr.String(), valAddr2.String(), big.NewInt(1e18),
				}

				revertReasonCheck := execRevertedCheck.WithErrNested(CallerErrDelegationNotExist)

				_, _, err = s.factory.CallContractAndCheckLogs(
					differentSender.Priv,
					txArgs, callArgs,
					revertReasonCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				res, err := s.grpcHandler.GetRedelegations(contractAccAddr.String(), valAddr.String(), valAddr2.String())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("redelegation not found for delegator address %s from validator address %s", contractAccAddr, valAddr)))
				Expect(res).To(BeNil(), "expected no redelegations to be found")
			})

			It("should not redelegate when the validator does not exist", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					valAddr.String(), nonExistingVal.String(), big.NewInt(1e18),
				}

				revertReasonCheck := execRevertedCheck.WithErrNested(stakingtypes.ErrBadRedelegationDst.Error())

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					revertReasonCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				res, err := s.grpcHandler.GetRedelegations(contractAccAddr.String(), valAddr.String(), nonExistingVal.String())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("redelegation not found for delegator address %s from validator address %s", contractAccAddr, valAddr)))
				Expect(res).To(BeNil())
			})
		})

		Context("canceling unbonding delegations", func() {
			// expCreationHeight is the expected creation height of the unbonding delegation
			var expCreationHeight int64
			var contractAccAddr sdk.AccAddress

			BeforeEach(func() {
				contractAccAddr = sdk.AccAddress(contractAddr.Bytes())

				callArgs.MethodName = "testCancelUnbonding"

				// delegate to undelegate
				_, _, err = s.factory.CallContractAndCheckLogs(
					s.keyring.GetPrivKey(0),
					evmtypes.EvmTxArgs{
						To:       &contractAddr,
						Amount:   big.NewInt(1e18),
						GasPrice: big.NewInt(1e9),
						GasLimit: 500_000,
					},
					testutiltypes.CallArgs{
						ContractABI: stakingCallerContract.ABI,
						MethodName:  "testDelegate",
						Args: []interface{}{
							valAddr.String(),
						},
					},
					passCheck.WithExpEvents(staking.EventTypeDelegate),
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil(), "failed to advance block")

				// undelegate to cancel unbonding
				delegator := s.keyring.GetKey(0)
				txArgs.Amount = big.NewInt(0)
				undelegateArgs := testutiltypes.CallArgs{
					ContractABI: stakingCallerContract.ABI,
					MethodName:  "testUndelegate",
					Args:        []interface{}{valAddr.String(), big.NewInt(1e18)},
				}

				logCheckArgs := defaultLogCheck.
					WithExpEvents(staking.EventTypeUnbond).
					WithExpPass(true)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, undelegateArgs,
					logCheckArgs,
				)
				Expect(err).To(BeNil(), "error while setting up an unbonding delegation: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				expCreationHeight = s.network.GetContext().BlockHeight()
				// Check that the unbonding delegation was created
				res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(contractAccAddr.String())
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(HaveLen(1), "expected one unbonding delegation to be found")
				Expect(res.UnbondingResponses[0].DelegatorAddress).To(Equal(contractAccAddr.String()), "expected delegator address to be %s", contractAccAddr)
				Expect(res.UnbondingResponses[0].ValidatorAddress).To(Equal(valAddr.String()), "expected validator address to be %s", valAddr)
				Expect(res.UnbondingResponses[0].Entries).To(HaveLen(1), "expected one unbonding delegation entry to be found")
				Expect(res.UnbondingResponses[0].Entries[0].CreationHeight).To(Equal(expCreationHeight), "expected different creation height")
				Expect(res.UnbondingResponses[0].Entries[0].Balance).To(Equal(math.NewInt(1e18)), "expected different balance")
			})

			It("should cancel unbonding delegations", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					valAddr.String(), big.NewInt(1e18), big.NewInt(expCreationHeight),
				}

				txArgs.GasLimit = 1e9

				logCheckArgs := passCheck.
					WithExpEvents(staking.EventTypeCancelUnbondingDelegation)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					logCheckArgs,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(contractAccAddr.String())
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(BeEmpty(), "expected unbonding delegation to be canceled")
			})

			It("should not cancel unbonding any delegations when unbonding delegation does not exist", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					nonExistingVal.String(),
					big.NewInt(1e18),
					big.NewInt(expCreationHeight),
				}

				revertReasonCheck := execRevertedCheck.WithErrNested(CallerErrUnbondingDelegationNotExist)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs,
					callArgs,
					revertReasonCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(contractAccAddr.String())
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(HaveLen(1), "expected unbonding delegation to not be canceled")
			})
		})

		Context("querying validator", func() {
			BeforeEach(func() {
				callArgs.MethodName = "getValidator"
			})
			It("with non-existing address should return an empty validator", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					nonExistingAddr,
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)
				Expect(valOut.Validator.OperatorAddress).To(Equal(""), "expected empty validator address")
				Expect(valOut.Validator.Status).To(Equal(uint8(0)), "expected validator status to be 0 (unspecified)")
			})

			It("with existing address should return the validator", func() {
				delegator := s.keyring.GetKey(0)

				valHexAddr := common.BytesToAddress(valAddr.Bytes())
				callArgs.Args = []interface{}{valHexAddr}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)
				Expect(valOut.Validator.OperatorAddress).To(Equal(valHexAddr.String()), "expected validator address to match")
				Expect(valOut.Validator.DelegatorShares).To(Equal(big.NewInt(1e18)), "expected different delegator shares")
			})

			It("with status bonded and pagination", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.MethodName = "getValidators"
				callArgs.Args = []interface{}{
					stakingtypes.Bonded.String(),
					query.PageRequest{
						Limit:      1,
						CountTotal: true,
					},
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorsOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorsMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)
				Expect(valOut.PageResponse.Total).To(Equal(uint64(len(s.network.GetValidators()))))
				Expect(valOut.PageResponse.NextKey).NotTo(BeEmpty())
				Expect(valOut.Validators[0].DelegatorShares).To(Equal(big.NewInt(1e18)), "expected different delegator shares")
			})
		})

		Context("querying validators", func() {
			BeforeEach(func() {
				callArgs.MethodName = "getValidators"
			})
			It("should return validators (default pagination)", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					stakingtypes.Bonded.String(),
					query.PageRequest{},
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorsOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorsMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)
				Expect(valOut.PageResponse.Total).To(Equal(uint64(len(s.network.GetValidators()))))
				Expect(valOut.PageResponse.NextKey).To(BeEmpty())
				Expect(valOut.Validators).To(HaveLen(len(s.network.GetValidators())), "expected all validators to be returned")
				// return order can change, that's why each validator is checked individually
				for _, val := range valOut.Validators {
					s.CheckValidatorOutput(val)
				}
			})

			//nolint:dupl // this is a duplicate of the test for EOA calls to the precompile
			It("should return validators with pagination limit = 1", func() {
				const limit uint64 = 1
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					stakingtypes.Bonded.String(),
					query.PageRequest{
						Limit:      limit,
						CountTotal: true,
					},
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorsOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorsMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)

				// no pagination, should return default values
				Expect(valOut.PageResponse.NextKey).NotTo(BeEmpty())
				Expect(valOut.PageResponse.Total).To(Equal(uint64(len(s.network.GetValidators()))))

				Expect(valOut.Validators).To(HaveLen(int(limit)), "expected one validator to be returned")

				// return order can change, that's why each validator is checked individually
				for _, val := range valOut.Validators {
					s.CheckValidatorOutput(val)
				}
			})

			It("should revert the execution if the bonding type is not known", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					"15", // invalid bonding type
					query.PageRequest{},
				}

				revertReasonCheck := execRevertedCheck.WithErrNested(
					fmt.Sprintf("rpc error: code = %s desc = invalid validator status %s", codes.InvalidArgument, "15"),
				)

				_, _, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					revertReasonCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
			})

			It("should return an empty array if there are no validators with the given bonding type", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					stakingtypes.Unbonded.String(),
					query.PageRequest{},
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var valOut staking.ValidatorsOutput
				err = s.precompile.UnpackIntoInterface(&valOut, staking.ValidatorsMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the validator output: %v", err)

				Expect(valOut.PageResponse.NextKey).To(BeEmpty())
				Expect(valOut.PageResponse.Total).To(Equal(uint64(0)))
				Expect(valOut.Validators).To(HaveLen(0), "expected no validators to be returned")
			})
		})

		Context("querying delegation", func() {
			BeforeEach(func() {
				callArgs.MethodName = "getDelegation"
			})
			It("which does not exist should return an empty delegation", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					nonExistingAddr, valAddr.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var delOut staking.DelegationOutput
				err = s.precompile.UnpackIntoInterface(&delOut, staking.DelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the delegation output: %v", err)
				Expect(delOut.Balance.Amount.Int64()).To(Equal(int64(0)), "expected a different delegation balance")
				Expect(delOut.Balance.Denom).To(Equal(cosmosevmutil.ExampleAttoDenom), "expected a different delegation balance")
			})

			It("which exists should return the delegation", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					delegator.Addr, valAddr.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var delOut staking.DelegationOutput
				err = s.precompile.UnpackIntoInterface(&delOut, staking.DelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the delegation output: %v", err)
				Expect(delOut.Balance).To(Equal(
					cmn.Coin{Denom: cosmosevmutil.ExampleAttoDenom, Amount: big.NewInt(1e18)}),
					"expected a different delegation balance",
				)
			})
		})

		Context("querying redelegation", func() {
			var contractAccAddr sdk.AccAddress

			BeforeEach(func() {
				callArgs.MethodName = "getRedelegation"
				contractAccAddr = sdk.AccAddress(contractAddr.Bytes())

				// delegate to redelegate
				_, _, err = s.factory.CallContractAndCheckLogs(
					s.keyring.GetPrivKey(0),
					evmtypes.EvmTxArgs{
						To:       &contractAddr,
						Amount:   big.NewInt(1e18),
						GasPrice: big.NewInt(1e9),
						GasLimit: 500_000,
					},
					testutiltypes.CallArgs{
						ContractABI: stakingCallerContract.ABI,
						MethodName:  "testDelegate",
						Args: []interface{}{
							valAddr.String(),
						},
					},
					passCheck.WithExpEvents(staking.EventTypeDelegate),
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil(), "failed to advance block")
			})

			It("which does not exist should return an empty redelegation", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					delegator.Addr, valAddr.String(), nonExistingVal.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var redOut staking.RedelegationOutput
				err = s.precompile.UnpackIntoInterface(&redOut, staking.RedelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the redelegation output: %v", err)
				Expect(redOut.Redelegation.Entries).To(HaveLen(0), "expected no redelegation entries")
			})

			It("which exists should return the redelegation", func() {
				delegator := s.keyring.GetKey(0)

				// set up redelegation
				redelegateArgs := testutiltypes.CallArgs{
					ContractABI: stakingCallerContract.ABI,
					MethodName:  "testRedelegate",
					Args:        []interface{}{valAddr.String(), valAddr2.String(), big.NewInt(1)},
				}

				redelegateCheck := passCheck.
					WithExpEvents(staking.EventTypeRedelegate)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, redelegateArgs,
					redelegateCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				// check that the redelegation was created
				res, err := s.grpcHandler.GetRedelegations(contractAccAddr.String(), valAddr.String(), valAddr2.String())
				Expect(err).To(BeNil())
				Expect(res.RedelegationResponses).To(HaveLen(1), "expected one redelegation to be found")
				bech32Addr := contractAccAddr
				Expect(res.RedelegationResponses[0].Redelegation.DelegatorAddress).To(Equal(bech32Addr.String()), "expected delegator address to be %s", contractAddr)
				Expect(res.RedelegationResponses[0].Redelegation.ValidatorSrcAddress).To(Equal(valAddr.String()), "expected source validator address to be %s", valAddr)
				Expect(res.RedelegationResponses[0].Redelegation.ValidatorDstAddress).To(Equal(valAddr2.String()), "expected destination validator address to be %s", valAddr2)

				// query redelegation
				callArgs.Args = []interface{}{
					contractAddr, valAddr.String(), valAddr2.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var redOut staking.RedelegationOutput
				err = s.precompile.UnpackIntoInterface(&redOut, staking.RedelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the redelegation output: %v", err)
				Expect(redOut.Redelegation.Entries).To(HaveLen(1), "expected one redelegation entry to be returned")
			})
		})

		Describe("query redelegations", func() {
			var contractAccAddr sdk.AccAddress

			BeforeEach(func() {
				contractAccAddr = sdk.AccAddress(contractAddr.Bytes())

				callArgs.MethodName = "getRedelegations"

				// delegate to redelegate
				_, _, err = s.factory.CallContractAndCheckLogs(
					s.keyring.GetPrivKey(0),
					evmtypes.EvmTxArgs{
						To:       &contractAddr,
						Amount:   big.NewInt(1e18),
						GasPrice: big.NewInt(1e9),
						GasLimit: 500_000,
					},
					testutiltypes.CallArgs{
						ContractABI: stakingCallerContract.ABI,
						MethodName:  "testDelegate",
						Args: []interface{}{
							valAddr.String(),
						},
					},
					passCheck.WithExpEvents(staking.EventTypeDelegate),
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil(), "failed to advance block")
			})

			It("which exists should return all the existing redelegations w/pagination", func() {
				delegator := s.keyring.GetKey(0)

				// set up redelegation
				redelegateArgs := testutiltypes.CallArgs{
					ContractABI: stakingCallerContract.ABI,
					MethodName:  "testRedelegate",
					Args:        []interface{}{valAddr.String(), valAddr2.String(), big.NewInt(1)},
				}

				redelegateCheck := passCheck.
					WithExpEvents(staking.EventTypeRedelegate)
				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, redelegateArgs,
					redelegateCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				// check that the redelegation was created
				res, err := s.grpcHandler.GetRedelegations(contractAccAddr.String(), valAddr.String(), valAddr2.String())
				Expect(err).To(BeNil())
				Expect(res.RedelegationResponses).To(HaveLen(1), "expected one redelegation to be found")
				bech32Addr := contractAccAddr
				Expect(res.RedelegationResponses[0].Redelegation.DelegatorAddress).To(Equal(bech32Addr.String()), "expected delegator address to be %s", contractAccAddr)
				Expect(res.RedelegationResponses[0].Redelegation.ValidatorSrcAddress).To(Equal(valAddr.String()), "expected source validator address to be %s", valAddr)
				Expect(res.RedelegationResponses[0].Redelegation.ValidatorDstAddress).To(Equal(valAddr2.String()), "expected destination validator address to be %s", valAddr2)

				// query redelegations by delegator address
				callArgs.Args = []interface{}{
					contractAddr, "", "", query.PageRequest{Limit: 1, CountTotal: true},
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					passCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				var redOut staking.RedelegationsOutput
				err = s.precompile.UnpackIntoInterface(&redOut, staking.RedelegationsMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the redelegation output: %v", err)
				Expect(redOut.Response).To(HaveLen(1), "expected one redelegation entry to be returned")
				Expect(redOut.Response[0].Entries).To(HaveLen(1), "expected one redelegation entry to be returned")
				Expect(redOut.PageResponse.Total).To(Equal(uint64(1)))
				Expect(redOut.PageResponse.NextKey).To(BeEmpty())
			})
		})

		Context("querying unbonding delegation", func() {
			var contractAccAddr sdk.AccAddress

			BeforeEach(func() {
				delegator := s.keyring.GetKey(0)
				contractAccAddr = sdk.AccAddress(contractAddr.Bytes())

				callArgs.MethodName = "getUnbondingDelegation"

				// delegate to redelegate
				_, _, err = s.factory.CallContractAndCheckLogs(
					s.keyring.GetPrivKey(0),
					evmtypes.EvmTxArgs{
						To:       &contractAddr,
						Amount:   big.NewInt(1e18),
						GasPrice: big.NewInt(1e9),
						GasLimit: 500_000,
					},
					testutiltypes.CallArgs{
						ContractABI: stakingCallerContract.ABI,
						MethodName:  "testDelegate",
						Args: []interface{}{
							valAddr.String(),
						},
					},
					passCheck.WithExpEvents(staking.EventTypeDelegate),
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil(), "failed to advance block")

				// undelegate
				undelegateArgs := testutiltypes.CallArgs{
					ContractABI: stakingCallerContract.ABI,
					MethodName:  "testUndelegate",
					Args:        []interface{}{valAddr.String(), big.NewInt(1e18)},
				}

				logCheckArgs := passCheck.
					WithExpEvents(staking.EventTypeUnbond)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, undelegateArgs, logCheckArgs)
				Expect(err).To(BeNil(), "error while setting up an unbonding delegation: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				// Check that the unbonding delegation was created
				res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(contractAccAddr.String())
				Expect(err).To(BeNil())
				Expect(res.UnbondingResponses).To(HaveLen(1), "expected one unbonding delegation to be found")
				Expect(res.UnbondingResponses[0].DelegatorAddress).To(Equal(contractAccAddr.String()), "expected delegator address to be %s", contractAddr)
				Expect(res.UnbondingResponses[0].ValidatorAddress).To(Equal(valAddr.String()), "expected validator address to be %s", valAddr)
				Expect(res.UnbondingResponses[0].Entries).To(HaveLen(1), "expected one unbonding delegation entry to be found")
				Expect(res.UnbondingResponses[0].Entries[0].CreationHeight).To(Equal(s.network.GetContext().BlockHeight()), "expected different creation height")
				Expect(res.UnbondingResponses[0].Entries[0].Balance).To(Equal(math.NewInt(1e18)), "expected different balance")
			})

			It("which does not exist should return an empty unbonding delegation", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					delegator.Addr, valAddr2.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs, passCheck)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var unbondingDelegationOutput staking.UnbondingDelegationOutput
				err = s.precompile.UnpackIntoInterface(&unbondingDelegationOutput, staking.UnbondingDelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the unbonding delegation output: %v", err)
				Expect(unbondingDelegationOutput.UnbondingDelegation.Entries).To(HaveLen(0), "expected one unbonding delegation entry")
			})

			It("which exists should return the unbonding delegation", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.Args = []interface{}{
					contractAddr, valAddr.String(),
				}

				_, ethRes, err := s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs, passCheck)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)

				var unbondOut staking.UnbondingDelegationOutput
				err = s.precompile.UnpackIntoInterface(&unbondOut, staking.UnbondingDelegationMethod, ethRes.Ret)
				Expect(err).To(BeNil(), "error while unpacking the unbonding delegation output: %v", err)
				Expect(unbondOut.UnbondingDelegation.Entries).To(HaveLen(1), "expected one unbonding delegation entry to be returned")
				Expect(unbondOut.UnbondingDelegation.Entries[0].Balance).To(Equal(big.NewInt(1e18)), "expected different balance")
			})
		})

		Context("when using special call opcodes", func() {
			var contractAccAddr sdk.AccAddress

			BeforeEach(func() {
				contractAccAddr = sdk.AccAddress(contractAddr.Bytes())

				// delegate to undelegate
				_, _, err = s.factory.CallContractAndCheckLogs(
					s.keyring.GetPrivKey(0),
					evmtypes.EvmTxArgs{
						To:       &contractAddr,
						Amount:   big.NewInt(1e18),
						GasPrice: big.NewInt(1e9),
						GasLimit: 500_000,
					},
					testutiltypes.CallArgs{
						ContractABI: stakingCallerContract.ABI,
						MethodName:  "testDelegate",
						Args: []interface{}{
							valAddr2.String(),
						},
					},
					passCheck.WithExpEvents(staking.EventTypeDelegate),
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil(), "failed to advance block")
			})

			testcases := []struct {
				// calltype is the opcode to use
				calltype string
				// expTxPass defines if executing transactions should be possible with the given opcode.
				// Queries should work for all options.
				expTxPass bool
			}{
				{"call", true},
				// {"callcode", false}, //todo: fix this - stops working after bech32 prefix changes off of evmos - the validator being sent in as arg contains a wrong checksum
				{"staticcall", false},
				{"delegatecall", false},
			}

			for _, tc := range testcases {
				// NOTE: this is necessary because of Ginkgo behavior -- if not done, the value of tc
				// inside the It block will always be the last entry in the testcases slice
				testcase := tc

				It(fmt.Sprintf("should not execute transactions for calltype %q", testcase.calltype), func() {
					delegator := s.keyring.GetKey(0)

					callArgs.MethodName = "testCallUndelegate"
					callArgs.Args = []interface{}{
						valAddr2.String(), big.NewInt(1e18), testcase.calltype,
					}

					checkArgs := execRevertedCheck.WithErrNested(fmt.Sprintf("failed %s to precompile", testcase.calltype))
					if testcase.expTxPass {
						checkArgs = passCheck.WithExpEvents(staking.EventTypeUnbond)
					}

					_, _, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs,
						checkArgs,
					)
					Expect(err).To(BeNil(), "error while calling the smart contract for calltype %s: %v", testcase.calltype, err)
					Expect(s.network.NextBlock()).To(BeNil())

					// check no delegations are unbonding
					res, err := s.grpcHandler.GetDelegatorUnbondingDelegations(contractAccAddr.String())
					Expect(err).To(BeNil())

					if testcase.expTxPass {
						Expect(res.UnbondingResponses).To(HaveLen(1), "expected an unbonding delegation")
						Expect(res.UnbondingResponses[0].ValidatorAddress).To(Equal(valAddr2.String()), "expected different validator address")
						Expect(res.UnbondingResponses[0].DelegatorAddress).To(Equal(contractAccAddr.String()), "expected different delegator address")
					} else {
						Expect(res.UnbondingResponses).To(HaveLen(0), "expected no unbonding delegations for calltype %s", testcase.calltype)
					}
				})

				It(fmt.Sprintf("should execute queries for calltype %q", testcase.calltype), func() {
					delegator := s.keyring.GetKey(0)

					callArgs.MethodName = "testCallDelegation"
					callArgs.Args = []interface{}{contractAddr, valAddr2.String(), testcase.calltype}

					_, ethRes, err := s.factory.CallContractAndCheckLogs(
						delegator.Priv,
						txArgs, callArgs, passCheck)
					Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
					Expect(s.network.NextBlock()).To(BeNil())

					var delOut staking.DelegationOutput
					err = s.precompile.UnpackIntoInterface(&delOut, staking.DelegationMethod, ethRes.Ret)
					Expect(err).To(BeNil(), "error while unpacking the delegation output: %v", err)
					Expect(delOut.Shares).To(Equal(math.LegacyNewDec(1).BigInt()), "expected different delegation shares")
					Expect(delOut.Balance.Amount).To(Equal(big.NewInt(1e18)), "expected different delegation balance")
					if testcase.calltype != "callcode" { // having some trouble with returning the denom from inline assembly but that's a very special edge case which might never be used
						Expect(delOut.Balance.Denom).To(Equal(s.bondDenom), "expected different denomination")
					}
				})
			}
		})

		// NOTE: These tests were added to replicate a problematic behavior, that occurred when a contract
		// adjusted the state in multiple subsequent function calls, which adjusted the EVM state as well as
		// things from the Cosmos SDK state (e.g. a bank balance).
		// The result was, that changes made to the Cosmos SDK state have been overwritten during the next function
		// call, because the EVM state was not updated in between.
		//
		// This behavior was fixed by updating the EVM state after each function call.
		Context("when triggering multiple state changes in one function", func() {
			// delegationAmount is the amount to be delegated
			delegationAmount := big.NewInt(1e18)

			BeforeEach(func() {
				// Set up funding for the contract address.
				// NOTE: we are first asserting that no balance exists and then check successful
				// funding afterwards.
				resBal, err := s.grpcHandler.GetBalanceFromBank(contractAddr.Bytes(), s.bondDenom)
				Expect(err).To(BeNil(), "error while getting balance")

				balanceBefore := resBal.Balance
				Expect(balanceBefore.Amount.Int64()).To(BeZero(), "expected contract balance to be 0 before funding")

				// Check no delegation exists from the contract to the validator
				res, err := s.grpcHandler.GetDelegation(sdk.AccAddress(contractAddr.Bytes()).String(), valAddr.String())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(fmt.Sprintf("delegation with delegator %s not found for validator %s", sdk.AccAddress(contractAddr.Bytes()), valAddr)))
				Expect(res).To(BeNil())
			})

			It("delegating and increasing counter should change the bank balance accordingly", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.MethodName = "testDelegateIncrementCounter"
				callArgs.Args = []interface{}{valAddr.String()}
				txArgs.GasLimit = 1e9
				txArgs.Amount = delegationAmount

				delegationCheck := passCheck.WithExpEvents(
					staking.EventTypeDelegate,
				)

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					delegationCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				res, err := s.grpcHandler.GetDelegation(sdk.AccAddress(contractAddr.Bytes()).String(), valAddr.String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponse).NotTo(BeNil())
				Expect(res.DelegationResponse.Delegation.GetShares().BigInt()).To(Equal(delegationAmount), "expected different delegation shares")

				resBal, err := s.grpcHandler.GetBalanceFromBank(contractAddr.Bytes(), s.bondDenom)
				Expect(err).To(BeNil(), "error while getting balance")

				postBalance := resBal.Balance
				Expect(postBalance.Amount.Int64()).To(BeZero(), "expected balance to be 0 after contract call")
			})
		})

		Context("when updating the stateDB prior to calling the precompile", func() {
			It("should utilize the same contract balance to delegate", func() {
				delegator := s.keyring.GetKey(0)
				fundAmount := big.NewInt(1e18)
				delegationAmount := big.NewInt(1e18)

				// fund the contract before calling the precompile
				err = utils.FundAccountWithBaseDenom(s.factory, s.network, s.keyring.GetKey(0), contractAddr.Bytes(), math.NewIntFromBigInt(fundAmount))
				Expect(err).To(BeNil(), "error while funding account")
				Expect(s.network.NextBlock()).To(BeNil())

				resBal, err := s.grpcHandler.GetBalanceFromBank(contractAddr.Bytes(), s.bondDenom)
				Expect(err).To(BeNil(), "error while getting balance")

				balanceAfterFunding := resBal.Balance
				Expect(balanceAfterFunding.Amount.BigInt()).To(Equal(fundAmount), "expected different contract balance after funding")

				// delegate
				callArgs.MethodName = "testDelegateAndFailCustomLogic"
				callArgs.Args = []interface{}{valAddr.String()}

				txArgs.Amount = delegationAmount
				txArgs.GasLimit = 1e9

				delegationCheck := passCheck.WithExpEvents(
					staking.EventTypeDelegate,
				)
				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					delegationCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				resBal, err = s.grpcHandler.GetBalanceFromBank(contractAddr.Bytes(), s.bondDenom)
				Expect(err).To(BeNil(), "error while getting balance")
				balance := resBal.Balance

				Expect(balance.Amount.Int64()).To(BeZero(), "expected different contract balance after funding")
				res, err := s.grpcHandler.GetDelegatorDelegations(sdk.AccAddress(contractAddr.Bytes()).String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponses).To(HaveLen(1), "expected one delegation")
				Expect(res.DelegationResponses[0].Delegation.GetShares().BigInt()).To(Equal(big.NewInt(1e18)), "expected different delegation shares")
			})

			//nolint:dupl
			It("should revert the contract balance to the original value when the custom logic after the precompile fails ", func() {
				delegator := s.keyring.GetKey(0)

				callArgs.MethodName = "testDelegateAndFailCustomLogic"
				callArgs.Args = []interface{}{valAddr.String()}

				txArgs.Amount = big.NewInt(2e18)
				txArgs.GasLimit = 1e9

				delegationCheck := defaultLogCheck.WithErrContains(vm.ErrExecutionReverted.Error())
				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					delegationCheck,
				)
				Expect(err).To(BeNil(), "error while calling the smart contract: %v", err)
				Expect(s.network.NextBlock()).To(BeNil())

				resBal, err := s.grpcHandler.GetBalanceFromBank(contractAddr.Bytes(), s.bondDenom)
				Expect(err).To(BeNil(), "error while getting balance")

				balance := resBal.Balance
				Expect(balance.Amount.Int64()).To(BeZero(), "expected different contract balance after funding")
				res, err := s.grpcHandler.GetDelegatorDelegations(sdk.AccAddress(contractAddr.Bytes()).String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponses).To(HaveLen(0), "expected no delegations")
			})
		})
	})

	// These tests are used to check that when batching multiple state changing transactions
	// in one block, both states (Cosmos and EVM) are updated or reverted correctly.
	//
	// For this purpose, we are deploying an ERC20 contract and updating StakingCaller.sol
	// to include a method where an ERC20 balance is sent between accounts as well as
	// an interaction with the staking precompile is made.
	//
	// There are ERC20 tokens minted to the address of the deployed StakingCaller contract,
	// which will transfer these to the message sender when successfully executed.
	_ = Describe("Batching cosmos and eth interactions", func() {
		const (
			erc20Name     = "Test"
			erc20Token    = "TTT"
			erc20Decimals = uint8(18)
		)

		var (
			// s is the precompile test suite to use for the tests
			s *PrecompileTestSuite
			// contractAddr is the address of the deployed StakingCaller contract
			contractAddr common.Address
			// contractAccAddr is the bech32 encoded account address of the deployed StakingCaller contract
			contractAccAddr sdk.AccAddress
			// stakingCallerContract is the contract instance calling into the staking precompile
			stakingCallerContract evmtypes.CompiledContract
			// erc20ContractAddr is the address of the deployed ERC20 contract
			erc20ContractAddr common.Address
			// erc20Contract is the compiled ERC20 contract
			erc20Contract = compiledcontracts.ERC20MinterBurnerDecimalsContract

			// err is a standard error
			err error
			// execRevertedCheck is a standard log check for a reverted transaction
			execRevertedCheck = defaultLogCheck.WithErrContains(vm.ErrExecutionReverted.Error())

			// mintAmount is the amount of ERC20 tokens minted to the StakingCaller contract
			mintAmount = big.NewInt(1e18)
			// transferredAmount is the amount of ERC20 tokens to transfer during the tests
			transferredAmount = big.NewInt(1234e9)
		)

		BeforeEach(func() {
			s = NewPrecompileTestSuite(create, options...)
			s.SetupTest()
			delegator := s.keyring.GetKey(0)

			stakingCallerContract, err = testdata.LoadStakingCallerContract()
			Expect(err).To(BeNil(), "error while loading the StakingCaller contract")

			// Deploy StakingCaller contract
			contractAddr, err = s.factory.DeployContract(
				delegator.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract: stakingCallerContract,
				},
			)
			Expect(err).To(BeNil(), "error while deploying the StakingCaller contract")
			Expect(s.network.NextBlock()).To(BeNil())

			contractAccAddr = sdk.AccAddress(contractAddr.Bytes())
			Expect(err).To(BeNil())

			// Deploy ERC20 contract
			erc20ContractAddr, err = s.factory.DeployContract(
				delegator.Priv,
				evmtypes.EvmTxArgs{}, // NOTE: passing empty struct to use default values
				testutiltypes.ContractDeploymentData{
					Contract:        erc20Contract,
					ConstructorArgs: []interface{}{erc20Name, erc20Token, erc20Decimals},
				},
			)
			Expect(err).To(BeNil(), "error while deploying the ERC20 contract")
			Expect(s.network.NextBlock()).To(BeNil())

			// Mint tokens to the StakingCaller contract
			mintArgs := testutiltypes.CallArgs{
				ContractABI: erc20Contract.ABI,
				MethodName:  "mint",
				Args:        []interface{}{contractAddr, mintAmount},
			}

			txArgs = evmtypes.EvmTxArgs{
				To: &erc20ContractAddr,
			}

			mintCheck := testutil.LogCheckArgs{
				ABIEvents: erc20Contract.ABI.Events,
				ExpEvents: []string{"Transfer"}, // minting produces a Transfer event
				ExpPass:   true,
			}

			_, _, err = s.factory.CallContractAndCheckLogs(
				delegator.Priv,
				txArgs, mintArgs, mintCheck)
			Expect(err).To(BeNil(), "error while minting tokens to the StakingCaller contract")
			Expect(s.network.NextBlock()).To(BeNil())

			// Check that the StakingCaller contract has the correct balance
			erc20Balance := s.network.App.GetErc20Keeper().BalanceOf(s.network.GetContext(), erc20Contract.ABI, erc20ContractAddr, contractAddr)
			Expect(erc20Balance).To(Equal(mintAmount), "expected different ERC20 balance for the StakingCaller contract")

			// populate default call args
			callArgs = testutiltypes.CallArgs{
				ContractABI: stakingCallerContract.ABI,
				MethodName:  "callERC20AndDelegate",
			}

			txArgs.To = &contractAddr

			// populate default log check args
			defaultLogCheck = testutil.LogCheckArgs{
				ABIEvents: s.precompile.Events,
			}
			execRevertedCheck = defaultLogCheck.WithErrContains(vm.ErrExecutionReverted.Error())
			passCheck = defaultLogCheck.WithExpPass(true)
		})

		Describe("when batching multiple transactions", func() {
			// validator is the validator address used for testing
			var validator sdk.ValAddress

			BeforeEach(func() {
				delegator := s.keyring.GetKey(0)

				res, err := s.grpcHandler.GetDelegatorDelegations(delegator.AccAddr.String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponses).ToNot(HaveLen(0), "expected address to have delegations")

				validator, err = sdk.ValAddressFromBech32(res.DelegationResponses[0].Delegation.ValidatorAddress)
				Expect(err).To(BeNil())

				_ = erc20ContractAddr

				// delegate
				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					evmtypes.EvmTxArgs{
						To:       &contractAddr,
						Amount:   big.NewInt(1e18),
						GasPrice: big.NewInt(1e9),
						GasLimit: 500_000,
					},
					testutiltypes.CallArgs{
						ContractABI: stakingCallerContract.ABI,
						MethodName:  "testDelegate",
						Args: []interface{}{
							validator.String(),
						},
					},
					passCheck.WithExpEvents(staking.EventTypeDelegate),
				)
				Expect(err).To(BeNil(), "error while calling the StakingCaller contract")
				Expect(s.network.NextBlock()).To(BeNil())
			})

			It("should revert both states if a staking transaction fails", func() {
				delegator := s.keyring.GetKey(0)

				res, err := s.grpcHandler.GetDelegation(contractAccAddr.String(), validator.String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponse).NotTo(BeNil())

				delegationPre := res.DelegationResponse.Delegation
				sharesPre := delegationPre.GetShares()

				// NOTE: passing an invalid validator address here should fail AFTER the erc20 transfer was made in the smart contract.
				// Therefore this can be used to check that both EVM and Cosmos states are reverted correctly.
				callArgs.Args = []interface{}{erc20ContractAddr, "invalid validator", transferredAmount}

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					execRevertedCheck)
				Expect(err).To(BeNil(), "expected error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())

				res, err = s.grpcHandler.GetDelegation(contractAccAddr.String(), validator.String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponse).NotTo(BeNil())
				delegationPost := res.DelegationResponse.Delegation
				sharesPost := delegationPost.GetShares()
				erc20BalancePost := s.network.App.GetErc20Keeper().BalanceOf(s.network.GetContext(), erc20Contract.ABI, erc20ContractAddr, delegator.Addr)

				Expect(sharesPost).To(Equal(sharesPre), "expected shares to be equal when reverting state")
				Expect(erc20BalancePost.Int64()).To(BeZero(), "expected erc20 balance of target address to be zero when reverting state")
			})

			It("should revert both states if an ERC20 transaction fails", func() {
				delegator := s.keyring.GetKey(0)

				res, err := s.grpcHandler.GetDelegation(contractAccAddr.String(), validator.String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponse).NotTo(BeNil())

				delegationPre := res.DelegationResponse.Delegation
				sharesPre := delegationPre.GetShares()

				// NOTE: trying to transfer more than the balance of the contract should fail in the smart contract.
				// Therefore this can be used to check that both EVM and Cosmos states are reverted correctly.
				moreThanMintedAmount := new(big.Int).Add(mintAmount, big.NewInt(1))
				callArgs.Args = []interface{}{erc20ContractAddr, s.network.GetValidators()[0].OperatorAddress, moreThanMintedAmount}

				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					execRevertedCheck)
				Expect(err).To(BeNil(), "expected error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())

				res, err = s.grpcHandler.GetDelegation(contractAccAddr.String(), validator.String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponse).NotTo(BeNil())
				delegationPost := res.DelegationResponse.Delegation
				sharesPost := delegationPost.GetShares()
				erc20BalancePost := s.network.App.GetErc20Keeper().BalanceOf(s.network.GetContext(), erc20Contract.ABI, erc20ContractAddr, delegator.Addr)

				Expect(sharesPost).To(Equal(sharesPre), "expected shares to be equal when reverting state")
				Expect(erc20BalancePost.Int64()).To(BeZero(), "expected erc20 balance of target address to be zero when reverting state")
			})

			It("should persist changes in both the cosmos and eth states", func() {
				delegator := s.keyring.GetKey(0)

				res, err := s.grpcHandler.GetDelegation(contractAccAddr.String(), validator.String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponse).NotTo(BeNil())

				delegationPre := res.DelegationResponse.Delegation
				sharesPre := delegationPre.GetShares()

				// NOTE: trying to transfer more than the balance of the contract should fail in the smart contract.
				// Therefore this can be used to check that both EVM and Cosmos states are reverted correctly.
				callArgs.Args = []interface{}{erc20ContractAddr, s.network.GetValidators()[0].OperatorAddress, transferredAmount}

				// Build combined map of ABI events to check for both ERC20 Transfer event as well as precompile events
				combinedABIEvents := s.precompile.Events
				combinedABIEvents["Transfer"] = erc20Contract.ABI.Events["Transfer"]

				successCheck := passCheck.
					WithABIEvents(combinedABIEvents).
					WithExpEvents(
						"Transfer", staking.EventTypeDelegate,
					)

				txArgs.Amount = big.NewInt(1e18)
				txArgs.GasPrice = big.NewInt(1e9)
				txArgs.GasLimit = 500_000
				_, _, err = s.factory.CallContractAndCheckLogs(
					delegator.Priv,
					txArgs, callArgs,
					successCheck)
				Expect(err).ToNot(HaveOccurred(), "error while calling the smart contract")
				Expect(s.network.NextBlock()).To(BeNil())

				res, err = s.grpcHandler.GetDelegation(contractAccAddr.String(), validator.String())
				Expect(err).To(BeNil())
				Expect(res.DelegationResponse).NotTo(BeNil(),
					"expected delegation from %s to validator %s to be found after calling the smart contract",
					delegator.AccAddr.String(), validator.String(),
				)
				delegationPost := res.DelegationResponse.Delegation
				sharesPost := delegationPost.GetShares()
				erc20BalancePost := s.network.App.GetErc20Keeper().BalanceOf(s.network.GetContext(), erc20Contract.ABI, erc20ContractAddr, delegator.Addr)

				Expect(sharesPost.GT(sharesPre)).To(BeTrue(), "expected shares to be more than before")
				Expect(erc20BalancePost).To(Equal(transferredAmount), "expected different erc20 balance of target address")
			})
		})
	})

	// Run Ginkgo integration tests
	RegisterFailHandler(Fail)
	RunSpecs(t, "Distribution Precompile Suite")
}
