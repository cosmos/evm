package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// TestAccount represents a test account with private key and address
type TestAccount struct {
	PrivateKey *ecdsa.PrivateKey
	Address    common.Address
}

// StateGenerator handles the creation of initial blockchain state for testing
type StateGenerator struct {
	client    *ethclient.Client
	chainID   *big.Int
	validator TestAccount
}

// NewStateGenerator creates a new state generator connected to the specified endpoint
func NewStateGenerator(endpoint string) (*StateGenerator, error) {
	client, err := ethclient.Dial(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to ethereum client: %v", err)
	}

	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to get chain ID: %v", err)
	}

	// Create validator account from standard testnet private key (mykey from local_node.sh)
	// Address: 0x7cb61d4117ae31a12e393a1cfa3bac666481d02e
	privateKey, err := crypto.HexToECDSA("e9b1d63e8acd7fe676acb43afb390d4b0202dab61abec9cf2a561e4becb147de")
	if err != nil {
		return nil, fmt.Errorf("failed to create private key: %v", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	validator := TestAccount{
		PrivateKey: privateKey,
		Address:    crypto.PubkeyToAddress(*publicKeyECDSA),
	}

	return &StateGenerator{
		client:    client,
		chainID:   chainID,
		validator: validator,
	}, nil
}

// CreateRandomAccount creates a new random account
func CreateRandomAccount() TestAccount {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("Failed to generate private key: %v", err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("Cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	return TestAccount{
		PrivateKey: privateKey,
		Address:    crypto.PubkeyToAddress(*publicKeyECDSA),
	}
}

// ValidateConnection checks if the connection is working and validator has funds
func (sg *StateGenerator) ValidateConnection() error {
	fmt.Printf("Connected to chain ID: %s\n", sg.chainID.String())
	fmt.Printf("Using validator address: %s\n", sg.validator.Address.Hex())

	// Check validator balance
	balance, err := sg.client.BalanceAt(context.Background(), sg.validator.Address, nil)
	if err != nil {
		return fmt.Errorf("failed to get balance: %v", err)
	}
	fmt.Printf("Validator balance: %s wei\n", balance.String())

	if balance.Cmp(big.NewInt(0)) == 0 {
		return fmt.Errorf("validator account has zero balance - cannot send transactions")
	}

	return nil
}

// SendValueTransfer sends a value transfer transaction
func (sg *StateGenerator) SendValueTransfer() (common.Hash, error) {
	fmt.Println("=== Sending Value Transfer Transaction ===")
	
	nonce, err := sg.client.PendingNonceAt(context.Background(), sg.validator.Address)
	if err != nil {
		return common.Hash{}, err
	}

	gasPrice, err := sg.client.SuggestGasPrice(context.Background())
	if err != nil {
		return common.Hash{}, err
	}

	// Create random recipient
	recipient := CreateRandomAccount()
	value := big.NewInt(1000000000000000000) // 1 ETH in wei

	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   sg.chainID,
		Nonce:     nonce,
		GasTipCap: big.NewInt(1000000000), // 1 gwei
		GasFeeCap: new(big.Int).Add(gasPrice, big.NewInt(1000000000)),
		Gas:       21000,
		To:        &recipient.Address,
		Value:     value,
	})

	signer := gethtypes.NewLondonSigner(sg.chainID)
	signedTx, err := gethtypes.SignTx(tx, signer, sg.validator.PrivateKey)
	if err != nil {
		return common.Hash{}, err
	}

	if err := sg.client.SendTransaction(context.Background(), signedTx); err != nil {
		return common.Hash{}, err
	}

	fmt.Printf("Value transfer tx sent: %s\n", signedTx.Hash().Hex())
	fmt.Printf("  From: %s\n", sg.validator.Address.Hex())
	fmt.Printf("  To: %s\n", recipient.Address.Hex())
	fmt.Printf("  Amount: %s wei\n", value.String())

	if err := sg.waitForTransaction(signedTx.Hash()); err != nil {
		return common.Hash{}, err
	}

	return signedTx.Hash(), nil
}

// DeployERC20Contract deploys an ERC20 contract and returns its address
func (sg *StateGenerator) DeployERC20Contract() (common.Address, common.Hash, error) {
	fmt.Println("=== Deploying ERC20 Contract ===")
	
	nonce, err := sg.client.PendingNonceAt(context.Background(), sg.validator.Address)
	if err != nil {
		return common.Address{}, common.Hash{}, err
	}

	gasPrice, err := sg.client.SuggestGasPrice(context.Background())
	if err != nil {
		return common.Address{}, common.Hash{}, err
	}

	gasTipCap, err := sg.client.SuggestGasTipCap(context.Background())
	if err != nil {
		return common.Address{}, common.Hash{}, err
	}

	// ERC20 contract bytecode (simplified ERC20 implementation)
	contractBytecode := "608060405234801561001057600080fd5b506040518060400160405280600881526020017f54657374436f696e0000000000000000000000000000000000000000000000008152506000908051906020019061005c929190610075565b506012600160006101000a81548160ff021916908360ff160217905550610179565b828054600181600116156101000203166002900490600052602060002090601f016020900481019282601f106100b657805160ff19168380011785556100e4565b828001600101855582156100e4579182015b828111156100e35782518255916020019190600101906100c8565b5b5090506100f191906100f5565b5090565b61011791905b8082111561011357600081600090555060010161010fb565b5090565b90565b610c9b806101286000396000f3fe608060405234801561001057600080fd5b50600436106100a95760003560e01c80633950935111610071578063395093511461025857806370a082311461029e57806395d89b41146102f6578063a457c2d714610379578063a9059cbb146103bf578063dd62ed3e14610405576100a9565b806306fdde03146100ae578063095ea7b31461013157806318160ddd1461017757806323b872dd14610195578063313ce5671461021b575b600080fd5b6100b661047d565b6040518080602001828103825283818151815260200191508051906020019080838360005b838110156100f65780820151818401526020810190506100db565b50505050905090810190601f1680156101235780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b61015d6004803603604081101561014757600080fd5b81019080803590602001909291908035906020019092919050505061051f565b604051808215151515815260200191505060405180910390f35b61017f61053d565b6040518082815260200191505060405180910390f35b610201600480360360608110156101ab57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff16906020019092919080359060200190929190505050610547565b604051808215151515815260200191505060405180910390f35b610223610620565b604051808260ff1660ff16815260200191505060405180910390f35b6102846004803603604081101561026e57600080fd5b8101908080359060200190929190803590602001909291905050506106377565b604051808215151515815260200191505060405180910390f35b6102e0600480360360208110156102b457600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff1690602001909291905050506106ea565b6040518082815260200191505060405180910390f35b6102fe610702565b6040518080602001828103825283818151815260200191508051906020019080838360005b8381101561033e578082015181840152602081019050610323565b50505050905090810190601f16801561036b5780820380516001836020036101000a031916815260200191505b509250505060405180910390f35b6103a56004803603604081101561038f57600080fd5b81019080803590602001909291908035906020019092919050505061073b565b604051808215151515815260200191505060405180910390f35b6103eb600480360360408110156103d557600080fd5b8101908080359060200190929190803590602001909291905050506107ee565b604051808215151515815260200191505060405180910390f35b6104676004803603604081101561041b57600080fd5b81019080803573ffffffffffffffffffffffffffffffffffffffff169060200190929190803573ffffffffffffffffffffffffffffffffffffffff16906020019092919050505061080c565b6040518082815260200191505060405180910390f35b606060008054600181600116156101000203166002900480601f0160208091040260200160405190810160405280929190818152602001828054600181600116156101000203166002900480156105155780601f106104ea57610100808354040283529160200191610515565b820191906000526020600020905b8154815290600101906020018083116104f857829003601f168201915b5050505050905090565b600061053361052c610893565b848461089b565b6001905092915050565b6000600254905090565b6000610554848484610a92565b6106158461056d610893565b61061085604051806060016040528060288152602001610c0460289139600160008b73ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060006105d3610893565b73ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054610d539092919063ffffffff16565b61089b565b600190509392505050565b6000600160009054906101000a900460ff16905090565b60006106e0610644610893565b846106db8560016000610655610893565b73ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008973ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054610e1390919063ffffffff16565b61089b565b6001905092915050565b60008060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050919050565b606060405180604001604052806008815260200167546f6b656e546573740000000000000000000000000000000000000000000000000000815250905090565b60006107e4610748610893565b846107df85604051806060016040528060258152602001610c41602591396001600061077a85600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff166106ea565b6107ef85600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff166106ea565b6108cd85600660009054906101000a900473ffffffffffffffffffffffffffffffffffffffff166106ea565b610893565b73ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008973ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054610d539092919063ffffffff16565b61089b565b6001905092915050565b60006108026107fb610893565b8484610a92565b6001905092915050565b6000600160008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020549050920050565b600033905090565b600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff161415610921576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526024815260200180610c1d6024913960400191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff1614156109a7576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526022815260200180610bbc6022913960400191505060405180910390fd5b80600160008573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002060008473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508173ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff167f8c5be1e5ebec7d5bd14f71427d1e84f3dd0314c0f7b2291e5b200ac8c7c3b925836040518082815260200191505060405180910390a3505050565b600073ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff161415610b18576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526025815260200180610bf86025913960400191505060405180910390fd5b600073ffffffffffffffffffffffffffffffffffffffff168273ffffffffffffffffffffffffffffffffffffffff161415610b9e576040517f08c379a0000000000000000000000000000000000000000000000000000000008152600401808060200182810382526023815260200180610b936023913960400191505060405180910390fd5b610c0983604051806060016040528060268152602001610bde602691396000808873ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054610d539092919063ffffffff16565b6000808673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002081905550610c9c816000808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff16815260200190815260200160002054610e1390919063ffffffff16565b6000808473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff168152602001908152602001600020819055508173ffffffffffffffffffffffffffffffffffffffff168373ffffffffffffffffffffffffffffffffffffffff167fddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef836040518082815260200191505060405180910390a3505050565b6000838311158290610e00576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004018080602001828103825283818151815260200191508051906020019080838360005b83811015610dc5578082015181840152602081019050610daa565b50505050905090810190601f168015610df25780820380516001836020036101000a031916815260200191505b509250505060405180910390fd5b5060008385039050809150509392505050565b600080828401905083811015610e91576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040180806020018281038252601b8152602001807f536166654d6174683a206164646974696f6e206f766572666c6f770000000081525060200191505060405180910390fd5b809150509291505056fe45524332303a207472616e7366657220746f20746865207a65726f206164647265737345524332303a20617070726f766520746f20746865207a65726f206164647265737345524332303a207472616e7366657220616d6f756e7420657863656564732062616c616e636545524332303a207472616e7366657220616d6f756e74206578636565647320616c6c6f77616e636545524332303a207472616e736665722066726f6d20746865207a65726f206164647265737345524332303a20617070726f76652066726f6d20746865207a65726f206164647265737345524332303a2064656372656173656420616c6c6f77616e63652062656c6f77207a65726fa265627a7a72305820c9b2c8a1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b1b164736f6c634300050a0032"

	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   sg.chainID,
		Nonce:     nonce,
		GasTipCap: gasTipCap,
		GasFeeCap: new(big.Int).Add(gasPrice, big.NewInt(1000000000)),
		Gas:       10000000, // Higher gas limit like ethrpc-checker
		Data:      common.FromHex(contractBytecode),
	})

	signer := gethtypes.NewLondonSigner(sg.chainID)
	signedTx, err := gethtypes.SignTx(tx, signer, sg.validator.PrivateKey)
	if err != nil {
		return common.Address{}, common.Hash{}, err
	}

	if err := sg.client.SendTransaction(context.Background(), signedTx); err != nil {
		return common.Address{}, common.Hash{}, err
	}

	fmt.Printf("Contract deploy tx sent: %s\n", signedTx.Hash().Hex())

	if err := sg.waitForTransaction(signedTx.Hash()); err != nil {
		return common.Address{}, common.Hash{}, err
	}

	// Get contract address from receipt
	receipt, err := sg.client.TransactionReceipt(context.Background(), signedTx.Hash())
	if err != nil {
		return common.Address{}, common.Hash{}, err
	}

	fmt.Printf("Contract deployed at: %s\n", receipt.ContractAddress.Hex())
	return receipt.ContractAddress, signedTx.Hash(), nil
}

// CallERC20Transfer performs an ERC20 transfer transaction
func (sg *StateGenerator) CallERC20Transfer(contractAddr common.Address) (common.Hash, error) {
	fmt.Println("=== Executing ERC20 Transfer Transaction ===")
	
	// ERC20 ABI for transfer function
	abiJSON := `[{"inputs":[{"internalType":"address","name":"_to","type":"address"},{"internalType":"uint256","name":"_value","type":"uint256"}],"name":"transfer","outputs":[{"internalType":"bool","name":"success","type":"bool"}],"stateMutability":"nonpayable","type":"function"}]`

	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to parse ABI: %v", err)
	}

	nonce, err := sg.client.PendingNonceAt(context.Background(), sg.validator.Address)
	if err != nil {
		return common.Hash{}, err
	}

	gasPrice, err := sg.client.SuggestGasPrice(context.Background())
	if err != nil {
		return common.Hash{}, err
	}

	// Create random recipient and transfer amount
	recipient := CreateRandomAccount()
	amount := big.NewInt(100)

	// Pack transfer function call
	data, err := parsedABI.Pack("transfer", recipient.Address, amount)
	if err != nil {
		return common.Hash{}, fmt.Errorf("failed to pack transfer call: %v", err)
	}

	tx := gethtypes.NewTx(&gethtypes.DynamicFeeTx{
		ChainID:   sg.chainID,
		Nonce:     nonce,
		GasTipCap: big.NewInt(1000000000), // 1 gwei
		GasFeeCap: new(big.Int).Add(gasPrice, big.NewInt(1000000000)),
		Gas:       300000,
		To:        &contractAddr,
		Data:      data,
	})

	signer := gethtypes.NewLondonSigner(sg.chainID)
	signedTx, err := gethtypes.SignTx(tx, signer, sg.validator.PrivateKey)
	if err != nil {
		return common.Hash{}, err
	}

	if err := sg.client.SendTransaction(context.Background(), signedTx); err != nil {
		return common.Hash{}, err
	}

	fmt.Printf("ERC20 transfer tx sent: %s\n", signedTx.Hash().Hex())
	fmt.Printf("  Contract: %s\n", contractAddr.Hex())
	fmt.Printf("  To: %s\n", recipient.Address.Hex())
	fmt.Printf("  Amount: %s\n", amount.String())

	if err := sg.waitForTransaction(signedTx.Hash()); err != nil {
		return common.Hash{}, err
	}

	return signedTx.Hash(), nil
}

// waitForTransaction waits for a transaction to be mined
func (sg *StateGenerator) waitForTransaction(txHash common.Hash) error {
	fmt.Printf("Waiting for tx %s to be mined...\n", txHash.Hex())

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for transaction %s", txHash.Hex())
		case <-ticker.C:
			receipt, err := sg.client.TransactionReceipt(context.Background(), txHash)
			if err != nil {
				continue // Transaction not yet mined
			}
			if receipt.Status == 1 {
				fmt.Printf("Transaction %s mined successfully in block %d\n", txHash.Hex(), receipt.BlockNumber.Uint64())
				return nil
			} else {
				return fmt.Errorf("transaction %s failed", txHash.Hex())
			}
		}
	}
}

// CreateInitialState performs operations to create initial test state
func (sg *StateGenerator) CreateInitialState() error {
	fmt.Println("=== Creating Initial Test State ===")

	// Validate connection and account balance
	if err := sg.ValidateConnection(); err != nil {
		return err
	}

	// Step 1: Send first value transfer
	if _, err := sg.SendValueTransfer(); err != nil {
		return fmt.Errorf("failed to send first value transfer: %v", err)
	}

	// Step 2: Send second value transfer to create more transaction state
	if _, err := sg.SendValueTransfer(); err != nil {
		return fmt.Errorf("failed to send second value transfer: %v", err)
	}

	// Step 3: Send third value transfer to create even more state
	if _, err := sg.SendValueTransfer(); err != nil {
		return fmt.Errorf("failed to send third value transfer: %v", err)
	}

	fmt.Println("\n=== Initial state created successfully with multiple transactions! ===")
	return nil
}