package suite

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/testutil/integration/evm/network"
)

// EvmIntegrationConfig defines the inputs needed to spin up an integration
// network for a given app configuration.
type EvmIntegrationConfig struct {
	Name    string
	Create  network.CreateEvmApp
	Options []network.ConfigOption
}

// TestifySuiteConfig defines the inputs for running testify suites over
// multiple app configurations.
type TestifySuiteConfig struct {
	Name   string
	Create network.CreateEvmApp
}

// SuiteFactory builds a testify suite for a given app creator.
type SuiteFactory func(create network.CreateEvmApp) suite.TestingSuite

// Name returns a human-readable suite name that includes the base description
// and the specific configuration being exercised.
func Name(base, cfgName string, idx int) string {
	if cfgName == "" {
		cfgName = fmt.Sprintf("config-%d", idx+1)
	}
	return fmt.Sprintf("%s - %s", base, cfgName)
}

// RunTestifySuites executes testify suites once per configuration and ensures
// a stable subtest name for each configuration.
func RunTestifySuites(t *testing.T, baseName string, factory SuiteFactory, configs ...TestifySuiteConfig) {
	t.Helper()
	if len(configs) == 0 {
		t.Fatalf("no suite configs provided for %s", baseName)
	}

	for i, cfg := range configs {
		cfg := cfg
		t.Run(Name(baseName, cfg.Name, i), func(t *testing.T) {
			suite.Run(t, factory(cfg.Create))
		})
	}
}
