package mempool_test

import (
	"context"
	"crypto/ecdsa"
	"math/big"
	"strconv"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/encoding"
	evmmempool "github.com/cosmos/evm/mempool"
	"github.com/cosmos/evm/mempool/mocks"
	"github.com/cosmos/evm/mempool/txpool/legacypool"
	"github.com/cosmos/evm/testutil/constants"
	testutiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/vm/statedb"
	vmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log/v2"
	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	"github.com/cosmos/cosmos-sdk/client"
	cosmostx "github.com/cosmos/cosmos-sdk/client/tx"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/testutil"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdktxsigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

const (
	benchBondDenom     = "aatom"
	benchEVMPerAccount = 250
	benchBlockGasLimit = 30_000_000
	benchTxGas         = uint64(21_000)
	benchBalance       = uint64(1_000_000_000_000_000_000) // 1e18
)

// BenchmarkEVMMempoolIterator benchmarks the EVMMempoolIterator's SelectBy
// performance with mocked blockchain dependencies.
func BenchmarkEVMMempoolIterator(b *testing.B) {
	cases := []struct {
		name      string
		numEVM    int
		numCosmos int
	}{
		{"EVM_50/Cosmos_0", 50, 0},
		{"EVM_500/Cosmos_0", 500, 0},
		{"EVM_1000/Cosmos_0", 1000, 0},
		{"EVM_2500/Cosmos_0", 2500, 0},
		{"EVM_5000/Cosmos_0", 5000, 0},
		{"EVM_10000/Cosmos_0", 10000, 0},
		{"EVM_0/Cosmos_50", 0, 50},
		{"EVM_0/Cosmos_500", 0, 500},
		{"EVM_0/Cosmos_1000", 0, 1000},
		{"EVM_0/Cosmos_2500", 0, 2500},
		{"EVM_0/Cosmos_5000", 0, 5000},
		{"EVM_0/Cosmos_10000", 0, 10000},
		{"EVM_50/Cosmos_50", 50, 50},
		{"EVM_500/Cosmos_500", 500, 500},
		{"EVM_1000/Cosmos_1000", 1000, 1000},
		{"EVM_2500/Cosmos_2500", 2500, 2500},
		{"EVM_5000/Cosmos_5000", 5000, 5000},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			b.StopTimer()

			evmAccountCount := benchEVMAccountsNeeded(tc.numEVM)
			cosmosAccountCount := tc.numCosmos

			// Generate all EVM account keys
			evmAccounts := make([]benchAccount, evmAccountCount)
			for i := range evmAccountCount {
				key, err := crypto.GenerateKey()
				require.NoError(b, err)
				evmAccounts[i] = benchAccount{
					key:     key,
					address: crypto.PubkeyToAddress(key.PublicKey),
				}
			}

			// Generate all Cosmos account keys
			cosmosAccounts := make([]benchAccount, cosmosAccountCount)
			for i := range cosmosAccountCount {
				key, err := crypto.GenerateKey()
				require.NoError(b, err)
				cosmosAccounts[i] = benchAccount{
					key:     key,
					address: crypto.PubkeyToAddress(key.PublicKey),
				}
			}

			ctx, txConfig, mpool := setupBenchMempool(b, evmAccounts, cosmosAccounts)

			recipientAddr, _ := testutiltx.NewAddrKey()
			chainID := vmtypes.GetEthChainConfig().ChainID
			ethSigner := ethtypes.LatestSignerForChainID(chainID)
			gasPrice := big.NewInt(2_000_000_000)

			// Insert EVM transactions
			for i, acc := range evmAccounts {
				count := benchTxsForAccount(tc.numEVM, evmAccountCount, i)
				for nonce := range count {
					msgEthTx := vmtypes.NewTx(&vmtypes.EvmTxArgs{
						Nonce:    uint64(nonce), //#nosec G115
						To:       &recipientAddr,
						Amount:   big.NewInt(1000),
						GasLimit: benchTxGas,
						GasPrice: gasPrice,
						ChainID:  chainID,
					})
					msgEthTx.From = acc.address.Bytes()
					require.NoError(b, msgEthTx.Sign(ethSigner, testutiltx.NewSigner(benchPrivKeyToCosmos(b, acc.key))))

					tx, err := msgEthTx.BuildTx(txConfig.NewTxBuilder(), benchBondDenom)
					require.NoError(b, err)
					require.NoError(b, mpool.Insert(ctx, tx))
				}
			}

			// Insert Cosmos transactions
			for _, acc := range cosmosAccounts {
				cosmosPrivKey := benchPrivKeyToCosmos(b, acc.key)
				fromAddr := sdk.AccAddress(cosmosPrivKey.PubKey().Address().Bytes())
				toAddr := sdk.AccAddress(recipientAddr.Bytes())

				msg := banktypes.NewMsgSend(fromAddr, toAddr,
					sdk.NewCoins(sdk.NewInt64Coin(benchBondDenom, 1000)))

				tx, err := buildBenchCosmosTx(txConfig, cosmosPrivKey, msg, gasPrice)
				require.NoError(b, err)
				require.NoError(b, mpool.Insert(ctx, tx))
			}

			expectedCount := tc.numEVM + tc.numCosmos
			require.Equal(b, expectedCount, mpool.CountTx(), "mempool should contain all inserted transactions")

			selectCtx := ctx.WithBlockHeight(ctx.BlockHeight() + 1)

			b.Cleanup(func() {
				_ = mpool.Close()
			})

			b.ReportAllocs()
			b.StartTimer()

			for b.Loop() {
				mpool.SelectBy(selectCtx, nil, func(sdk.Tx) bool { return true })
			}
		})
	}
}

type benchAccount struct {
	key     *ecdsa.PrivateKey
	address common.Address
}

// benchPrivKeyToCosmos converts an ECDSA private key to a Cosmos SDK private key.
func benchPrivKeyToCosmos(b *testing.B, key *ecdsa.PrivateKey) cryptotypes.PrivKey {
	b.Helper()
	privBytes := crypto.FromECDSA(key)
	cosmosKey := &ethsecp256k1.PrivKey{Key: privBytes}
	return cosmosKey
}

// setupBenchMempool creates an ExperimentalEVMMempool with mocked state.
func setupBenchMempool(b *testing.B, evmAccounts, cosmosAccounts []benchAccount) (sdk.Context, client.TxConfig, *evmmempool.ExperimentalEVMMempool) {
	b.Helper()

	ethCfg := vmtypes.DefaultChainConfig(constants.EighteenDecimalsChainID)
	_ = vmtypes.SetChainConfig(ethCfg) // ignore if already set

	_ = vmtypes.NewEVMConfigurator().
		WithEVMCoinInfo(constants.ChainsCoinInfo[constants.EighteenDecimalsChainID]).
		Configure() // ignore if already configured

	mockVMKeeper := mocks.NewVMKeeper(b)
	mockFeeMarketKeeper := mocks.NewFeeMarketKeeper(b)

	mockVMKeeper.On("GetBaseFee", mock.Anything).Return(big.NewInt(1e9)).Maybe()
	mockVMKeeper.On("GetParams", mock.Anything).Return(vmtypes.DefaultParams()).Maybe()
	mockFeeMarketKeeper.On("GetBlockGasWanted", mock.Anything).Return(uint64(10_000_000)).Maybe()
	mockVMKeeper.On("GetEvmCoinInfo", mock.Anything).Return(constants.ChainsCoinInfo[constants.EighteenDecimalsChainID]).Maybe()

	// Register each account with proper balance
	for _, acc := range evmAccounts {
		mockVMKeeper.On("GetAccount", mock.Anything, acc.address).Return(&statedb.Account{
			Nonce:   0,
			Balance: uint256.NewInt(benchBalance),
		}).Maybe()
		mockVMKeeper.On("GetNonce", acc.address).Return(uint64(0)).Maybe()
		mockVMKeeper.On("GetBalance", acc.address).Return(uint256.NewInt(benchBalance)).Maybe()
		mockVMKeeper.On("GetCodeHash", acc.address).Return(common.Hash{}).Maybe()
	}
	for _, acc := range cosmosAccounts {
		mockVMKeeper.On("GetAccount", mock.Anything, acc.address).Return(&statedb.Account{
			Nonce:   0,
			Balance: uint256.NewInt(benchBalance),
		}).Maybe()
		mockVMKeeper.On("GetNonce", acc.address).Return(uint64(0)).Maybe()
		mockVMKeeper.On("GetBalance", acc.address).Return(uint256.NewInt(benchBalance)).Maybe()
		mockVMKeeper.On("GetCodeHash", acc.address).Return(common.Hash{}).Maybe()
	}

	mockVMKeeper.On("GetState", mock.Anything, mock.Anything).Return(common.Hash{}).Maybe()
	mockVMKeeper.On("GetCode", mock.Anything, mock.Anything).Return([]byte{}).Maybe()
	mockVMKeeper.On("ForEachStorage", mock.Anything, mock.Anything, mock.Anything).Maybe()
	mockVMKeeper.On("KVStoreKeys").Return(make(map[string]*storetypes.KVStoreKey)).Maybe()
	mockVMKeeper.On("SetEvmMempool", mock.Anything).Maybe()

	var latestHeight int64 = 1

	getCtxCallback := func(height int64, _ bool) (sdk.Context, error) {
		storeKey := storetypes.NewKVStoreKey("test")
		transientKey := storetypes.NewTransientStoreKey("transient_test")
		ctx := testutil.DefaultContext(storeKey, transientKey)
		if height == 0 {
			height = latestHeight
		}
		return ctx.
			WithBlockTime(time.Now()).
			WithBlockHeader(cmtproto.Header{AppHash: []byte("00000000000000000000000000000000")}).
			WithBlockHeight(height).
			WithChainID(strconv.Itoa(constants.EighteenDecimalsChainID)), nil
	}

	encodingConfig := encoding.MakeConfig(constants.EighteenDecimalsChainID)
	vmtypes.RegisterInterfaces(encodingConfig.InterfaceRegistry)
	txConfig := encodingConfig.TxConfig

	clientCtx := client.Context{}.
		WithCodec(encodingConfig.Codec).
		WithInterfaceRegistry(encodingConfig.InterfaceRegistry).
		WithTxConfig(txConfig)

	legacyConfig := legacypool.DefaultConfig
	legacyConfig.Journal = ""
	legacyConfig.PriceLimit = 1
	legacyConfig.PriceBump = 10
	legacyConfig.GlobalSlots = 500_000
	legacyConfig.AccountSlots = 500_000

	noopAnteHandler := func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) {
		return ctx, nil
	}

	config := &evmmempool.EVMMempoolConfig{
		LegacyPoolConfig: &legacyConfig,
		BlockGasLimit:    benchBlockGasLimit,
		MinTip:           uint256.NewInt(0),
		AnteHandler:      noopAnteHandler,
	}

	mpool := evmmempool.NewExperimentalEVMMempool(
		getCtxCallback,
		log.NewNopLogger(),
		mockVMKeeper,
		mockFeeMarketKeeper,
		txConfig,
		config,
		0, // cosmosPoolMaxTx (0 = unlimited)
	)
	mpool.SetClientCtx(clientCtx)
	require.NotNil(b, mpool)

	// Start EventBus and attach to mempool (required for tx pool reorg loop)
	eventBus := cmttypes.NewEventBus()
	require.NoError(b, eventBus.Start())
	mpool.SetEventBus(eventBus)

	b.Cleanup(func() {
		_ = eventBus.Stop()
	})

	ctx, err := getCtxCallback(latestHeight, false)
	require.NoError(b, err)

	return ctx, txConfig, mpool
}

// buildBenchCosmosTx creates a signed Cosmos SDK transaction without a running network.
func buildBenchCosmosTx(txConfig client.TxConfig, privKey cryptotypes.PrivKey, msg sdk.Msg, gasPrice *big.Int) (authsigning.Tx, error) {
	txBuilder := txConfig.NewTxBuilder()

	if err := txBuilder.SetMsgs(msg); err != nil {
		return nil, err
	}

	txBuilder.SetGasLimit(benchTxGas)
	feeAmount := new(big.Int).Mul(gasPrice, big.NewInt(int64(benchTxGas))) //#nosec G115
	txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(benchBondDenom, sdkmath.NewIntFromBigInt(feeAmount))))

	signMode, err := authsigning.APISignModeToInternal(txConfig.SignModeHandler().DefaultMode())
	if err != nil {
		return nil, err
	}

	emptySignature := sdktxsigning.SignatureV2{
		PubKey: privKey.PubKey(),
		Data: &sdktxsigning.SingleSignatureData{
			SignMode:  signMode,
			Signature: nil,
		},
		Sequence: 0,
	}
	if err := txBuilder.SetSignatures(emptySignature); err != nil {
		return nil, err
	}

	signerData := authsigning.SignerData{
		ChainID:       strconv.Itoa(constants.EighteenDecimalsChainID),
		AccountNumber: 0,
		Sequence:      0,
		Address:       sdk.AccAddress(privKey.PubKey().Address().Bytes()).String(),
		PubKey:        privKey.PubKey(),
	}

	signature, err := cosmostx.SignWithPrivKey(context.TODO(), signMode, signerData, txBuilder, privKey, txConfig, 0)
	if err != nil {
		return nil, err
	}

	if err := txBuilder.SetSignatures(signature); err != nil {
		return nil, err
	}

	return txBuilder.GetTx(), nil
}

func benchEVMAccountsNeeded(numTxs int) int {
	if numTxs == 0 {
		return 0
	}
	return (numTxs + benchEVMPerAccount - 1) / benchEVMPerAccount
}

func benchTxsForAccount(totalTxs, numAccounts, accountIndex int) int {
	perAccount := totalTxs / numAccounts
	remainder := totalTxs % numAccounts
	if accountIndex < remainder {
		return perAccount + 1
	}
	return perAccount
}
