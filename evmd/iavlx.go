package evmd

import (
	"encoding/json"
	"fmt"
	"github.com/cosmos/cosmos-sdk/iavl"
	flags2 "github.com/cosmos/evm/server/flags"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	"github.com/spf13/cast"
)

// iavlxStorage enables IAVLX if the flag is set to true
func iavlxStorage(appOpts servertypes.AppOptions) func(*baseapp.BaseApp) {

	homeDir := cast.ToString(appOpts.Get(flags.FlagHome))

	return func(app *baseapp.BaseApp) {
		var opts iavl.Options
		optsJson, ok := appOpts.Get(flags2.IAVLXOptions).(string)
		if !ok || optsJson == "" {
			fmt.Println("Using iavl/v1")
			return
		}
		fmt.Println("Using iavlx")

		err := json.Unmarshal([]byte(optsJson), &opts)
		if err != nil {
			panic(fmt.Errorf("failed to unmarshal iavlx options: %w", err))
		}

		db, err := iavl.LoadCommitMultiTree(
			filepath.Join(homeDir, "data", "iavlx"),
			opts,
			app.Logger(),
		)
		if err != nil {
			panic(fmt.Errorf("failed to load iavlx db: %w", err))
		}
		fmt.Println("Setting up IAVLX as the underlying commit multi-store")
		app.SetCMS(db)
	}
}
