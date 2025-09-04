package suite

import (
	"fmt"
	"math/big"
	"time"
)

func (s *SystemTestSuite) NonceAt(nodeID string, accID string) (uint64, error) {
	ctx, cli, addr := s.EthClient.RequestArgs(nodeID, accID)
	blockNumber, err := s.EthClient.Clients[nodeID].BlockNumber(ctx)
	if err != nil {
		return uint64(0), fmt.Errorf("failed to get block number from %s", nodeID)
	}
	if int64(blockNumber) < 0 {
		return uint64(0), fmt.Errorf("invaid block number %d", blockNumber)
	}
	return cli.NonceAt(ctx, addr, big.NewInt(int64(blockNumber)))
}

func (s *SystemTestSuite) GetLatestBaseFee(nodeID string) (*big.Int, error) {
	ctx, cli, _ := s.EthClient.RequestArgs(nodeID, "acc0")
	blockNumber, err := cli.BlockNumber(ctx)
	if err != nil {
		return nil, err
	}

	block, err := cli.BlockByNumber(ctx, big.NewInt(int64(blockNumber)))
	if err != nil {
		return nil, err
	}

	return block.BaseFee(), nil
}

func (s *SystemTestSuite) WaitForCommit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	if s.TestOption.TestType == TxTypeEVM {
		return s.waitForEthCommmit(nodeID, txHash, timeout)
	}
	return s.waitForCosmosCommmit(nodeID, txHash, timeout)
}

func (s *SystemTestSuite) waitForEthCommmit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	receipt, err := s.EthClient.WaitForCommit(nodeID, txHash, timeout)
	if err != nil {
		return fmt.Errorf("failed to get receipt for tx(%s): %v", txHash, err)
	}

	if receipt.Status != 1 {
		return fmt.Errorf("tx(%s) is committed but failed: %v", txHash, err)
	}

	return nil
}

func (s *SystemTestSuite) waitForCosmosCommmit(
	nodeID string,
	txHash string,
	timeout time.Duration,
) error {
	result, err := s.CosmosClient.WaitForCommit(nodeID, txHash, timeout)
	if err != nil {
		return fmt.Errorf("failed to get receipt for tx(%s): %v", txHash, err)
	}

	if result.TxResult.Code != 0 {
		return fmt.Errorf("tx(%s) is committed but failed: %v", result.Hash.String(), err)
	}

	return nil
}
