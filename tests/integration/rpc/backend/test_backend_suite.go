package backend

import (
	"bufio"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/trie"
	"github.com/stretchr/testify/suite"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtrpctypes "github.com/cometbft/cometbft/rpc/core/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/crypto/hd"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/indexer"
	rpcbackend "github.com/cosmos/evm/rpc/backend"
	"github.com/cosmos/evm/rpc/backend/mocks"
	rpctypes "github.com/cosmos/evm/rpc/types"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/server"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type TestSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption
	backend *rpcbackend.Backend
	from    common.Address
	acc     sdk.AccAddress
	signer  keyring.Signer
}

func NewTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *TestSuite {
	return &TestSuite{
		create:  create,
		options: options,
	}
}

var ChainID = constants.ExampleChainID

// SetupTest is executed before every TestSuite test
func (s *TestSuite) SetupTest() {
	ctx := server.NewDefaultContext()
	ctx.Viper.Set("telemetry.global-labels", []interface{}{})
	ctx.Viper.Set("evm.evm-chain-id", ChainID.EVMChainID)

	baseDir := s.T().TempDir()
	nodeDirName := "node"
	clientDir := filepath.Join(baseDir, nodeDirName, "evmoscli")
	keyRing, err := s.generateTestKeyring(clientDir)
	if err != nil {
		panic(err)
	}

	// Create Account with set sequence
	s.acc = sdk.AccAddress(utiltx.GenerateAddress().Bytes())
	accounts := map[string]client.TestAccount{}
	accounts[s.acc.String()] = client.TestAccount{
		Address: s.acc,
		Num:     uint64(1),
		Seq:     uint64(1),
	}

	from, priv := utiltx.NewAddrKey()
	s.from = from
	s.signer = utiltx.NewSigner(priv)
	s.Require().NoError(err)

	nw := network.New(s.create, s.options...)
	encodingConfig := nw.GetEncodingConfig()
	clientCtx := client.Context{}.WithChainID(ChainID.ChainID).
		WithHeight(1).
		WithTxConfig(encodingConfig.TxConfig).
		WithCodec(encodingConfig.Codec).
		WithKeyringDir(clientDir).
		WithKeyring(keyRing).
		WithAccountRetriever(client.TestAccountRetriever{Accounts: accounts}).
		WithClient(mocks.NewClient(s.T()))

	allowUnprotectedTxs := false
	idxer := indexer.NewKVIndexer(dbm.NewMemDB(), ctx.Logger, clientCtx)

	s.backend = rpcbackend.NewBackend(ctx, ctx.Logger, clientCtx, allowUnprotectedTxs, idxer, nil)
	s.backend.Cfg.JSONRPC.GasCap = 0
	s.backend.Cfg.JSONRPC.EVMTimeout = 0
	s.backend.Cfg.JSONRPC.AllowInsecureUnlock = true
	s.backend.Cfg.EVM.EVMChainID = ChainID.EVMChainID
	s.backend.QueryClient.QueryClient = mocks.NewEVMQueryClient(s.T())
	s.backend.QueryClient.FeeMarket = mocks.NewFeeMarketQueryClient(s.T())
	s.backend.Ctx = rpctypes.ContextWithHeight(1)

	// Add codec
	s.backend.ClientCtx.Codec = encodingConfig.Codec
}

// buildEthereumTx returns an example legacy Ethereum transaction
func (s *TestSuite) buildEthereumTx() (*evmtypes.MsgEthereumTx, []byte) {
	ethTxParams := evmtypes.EvmTxArgs{
		ChainID:  s.backend.EvmChainID,
		Nonce:    uint64(0),
		To:       &common.Address{},
		Amount:   big.NewInt(0),
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
	}
	msgEthereumTx := evmtypes.NewTx(&ethTxParams)

	// A valid msg should have empty `From`
	msgEthereumTx.From = s.from.Bytes()

	txBuilder := s.backend.ClientCtx.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgEthereumTx)
	s.Require().NoError(err)

	bz, err := s.backend.ClientCtx.TxConfig.TxEncoder()(txBuilder.GetTx())
	s.Require().NoError(err)

	// decode again to get canonical representation
	tx, err := s.backend.ClientCtx.TxConfig.TxDecoder()(bz)
	s.Require().NoError(err)

	msgs := tx.GetMsgs()
	s.Require().NotEmpty(msgs)
	return msgs[0].(*evmtypes.MsgEthereumTx), bz
}

// buildEthereumTx returns an example legacy Ethereum transaction
func (s *TestSuite) buildEthereumTxWithChainID(eip155ChainID *big.Int) *evmtypes.MsgEthereumTx {
	ethTxParams := evmtypes.EvmTxArgs{
		ChainID:  eip155ChainID,
		Nonce:    uint64(0),
		To:       &common.Address{},
		Amount:   big.NewInt(0),
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
	}
	msgEthereumTx := evmtypes.NewTx(&ethTxParams)

	// A valid msg should have empty `From`
	msgEthereumTx.From = s.from.Bytes()

	txBuilder := s.backend.ClientCtx.TxConfig.NewTxBuilder()
	err := txBuilder.SetMsgs(msgEthereumTx)
	s.Require().NoError(err)

	return msgEthereumTx
}

// buildFormattedBlock returns a formatted block for testing
func (s *TestSuite) buildFormattedBlock(
	blockRes *cmtrpctypes.ResultBlockResults,
	resBlock *cmtrpctypes.ResultBlock,
	fullTx bool,
	tx *evmtypes.MsgEthereumTx,
	validator sdk.AccAddress,
	baseFee *big.Int,
) map[string]interface{} {
	var msgs []*evmtypes.MsgEthereumTx
	if tx != nil {
		msgs = []*evmtypes.MsgEthereumTx{tx}
	}
	ethBlock := s.buildEthBlock(blockRes, resBlock, msgs, validator, baseFee)
	res, err := rpctypes.RPCMarshalBlock(ethBlock, resBlock, msgs, true, fullTx, s.backend.ChainConfig())
	s.Require().NoError(err)

	return res
}

func (s *TestSuite) buildEthBlock(
	blockRes *cmtrpctypes.ResultBlockResults,
	resBlock *cmtrpctypes.ResultBlock,
	msgs []*evmtypes.MsgEthereumTx,
	validator sdk.AccAddress,
	baseFee *big.Int,
) *ethtypes.Block {
	// Replay core steps of EthBlockFromCometBlock using known inputs
	cmtHeader := resBlock.Block.Header

	// 1) Gas limit from consensus params
	// if failed to query consensus params, default gasLimit is applied.
	gasLimit, _ := rpctypes.BlockMaxGasFromConsensusParams(rpctypes.ContextWithHeight(cmtHeader.Height), s.backend.ClientCtx, cmtHeader.Height)

	// 2) Miner from provided validator
	miner := common.BytesToAddress(validator.Bytes())

	// 3) Build ethereum header
	ethHeader := rpctypes.MakeHeader(cmtHeader, gasLimit, miner, baseFee)

	// 4) Prepare msgs and txs
	txs := make([]*ethtypes.Transaction, len(msgs))
	for i, m := range msgs {
		txs[i] = m.AsTransaction()
	}

	// 5) Build receipts
	receipts, err := s.backend.ReceiptsFromCometBlock(resBlock, blockRes, msgs)
	s.Require().NoError(err)

	// 6) Gas used
	var gasUsed uint64
	for _, r := range blockRes.TxsResults {
		if shouldIgnoreGasUsed(r) {
			break
		}
		gas := r.GetGasUsed()
		if gas < 0 {
			s.T().Errorf("negative gas used value: %d", gas)
			continue
		}
		gasUsed += uint64(gas)
	}
	ethHeader.GasUsed = gasUsed

	// 7) Construct eth block and marshal
	body := &ethtypes.Body{Transactions: txs, Uncles: []*ethtypes.Header{}, Withdrawals: []*ethtypes.Withdrawal{}}
	return ethtypes.NewBlock(ethHeader, body, receipts, trie.NewStackTrie(nil))
}

func shouldIgnoreGasUsed(res *abci.ExecTxResult) bool {
	return res.GetCode() == 11 && strings.Contains(res.GetLog(), "no block gas left to run tx: out of gas")
}

func (s *TestSuite) generateTestKeyring(clientDir string) (keyring.Keyring, error) {
	buf := bufio.NewReader(os.Stdin)
	encCfg := encoding.MakeConfig(ChainID.EVMChainID)
	return keyring.New(sdk.KeyringServiceName(), keyring.BackendTest, clientDir, buf, encCfg.Codec, []keyring.Option{hd.EthSecp256k1Option()}...)
}

func (s *TestSuite) signAndEncodeEthTx(msgEthereumTx *evmtypes.MsgEthereumTx) []byte {
	from, priv := utiltx.NewAddrKey()
	signer := utiltx.NewSigner(priv)

	ethSigner := ethtypes.LatestSigner(s.backend.ChainConfig())
	msgEthereumTx.From = from.Bytes()
	err := msgEthereumTx.Sign(ethSigner, signer)
	s.Require().NoError(err)

	evmDenom := evmtypes.GetEVMCoinDenom()
	tx, err := msgEthereumTx.BuildTx(s.backend.ClientCtx.TxConfig.NewTxBuilder(), evmDenom)
	s.Require().NoError(err)

	txEncoder := s.backend.ClientCtx.TxConfig.TxEncoder()
	txBz, err := txEncoder(tx)
	s.Require().NoError(err)

	return txBz
}
