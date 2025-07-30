package keeper_test

import (
	"testing"

	"github.com/cosmos/evm/x/vm/types"
)

func BenchmarkSetParams(b *testing.B) {
	suite := KeeperTestSuite{}
	suite.SetupTest()
	params := types.DefaultParams()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = suite.network.App.EVMKeeper.SetParams(suite.network.GetContext(), params)
	}
}

func BenchmarkGetParams(b *testing.B) {
	suite := KeeperTestSuite{}
	suite.SetupTest()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = suite.network.App.EVMKeeper.GetParams(suite.network.GetContext())
	}
}
