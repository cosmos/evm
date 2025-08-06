package vm

import (
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/testutil/integration/evm/network"
)

type EvmAnteTestSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption
}

func NewEvmAnteTestSuite(create network.CreateEvmApp, opts ...network.ConfigOption) *EvmAnteTestSuite {
	return &EvmAnteTestSuite{
		create:  create,
		options: opts,
	}
}
