package integration

import (
	"bytes"
	"math/big"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testtx "github.com/cosmos/evm/testutil/tx"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/cosmos-sdk/client"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
	sdkmath "cosmossdk.io/math"
	"github.com/stretchr/testify/require"
)

const (
	defaultStressNodes       = 17
	defaultStressBlocks      = 600
	defaultStressAccountPool = 64
	defaultStressTxsPerBlock = 96
	defaultStressGasLimit    = 100_000
	contractStressGasLimit   = 300_000
)

type payloadKind string

const (
	payloadDirectTransfer     payloadKind = "direct_transfer"
	payloadContractNativeSend payloadKind = "contract_native_send"
	payloadContractCreateSend payloadKind = "contract_create_send"
	payloadMixed              payloadKind = "mixed"
)

func TestBlockSTMEVMGasTokenTransfersStress(t *testing.T) {
	if !shouldRunPayload(payloadDirectTransfer) {
		t.Skipf("skipping payload %q", payloadDirectTransfer)
	}
	if testing.Short() {
		t.Skip("skipping long blockstm stress test")
	}

	nodeCount := envInt("EVM_BLOCKSTM_STRESS_NODES", defaultStressNodes)
	blockCount := envInt("EVM_BLOCKSTM_STRESS_BLOCKS", defaultStressBlocks)
	accountCount := envInt("EVM_BLOCKSTM_STRESS_ACCOUNTS", defaultStressAccountPool)
	txsPerBlock := envInt("EVM_BLOCKSTM_STRESS_TXS_PER_BLOCK", defaultStressTxsPerBlock)

	require.GreaterOrEqual(t, accountCount, 16, "need enough accounts to create hot and cold contention sets")
	require.GreaterOrEqual(t, txsPerBlock, 8, "need enough transactions per block to create contention")

	accounts := make([]sdk.AccAddress, 0, accountCount)
	privs := make([]*ethsecp256k1.PrivKey, 0, accountCount)
	nonceBySender := make(map[string]uint64, accountCount)

	for i := 0; i < accountCount; i++ {
		addr, priv := testtx.NewAccAddressAndKey()
		accounts = append(accounts, addr)
		privs = append(privs, priv)
		nonceBySender[addr.String()] = 0
	}

	reference := newStressNetwork(t, true, accounts...)
	nodes := make([]*network.UnitTestNetwork, 0, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes = append(nodes, newStressNetwork(t, false, accounts...))
	}

	for block := 0; block < blockCount; block++ {
		txBytes := buildStressBlock(t, reference.App.GetTxConfig(), privs, accounts, nonceBySender, txsPerBlock)

		_, err := reference.NextBlockWithTxs(txBytes...)
		require.NoError(t, err)
		referenceHash := reference.App.GetBaseApp().LastCommitID().Hash

		hashes := finalizeNodesConcurrently(t, nodes, txBytes)
		for nodeIdx, hash := range hashes {
			if !bytes.Equal(referenceHash, hash) {
				require.Equalf(
					t,
					captureBalances(reference, accounts),
					captureBalances(nodes[nodeIdx], accounts),
					"block %d node %d diverged after deterministic EVM gas-token sends",
					block+1,
					nodeIdx,
				)
			}
		}

		if (block+1)%50 == 0 {
			t.Logf("executed %d/%d blocks across %d blockstm nodes", block+1, blockCount, nodeCount)
		}
	}
}

func TestBlockSTMContractNativeSendStress(t *testing.T) {
	if !shouldRunPayload(payloadContractNativeSend) {
		t.Skipf("skipping payload %q", payloadContractNativeSend)
	}
	if testing.Short() {
		t.Skip("skipping long blockstm stress test")
	}

	nodeCount := envInt("EVM_BLOCKSTM_CONTRACT_STRESS_NODES", defaultStressNodes)
	blockCount := envInt("EVM_BLOCKSTM_CONTRACT_STRESS_BLOCKS", 400)
	accountCount := envInt("EVM_BLOCKSTM_CONTRACT_STRESS_ACCOUNTS", 40)
	txsPerBlock := envInt("EVM_BLOCKSTM_CONTRACT_STRESS_TXS_PER_BLOCK", 64)

	require.GreaterOrEqual(t, accountCount, 8, "need at least 8 accounts")
	require.GreaterOrEqual(t, txsPerBlock, 4, "need at least 4 txs per block")

	accounts := make([]sdk.AccAddress, 0, accountCount)
	privs := make([]*ethsecp256k1.PrivKey, 0, accountCount)
	nonceBySender := make(map[string]uint64, accountCount)

	for i := 0; i < accountCount; i++ {
		addr, priv := testtx.NewAccAddressAndKey()
		accounts = append(accounts, addr)
		privs = append(privs, priv)
		nonceBySender[addr.String()] = 0
	}

	reference := newStressNetwork(t, true, accounts...)
	nodes := make([]*network.UnitTestNetwork, 0, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes = append(nodes, newStressNetwork(t, false, accounts...))
	}

	contract, err := contracts.LoadSequentialOperationsTester()
	require.NoError(t, err)

	deployerPriv := privs[0]
	deployTx := buildContractDeployTx(t, reference.App.GetTxConfig(), deployerPriv, contract)
	_, err = reference.NextBlockWithTxs(deployTx)
	require.NoError(t, err)
	for _, node := range nodes {
		_, err := node.NextBlockWithTxs(deployTx)
		require.NoError(t, err)
	}
	nonceBySender[accounts[0].String()]++

	contractAddr := crypto.CreateAddress(common.BytesToAddress(deployerPriv.PubKey().Address().Bytes()), 0)
	contractSDKAddr := sdk.AccAddress(contractAddr.Bytes())
	fundAmount := sdk.NewCoins(sdk.NewCoin(reference.GetBaseDenom(), sdkmath.NewInt(int64(blockCount*txsPerBlock*2))))

	fundContractBalance(t, reference, contractSDKAddr, fundAmount)
	for _, node := range nodes {
		fundContractBalance(t, node, contractSDKAddr, fundAmount)
	}

	recipient := accounts[1]
	callData, err := contract.ABI.Pack("testNativeTransfer", common.BytesToAddress(recipient.Bytes()), big.NewInt(1))
	require.NoError(t, err)

	for block := 0; block < blockCount; block++ {
		txBytes := buildContractCallBlock(t, reference.App.GetTxConfig(), privs[2:], contractAddr, callData, nonceBySender, txsPerBlock)

		_, err := reference.NextBlockWithTxs(txBytes...)
		require.NoError(t, err)
		referenceHash := reference.App.GetBaseApp().LastCommitID().Hash

		hashes := finalizeNodesConcurrently(t, nodes, txBytes)
		for nodeIdx, hash := range hashes {
			if !bytes.Equal(referenceHash, hash) {
				require.Equalf(
					t,
					captureBalances(reference, append(accounts, contractSDKAddr)),
					captureBalances(nodes[nodeIdx], append(accounts, contractSDKAddr)),
					"contract native-send stress diverged at block %d node %d",
					block+1,
					nodeIdx,
				)
			}
		}
	}
}

func TestBlockSTMContractCreateSendStress(t *testing.T) {
	if !shouldRunPayload(payloadContractCreateSend) {
		t.Skipf("skipping payload %q", payloadContractCreateSend)
	}
	if testing.Short() {
		t.Skip("skipping long blockstm stress test")
	}

	nodeCount := envInt("EVM_BLOCKSTM_CREATE_STRESS_NODES", defaultStressNodes)
	blockCount := envInt("EVM_BLOCKSTM_CREATE_STRESS_BLOCKS", 300)
	accountCount := envInt("EVM_BLOCKSTM_CREATE_STRESS_ACCOUNTS", 24)
	txsPerBlock := envInt("EVM_BLOCKSTM_CREATE_STRESS_TXS_PER_BLOCK", 32)

	require.GreaterOrEqual(t, accountCount, 8, "need at least 8 accounts")
	require.GreaterOrEqual(t, txsPerBlock, 4, "need at least 4 txs per block")

	accounts := make([]sdk.AccAddress, 0, accountCount)
	privs := make([]*ethsecp256k1.PrivKey, 0, accountCount)
	nonceBySender := make(map[string]uint64, accountCount)

	for i := 0; i < accountCount; i++ {
		addr, priv := testtx.NewAccAddressAndKey()
		accounts = append(accounts, addr)
		privs = append(privs, priv)
		nonceBySender[addr.String()] = 0
	}

	reference := newStressNetwork(t, true, accounts...)
	nodes := make([]*network.UnitTestNetwork, 0, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes = append(nodes, newStressNetwork(t, false, accounts...))
	}

	contract, err := contracts.LoadContractCreationTester()
	require.NoError(t, err)

	deployerPriv := privs[0]
	deployTx := buildContractDeployTx(t, reference.App.GetTxConfig(), deployerPriv, contract)
	_, err = reference.NextBlockWithTxs(deployTx)
	require.NoError(t, err)
	for _, node := range nodes {
		_, err := node.NextBlockWithTxs(deployTx)
		require.NoError(t, err)
	}
	nonceBySender[accounts[0].String()]++

	contractAddr := crypto.CreateAddress(common.BytesToAddress(deployerPriv.PubKey().Address().Bytes()), 0)
	contractSDKAddr := sdk.AccAddress(contractAddr.Bytes())
	fundAmount := sdk.NewCoins(sdk.NewCoin(reference.GetBaseDenom(), sdkmath.NewInt(int64(blockCount*txsPerBlock*4))))

	fundContractBalance(t, reference, contractSDKAddr, fundAmount)
	for _, node := range nodes {
		fundContractBalance(t, node, contractSDKAddr, fundAmount)
	}

	validatorAddr := firstValidatorAddress(t, reference)
	callData, err := contract.ABI.Pack(
		"scenario7_createDelegateRevertSend",
		big.NewInt(1),
		validatorAddr,
		new(big.Int).Mul(big.NewInt(1_000_000_000_000_000_000), big.NewInt(1_000_000)),
		big.NewInt(1),
	)
	require.NoError(t, err)

	for block := 0; block < blockCount; block++ {
		txBytes := buildContractCallBlock(t, reference.App.GetTxConfig(), privs[2:], contractAddr, callData, nonceBySender, txsPerBlock)

		_, err := reference.NextBlockWithTxs(txBytes...)
		require.NoError(t, err)
		referenceHash := reference.App.GetBaseApp().LastCommitID().Hash

		hashes := finalizeNodesConcurrently(t, nodes, txBytes)
		for nodeIdx, hash := range hashes {
			if !bytes.Equal(referenceHash, hash) {
				require.Equalf(
					t,
					captureBalances(reference, append(accounts, contractSDKAddr)),
					captureBalances(nodes[nodeIdx], append(accounts, contractSDKAddr)),
					"contract create-send stress diverged at block %d node %d",
					block+1,
					nodeIdx,
				)
			}
		}
	}
}

func TestBlockSTMMixedPayloadStress(t *testing.T) {
	if !shouldRunPayload(payloadMixed) {
		t.Skipf("skipping payload %q", payloadMixed)
	}
	if testing.Short() {
		t.Skip("skipping long blockstm stress test")
	}

	nodeCount := envInt("EVM_BLOCKSTM_MIXED_STRESS_NODES", defaultStressNodes)
	blockCount := envInt("EVM_BLOCKSTM_MIXED_STRESS_BLOCKS", 400)
	accountCount := envInt("EVM_BLOCKSTM_MIXED_STRESS_ACCOUNTS", 48)
	txsPerBlock := envInt("EVM_BLOCKSTM_MIXED_STRESS_TXS_PER_BLOCK", 96)

	require.GreaterOrEqual(t, accountCount, 16, "need at least 16 accounts")
	require.GreaterOrEqual(t, txsPerBlock, 12, "need at least 12 txs per block")

	accounts := make([]sdk.AccAddress, 0, accountCount)
	privs := make([]*ethsecp256k1.PrivKey, 0, accountCount)
	nonceBySender := make(map[string]uint64, accountCount)

	for i := 0; i < accountCount; i++ {
		addr, priv := testtx.NewAccAddressAndKey()
		accounts = append(accounts, addr)
		privs = append(privs, priv)
		nonceBySender[addr.String()] = 0
	}

	reference := newStressNetwork(t, true, accounts...)
	nodes := make([]*network.UnitTestNetwork, 0, nodeCount)
	for i := 0; i < nodeCount; i++ {
		nodes = append(nodes, newStressNetwork(t, false, accounts...))
	}

	seqContract, err := contracts.LoadSequentialOperationsTester()
	require.NoError(t, err)
	createContract, err := contracts.LoadContractCreationTester()
	require.NoError(t, err)

	deployerPriv := privs[0]
	seqDeployTx := buildContractDeployTx(t, reference.App.GetTxConfig(), deployerPriv, seqContract)
	createDeployTx := buildContractDeployTxWithNonce(t, reference.App.GetTxConfig(), deployerPriv, createContract, 1)

	for _, txBytes := range [][]byte{seqDeployTx, createDeployTx} {
		_, err := reference.NextBlockWithTxs(txBytes)
		require.NoError(t, err)
		for _, node := range nodes {
			_, err := node.NextBlockWithTxs(txBytes)
			require.NoError(t, err)
		}
	}
	nonceBySender[accounts[0].String()] += 2

	deployerAddr := common.BytesToAddress(deployerPriv.PubKey().Address().Bytes())
	seqContractAddr := crypto.CreateAddress(deployerAddr, 0)
	createContractAddr := crypto.CreateAddress(deployerAddr, 1)
	seqContractSDKAddr := sdk.AccAddress(seqContractAddr.Bytes())
	createContractSDKAddr := sdk.AccAddress(createContractAddr.Bytes())

	fundAmount := sdk.NewCoins(sdk.NewCoin(reference.GetBaseDenom(), sdkmath.NewInt(int64(blockCount*txsPerBlock*4))))
	fundContractBalance(t, reference, seqContractSDKAddr, fundAmount)
	fundContractBalance(t, reference, createContractSDKAddr, fundAmount)
	for _, node := range nodes {
		fundContractBalance(t, node, seqContractSDKAddr, fundAmount)
		fundContractBalance(t, node, createContractSDKAddr, fundAmount)
	}

	validatorAddr := firstValidatorAddress(t, reference)
	nativeCallData, err := seqContract.ABI.Pack("testNativeTransfer", common.BytesToAddress(accounts[1].Bytes()), big.NewInt(1))
	require.NoError(t, err)
	createCallData, err := createContract.ABI.Pack(
		"scenario7_createDelegateRevertSend",
		big.NewInt(1),
		validatorAddr,
		new(big.Int).Mul(big.NewInt(1_000_000_000_000_000_000), big.NewInt(1_000_000)),
		big.NewInt(1),
	)
	require.NoError(t, err)

	for block := 0; block < blockCount; block++ {
		txBytes := buildMixedBlock(
			t,
			reference.App.GetTxConfig(),
			privs,
			accounts,
			nonceBySender,
			txsPerBlock,
			seqContractAddr,
			nativeCallData,
			createContractAddr,
			createCallData,
		)

		_, err := reference.NextBlockWithTxs(txBytes...)
		require.NoError(t, err)
		referenceHash := reference.App.GetBaseApp().LastCommitID().Hash

		hashes := finalizeNodesConcurrently(t, nodes, txBytes)
		for nodeIdx, hash := range hashes {
			if !bytes.Equal(referenceHash, hash) {
				require.Equalf(
					t,
					captureBalances(reference, append(accounts, seqContractSDKAddr, createContractSDKAddr)),
					captureBalances(nodes[nodeIdx], append(accounts, seqContractSDKAddr, createContractSDKAddr)),
					"mixed payload stress diverged at block %d node %d",
					block+1,
					nodeIdx,
				)
			}
		}
	}
}

func newStressNetwork(t *testing.T, disableBlockSTM bool, accounts ...sdk.AccAddress) *network.UnitTestNetwork {
	t.Helper()

	customGenesis := network.CustomGenesisState{}
	feeMarketGenesis := feemarkettypes.DefaultGenesisState()
	feeMarketGenesis.Params.NoBaseFee = true
	customGenesis[feemarkettypes.ModuleName] = feeMarketGenesis
	validatorCount := envInt("EVM_BLOCKSTM_VALIDATORS", 1)

	opts := []network.ConfigOption{
		network.WithChainID(testconstants.ExampleChainID),
		network.WithAmountOfValidators(validatorCount),
		network.WithPreFundedAccounts(accounts...),
		network.WithCustomGenesis(customGenesis),
	}

	nw := network.NewUnitTestNetwork(CreateEvmd, opts...)
	if disableBlockSTM {
		nw.App.GetBaseApp().SetBlockSTMTxRunner(nil)
	}

	return nw
}

func buildStressBlock(
	t *testing.T,
	txConfig client.TxConfig,
	privs []*ethsecp256k1.PrivKey,
	accounts []sdk.AccAddress,
	nonceBySender map[string]uint64,
	txsPerBlock int,
) [][]byte {
	t.Helper()

	specs := make([]transferSpec, 0, txsPerBlock)
	hotSenders := privs[:4]
	hotRecipients := accounts[4:8]
	coldSenderPrivs := privs[8:]
	coldRecipients := accounts[8:]

	for i := 0; i < txsPerBlock; i++ {
		switch i % 3 {
		case 0:
			// Fan-in: many cold senders repeatedly credit the same hot recipient.
			sender := coldSenderPrivs[i%len(coldSenderPrivs)]
			recipient := hotRecipients[(i/3)%len(hotRecipients)]
			specs = append(specs, transferSpec{priv: sender, recipient: recipient, amount: big.NewInt(1)})
		case 1:
			// Fan-out: one hot sender repeatedly pays many cold recipients.
			sender := hotSenders[(i/3)%len(hotSenders)]
			recipient := coldRecipients[i%len(coldRecipients)]
			specs = append(specs, transferSpec{priv: sender, recipient: recipient, amount: big.NewInt(1)})
		default:
			// Hot-to-hot churn: repeatedly update the same small account set.
			sender := hotSenders[(i/3)%len(hotSenders)]
			recipient := hotRecipients[(i/2)%len(hotRecipients)]
			if common.BytesToAddress(sender.PubKey().Address().Bytes()) == common.BytesToAddress(recipient.Bytes()) {
				recipient = hotRecipients[(i/2+1)%len(hotRecipients)]
			}
			specs = append(specs, transferSpec{priv: sender, recipient: recipient, amount: big.NewInt(1)})
		}
	}

	return encodeTransferSpecs(t, txConfig, specs, nonceBySender)
}

type transferSpec struct {
	priv      *ethsecp256k1.PrivKey
	recipient sdk.AccAddress
	amount    *big.Int
}

func encodeTransferSpecs(t *testing.T, txConfig client.TxConfig, specs []transferSpec, nonceBySender map[string]uint64) [][]byte {
	t.Helper()

	signer := ethtypes.LatestSignerForChainID(evmtypes.GetEthChainConfig().ChainID)
	txBytes := make([][]byte, 0, len(specs))

	for _, spec := range specs {
		from := common.BytesToAddress(spec.priv.PubKey().Address().Bytes())
		to := common.BytesToAddress(spec.recipient.Bytes())
		nonce := nonceBySender[sdk.AccAddress(from.Bytes()).String()]

		msg := evmtypes.NewTx(&evmtypes.EvmTxArgs{
			ChainID:  evmtypes.GetEthChainConfig().ChainID,
			Nonce:    nonce,
			To:       &to,
			Amount:   spec.amount,
			GasLimit: defaultStressGasLimit,
			GasPrice: big.NewInt(1),
			Accesses: &ethtypes.AccessList{},
		})
		msg.From = from.Bytes()
		require.NoError(t, msg.Sign(signer, testtx.NewSigner(spec.priv)))

		tx, err := testtx.PrepareEthTx(txConfig, nil, msg)
		require.NoError(t, err)

		bz, err := txConfig.TxEncoder()(tx)
		require.NoError(t, err)
		txBytes = append(txBytes, bz)
		nonceBySender[sdk.AccAddress(from.Bytes()).String()] = nonce + 1
	}

	return txBytes
}

func captureBalances(nw *network.UnitTestNetwork, accounts []sdk.AccAddress) map[string]string {
	denom := evmtypes.GetEVMCoinExtendedDenom()
	balances := make(map[string]string, len(accounts))
	ctx := nw.GetContext()

	for _, addr := range accounts {
		coin := nw.App.GetBankKeeper().GetBalance(ctx, addr, denom)
		balances[addr.String()] = coin.String()
	}

	return balances
}

func buildContractDeployTx(t *testing.T, txConfig client.TxConfig, priv *ethsecp256k1.PrivKey, contract evmtypes.CompiledContract) []byte {
	return buildContractDeployTxWithNonce(t, txConfig, priv, contract, 0)
}

func buildContractDeployTxWithNonce(t *testing.T, txConfig client.TxConfig, priv *ethsecp256k1.PrivKey, contract evmtypes.CompiledContract, nonce uint64) []byte {
	t.Helper()

	from := common.BytesToAddress(priv.PubKey().Address().Bytes())
	signer := ethtypes.LatestSignerForChainID(evmtypes.GetEthChainConfig().ChainID)

	msg := evmtypes.NewTx(&evmtypes.EvmTxArgs{
		ChainID:  evmtypes.GetEthChainConfig().ChainID,
		Nonce:    nonce,
		GasLimit: 1_500_000,
		GasPrice: big.NewInt(1),
		Input:    contract.Bin,
		Accesses: &ethtypes.AccessList{},
	})
	msg.From = from.Bytes()
	require.NoError(t, msg.Sign(signer, testtx.NewSigner(priv)))

	tx, err := testtx.PrepareEthTx(txConfig, nil, msg)
	require.NoError(t, err)
	bz, err := txConfig.TxEncoder()(tx)
	require.NoError(t, err)
	return bz
}

func fundContractBalance(t *testing.T, nw *network.UnitTestNetwork, contract sdk.AccAddress, amount sdk.Coins) {
	t.Helper()

	ctx := nw.GetContext()
	require.NoError(t, nw.App.GetBankKeeper().MintCoins(ctx, minttypes.ModuleName, amount))
	require.NoError(t, nw.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, contract, amount))
	require.NoError(t, nw.NextBlock())
}

func firstValidatorAddress(t *testing.T, nw *network.UnitTestNetwork) string {
	t.Helper()

	ctx := nw.GetContext()
	vals, err := nw.App.GetStakingKeeper().GetAllValidators(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, vals)
	return vals[0].OperatorAddress
}

func buildContractCallBlock(
	t *testing.T,
	txConfig client.TxConfig,
	privs []*ethsecp256k1.PrivKey,
	contract common.Address,
	callData []byte,
	nonceBySender map[string]uint64,
	txsPerBlock int,
) [][]byte {
	t.Helper()

	signer := ethtypes.LatestSignerForChainID(evmtypes.GetEthChainConfig().ChainID)
	txBytes := make([][]byte, 0, txsPerBlock)

	for i := 0; i < txsPerBlock; i++ {
		priv := privs[i%len(privs)]
		from := sdk.AccAddress(priv.PubKey().Address().Bytes())
		nonce := nonceBySender[from.String()]

		msg := evmtypes.NewTx(&evmtypes.EvmTxArgs{
			ChainID:  evmtypes.GetEthChainConfig().ChainID,
			Nonce:    nonce,
			To:       &contract,
			GasLimit: contractStressGasLimit,
			GasPrice: big.NewInt(1),
			Input:    callData,
			Accesses: &ethtypes.AccessList{},
		})
		msg.From = from.Bytes()
		require.NoError(t, msg.Sign(signer, testtx.NewSigner(priv)))

		tx, err := testtx.PrepareEthTx(txConfig, nil, msg)
		require.NoError(t, err)
		bz, err := txConfig.TxEncoder()(tx)
		require.NoError(t, err)
		txBytes = append(txBytes, bz)
		nonceBySender[from.String()] = nonce + 1
	}

	return txBytes
}

func buildMixedBlock(
	t *testing.T,
	txConfig client.TxConfig,
	privs []*ethsecp256k1.PrivKey,
	accounts []sdk.AccAddress,
	nonceBySender map[string]uint64,
	txsPerBlock int,
	seqContract common.Address,
	nativeCallData []byte,
	createContract common.Address,
	createCallData []byte,
) [][]byte {
	t.Helper()

	signer := ethtypes.LatestSignerForChainID(evmtypes.GetEthChainConfig().ChainID)
	txBytes := make([][]byte, 0, txsPerBlock)

	for i := 0; i < txsPerBlock; i++ {
		priv := privs[2+i%len(privs[2:])]
		from := sdk.AccAddress(priv.PubKey().Address().Bytes())
		nonce := nonceBySender[from.String()]

		args := &evmtypes.EvmTxArgs{
			ChainID:  evmtypes.GetEthChainConfig().ChainID,
			Nonce:    nonce,
			GasPrice: big.NewInt(1),
			Accesses: &ethtypes.AccessList{},
		}

		switch i % 3 {
		case 0:
			recipient := common.BytesToAddress(accounts[2+(i%8)].Bytes())
			args.To = &recipient
			args.Amount = big.NewInt(1)
			args.GasLimit = defaultStressGasLimit
		case 1:
			args.To = &seqContract
			args.Input = nativeCallData
			args.GasLimit = contractStressGasLimit
		default:
			args.To = &createContract
			args.Input = createCallData
			args.GasLimit = contractStressGasLimit
		}

		msg := evmtypes.NewTx(args)
		msg.From = from.Bytes()
		require.NoError(t, msg.Sign(signer, testtx.NewSigner(priv)))

		tx, err := testtx.PrepareEthTx(txConfig, nil, msg)
		require.NoError(t, err)
		bz, err := txConfig.TxEncoder()(tx)
		require.NoError(t, err)
		txBytes = append(txBytes, bz)
		nonceBySender[from.String()] = nonce + 1
	}

	return txBytes
}

func envInt(name string, fallback int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}

	return value
}

func shouldRunPayload(kind payloadKind) bool {
	raw := os.Getenv("EVM_BLOCKSTM_PAYLOADS")
	if raw == "" {
		return true
	}

	for _, entry := range bytes.Split([]byte(raw), []byte(",")) {
		if string(bytes.TrimSpace(entry)) == string(kind) {
			return true
		}
	}

	return false
}

func finalizeNodesConcurrently(t *testing.T, nodes []*network.UnitTestNetwork, txBytes [][]byte) [][]byte {
	t.Helper()

	hashes := make([][]byte, len(nodes))
	errs := make([]error, len(nodes))
	var wg sync.WaitGroup

	for i, node := range nodes {
		wg.Add(1)
		go func(idx int, nw *network.UnitTestNetwork) {
			defer wg.Done()
			_, err := nw.NextBlockWithTxs(txBytes...)
			errs[idx] = err
			if err == nil {
				hashes[idx] = nw.App.GetBaseApp().LastCommitID().Hash
			}
		}(i, node)
	}

	wg.Wait()

	for i, err := range errs {
		require.NoErrorf(t, err, "parallel node %d failed to finalize block", i)
	}

	return hashes
}
