package client

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cometbft/cometbft/libs/cli"

	"github.com/cosmos/cosmos-sdk/client/flags"
)

// InitConfig adds the chain-id, encoding and output flags to the persistent flag set.
func InitConfig(cmd *cobra.Command) error {
	home, err := cmd.PersistentFlags().GetString(cli.HomeFlag)
	if err != nil {
		return err
	}

	configFile := filepath.Join(home, "config", "config.toml")
	_, err = os.Stat(configFile)
	if err != nil && !os.IsNotExist(err) {
		// Immediately return if the error isn't related to the file not existing.
		// See issue https://github.com/evmos/ethermint/issues/539
		return err
	}
	if err == nil {
		viper.SetConfigFile(configFile)

		if err := viper.ReadInConfig(); err != nil {
			return err
		}
	}

	if err := viper.BindPFlag(flags.FlagChainID, cmd.PersistentFlags().Lookup(flags.FlagChainID)); err != nil {
		return err
	}

	if err := viper.BindPFlag(cli.EncodingFlag, cmd.PersistentFlags().Lookup(cli.EncodingFlag)); err != nil {
		return err
	}

	return viper.BindPFlag(cli.OutputFlag, cmd.PersistentFlags().Lookup(cli.OutputFlag))
}
