package config_test

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/require"

	serverconfig "github.com/cosmos/evm/server/config"
	"github.com/cosmos/evm/testutil/constants"
)

func TestDefaultConfig(t *testing.T) {
	cfg := serverconfig.DefaultConfig()
	require.False(t, cfg.JSONRPC.Enable)
	require.Equal(t, cfg.JSONRPC.Address, serverconfig.DefaultJSONRPCAddress)
	require.Equal(t, cfg.JSONRPC.WsAddress, serverconfig.DefaultJSONRPCWsAddress)
	require.Equal(t, serverconfig.DefaultFilterCap, cfg.JSONRPC.FilterCap)
	require.Equal(t, serverconfig.DefaultFilterClientCap, cfg.JSONRPC.FilterClientCap)
	require.Equal(t, serverconfig.DefaultFilterTimeout, cfg.JSONRPC.FilterTimeout)
	require.Equal(t, serverconfig.DefaultFilterCleanupInterval, cfg.JSONRPC.FilterCleanupInterval)
}

func TestJSONRPCConfigValidate_FilterProtectionFields(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(c *serverconfig.JSONRPCConfig)
		errText string
	}{
		{
			name: "negative filter-client-cap",
			mutate: func(c *serverconfig.JSONRPCConfig) {
				c.FilterClientCap = -1
			},
			errText: "filter-client-cap cannot be negative",
		},
		{
			name: "zero filter-timeout",
			mutate: func(c *serverconfig.JSONRPCConfig) {
				c.FilterTimeout = 0
			},
			errText: "filter-timeout must be greater than 0",
		},
		{
			name: "zero cleanup interval",
			mutate: func(c *serverconfig.JSONRPCConfig) {
				c.FilterCleanupInterval = 0
			},
			errText: "filter-cleanup-interval must be greater than 0",
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
