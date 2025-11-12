package config

import (
	"os"
	"path/filepath"

	"github.com/cosmos/evm/crypto/hd"
	"github.com/spf13/viper"

	clienthelpers "cosmossdk.io/client/v2/helpers"

	"github.com/cosmos/cosmos-sdk/client/config"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	Bech32Prefix         = "cosmos"
	Bech32PrefixAccAddr  = Bech32Prefix
	Bech32PrefixAccPub   = Bech32Prefix + sdk.PrefixPublic
	Bech32PrefixValAddr  = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixOperator
	Bech32PrefixValPub   = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixOperator + sdk.PrefixPublic
	Bech32PrefixConsAddr = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixConsensus
	Bech32PrefixConsPub  = Bech32Prefix + sdk.PrefixValidator + sdk.PrefixConsensus + sdk.PrefixPublic
	DisplayDenom         = "stake"
	BaseDenom            = "astake"
)

// SetBech32Prefixes sets the global prefixes to be used when serializing addresses and public keys to Bech32 strings.
func SetBech32Prefixes(cfg *sdk.Config) {
	cfg.SetBech32PrefixForAccount(Bech32PrefixAccAddr, Bech32PrefixAccPub)
	cfg.SetBech32PrefixForValidator(Bech32PrefixValAddr, Bech32PrefixValPub)
	cfg.SetBech32PrefixForConsensusNode(Bech32PrefixConsAddr, Bech32PrefixConsPub)
}

// SetBip44CoinType sets the global coin type to be used in hierarchical deterministic wallets.
func SetBip44CoinType(cfg *sdk.Config) {
	cfg.SetCoinType(hd.Bip44CoinType)
	cfg.SetPurpose(sdk.Purpose) // Shared
}

func MustGetDefaultNodeHome() string {
	defaultNodeHome, err := clienthelpers.GetNodeHomeDirectory(".evmd")
	if err != nil {
		panic(err)
	}

	if err := os.MkdirAll(defaultNodeHome, 0o700); err != nil {
		panic(err)
	}

	return defaultNodeHome
}

// GetChainIDFromHome returns the chain ID from the client configuration
// in the given home directory.
func GetChainIDFromHome(home string) (string, error) {
	v := viper.New()
	v.AddConfigPath(filepath.Join(home, "config"))
	v.SetConfigName("client")
	v.SetConfigType("toml")

	if err := v.ReadInConfig(); err != nil {
		return "", err
	}
	conf := new(config.ClientConfig)

	if err := v.Unmarshal(conf); err != nil {
		return "", err
	}

	return conf.ChainID, nil
}
