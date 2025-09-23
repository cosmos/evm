package main

import (
	"fmt"
	config2 "github.com/cosmos/evm/config"
	"os"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/evmd/cmd/evmd/cmd"
)

func main() {
	setupSDKConfig()

	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, "evmd", config2.MustGetDefaultNodeHome()); err != nil {
		fmt.Fprintln(rootCmd.OutOrStderr(), err)
		os.Exit(1)
	}
}

func setupSDKConfig() {
	config := sdk.GetConfig()
	config2.SetBech32Prefixes(config)
	config.Seal()
}
