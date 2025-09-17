package flags

import (
	"github.com/cosmos/evm/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// Client configuration flags
const (
	// Custom client flags that will be added to client.toml
	DisplayDenom     = "coin-info.display-denom"
	Decimals         = "coin-info.decimals"
	ExtendedDecimals = "coin-info.extended-decimals"
)

// Default values for client flags
const (
	DefaultDisplayDenom     = config.DisplayDenom
	DefaultDecimals         = config.Decimals
	DefaultExtendedDecimals = config.ExtendedDecimals
)

// AddClientFlags adds custom client flags to the command
func AddClientFlags(cmd *cobra.Command) error {
	cmd.PersistentFlags().String(DisplayDenom, DefaultDisplayDenom, "the display denom used to derive the denom and extended denom")
	cmd.PersistentFlags().Uint8(Decimals, uint8(DefaultDecimals), "the decimals for the base denomination")
	cmd.PersistentFlags().Uint8(ExtendedDecimals, uint8(DefaultExtendedDecimals), "the decimals for the extended denomination")

	// Bind flags to viper for client.toml precedence
	if err := viper.BindPFlag(DisplayDenom, cmd.PersistentFlags().Lookup(DisplayDenom)); err != nil {
		return err
	}
	if err := viper.BindPFlag(Decimals, cmd.PersistentFlags().Lookup(Decimals)); err != nil {
		return err
	}
	if err := viper.BindPFlag(ExtendedDecimals, cmd.PersistentFlags().Lookup(ExtendedDecimals)); err != nil {
		return err
	}

	return nil
}
