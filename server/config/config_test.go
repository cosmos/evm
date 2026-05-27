package config_test

import (
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	cmtcfg "github.com/cometbft/cometbft/config"

	serverconfig "github.com/cosmos/evm/server/config"
	"github.com/cosmos/evm/testutil/constants"
)

func TestDefaultConfig(t *testing.T) {
	cfg := serverconfig.DefaultConfig()
	require.False(t, cfg.JSONRPC.Enable)
	require.Equal(t, cfg.JSONRPC.Address, serverconfig.DefaultJSONRPCAddress)
	require.Equal(t, cfg.JSONRPC.WsAddress, serverconfig.DefaultJSONRPCWsAddress)
	require.Equal(t, serverconfig.DefaultFilterTimeout, cfg.JSONRPC.FilterTimeout)
	require.Equal(t, serverconfig.DefaultFilterCleanupInterval, cfg.JSONRPC.FilterCleanupInterval)
	require.Equal(t, cfg.EVM.Mempool.CheckTxTimeout, 5*time.Second)
	require.Equal(t, cfg.JSONRPC.HTTPBodyLimit, serverconfig.DefaultHTTPBodyLimit)
}

func TestJSONRPCConfigValidate_FilterProtectionFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *serverconfig.JSONRPCConfig)
		errText string
	}{
		{
			name: "negative filter-timeout",
			mutate: func(c *serverconfig.JSONRPCConfig) {
				c.FilterTimeout = -1
			},
			errText: "filter-timeout cannot be negative",
		},
		{
			name: "negative cleanup interval",
			mutate: func(c *serverconfig.JSONRPCConfig) {
				c.FilterCleanupInterval = -1
			},
			errText: "filter-cleanup-interval cannot be negative",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := *serverconfig.DefaultJSONRPCConfig()
			tc.mutate(&cfg)

			err := cfg.Validate()
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.errText)
		})
	}
}

func TestGetConfig(t *testing.T) {
	tests := []struct {
		name    string
		args    func() *viper.Viper
		want    func() serverconfig.Config
		wantErr bool
	}{
		{
			"test unmarshal embedded structs",
			func() *viper.Viper {
				v := viper.New()
				v.Set("minimum-gas-prices", fmt.Sprintf("100%s", constants.ExampleAttoDenom))
				return v
			},
			func() serverconfig.Config {
				cfg := serverconfig.DefaultConfig()
				cfg.MinGasPrices = fmt.Sprintf("100%s", constants.ExampleAttoDenom)
				return *cfg
			},
			false,
		},
		{
			"test unmarshal EVMConfig",
			func() *viper.Viper {
				v := viper.New()
				v.Set("evm.tracer", "struct")
				return v
			},
			func() serverconfig.Config {
				cfg := serverconfig.DefaultConfig()
				require.NotEqual(t, "struct", cfg.EVM.Tracer)
				cfg.EVM.Tracer = "struct"
				return *cfg
			},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := serverconfig.GetConfig(tt.args())
			if (err != nil) != tt.wantErr {
				t.Errorf("GetConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want()) {
				t.Errorf("GetConfig() got = %v, want %v", got, tt.want())
			}
		})
	}
}

func TestValidateCrossConfig(t *testing.T) {
	for _, tt := range []struct {
		name         string
		cometType    string
		mempoolMaxTx int
		nilCometCfg  bool
		errContains  string
	}{
		{
			// both enabled
			name:         "comet-app:evm-on",
			cometType:    "app",
			mempoolMaxTx: 0,
		},
		{
			name:         "comet-flood:evm-off",
			cometType:    "flood",
			mempoolMaxTx: 0,
			errContains:  "invalid config.toml:mempool.type",
		},
		{
			name:         "comet-app:evm-off",
			cometType:    "app",
			mempoolMaxTx: -1,
			errContains:  "EVM mempool is disabled",
		},
		{
			// both disabled
			name:         "comet-flood:evm-on",
			mempoolMaxTx: -1,
			cometType:    "flood",
		},
		// nil check
		{
			name:         "nil comet config",
			mempoolMaxTx: 0,
			nilCometCfg:  true,
			errContains:  "comet and app configs are required",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE
			appCfg := serverconfig.DefaultConfig()
			appCfg.Mempool.MaxTxs = tt.mempoolMaxTx

			cometCfg := cmtcfg.DefaultConfig()
			cometCfg.Mempool.Type = tt.cometType

			if tt.nilCometCfg {
				cometCfg = nil
			}

			// ACT
			err := serverconfig.ValidateCrossConfig(cometCfg, appCfg)

			// ASSERT
			if tt.errContains != "" {
				require.ErrorContains(t, err, tt.errContains)
				return
			}
			require.NoError(t, err)
		})
	}
}
