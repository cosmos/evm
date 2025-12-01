package cmd

import (
	"testing"
	"time"

	cmtconfig "github.com/cometbft/cometbft/config"
	"github.com/stretchr/testify/require"
)

func TestParseAndApplyConfigChanges(t *testing.T) {
	tests := []struct {
		name          string
		override      string
		expectedValue interface{}
		checkFunc     func(*cmtconfig.Config) interface{}
	}{
		{
			name:          "consensus timeout_propose",
			override:      "consensus.timeout_propose=10s",
			expectedValue: 10 * time.Second,
			checkFunc:     func(cfg *cmtconfig.Config) interface{} { return cfg.Consensus.TimeoutPropose },
		},
		{
			name:          "consensus create_empty_blocks",
			override:      "consensus.create_empty_blocks=false",
			expectedValue: false,
			checkFunc:     func(cfg *cmtconfig.Config) interface{} { return cfg.Consensus.CreateEmptyBlocks },
		},
		{
			name:          "consensus double_sign_check_height",
			override:      "consensus.double_sign_check_height=500",
			expectedValue: int64(500),
			checkFunc:     func(cfg *cmtconfig.Config) interface{} { return cfg.Consensus.DoubleSignCheckHeight },
		},
		{
			name:          "baseconfig home",
			override:      "home=/custom/path",
			expectedValue: "/custom/path",
			checkFunc:     func(cfg *cmtconfig.Config) interface{} { return cfg.RootDir },
		},
		{
			name:          "baseconfig log_level",
			override:      "log_level=error",
			expectedValue: "error",
			checkFunc:     func(cfg *cmtconfig.Config) interface{} { return cfg.LogLevel },
		},
		{
			name:          "baseconfig log_format",
			override:      "log_format=json",
			expectedValue: "json",
			checkFunc:     func(cfg *cmtconfig.Config) interface{} { return cfg.LogFormat },
		},
		{
			name:          "baseconfig db_backend",
			override:      "db_backend=badgerdb",
			expectedValue: "badgerdb",
			checkFunc:     func(cfg *cmtconfig.Config) interface{} { return cfg.DBBackend },
		},
		{
			name:          "string slice single value",
			override:      "statesync.rpc_servers=production",
			expectedValue: []string{"production"},
			checkFunc: func(cfg *cmtconfig.Config) interface{} {
				return cfg.StateSync.RPCServers
			},
		},
		{
			name:          "string slice multiple values",
			override:      "statesync.rpc_servers=production,monitoring,critical",
			expectedValue: []string{"production", "monitoring", "critical"},
			checkFunc: func(cfg *cmtconfig.Config) interface{} {
				return cfg.StateSync.RPCServers
			},
		},
		{
			name:          "string slice empty",
			override:      "statesync.rpc_servers=",
			expectedValue: []string(nil),
			checkFunc: func(cfg *cmtconfig.Config) interface{} {
				return cfg.StateSync.RPCServers
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := cmtconfig.DefaultConfig()

			err := parseAndApplyConfigChanges(cfg, []string{tt.override})
			require.NoError(t, err)

			actualValue := tt.checkFunc(cfg)
			require.Equal(t, tt.expectedValue, actualValue)
		})
	}
}
