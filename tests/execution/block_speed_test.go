package execution

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"fmt"
	"math/big"
	"math/rand"
	"os"
	"path/filepath"
	"testing"
	"time"

	abci "github.com/cometbft/cometbft/abci/types"
	cmtjson "github.com/cometbft/cometbft/libs/json"
	comettypes "github.com/cometbft/cometbft/types"
	dbm "github.com/cosmos/cosmos-db"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/evmd"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	crypto2 "github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/cosmos-sdk/baseapp"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	iavlx "github.com/cosmos/cosmos-sdk/iavl"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	sdk "github.com/cosmos/cosmos-sdk/types"
	moduletestutil "github.com/cosmos/cosmos-sdk/types/module/testutil"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"

	"cosmossdk.io/log"
)

var (
	ERC20PrecompileAddr = common.HexToAddress("0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE")
)

// BlockExecutionBenchConfig holds configuration for block execution benchmarks
type BlockExecutionBenchConfig struct {
	NumAccounts    int
	TxsPerBlock    int
	NumBlocks      int
	DBBackend      string
	IAVLXOptions   *iavlx.Options
	SendAmount     int64
	InitialBalance int64
}

//func TestMain(m *testing.M) {
//	telemetry.TestingMain(m, nil)
//}

// DefaultBlockExecutionBenchConfig returns a default configuration
func DefaultBlockExecutionBenchConfig() BlockExecutionBenchConfig {
	return BlockExecutionBenchConfig{
		NumAccounts:    65_000,
		TxsPerBlock:    5_000,
		NumBlocks:      150,
		DBBackend:      "memdb",
		SendAmount:     10000,
		InitialBalance: 1_000_000_000_000_000_000,
	}
}

type accountInfo struct {
	privKey  cryptotypes.PrivKey
	ecdsaKey *ecdsa.PrivateKey
	address  sdk.AccAddress
	accNum   uint64
	seqNum   uint64
}

// BenchmarkBlockExecution benchmarks block execution with pre-built transactions
func BenchmarkBlockExecution(b *testing.B) {
	config := getBenchConfigFromEnv()
	runBlockExecutionBenchmark(b, config)
}

// BenchmarkBlockExecutionMemDB runs the benchmark with memdb
func BenchmarkBlockExecutionLevelDB(b *testing.B) {
	config := DefaultBlockExecutionBenchConfig()
	config.DBBackend = string(dbm.GoLevelDBBackend)
	runBlockExecutionBenchmark(b, config)
}

// BenchmarkBlockExecutionIAVLX runs the benchmark with IAVLX
func BenchmarkBlockExecutionIAVLX(b *testing.B) {
	config := DefaultBlockExecutionBenchConfig()
	config.DBBackend = "iavlx"
	var iavlxOpts iavlx.Options
	iavlxOptsBz := []byte(`{"zero_copy":true,"evict_depth":20,"write_wal":true,"wal_sync_buffer":256, "fsync_interval":100,"compact_wal":true,"disable_compaction":false,"compaction_orphan_ratio":0.75,"compaction_orphan_age":10,"retain_versions":3,"min_compaction_seconds":60,"changeset_max_target":1073741824,"compaction_max_target":4294967295,"compact_after_versions":1000,"reader_update_interval":256}`)
	err := json.Unmarshal(iavlxOptsBz, &iavlxOpts)
	require.NoError(b, err)
	iavlxOpts.ReaderUpdateInterval = 1
	config.IAVLXOptions = &iavlxOpts
	runBlockExecutionBenchmark(b, config)
}

func runBlockExecutionBenchmark(b *testing.B, config BlockExecutionBenchConfig) {
	b.ReportAllocs()

	b.Logf("Benchmark Configuration:")
	b.Logf("  Accounts: %d", config.NumAccounts)
	b.Logf("  Txs/Block: %d", config.TxsPerBlock)
	b.Logf("  Num Blocks: %d", config.NumBlocks)
	b.Logf("  DB Backend: %s", config.DBBackend)
	b.Logf("  Total Txs: %d", config.TxsPerBlock*config.NumBlocks)

	// Setup database
	var db dbm.DB
	var err error
	dir := b.TempDir()

	if config.DBBackend == "iavlx" {
		db, err = dbm.NewDB("application", "goleveldb", dir)
		require.NoError(b, err)
	} else {
		db, err = dbm.NewDB("application", dbm.BackendType(config.DBBackend), dir)
		require.NoError(b, err)
	}
	defer db.Close()

	homeDir := filepath.Join(dir, "home")
	err = os.MkdirAll(homeDir, 0o755)
	require.NoError(b, err)

	// Create startup configuration
	startupConfig := simtestutil.DefaultStartUpConfig()
	startupConfig.DB = db
	startupConfig.AtGenesis = true // Stay at genesis, don't finalize block 1 yet
	chainID := big.NewInt(9001)    // TODO: probably update

	// Setup IAVLX if needed
	if config.DBBackend == "iavlx" {
		if config.IAVLXOptions == nil {
			config.IAVLXOptions = &iavlx.Options{
				WriteWAL:       false,
				CompactWAL:     false,
				EvictDepth:     255,
				RetainVersions: 1,
			}
		}

		startupConfig.BaseAppOption = func(app *baseapp.BaseApp) {
			iavlxDir := filepath.Join(homeDir, "data", "iavlx")
			iavlxDB, err := iavlx.LoadDB(iavlxDir, config.IAVLXOptions, log.NewNopLogger())
			if err != nil {
				b.Fatalf("Failed to load IAVLX DB: %v", err)
			}
			app.SetCMS(iavlxDB)
		}
	}

	// Create genesis accounts with funding
	fundingAmount := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, config.InitialBalance))
	genesisAccounts := make([]simtestutil.GenesisAccount, config.NumAccounts)
	accounts := make([]accountInfo, config.NumAccounts)

	for i := 0; i < config.NumAccounts; i++ {
		privKey, err := ethsecp256k1.GenerateKey()
		require.NoError(b, err)
		ecsdaKey, err := crypto2.ToECDSA(privKey.Key)
		require.NoError(b, err)
		addr := sdk.AccAddress(privKey.PubKey().Address())
		baseAcc := authtypes.NewBaseAccount(addr, privKey.PubKey(), uint64(i), 0)

		genesisAccounts[i] = simtestutil.GenesisAccount{
			GenesisAccount: baseAcc,
			Coins:          fundingAmount,
		}

		accounts[i] = accountInfo{
			privKey:  privKey,
			ecdsaKey: ecsdaKey,
			address:  addr,
			accNum:   uint64(i),
			seqNum:   0,
		}
		if i != 0 && i%5_000 == 0 {
			b.Logf("Built accounts %d", i)
		}
	}

	startupConfig.GenesisAccounts = genesisAccounts

	app, valSet := CreateApp(b, startupConfig, dir, chainID)
	require.NotNil(b, app)

	// Pre-build all transactions for all blocks
	b.Log("Pre-building transactions...")
	allBlockTxs := make([][][]byte, config.NumBlocks)
	txConfig := moduletestutil.MakeTestTxConfig()
	txEncoder := txConfig.TxEncoder()
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	for blockIdx := 0; blockIdx < config.NumBlocks; blockIdx++ {
		blockTxs := make([][]byte, config.TxsPerBlock)

		for txIdx := 0; txIdx < config.TxsPerBlock; txIdx++ {
			// Select sender and recipient (ensure they're different)
			senderIdx := r.Intn(config.NumAccounts)
			recipientIdx := (senderIdx + 1 + r.Intn(config.NumAccounts-1)) % config.NumAccounts

			sender := accounts[senderIdx]
			recipient := accounts[recipientIdx]

			// Create MsgSend
			ethTx := createMsgNativeERC20Transfer(
				b,
				config.SendAmount,
				ERC20PrecompileAddr,
				common.Address(sender.address.Bytes()),
				common.Address(recipient.address.Bytes()),
				sender.seqNum,
				func(address common.Address, transaction *types.Transaction) (*types.Transaction, error) {
					signer := types.NewLondonSigner(big.NewInt(262144))
					return types.SignTx(transaction, signer, sender.ecdsaKey)
				})
			msg := &evmtypes.MsgEthereumTx{}
			msg.FromEthereumTx(ethTx)
			msg.From = sender.address.Bytes()
			builder := app.TxConfig().NewTxBuilder()
			tx, err := msg.BuildTx(builder, sdk.DefaultBondDenom)
			require.NoError(b, err)

			// Encode transaction
			txBytes, err := txEncoder(tx)
			require.NoError(b, err)

			blockTxs[txIdx] = txBytes

			// Update sequence number for next transaction
			accounts[senderIdx].seqNum++
		}

		b.Logf("Block %d built", blockIdx)
		allBlockTxs[blockIdx] = blockTxs
	}

	b.Log("Finished pre-building transactions")

	// Reset timer to exclude setup time
	b.ResetTimer()

	// Execute blocks (start from height 1 after genesis)
	height := int64(1)
	for blockIdx := range config.NumBlocks {
		txs := allBlockTxs[blockIdx]
		_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{
			Height:          height,
			Txs:             txs,
			Time:            time.Now(),
			ProposerAddress: valSet.Proposer.Address,
		})
		if err != nil {
			b.Fatalf("FinalizeBlock failed at height %d: %v", height, err)
		}

		_, err = app.Commit()
		if err != nil {
			b.Fatalf("Commit failed at height %d: %v", height, err)
		}

		height++
	}

	b.StopTimer()

	// Report statistics
	totalTxs := config.TxsPerBlock * config.NumBlocks
	b.ReportMetric(float64(totalTxs)/b.Elapsed().Seconds(), "txs/sec")
	b.ReportMetric(float64(config.NumBlocks)/b.Elapsed().Seconds(), "blocks/sec")
	b.ReportMetric(float64(config.NumBlocks), "blocks_executed")
	b.ReportMetric(b.Elapsed().Seconds(), "execution_elapsed_time")
	b.ReportMetric(float64(totalTxs), "total_txs")
}

func getBenchConfigFromEnv() BlockExecutionBenchConfig {
	config := DefaultBlockExecutionBenchConfig()

	if val := os.Getenv("BENCH_NUM_ACCOUNTS"); val != "" {
		fmt.Sscanf(val, "%d", &config.NumAccounts)
	}

	if val := os.Getenv("BENCH_TXS_PER_BLOCK"); val != "" {
		fmt.Sscanf(val, "%d", &config.TxsPerBlock)
	}

	if val := os.Getenv("BENCH_NUM_BLOCKS"); val != "" {
		fmt.Sscanf(val, "%d", &config.NumBlocks)
	}

	if val := os.Getenv("BENCH_DB_BACKEND"); val != "" {
		config.DBBackend = val
	}

	// Check for IAVLX configuration
	if iavlxOpts := os.Getenv("IAVLX"); iavlxOpts != "" {
		config.DBBackend = "iavlx"
		var opts iavlx.Options
		err := json.Unmarshal([]byte(iavlxOpts), &opts)
		if err == nil {
			config.IAVLXOptions = &opts
		}
	}

	return config
}

func createMsgNativeERC20Transfer(t testing.TB, sendAmt int64, precompileAddress common.Address, fromAddr common.Address, recipientAddr common.Address, nonce uint64, signerFn bind.SignerFn) *types.Transaction {
	t.Helper()
	// random amount. weth calls amounts wad for some reason. we continue that trend here.
	wad := big.NewInt(int64(rand.Intn(int(sendAmt))))

	// we use the weth transactor even though were interacting with the native precompile since they share the same interface,
	// and the call data constructed here will be the same.
	wethInstance, err := NewWethTransactor(precompileAddress, nil)
	require.NoError(t, err)
	txOpts := &bind.TransactOpts{
		From:      fromAddr,
		Signer:    signerFn,
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: big.NewInt(25_000),
		GasFeeCap: big.NewInt(25_000),
		Context:   context.Background(),
		GasLimit:  250_000,
		NoSend:    true,
	}
	tx, err := wethInstance.Transfer(txOpts, recipientAddr, wad)
	require.NoError(t, err)
	return tx
}

func CreateApp(t testing.TB, startupConfig simtestutil.StartupConfig, dir string, chainID *big.Int, extraOutputs ...any) (*evmd.EVMD, *comettypes.ValidatorSet) {
	t.Helper()
	bopts := make([]func(*baseapp.BaseApp), 0)
	bopts = append(bopts, baseapp.SetChainID(chainID.String()))
	if startupConfig.BaseAppOption != nil {
		bopts = append(bopts, startupConfig.BaseAppOption)
	}
	app := evmd.NewExampleApp(log.NewNopLogger(), startupConfig.DB, nil, true, simtestutil.NewAppOptionsWithFlagHome(dir), bopts...)

	// create validator set
	valSet, err := startupConfig.ValidatorSet()
	require.NoError(t, err)

	var (
		balances    []banktypes.Balance
		genAccounts []authtypes.GenesisAccount
	)
	for _, ga := range startupConfig.GenesisAccounts {
		genAccounts = append(genAccounts, ga.GenesisAccount)
		balances = append(balances, banktypes.Balance{Address: ga.GenesisAccount.GetAddress().String(), Coins: ga.Coins})
	}

	genesisState, err := simtestutil.GenesisStateWithValSet(app.AppCodec(), app.DefaultGenesis(), valSet, genAccounts, balances...)
	require.NoError(t, err)

	cdc := app.AppCodec()
	bankGenesis := genesisState[banktypes.ModuleName]
	var bankGen banktypes.GenesisState
	require.NoError(t, cdc.UnmarshalJSON(bankGenesis, &bankGen))
	bankGen.DenomMetadata = append(bankGen.DenomMetadata, banktypes.Metadata{
		Description: "some stuff",
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    sdk.DefaultBondDenom,
				Exponent: 18,
				Aliases:  nil,
			},
		},
		Base:    sdk.DefaultBondDenom,
		Display: sdk.DefaultBondDenom,
		Name:    sdk.DefaultBondDenom,
		Symbol:  sdk.DefaultBondDenom,
	})
	bz, err := cdc.MarshalJSON(&bankGen)
	require.NoError(t, err)
	genesisState[banktypes.ModuleName] = bz

	erc20Genesis := genesisState[erc20types.ModuleName]
	var erc20Gen erc20types.GenesisState
	require.NoError(t, erc20Genesis.UnmarshalJSON(bz))
	erc20Gen.NativePrecompiles = append(erc20Gen.NativePrecompiles, "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE")
	erc20Gen.TokenPairs = append(erc20Gen.TokenPairs, erc20types.TokenPair{
		Erc20Address:  "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE",
		Denom:         sdk.DefaultBondDenom,
		Enabled:       true,
		ContractOwner: 1,
	})
	erc20Bz, err := cdc.MarshalJSON(&erc20Gen)
	require.NoError(t, err)
	genesisState[erc20types.ModuleName] = erc20Bz

	feeMarketGenesis := genesisState[feemarkettypes.ModuleName]
	var feeMarketGen feemarkettypes.GenesisState
	require.NoError(t, feeMarketGenesis.UnmarshalJSON(feeMarketGenesis))
	feeMarketGen.Params.NoBaseFee = true
	bz = cdc.MustMarshalJSON(&feeMarketGen)
	genesisState[feemarkettypes.ModuleName] = bz

	// init chain must be called to stop deliverState from being nil
	stateBytes, err := cmtjson.MarshalIndent(genesisState, "", " ")
	require.NoError(t, err)

	// init chain will set the validator set and initialize the genesis accounts
	_, err = app.InitChain(&abci.RequestInitChain{
		ChainId:         chainID.String(),
		Validators:      []abci.ValidatorUpdate{},
		ConsensusParams: simtestutil.DefaultConsensusParams,
		AppStateBytes:   stateBytes,
	})
	require.NoError(t, err)

	// commit genesis changes
	if !startupConfig.AtGenesis {
		_, err = app.FinalizeBlock(&abci.RequestFinalizeBlock{
			Height:             app.LastBlockHeight() + 1,
			NextValidatorsHash: valSet.Hash(),
		})
		require.NoError(t, err)
	}

	return app, valSet
}
