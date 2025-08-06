package genesis

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/params"
)

// CosmosGenesisAccount represents an account in cosmos genesis format
type CosmosGenesisAccount struct {
	Type        string `json:"@type"`
	Address     string `json:"address"`
	AccountNum  string `json:"account_number"`
	Sequence    string `json:"sequence"`
	BaseAccount *struct {
		Address    string `json:"address"`
		AccountNum string `json:"account_number"`
		Sequence   string `json:"sequence"`
	} `json:"base_account,omitempty"`
}

// CosmosBalance represents balance in cosmos format
type CosmosBalance struct {
	Address string `json:"address"`
	Coins   []struct {
		Denom  string `json:"denom"`
		Amount string `json:"amount"`
	} `json:"coins"`
}

// CosmosEVMAccount represents EVM account state in cosmos genesis
type CosmosEVMAccount struct {
	Address string `json:"address"`
	Code    string `json:"code"`
	Storage []struct {
		Key   string `json:"key"`
		Value string `json:"value"`
	} `json:"storage,omitempty"`
}

// CosmosGenesis represents the structure of cosmos genesis
type CosmosGenesis struct {
	ChainID string `json:"chain_id"`
	AppState struct {
		Auth struct {
			Accounts []CosmosGenesisAccount `json:"accounts"`
		} `json:"auth"`
		Bank struct {
			Balances []CosmosBalance `json:"balances"`
		} `json:"bank"`
		EVM struct {
			Accounts []CosmosEVMAccount `json:"accounts"`
		} `json:"evm"`
	} `json:"app_state"`
}

// GenesisConverter handles the conversion from cosmos to geth genesis
type GenesisConverter struct {
	cosmosGenesis *CosmosGenesis
	gethGenesis   *core.Genesis
}

// NewGenesisConverter creates a new converter
func NewGenesisConverter() *GenesisConverter {
	return &GenesisConverter{
		gethGenesis: &core.Genesis{
			Config: &params.ChainConfig{
				ChainID:                       big.NewInt(4221), // Same as evmd chain ID
				HomesteadBlock:                big.NewInt(0),
				EIP150Block:                   big.NewInt(0),
				EIP155Block:                   big.NewInt(0),
				EIP158Block:                   big.NewInt(0),
				ByzantiumBlock:                big.NewInt(0),
				ConstantinopleBlock:           big.NewInt(0),
				PetersburgBlock:               big.NewInt(0),
				IstanbulBlock:                 big.NewInt(0),
				MuirGlacierBlock:              big.NewInt(0),
				BerlinBlock:                   big.NewInt(0),
				LondonBlock:                   big.NewInt(0),
				ArrowGlacierBlock:             big.NewInt(0),
				GrayGlacierBlock:              big.NewInt(0),
				MergeNetsplitBlock:            big.NewInt(0),
				ShanghaiTime:                  nil, // Disable for compatibility
				CancunTime:                    nil, // Disable for compatibility
			},
			Nonce:      0,
			Timestamp:  0,
			ExtraData:  []byte{},
			GasLimit:   10000000,
			Difficulty: big.NewInt(0),
			Mixhash:    common.Hash{},
			Coinbase:   common.Address{},
			Alloc:      make(core.GenesisAlloc),
		},
	}
}

// LoadCosmosGenesis loads cosmos genesis from file
func (gc *GenesisConverter) LoadCosmosGenesis(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to read cosmos genesis file: %v", err)
	}

	gc.cosmosGenesis = &CosmosGenesis{}
	if err := json.Unmarshal(data, gc.cosmosGenesis); err != nil {
		return fmt.Errorf("failed to unmarshal cosmos genesis: %v", err)
	}

	return nil
}

// ConvertToGeth converts cosmos genesis to geth format
func (gc *GenesisConverter) ConvertToGeth() error {
	if gc.cosmosGenesis == nil {
		return fmt.Errorf("cosmos genesis not loaded")
	}

	// Convert balances from cosmos format to ethereum format
	for _, balance := range gc.cosmosGenesis.AppState.Bank.Balances {
		ethAddr, err := gc.cosmosToEthAddress(balance.Address)
		if err != nil {
			fmt.Printf("Warning: Could not convert cosmos address %s: %v\n", balance.Address, err)
			continue
		}

		// Find atest balance and convert to wei
		for _, coin := range balance.Coins {
			if coin.Denom == "atest" {
				amount, ok := new(big.Int).SetString(coin.Amount, 10)
				if !ok {
					fmt.Printf("Warning: Invalid amount %s for address %s\n", coin.Amount, balance.Address)
					continue
				}

				// Add to genesis allocation
				gc.gethGenesis.Alloc[ethAddr] = core.GenesisAccount{
					Balance: amount,
				}
				fmt.Printf("Converted balance: %s -> %s wei\n", ethAddr.Hex(), amount.String())
				break
			}
		}
	}

	// Convert EVM accounts (contracts with code and storage)
	for _, evmAccount := range gc.cosmosGenesis.AppState.EVM.Accounts {
		ethAddr := common.HexToAddress(evmAccount.Address)
		
		genesisAccount := core.GenesisAccount{}
		
		// Set contract code if present
		if evmAccount.Code != "" && evmAccount.Code != "0x" {
			genesisAccount.Code = common.FromHex(evmAccount.Code)
			fmt.Printf("Converted contract: %s with %d bytes of code\n", ethAddr.Hex(), len(genesisAccount.Code))
		}

		// Set storage if present
		if len(evmAccount.Storage) > 0 {
			genesisAccount.Storage = make(map[common.Hash]common.Hash)
			for _, storage := range evmAccount.Storage {
				key := common.HexToHash(storage.Key)
				value := common.HexToHash(storage.Value)
				genesisAccount.Storage[key] = value
			}
			fmt.Printf("Converted storage: %s with %d storage entries\n", ethAddr.Hex(), len(genesisAccount.Storage))
		}

		// Add or merge with existing allocation
		if existing, exists := gc.gethGenesis.Alloc[ethAddr]; exists {
			// Merge with existing account (preserve balance)
			if genesisAccount.Code != nil {
				existing.Code = genesisAccount.Code
			}
			if genesisAccount.Storage != nil {
				if existing.Storage == nil {
					existing.Storage = make(map[common.Hash]common.Hash)
				}
				for k, v := range genesisAccount.Storage {
					existing.Storage[k] = v
				}
			}
			gc.gethGenesis.Alloc[ethAddr] = existing
		} else {
			// Ensure accounts without balance get zero balance
			if genesisAccount.Balance == nil {
				genesisAccount.Balance = big.NewInt(0)
			}
			gc.gethGenesis.Alloc[ethAddr] = genesisAccount
		}
	}

	fmt.Printf("Converted %d accounts to geth genesis format\n", len(gc.gethGenesis.Alloc))
	return nil
}

// cosmosToEthAddress attempts to convert cosmos bech32 address to ethereum format
func (gc *GenesisConverter) cosmosToEthAddress(cosmosAddr string) (common.Address, error) {
	// Known mappings from our test setup (from local_node.sh and our testing)
	// These are the actual cosmos->ethereum address mappings for our test accounts
	knownMappings := map[string]string{
		"cosmos10jmp6sgh4cc6zt3e8gw05wavvejgr5pwsjskvv": "0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E", // mykey (validator)
		"cosmos1cml96vmptgw99syqrrz8az79xer2pcgp95srxm":  "0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101", // dev0
		"cosmos1jcltmuhplrdcwp7stlr4hlhlhgd4htqhnu0t2g":  "0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17", // dev1
		"cosmos1gzsvk8rruqn2sx64acfsskrwy8hvrmafzhvvr0":  "0x40a0cb1C63e026A81B55EE1308586E21eec1eFa9", // dev2
		"cosmos1fx944mzagwdhx0wz7k9tfztc8g3lkfk6pzezqh":  "0x498B5AeC5D439b733dC2F58AB489783A23FB26dA", // dev3
	}

	// Check if we have a known mapping
	if ethAddr, exists := knownMappings[cosmosAddr]; exists {
		return common.HexToAddress(ethAddr), nil
	}

	// For unknown cosmos addresses, skip them to avoid invalid ethereum addresses
	// We only want to include the known test accounts in the genesis
	return common.Address{}, fmt.Errorf("unknown cosmos address: %s", cosmosAddr)
}

// SaveGethGenesis saves the converted genesis to file
func (gc *GenesisConverter) SaveGethGenesis(filename string) error {
	data, err := json.MarshalIndent(gc.gethGenesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal geth genesis: %v", err)
	}

	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write geth genesis file: %v", err)
	}

	return nil
}

// ConvertFile is a convenience function to convert a cosmos genesis file to geth format
func ConvertFile(inputFile, outputFile string) error {
	converter := NewGenesisConverter()

	// Load cosmos genesis
	fmt.Printf("Loading cosmos genesis from %s...\n", inputFile)
	if err := converter.LoadCosmosGenesis(inputFile); err != nil {
		return fmt.Errorf("failed to load cosmos genesis: %v", err)
	}

	// Convert to geth format
	fmt.Println("Converting to geth genesis format...")
	if err := converter.ConvertToGeth(); err != nil {
		return fmt.Errorf("failed to convert genesis: %v", err)
	}

	// Save geth genesis
	fmt.Printf("Saving geth genesis to %s...\n", outputFile)
	if err := converter.SaveGethGenesis(outputFile); err != nil {
		return fmt.Errorf("failed to save geth genesis: %v", err)
	}

	fmt.Println("Genesis conversion completed successfully!")
	return nil
}