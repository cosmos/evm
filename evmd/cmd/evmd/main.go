package main

import (
	"fmt"
	"os"

	"github.com/cosmos/evm/evmd/cmd/evmd/cmd"
	"github.com/cosmos/evm/evmd/cmd/evmd/config"
	"github.com/cosmos/evm/utils"

	clienthelpers "cosmossdk.io/client/v2/helpers"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func main() {
	setupSDKConfig()

	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, clienthelpers.EnvPrefix, config.MustGetDefaultNodeHome()); err != nil {
		fmt.Fprintln(rootCmd.OutOrStderr(), err)
		os.Exit(1)
	}
}

func setupSDKConfig() {
	cfg := sdk.GetConfig()
	sdk.DefaultPowerReduction = utils.AttoPowerReduction
	config.SetBech32Prefixes(cfg)
	config.SetBip44CoinType(cfg)
	cfg.Seal()
}
