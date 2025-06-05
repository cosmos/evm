//go:build test

package testutil

import (
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/testutil/integration/evm/network"
)

type TestSuite struct {
	suite.Suite

	create  network.CreateEvmApp
	options []network.ConfigOption
}

func NewTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *TestSuite {
	return &TestSuite{
		create:  create,
		options: options,
	}
}
