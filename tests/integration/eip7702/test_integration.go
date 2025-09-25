package eip7702

import (
	"testing"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/ginkgo/v2"

	"github.com/cosmos/evm/testutil/integration/evm/network"
)

func TestEIP7702IntegrationTestSuite(t *testing.T, create network.CreateEvmApp, options ...network.ConfigOption) {
	_ = Describe("", func() {
		var s *EIP7702IntegrationTestSuite

		BeforeAll(func() {
			s = NewEIP7702IntegrationTestSuite(create, options...)
		})

		BeforeEach(func() {
			s.SetupTest()
		})

		Context("", func() {
			It("", func() {

			})
		})
	})
}
