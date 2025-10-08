package ibc

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"

	abcitypes "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/contracts"
	cmnfactory "github.com/cosmos/evm/testutil/integration/base/factory"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	"github.com/cosmos/ibc-go/v10/modules/core/exported"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type KeeperTestSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption
	network *network.UnitTestNetwork
	handler grpc.Handler
	keyring keyring.Keyring
	factory factory.TxFactory

	otherDenom string
}

var timeoutHeight = clienttypes.NewHeight(1000, 1000)

func NewKeeperTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *KeeperTestSuite {
	return &KeeperTestSuite{
		create:  create,
		options: options,
	}
}

func (s *KeeperTestSuite) SetupTest() {
	keys := keyring.New(2)
	s.otherDenom = "xmpl"

	options := []network.ConfigOption{
		network.WithPreFundedAccounts(keys.GetAllAccAddrs()...),
		network.WithOtherDenoms([]string{s.otherDenom}),
	}
	options = append(options, s.options...)
	nw := network.NewUnitTestNetwork(s.create, options...)
	gh := grpc.NewIntegrationHandler(nw)
	tf := factory.New(nw, gh)

	s.network = nw
	s.factory = tf
	s.handler = gh
	s.keyring = keys
}

var _ transfertypes.ChannelKeeper = &MockChannelKeeper{}

type MockChannelKeeper struct {
	mock.Mock
}

func (b *MockChannelKeeper) SetICS4Wrapper(_ porttypes.ICS4Wrapper) {}

func (b *MockChannelKeeper) GetChannel(ctx sdk.Context, srcPort, srcChan string) (channel channeltypes.Channel, found bool) {
	args := b.Called(mock.Anything, mock.Anything, mock.Anything)
	return args.Get(0).(channeltypes.Channel), true
}

func (b *MockChannelKeeper) HasChannel(ctx sdk.Context, srcPort, srcChan string) bool {
	_ = b.Called(mock.Anything, mock.Anything, mock.Anything)
	return true
}

func (b *MockChannelKeeper) GetNextSequenceSend(ctx sdk.Context, portID, channelID string) (uint64, bool) {
	_ = b.Called(mock.Anything, mock.Anything, mock.Anything)
	return 1, true
}

func (b *MockChannelKeeper) GetAllChannelsWithPortPrefix(ctx sdk.Context, portPrefix string) []channeltypes.IdentifiedChannel {
	return []channeltypes.IdentifiedChannel{}
}

func (b *MockChannelKeeper) WriteAcknowledgement(_ sdk.Context, _ exported.PacketI, _ exported.Acknowledgement) error {
	return nil
}

func (b *MockChannelKeeper) GetAppVersion(ctx sdk.Context, portID string, channelID string) (string, bool) {
	return "", false
}

func (b *MockChannelKeeper) SendPacket(
	ctx sdk.Context,
	sourcePort string,
	sourceChannel string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
	data []byte,
) (sequence uint64, err error) {
	// _ = b.Called(mock.Anything, mock.Anything, mock.Anything)
	return 0, nil
}

var _ porttypes.ICS4Wrapper = &MockICS4Wrapper{}

type MockICS4Wrapper struct {
	mock.Mock
}

func (b *MockICS4Wrapper) WriteAcknowledgement(_ sdk.Context, _ exported.PacketI, _ exported.Acknowledgement) error {
	return nil
}

func (b *MockICS4Wrapper) GetAppVersion(ctx sdk.Context, portID string, channelID string) (string, bool) {
	return "", false
}

func (b *MockICS4Wrapper) SendPacket(
	ctx sdk.Context,
	sourcePort string,
	sourceChannel string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
	data []byte,
) (sequence uint64, err error) {
	// _ = b.Called(mock.Anything, mock.Anything, mock.Anything)
	return 0, nil
}

func (s *KeeperTestSuite) MintERC20Token(contractAddr, to common.Address, amount *big.Int) (abcitypes.ExecTxResult, error) {
	res, err := s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{
			To: &contractAddr,
		},
		testutiltypes.CallArgs{
			ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
			MethodName:  "mint",
			Args:        []interface{}{to, amount},
		},
	)
	if err != nil {
		return res, err
	}

	return res, s.network.NextBlock()
}

func (s *KeeperTestSuite) DeployContract(name, symbol string, decimals uint8) (common.Address, error) {
	addr, err := s.factory.DeployContract(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{},
		testutiltypes.ContractDeploymentData{
			Contract:        contracts.ERC20MinterBurnerDecimalsContract,
			ConstructorArgs: []interface{}{name, symbol, decimals},
		},
	)
	if err != nil {
		return common.Address{}, err
	}

	return addr, s.network.NextBlock()
}

func (s *KeeperTestSuite) ConvertERC20(sender keyring.Key, contractAddr common.Address, amt math.Int) error {
	msg := &erc20types.MsgConvertERC20{
		ContractAddress: contractAddr.Hex(),
		Amount:          amt,
		Sender:          sender.Addr.String(),
		Receiver:        sender.AccAddr.String(),
	}
	_, err := s.factory.CommitCosmosTx(sender.Priv, cmnfactory.CosmosTxArgs{
		Msgs: []sdk.Msg{msg},
	})

	return err
}
