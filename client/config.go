// package client

import (
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/cometbft/cometbft/libs/cli"

	"github.com/cosmos/cosmos-sdk/client/flags"
)

/**
 * @title InitConfig
 * @notice Initializes the application configuration by loading the config.toml file
 * and binding essential command-line flags (chain-id, encoding, output) to Viper.
 * @dev This function is critical for setting up the CLI environment in CometBFT/Cosmos SDK apps.
 * @param cmd The root or persistent command object.
 * @return error if any configuration or file access fails.
 */
func InitConfig(cmd *cobra.Command) error {
	// Retrieve the home directory path from the persistent flags.
	home, err := cmd.PersistentFlags().GetString(cli.HomeFlag)
	if err != nil {
		return err
	}

	configFile := filepath.Join(home, "config", "config.toml")
	
	// Check if the configuration file exists.
	stat, err := os.Stat(configFile)

	if err == nil && !stat.IsDir() {
		// File exists and is not a directory. Proceed with loading.
		viper.SetConfigFile(configFile)

		if err := viper.ReadInConfig(); err != nil {
			// Return error if reading the existing config file fails.
			return err
		}
	} else if err != nil && !os.IsNotExist(err) {
		// Immediately return if the error is not 'file not found' (e.g., permission error).
		// This preserves the original logic's intent regarding non-existence errors.
		return err
	}
	
	// Define the core flags to be bound to Viper.
	flagsToBind := []string{
		flags.FlagChainID,
		cli.EncodingFlag,
		cli.OutputFlag,
	}

	// Iterate over the flags and bind them persistently to Viper.
	// This ensures flag values override config file values if set.
	for _, flagName := range flagsToBind {
		pFlag := cmd.PersistentFlags().Lookup(flagName)
		if pFlag == nil {
			// Should not happen for core flags, but good practice to check.
			continue 
		}

		if err := viper.BindPFlag(flagName, pFlag); err != nil {
			return err
		}
	}

	return nil
}
