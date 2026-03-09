package integration

import (
	"testing"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/tests/integration/indexer"
	testapp "github.com/cosmos/evm/testutil/app"
)

func TestKVIndexer(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")
	indexer.TestKVIndexer(t, create)
}

func TestBankTransferTransformer(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")

	t.Run("CanHandle", func(t *testing.T) {
		indexer.TestBankTransferTransformer(t, create)
	})
	t.Run("Transform", func(t *testing.T) {
		indexer.TestBankTransferTransformerTransform(t, create)
	})
	t.Run("DeterministicHash", func(t *testing.T) {
		indexer.TestBankTransferTransformerDeterministicHash(t, create)
	})
}

func TestStakingDelegateTransformer(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")

	t.Run("CanHandle", func(t *testing.T) {
		indexer.TestStakingDelegateTransformer(t, create)
	})
	t.Run("Transform", func(t *testing.T) {
		indexer.TestStakingDelegateTransformerTransform(t, create)
	})
	t.Run("DeterministicHash", func(t *testing.T) {
		indexer.TestStakingDelegateTransformerDeterministicHash(t, create)
	})
}

func TestTransformer(t *testing.T) {
	create := testapp.ToEvmAppCreator[evm.IntegrationNetworkApp](CreateEvmd, "evm.IntegrationNetworkApp")

	t.Run("CosmosEventOnly", func(t *testing.T) {
		indexer.TestTransformerCosmosEventOnly(t, create)
	})
	t.Run("NoTransformerSkipsEvent", func(t *testing.T) {
		indexer.TestTransformerNoTransformerSkipsEvent(t, create)
	})
	t.Run("StakingDelegate", func(t *testing.T) {
		indexer.TestTransformerStakingDelegate(t, create)
	})
	t.Run("MultipleTransformers", func(t *testing.T) {
		indexer.TestTransformerMultipleTransformers(t, create)
	})
	t.Run("LogIndexOrdering", func(t *testing.T) {
		indexer.TestTransformerLogIndexOrdering(t, create)
	})
	t.Run("MultiplePhases", func(t *testing.T) {
		indexer.TestTransformerMultiplePhases(t, create)
	})
	t.Run("PhaseEthTxIndexOrdering", func(t *testing.T) {
		indexer.TestTransformerPhaseEthTxIndexOrdering(t, create)
	})
	t.Run("EmptyPhaseSkipped", func(t *testing.T) {
		indexer.TestTransformerEmptyPhaseSkipped(t, create)
	})
	t.Run("MixedPhasesAndDeliverTx", func(t *testing.T) {
		indexer.TestTransformerMixedPhasesAndDeliverTx(t, create)
	})
	t.Run("CumulativeGasUsed", func(t *testing.T) {
		indexer.TestTransformerCumulativeGasUsed(t, create)
	})
}
