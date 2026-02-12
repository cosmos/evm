package evmd

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/iavlx"
	sdkserver "github.com/cosmos/cosmos-sdk/server"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	evmserverflags "github.com/cosmos/evm/server/flags"
	"github.com/spf13/cast"
)

// iavlxStorage enables IAVLX if the flag is set to true
func iavlxStorage(appOpts servertypes.AppOptions) func(*baseapp.BaseApp) {
	defaultOptions := &iavlx.Options{
		EvictDepth:            20,
		ReaderUpdateInterval:  1,
		WriteWAL:              true,
		MinCompactionSeconds:  30,
		RetainVersions:        1,
		CompactWAL:            true,
		DisableCompaction:     false,
		CompactionOrphanAge:   200,
		CompactionOrphanRatio: 0.95,
		CompactAfterVersions:  2000,
		ChangesetMaxTarget:    2147483648,
		ZeroCopy:              true,
		FsyncInterval:         1000,
	}

	// todo: should be derived from the app logger
	// (after logger-v2 integration that supports slog)
	dbLogger := slog.New(slog.NewTextHandler(os.Stdin, &slog.HandlerOptions{}))

	return func(app *baseapp.BaseApp) {
		logger := app.Logger().With("option", "iavlx")

		enabled := cast.ToBool(appOpts.Get(evmserverflags.IAVLXEnable))

		if !enabled {
			logger.Info("IAVLX storage is not enabled, skipping setup")
			return
		}

		// resolve home directory
		homeDir := cast.ToString(appOpts.Get(flags.FlagHome))
		dbPath := filepath.Join(homeDir, "data", "iavlx")

		// resolve options or fallback to default
		opts := &iavlx.Options{}

		raw, ok := appOpts.Get(sdkserver.FlagIAVLXOptions).(string)
		customConfig := ok && raw != ""

		if customConfig {
			if err := json.Unmarshal([]byte(raw), opts); err != nil {
				panic(fmt.Errorf("unable to parse iavlx options %q: %w", raw, err))
			}
		} else {
			opts = defaultOptions
		}

		db, err := iavlx.LoadDB(dbPath, opts, dbLogger)
		if err != nil {
			panic(fmt.Errorf("unable to load iavlx db at %s: %w", dbPath, err))
		}

		logger.Info(
			"Enabling IAVLX storage",
			"path", dbPath,
			"options", opts,
			"custom_config", customConfig,
		)

		app.SetCMS(db)
	}
}
