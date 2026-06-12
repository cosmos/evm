package backend

import (
	"math/big"
	"os"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/suite"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/indexer"
	"github.com/cosmos/evm/rpc/backend/mocks"
	"github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	"cosmossdk.io/log"
)

// TestMain initializes the global EVM chain config required by NewBackend.
// Without this, GetEthChainConfig() panics on a nil dereference.
func TestMain(m *testing.M) {
	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	ethCfg := evmtypes.DefaultChainConfig(constants.ExampleChainID.EVMChainID)
	if err := evmtypes.SetChainConfig(ethCfg); err != nil {
		panic(err)
	}
	coinInfo := constants.ExampleChainCoinInfo[constants.ExampleChainID]
	if err := evmtypes.NewEVMConfigurator().
		WithEVMCoinInfo(coinInfo).
		Configure(); err != nil {
		panic(err)
	}
	os.Exit(m.Run())
}

// BackendTestSuite wraps setupMockBackend and adds helpers used by TraceTransaction tests.
type BackendTestSuite struct {
	suite.Suite
	backend *Backend
	signer  keyring.Signer
}

func TestBackendTestSuite(t *testing.T) {
	suite.Run(t, new(BackendTestSuite))
}

func (suite *BackendTestSuite) SetupTest() {
	suite.backend = setupMockBackend(suite.T())
	_, priv := utiltx.NewAddrKey()
	suite.signer = utiltx.NewSigner(priv)
}

// buildEthereumTx returns an unsigned legacy EVM tx and its pre-encoded bytes.
// From is left empty; call signAndEncodeEthTx for a fully signed single-msg tx.
func (suite *BackendTestSuite) buildEthereumTx() (*evmtypes.MsgEthereumTx, []byte) {
	ethTxParams := evmtypes.EvmTxArgs{
		ChainID:  suite.backend.EvmChainID,
		Nonce:    0,
		To:       &common.Address{},
		Amount:   big.NewInt(0),
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
	}
	msg := evmtypes.NewTx(&ethTxParams)

	txBuilder := suite.backend.ClientCtx.TxConfig.NewTxBuilder()
	suite.Require().NoError(txBuilder.SetMsgs(msg))
	bz, err := suite.backend.ClientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	suite.Require().NoError(err)
	return msg, bz
}

// signAndEncodeEthTx signs msg with a fresh ephemeral key and encodes it as a
// single-message Cosmos tx. The msg.From field is updated in-place; hashes
// computed via msg.AsTransaction().Hash() are valid after this returns.
func (suite *BackendTestSuite) signAndEncodeEthTx(msg *evmtypes.MsgEthereumTx) []byte {
	from, priv := utiltx.NewAddrKey()
	signer := utiltx.NewSigner(priv)
	ethSigner := ethtypes.LatestSigner(suite.backend.ChainConfig())
	msg.From = from.Bytes()
	suite.Require().NoError(msg.Sign(ethSigner, signer))

	evmDenom := evmtypes.GetEVMCoinDenom()
	tx, err := msg.BuildTx(suite.backend.ClientCtx.TxConfig.NewTxBuilder(), evmDenom)
	suite.Require().NoError(err)
	txBz, err := suite.backend.ClientCtx.TxConfig.TxEncoder()(tx)
	suite.Require().NoError(err)
	return txBz
}

// buildAndEncodeMultiMsgEthTx builds a single EVM Cosmos tx containing multiple
// MsgEthereumTx messages. Each message is signed with a fresh ephemeral key so
// their hashes are unique even when underlying tx params are identical.
func (suite *BackendTestSuite) buildAndEncodeMultiMsgEthTx(msgs ...*evmtypes.MsgEthereumTx) []byte {
	ethSigner := ethtypes.LatestSigner(suite.backend.ChainConfig())
	for _, msg := range msgs {
		from, priv := utiltx.NewAddrKey()
		signer := utiltx.NewSigner(priv)
		msg.From = from.Bytes()
		suite.Require().NoError(msg.Sign(ethSigner, signer))
		msg.From = nil // BuildTx expects empty From for multi-msg txs
	}

	extBuilder, ok := suite.backend.ClientCtx.TxConfig.NewTxBuilder().(authtx.ExtensionOptionsTxBuilder)
	suite.Require().True(ok)

	option, err := codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
	suite.Require().NoError(err)
	extBuilder.SetExtensionOptions(option)

	sdkMsgs := make([]sdk.Msg, len(msgs))
	for i, msg := range msgs {
		sdkMsgs[i] = msg
	}
	suite.Require().NoError(extBuilder.SetMsgs(sdkMsgs...))

	bz, err := suite.backend.ClientCtx.TxConfig.TxEncoder()(extBuilder.GetTx())
	suite.Require().NoError(err)
	return bz
}

// resetIndexer creates a fresh KV indexer and installs it on the backend.
func (suite *BackendTestSuite) resetIndexer() {
	suite.backend.Indexer = indexer.NewKVIndexer(
		dbm.NewMemDB(),
		log.NewNopLogger(),
		suite.backend.ClientCtx,
	)
}

// mockClient returns the mock CometBFT client used by this backend.
func (suite *BackendTestSuite) mockClient() *mocks.Client {
	return suite.backend.ClientCtx.Client.(*mocks.Client)
}

// mockQueryClient returns the mock EVM gRPC query client used by this backend.
func (suite *BackendTestSuite) mockQueryClient() *mocks.EVMQueryClient {
	return suite.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
}
