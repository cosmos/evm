package ante

import (
	"testing"

	"github.com/stretchr/testify/suite"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	testconstants "github.com/cosmos/evm/testutil/constants"
	commonfactory "github.com/cosmos/evm/testutil/integration/base/factory"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	testutiltx "github.com/cosmos/evm/testutil/tx"

	"cosmossdk.io/math"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	distrtypes "github.com/cosmos/cosmos-sdk/x/distribution/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

type IntegrationTestSuite struct {
	suite.Suite

	create      network.CreateEvmApp
	options     []network.ConfigOption
	network     network.Network
	factory     factory.TxFactory
	grpcHandler grpc.Handler
	keyring     testkeyring.Keyring
}

func NewIntegrationTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *IntegrationTestSuite {
	return &IntegrationTestSuite{
		create:  create,
		options: options,
	}
}

func (s *IntegrationTestSuite) SetupTest() {
	keyring := testkeyring.New(2)
	validatorsKeys := generateKeys(3)
	customGen := network.CustomGenesisState{}

	// set some slashing events for integration test
	distrGen := distrtypes.DefaultGenesisState()
	customGen[distrtypes.ModuleName] = distrGen

	// set non-zero inflation for rewards to accrue (use defaults from SDK for values)
	mintGen := minttypes.DefaultGenesisState()
	mintGen.Params.MintDenom = testconstants.ExampleAttoDenom
	customGen[minttypes.ModuleName] = mintGen

	operatorsAddr := make([]sdk.AccAddress, 3)
	for i, k := range validatorsKeys {
		operatorsAddr[i] = k.AccAddr
	}

	options := []network.ConfigOption{
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		network.WithCustomGenesis(customGen),
		network.WithValidatorOperators(operatorsAddr),
	}
	options = append(options, s.options...)
	nw := network.NewUnitTestNetwork(s.create, options...)
	grpcHandler := grpc.NewIntegrationHandler(nw)
	txFactory := factory.New(nw, grpcHandler)

	s.factory = txFactory
	s.grpcHandler = grpcHandler
	s.keyring = keyring
	s.network = nw
}

func TestIntegrationAnteHandler(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	if create == nil {
		panic("create function cannot be nil")
	}

	_ = Describe("when sending a Cosmos transaction", Label("AnteHandler"), Ordered, func() {
		var s *IntegrationTestSuite

		BeforeAll(func() {
			s = NewIntegrationTestSuite(create, options...)
			s.SetupTest()
		})

		Context("and the sender account has enough balance to pay for the transaction cost", Ordered, func() {
			var (
				rewards      sdk.DecCoins
				transferAmt  math.Int
				addr         sdk.AccAddress
				receiverAddr sdk.AccAddress
				priv         cryptotypes.PrivKey
				msg          sdk.Msg
			)

			BeforeEach(func() {
				key := s.keyring.GetKey(0)
				addr = key.AccAddr
				priv = key.Priv
				receiverAddr, _ = testutiltx.NewAccAddressAndKey()

				transferAmt = math.NewInt(1e14)
				msg = &banktypes.MsgSend{
					FromAddress: addr.String(),
					ToAddress:   receiverAddr.String(),
					Amount:      sdk.Coins{sdk.Coin{Amount: transferAmt, Denom: s.network.GetBaseDenom()}},
				}

				valAddr := s.network.GetValidators()[0].OperatorAddress
				delegationCoin := sdk.Coin{Amount: math.NewInt(1e15), Denom: s.network.GetBaseDenom()}
				err := s.factory.Delegate(priv, valAddr, delegationCoin)
				Expect(err).To(BeNil())

				minExpRewards := sdk.DecCoins{sdk.DecCoin{Amount: math.LegacyNewDec(1e5), Denom: s.network.GetBaseDenom()}}

				rewards, err = utils.WaitToAccrueRewards(s.network, s.grpcHandler, addr.String(), minExpRewards)
				Expect(err).To(BeNil())
			})

			It("should succeed & not withdraw any staking rewards", func() {
				prevBalanceRes, err := s.grpcHandler.GetBalanceFromBank(addr, s.network.GetBaseDenom())
				Expect(err).To(BeNil())

				baseFeeRes, err := s.grpcHandler.GetEvmBaseFee()
				Expect(err).To(BeNil())
				Expect(baseFeeRes).ToNot(BeNil(), "baseFeeRes is nil")

				gasPrice := baseFeeRes.BaseFee.AddRaw(100)

				res, err := s.factory.ExecuteCosmosTx(
					priv,
					commonfactory.CosmosTxArgs{
						Msgs:     []sdk.Msg{msg},
						GasPrice: &gasPrice,
					},
				)
				Expect(err).To(BeNil())
				Expect(res.IsOK()).To(BeTrue())

				// include the tx in a block to update state
				err = s.network.NextBlock()
				Expect(err).To(BeNil())

				feesAmt := math.NewInt(res.GasWanted).Mul(gasPrice)
				balanceRes, err := s.grpcHandler.GetBalanceFromBank(addr, s.network.GetBaseDenom())
				Expect(err).To(BeNil())
				Expect(balanceRes.Balance.Amount).To(Equal(prevBalanceRes.Balance.Amount.Sub(transferAmt).Sub(feesAmt)))

				rewardsRes, err := s.grpcHandler.GetDelegationTotalRewards(addr.String())
				Expect(err).To(BeNil())

				// rewards should not be used. Should be more
				// than the previous value queried
				Expect(rewardsRes.Total.Sub(rewards).IsAllPositive()).To(BeTrue())
			})
		})

		Context("and the sender account neither has enough balance nor sufficient staking rewards to pay for the transaction cost", func() {
			var (
				addr sdk.AccAddress
				priv cryptotypes.PrivKey
				msg  sdk.Msg
			)

			BeforeEach(func() {
				addr, priv = testutiltx.NewAccAddressAndKey()

				// this is a new address that does not exist on chain.
				// Transfer 1 aatom to this account so it is
				// added on chain
				err := s.factory.FundAccount(
					s.keyring.GetKey(0),
					addr,
					sdk.Coins{
						sdk.Coin{
							Amount: math.NewInt(1),
							Denom:  s.network.GetBaseDenom(),
						},
					},
				)
				Expect(err).To(BeNil())
				// persist the state changes
				Expect(s.network.NextBlock()).To(BeNil())

				msg = &banktypes.MsgSend{
					FromAddress: addr.String(),
					ToAddress:   "cosmos1dx67l23hz9l0k9hcher8xz04uj7wf3yu26l2yn",
					Amount:      sdk.Coins{sdk.Coin{Amount: math.NewInt(1e14), Denom: s.network.GetBaseDenom()}},
				}
			})

			It("should fail", func() {
				var gas uint64 = 200_000 // specify gas to avoid failing on simulation tx (internal call in the ExecuteCosmosTx if gas not specified)
				res, err := s.factory.ExecuteCosmosTx(
					priv,
					commonfactory.CosmosTxArgs{
						Msgs: []sdk.Msg{msg},
						Gas:  &gas,
					},
				)
				Expect(res.IsErr()).To(BeTrue())
				Expect(res.GetLog()).To(ContainSubstring("insufficient funds"))
				Expect(err).To(BeNil())
				Expect(s.network.NextBlock()).To(BeNil())
			})

			It("should not withdraw any staking rewards", func() {
				rewardsRes, err := s.grpcHandler.GetDelegationTotalRewards(addr.String())
				Expect(err).To(BeNil())
				Expect(rewardsRes.Total.Empty()).To(BeTrue())
			})
		})
	})

	RegisterFailHandler(Fail)
	RunSpecs(t, "AnteHandler Integration Test Suite")
}

func generateKeys(count int) []testkeyring.Key {
	accs := make([]testkeyring.Key, 0, count)
	for i := 0; i < count; i++ {
		acc := testkeyring.NewKey()
		accs = append(accs, acc)
	}
	return accs
}
