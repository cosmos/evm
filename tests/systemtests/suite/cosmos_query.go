package suite

import (
	"fmt"
	"time"
)

func (s *SystemTestSuite) WaitForCosmosCommmit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	result, err := s.CosmosClient.WaitForCommit(nodeID, txHash, timeout)
	fmt.Println("CHOI - result: ", result)

	if err != nil {
		return fmt.Errorf("failed to get receipt for tx(%s): %v", txHash, err)
	}

	if result.TxResult.Code != 0 {
		return fmt.Errorf("tx(%s) is committed but failed: %v", result.Hash.String(), err)
	}

	return nil
}
