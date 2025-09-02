package main

import (
	"fmt"
	"os"

	"github.com/cosmos/evm/evmd/cmd/evmd/cmd"
	evmdconfig "github.com/cosmos/evm/evmd/cmd/evmd/config"

	svrcmd "github.com/cosmos/cosmos-sdk/server/cmd"
)

func main() {
	rootCmd := cmd.NewRootCmd()
	if err := svrcmd.Execute(rootCmd, "evmd", evmdconfig.MustGetDefaultNodeHome()); err != nil {
		fmt.Fprintln(rootCmd.OutOrStderr(), err)
		os.Exit(1)
	}
}
