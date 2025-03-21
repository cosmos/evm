package keeper_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	abcitypes "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/contracts"
	cmnfactory "github.com/cosmos/evm/testutil/integration/common/factory"
	"github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	"github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evm "github.com/cosmos/evm/x/vm/types"
	capabilitytypes "github.com/cosmos/ibc-go/modules/capability/types"
	transfertypes "github.com/cosmos/ibc-go/v8/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v8/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v8/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v8/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v8/modules/core/exported"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type KeeperTestSuite struct {
	suite.Suite

	network *network.UnitTestNetwork
	handler grpc.Handler
	keyring keyring.Keyring
	factory factory.TxFactory

	otherDenom string
}

var timeoutHeight = clienttypes.NewHeight(1000, 1000)

func TestKeeperTestSuite(t *testing.T) {
	s := new(KeeperTestSuite)
	suite.Run(t, s)
}

func (suite *KeeperTestSuite) SetupTest() {
	keys := keyring.New(2)
	suite.otherDenom = "xmpl"

	// Set custom genesis with capability record
	customGenesis := network.CustomGenesisState{}

	capParams := capabilitytypes.DefaultGenesis()
	capParams.Index = 2
	capParams.Owners = []capabilitytypes.GenesisOwners{
		{
			Index: 1,
			IndexOwners: capabilitytypes.CapabilityOwners{
				Owners: []capabilitytypes.Owner{
					{
						Module: "ibc",
						Name:   "capabilities/ports/transfer/channels/channel-0",
					},
					{
						Module: "transfer",
						Name:   "capabilities/ports/transfer/channels/channel-0",
					},
				},
			},
		},
	}

	customGenesis[capabilitytypes.ModuleName] = capParams

	nw := network.NewUnitTestNetwork(
		network.WithPreFundedAccounts(keys.GetAllAccAddrs()...),
		network.WithOtherDenoms([]string{suite.otherDenom}),
		network.WithCustomGenesis(customGenesis),
	)
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)

	suite.network = nw
	suite.factory = tf
	suite.handler = gh
	suite.keyring = keys
}

var _ transfertypes.ChannelKeeper = &MockChannelKeeper{}

type MockChannelKeeper struct {
	mock.Mock
}

func (b *MockChannelKeeper) GetChannel(ctx sdk.Context, srcPort, srcChan string) (channel channeltypes.Channel, found bool) {
	args := b.Called(mock.Anything, mock.Anything, mock.Anything)
	return args.Get(0).(channeltypes.Channel), true
}

func (b *MockChannelKeeper) GetNextSequenceSend(ctx sdk.Context, portID, channelID string) (uint64, bool) {
	_ = b.Called(mock.Anything, mock.Anything, mock.Anything)
	return 1, true
}
func (b *MockChannelKeeper) GetAllChannelsWithPortPrefix(ctx sdk.Context, portPrefix string) []channeltypes.IdentifiedChannel {
	return []channeltypes.IdentifiedChannel{}
}

var _ porttypes.ICS4Wrapper = &MockICS4Wrapper{}

type MockICS4Wrapper struct {
	mock.Mock
}

func (b *MockICS4Wrapper) WriteAcknowledgement(_ sdk.Context, _ *capabilitytypes.Capability, _ exported.PacketI, _ exported.Acknowledgement) error {
	return nil
}

func (b *MockICS4Wrapper) GetAppVersion(ctx sdk.Context, portID string, channelID string) (string, bool) {
	return "", false
}
func (b *MockICS4Wrapper) SendPacket(
	ctx sdk.Context,
	channelCap *capabilitytypes.Capability,
	sourcePort string,
	sourceChannel string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
	data []byte,
) (sequence uint64, err error) {
	// _ = b.Called(mock.Anything, mock.Anything, mock.Anything)
	return 0, nil
}

func (suite *KeeperTestSuite) MintERC20Token(contractAddr, to common.Address, amount *big.Int) (abcitypes.ExecTxResult, error) {
	res, err := suite.factory.ExecuteContractCall(
		suite.keyring.GetPrivKey(0),
		evm.EvmTxArgs{
			To: &contractAddr,
		},
		factory.CallArgs{
			ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
			MethodName:  "mint",
			Args:        []interface{}{to, amount},
		},
	)
	if err != nil {
		return res, err
	}

	return res, suite.network.NextBlock()
}

func (suite *KeeperTestSuite) DeployContract(name, symbol string, decimals uint8) (common.Address, error) {
	addr, err := suite.factory.DeployContract(
		suite.keyring.GetPrivKey(0),
		evm.EvmTxArgs{},
		factory.ContractDeploymentData{
			Contract:        contracts.ERC20MinterBurnerDecimalsContract,
			ConstructorArgs: []interface{}{name, symbol, decimals},
		},
	)
	if err != nil {
		return common.Address{}, err
	}

	return addr, suite.network.NextBlock()
}

func (suite *KeeperTestSuite) ConvertERC20(sender keyring.Key, contractAddr common.Address, amt math.Int) error {
	msg := &erc20types.MsgConvertERC20{
		ContractAddress: contractAddr.Hex(),
		Amount:          amt,
		Sender:          sender.Addr.String(),
		Receiver:        sender.AccAddr.String(),
	}
	_, err := suite.factory.CommitCosmosTx(sender.Priv, cmnfactory.CosmosTxArgs{
		Msgs: []sdk.Msg{msg},
	})

	return err
}
