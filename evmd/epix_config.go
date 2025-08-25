package evmd

import (
	"encoding/json"
	"fmt"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	evmosencoding "github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/evmd/cmd/evmd/config"
)

// EpixChainConfig contains configuration specific to the Epix chain
type EpixChainConfig struct {
	ChainID         string
	Denom           string
	DisplayDenom    string
	Decimals        uint8
	GenesisSupply   math.Int
	AirdropSupply   math.Int
	CommunitySupply math.Int
	InitialMintRate math.Int
	MaxSupply       math.Int
}

// GetEpixMainnetConfig returns the configuration for Epix mainnet
func GetEpixMainnetConfig() EpixChainConfig {
	epixToAepix := math.NewIntWithDecimal(1, 18) // 1 EPIX = 10^18 aepix

	return EpixChainConfig{
		ChainID:         fmt.Sprintf("epix_%d-1", config.EpixMainnetChainID),
		Denom:           config.EpixChainDenom,
		DisplayDenom:    config.EpixDisplayDenom,
		Decimals:        18,
		GenesisSupply:   math.NewInt(23689538).Mul(epixToAepix),
		AirdropSupply:   math.NewInt(11844769).Mul(epixToAepix),
		CommunitySupply: math.NewInt(11844769).Mul(epixToAepix),
		InitialMintRate: math.NewInt(10527000000).Mul(epixToAepix), // 10.527B EPIX per year
		MaxSupply:       math.NewInt(42000000000).Mul(epixToAepix), // 42B EPIX
	}
}

// GetEpixTestnetConfig returns the configuration for Epix testnet
func GetEpixTestnetConfig() EpixChainConfig {
	epixToAepix := math.NewIntWithDecimal(1, 18) // 1 EPIX = 10^18 aepix

	return EpixChainConfig{
		ChainID:         fmt.Sprintf("epix_%d-1", config.EpixTestnetChainID),
		Denom:           config.EpixChainDenom,
		DisplayDenom:    config.EpixDisplayDenom,
		Decimals:        18,
		GenesisSupply:   math.NewInt(23689538).Mul(epixToAepix),
		AirdropSupply:   math.NewInt(11844769).Mul(epixToAepix),
		CommunitySupply: math.NewInt(11844769).Mul(epixToAepix),
		InitialMintRate: math.NewInt(10527000000).Mul(epixToAepix), // 10.527B EPIX per year
		MaxSupply:       math.NewInt(42000000000).Mul(epixToAepix), // 42B EPIX
	}
}

// CreateEpixDenomMetadata creates the denomination metadata for EPIX
func CreateEpixDenomMetadata() banktypes.Metadata {
	return banktypes.Metadata{
		Description: "The native token of the Epix blockchain",
		DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    config.EpixChainDenom,
				Exponent: 0,
				Aliases:  []string{"aepix"},
			},
			{
				Denom:    config.EpixDisplayDenom,
				Exponent: 18,
				Aliases:  []string{"EPIX"},
			},
		},
		Base:    config.EpixChainDenom,
		Display: config.EpixDisplayDenom,
		Name:    "Epix",
		Symbol:  "EPIX",
		URI:     "https://epix.zone",
		URIHash: "",
	}
}

// CreateEpixGenesisBalances creates the initial genesis balances for Epix chain
func CreateEpixGenesisBalances(chainConfig EpixChainConfig) []banktypes.Balance {
	// For now, we'll create placeholder addresses for airdrop and community pool
	// In a real deployment, these would be replaced with actual addresses

	airdropAddr := "epix1airdrop000000000000000000000000000000000" // Placeholder
	communityAddr := "epix1community00000000000000000000000000000" // Placeholder

	return []banktypes.Balance{
		{
			Address: airdropAddr,
			Coins:   sdk.NewCoins(sdk.NewCoin(chainConfig.Denom, chainConfig.AirdropSupply)),
		},
		{
			Address: communityAddr,
			Coins:   sdk.NewCoins(sdk.NewCoin(chainConfig.Denom, chainConfig.CommunitySupply)),
		},
	}
}

// CreateEpixBankGenesisState creates the bank genesis state for Epix chain
func CreateEpixBankGenesisState(chainConfig EpixChainConfig) *banktypes.GenesisState {
	balances := CreateEpixGenesisBalances(chainConfig)

	// Calculate total supply from balances
	totalSupply := sdk.NewCoins()
	for _, balance := range balances {
		totalSupply = totalSupply.Add(balance.Coins...)
	}

	metadata := CreateEpixDenomMetadata()

	return &banktypes.GenesisState{
		Params:        banktypes.DefaultParams(),
		Balances:      balances,
		Supply:        totalSupply,
		DenomMetadata: []banktypes.Metadata{metadata},
		SendEnabled:   []banktypes.SendEnabled{},
	}
}

// NewEpixAppGenesisForChain creates a complete genesis state for the specified Epix chain
func NewEpixAppGenesisForChain(evmChainID uint64) (map[string]json.RawMessage, error) {
	var chainConfig EpixChainConfig

	switch evmChainID {
	case config.EpixMainnetChainID:
		chainConfig = GetEpixMainnetConfig()
	case config.EpixTestnetChainID:
		chainConfig = GetEpixTestnetConfig()
	default:
		return nil, fmt.Errorf("unsupported Epix chain ID: %d", evmChainID)
	}

	// Create a temporary app instance to get the codec
	tempApp := &EVMD{}
	tempApp.appCodec = evmosencoding.MakeConfig(evmChainID).Codec

	// Get default genesis from the app
	genesis := tempApp.DefaultGenesisForChain(evmChainID)

	// Override bank genesis with Epix-specific configuration
	bankGenState := CreateEpixBankGenesisState(chainConfig)
	genesis[banktypes.ModuleName] = tempApp.appCodec.MustMarshalJSON(bankGenState)

	return genesis, nil
}

// IsEpixChain checks if the given chain ID is an Epix chain
func IsEpixChain(evmChainID uint64) bool {
	return evmChainID == config.EpixMainnetChainID || evmChainID == config.EpixTestnetChainID
}

// GetEpixChainConfig returns the appropriate chain configuration for the given chain ID
func GetEpixChainConfig(evmChainID uint64) (EpixChainConfig, error) {
	switch evmChainID {
	case config.EpixMainnetChainID:
		return GetEpixMainnetConfig(), nil
	case config.EpixTestnetChainID:
		return GetEpixTestnetConfig(), nil
	default:
		return EpixChainConfig{}, fmt.Errorf("unsupported Epix chain ID: %d", evmChainID)
	}
}
