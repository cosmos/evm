package systemtests

import (
	"math/big"
	"testing"

	"cosmossdk.io/systemtests"
	"github.com/evmos/tests/systemtests/clients"
	"github.com/evmos/tests/systemtests/config"
	"github.com/stretchr/testify/require"
)

type SystemTestSuite struct {
	*systemtests.SystemUnderTest
	EthClient *clients.EthClient
	BaseFee   *big.Int
	BaseFeeX2 *big.Int
}

func NewSystemTestSuite(t *testing.T) *SystemTestSuite {
	config, err := config.NewConfig()
	require.NoError(t, err)

	ethClient, err := clients.NewEthClient(config)
	require.NoError(t, err)

	return &SystemTestSuite{
		SystemUnderTest: systemtests.Sut,
		EthClient:       ethClient,
	}
}

func (s *SystemTestSuite) SetupTest(t *testing.T) {
	s.ResetChain(t)
	s.StartChain(t, config.DefaultNodeArgs()...)
	s.AwaitNBlocks(t, 10)
}

func (s *SystemTestSuite) BeforeEach(t *testing.T) {
	currentBaseFee, err := BaseFee(s.EthClient, "node0")
	require.NoError(t, err)

	s.BaseFee = currentBaseFee
	s.BaseFeeX2 = new(big.Int).Mul(currentBaseFee, big.NewInt(2))
}
