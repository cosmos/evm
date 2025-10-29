package utils

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/config"
	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

func SendTransaction(rCtx *types.RPCContext, from, to string, value *big.Int, isGeth bool) (string, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	// Create a simple transaction object for testing
	tx := map[string]interface{}{
		"from":     from,
		"to":       to,
		"value":    fmt.Sprintf("0x%x", value),
		"gas":      "0x5208",        // 21000 gas
		"gasPrice": "0x9184e72a000", // 10000000000000
	}

	var txHash string
	err := ethCli.RPCClient().Call(&txHash, string("eth_sendTransaction"), tx)
	if err != nil {
		return "", fmt.Errorf("failed to send transaction: %w", err)
	}

	return txHash, nil
}

func SendRawTransaction(rCtx *types.RPCContext, privKey string, to common.Address, value *big.Int, data []byte, isGeth bool) (*ethtypes.Receipt, error) {
	ctx := context.Background()

	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	// Get chain ID
	chainID, err := ethCli.ChainID(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %w", err)
	}

	// Get owner credentials
	privateKey, ownerAddr, err := GetPrivateKeyAndAddress(privKey)
	if err != nil {
		return nil, fmt.Errorf("failed to get owner credentials: %w", err)
	}

	// Get nonce
	nonce, err := ethCli.PendingNonceAt(ctx, ownerAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to get nonce: %w", err)
	}

	// Get gas pricing
	gasPrice, err := ethCli.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get gas price: %w", err)
	}

	tx := ethtypes.NewTransaction(nonce, ethCli.ERC20Addr, big.NewInt(0), 100000, gasPrice, data)

	// Sign transaction
	signer := ethtypes.NewEIP155Signer(chainID)
	signedTx, err := ethtypes.SignTx(tx, signer, privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %w", err)
	}

	// Send transaction
	err = ethCli.SendTransaction(ctx, signedTx)
	if err != nil {
		return nil, fmt.Errorf("failed to send transfer transaction: %w", err)
	}

	// Wait for transaction to be mined
	receipt, err := WaitForTx(rCtx, signedTx.Hash(), 30*time.Second, isGeth)
	if err != nil {
		return nil, fmt.Errorf("failed to get transfer receipt: %w", err)
	}

	return receipt, nil
}

// waitForTransactionReceipt waits for a transaction receipt
func WaitForTx(rCtx *types.RPCContext, txHash common.Hash, timeout time.Duration, isGeth bool) (*ethtypes.Receipt, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("timeout waiting for transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := ethCli.TransactionReceipt(context.Background(), txHash)
			if err != nil {
				continue // Transaction not mined yet
			}
			return receipt, nil
		}
	}
}

func GetAccounts(rCtx *types.RPCContext, isGeth bool) ([]string, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	var accounts []string
	err := ethCli.RPCClient().Call(&accounts, string("eth_accounts"))
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	return accounts, err
}

func NewERC20FilterLogs(rCtx *types.RPCContext, isGeth bool) (ethereum.FilterQuery, string, error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	fErc20Transfer := ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(0), // Start from genesis
		Addresses: []common.Address{ethCli.ERC20Addr},
		Topics: [][]common.Hash{
			{ethCli.ERC20Abi.Events["Transfer"].ID}, // Filter for Transfer event
		},
	}

	// Create filter on evmd
	args, err := ToFilterArg(fErc20Transfer)
	if err != nil {
		return fErc20Transfer, "", fmt.Errorf("failed to create filter args: %w", err)
	}
	var evmdFilterID string
	if err = ethCli.RPCClient().CallContext(rCtx, &evmdFilterID, "eth_newFilter", args); err != nil {
		return fErc20Transfer, "", fmt.Errorf("failed to create filter on evmd: %w", err)
	}

	return fErc20Transfer, evmdFilterID, nil
}

// Standard dev account addresses (matching evmd genesis accounts)
var StandardDevAccounts = map[string]common.Address{
	"dev0": common.HexToAddress("0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101"), // dev0 from local_node.sh
	"dev1": common.HexToAddress("0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17"), // dev1 from local_node.sh
	"dev2": common.HexToAddress("0x40a0cb1C63e026A81B55EE1308586E21eec1eFa9"), // dev2 from local_node.sh (CORRECTED)
	"dev3": common.HexToAddress("0x498B5AeC5D439b733dC2F58AB489783A23FB26dA"), // dev3 from local_node.sh (CORRECTED)
}

// Standard dev account balance (1000 ETH = 1000 * 10^18 wei)
var StandardDevBalance = new(big.Int).Mul(big.NewInt(1000), big.NewInt(1e18))

// FundingResult holds information about a funding transaction
type FundingResult struct {
	Account string         `json:"account"`
	Address common.Address `json:"address"`
	Amount  *big.Int       `json:"amount"`
	TxHash  common.Hash    `json:"txHash"`
	Success bool           `json:"success"`
	Error   string         `json:"error,omitempty"`
}

// FundStandardAccounts sends funds from geth coinbase to standard dev accounts
func FundStandardAccounts(rCtx *types.RPCContext, isGeth bool) ([]FundingResult, error) {
	results := make([]FundingResult, 0, len(StandardDevAccounts))

	// Get coinbase account (first account from eth_accounts)
	accounts, err := GetAccounts(rCtx, isGeth)
	if err != nil {
		return nil, fmt.Errorf("failed to get accounts: %w", err)
	}

	if len(accounts) == 0 {
		return nil, fmt.Errorf("no accounts found in geth")
	}

	coinbase := accounts[0] // First account is coinbase in dev mode

	// Fund each standard dev account using eth_sendTransaction
	for name, address := range StandardDevAccounts {
		result := FundingResult{
			Account: name,
			Address: address,
			Amount:  StandardDevBalance,
		}

		// Send transaction using eth_sendTransaction (coinbase is unlocked in dev mode)
		txHash, err := SendTransaction(rCtx, coinbase, address.Hex(), StandardDevBalance, isGeth)
		if err != nil {
			result.Success = false
			result.Error = err.Error()
		} else {
			result.Success = true
			result.TxHash = common.HexToHash(txHash)
		}

		results = append(results, result)
	}

	return results, nil
}

// CheckAccountBalances verifies that accounts have the expected balances
func Balances(client *ethclient.Client) (map[string]*big.Int, error) {
	ctx := context.Background()
	balances := make(map[string]*big.Int)

	for name, address := range StandardDevAccounts {
		balance, err := client.BalanceAt(ctx, address, nil)
		if err != nil {
			return nil, err
		}
		balances[name] = balance
	}

	return balances, nil
}

func DeployContract(rCtx *types.RPCContext, contractByteCode []byte, isGeth bool) (addr common.Address, txHash string, blockNum *big.Int, err error) {
	ethCli := rCtx.Evmd
	if isGeth {
		ethCli = rCtx.Geth
	}

	privateKey, fromAddress, err := config.GetDev0PrivateKeyAndAddress()
	if err != nil {
		return common.Address{}, "", nil, fmt.Errorf("failed to get dev0 credentials: %v", err)
	}

	fmt.Printf("Deploying ERC20 to evmd using dev0 (%s)...\n", fromAddress.Hex())

	evmdTxHash, err := deployContractViaDynamicFeeTx(ethCli.Client, privateKey, contractByteCode)
	if err != nil {
		return common.Address{}, "", nil, err
	} else {
		addr, blockNum, err = waitForContractDeployment(ethCli.Client, evmdTxHash, 30*time.Second)
		if err != nil {
			return common.Address{}, "", nil, err
		}
	}

	fmt.Printf("✓ evmd deployment successful: %s\n", addr.Hex())
	return addr, evmdTxHash, blockNum, nil
}

func deployContractViaDynamicFeeTx(client *ethclient.Client, privateKey *ecdsa.PrivateKey, contractByteCode []byte) (string, error) {
	ctx := context.Background()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		return "", err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("error casting public key to ECDSA")
	}
	fromAddress := crypto.PubkeyToAddress(*publicKeyECDSA)

	nonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		return "", err
	}

	maxPriorityFeePerGas, err := client.SuggestGasTipCap(ctx)
	if err != nil {
		return "", err
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return "", err
	}

	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		GasTipCap: maxPriorityFeePerGas,
		GasFeeCap: new(big.Int).Add(gasPrice, big.NewInt(1000000000)),
		Gas:       10000000,
		Data:      contractByteCode,
	})

	signer := ethtypes.NewLondonSigner(chainID)
	signedTx, err := ethtypes.SignTx(tx, signer, privateKey)
	if err != nil {
		return "", err
	}

	if err = client.SendTransaction(ctx, signedTx); err != nil {
		return "", err
	}

	return signedTx.Hash().Hex(), nil
}

// waitForContractDeployment waits for a deployment transaction to be mined and returns the contract address
func waitForContractDeployment(client *ethclient.Client, txHashStr string, timeout time.Duration) (common.Address, *big.Int, error) {
	fmt.Printf("Waiting for evmd deployment (tx: %s)...\n", txHashStr)

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	txHash := common.HexToHash(txHashStr)
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return common.Address{}, nil, fmt.Errorf("timeout waiting for deployment transaction %s", txHashStr)
		case <-ticker.C:
			receipt, err := client.TransactionReceipt(context.Background(), txHash)
			if err != nil {
				continue // Transaction not mined yet
			}

			if receipt.Status == 0 {
				return common.Address{}, nil, fmt.Errorf("deployment transaction failed: %s", txHashStr)
			}

			if receipt.ContractAddress == (common.Address{}) {
				return common.Address{}, nil, fmt.Errorf("no contract address in receipt for tx: %s", txHashStr)
			}

			return receipt.ContractAddress, receipt.BlockNumber, nil
		}
	}
}

// Generic test handler that makes an actual RPC call to determine if an API is implemented
func CallEthClient(rCtx *types.RPCContext, methodName types.RpcName, category string) (*types.RpcResult, error) {
	var result interface{}
	err := rCtx.Evmd.RPCClient().Call(&result, string(methodName))

	status := types.Ok
	errMsg := ""
	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(methodName)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(methodName)+" method not found" {

			status = types.NotImplemented
			errMsg = "Method not implemented in Cosmos EVM"
		} else {
			status = types.Error
			errMsg = err.Error()
		}
	}

	return &types.RpcResult{
		Method:   methodName,
		Status:   status,
		Value:    result,
		ErrMsg:   errMsg,
		Category: category,
	}, nil
}

func Legacy(rCtx *types.RPCContext, methodName types.RpcName, category string, replacementInfo string) (*types.RpcResult, error) {
	// First test if the API is actually implemented
	var result interface{}
	err := rCtx.Evmd.RPCClient().Call(&result, string(methodName))
	if err != nil {
		// Check if it's a "method not found" error (API not implemented)
		if err.Error() == "the method "+string(methodName)+" does not exist/is not available" ||
			err.Error() == "Method not found" ||
			err.Error() == string(methodName)+" method not found" {
			// API is not implemented, so it should be NOT_IMPL, not LEGACY
			return &types.RpcResult{
				Method:   methodName,
				Status:   types.NotImplemented,
				ErrMsg:   "Method not implemented in Cosmos EVM",
				Category: category,
			}, nil
		}
	}

	// API exists (either succeeded or failed with parameter issues), mark as LEGACY
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Legacy,
		Value:    fmt.Sprintf("Legacy API implemented in Cosmos EVM. %s", replacementInfo),
		ErrMsg:   replacementInfo,
		Category: category,
	}, nil
}

func Skip(methodName types.RpcName, category string, reason string) (*types.RpcResult, error) {
	return &types.RpcResult{
		Method:   methodName,
		Status:   types.Skipped,
		ErrMsg:   reason,
		Category: category,
	}, nil
}
