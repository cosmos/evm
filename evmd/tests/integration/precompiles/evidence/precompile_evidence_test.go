package evidence

import (
	"testing"

	"cosmosevm.io/evmd/tests/integration"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/tests/integration/precompiles/evidence"
)

func TestEvidencePrecompileTestSuite(t *testing.T) {
	s := evidence.NewPrecompileTestSuite(integration.CreateEvmd)
	suite.Run(t, s)
}
