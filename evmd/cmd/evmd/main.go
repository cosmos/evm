package main

import (
	"fmt"
	"os"

	"github.com/cosmos/evm/evmd/cmd/evmd/cmd"
	evmconfig "github.com/cosmos/evm/evmd/cmd/evmd/config"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, "evmd", evmconfig.MustGetDefaultNodeHome()); err != nil {
		fmt.Fprintln(rootCmd.OutOrStderr(), err)
		os.Exit(1)
	}
}
