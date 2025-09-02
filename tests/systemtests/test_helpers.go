package systemtests

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/evmos/tests/systemtests/clients"
)

func TransferLegacyTx(
	ethClient *clients.EthClient,
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	_ *big.Int,
) (common.Hash, error) {
	to := ethClient.Accs["acc3"].Address
	value := big.NewInt(1000)
	gasLimit := uint64(50_000)

	tx := ethtypes.NewTransaction(nonce, to, value, gasLimit, gasPrice, nil)

	return ethClient.SendRawTransaction(nodeID, accID, tx)
}

func TransferDynamicFeeTx(
	ethClient *clients.EthClient,
	nodeID string,
	accID string,
	nonce uint64,
	gasFeeCap *big.Int,
	gasTipCap *big.Int,
) (common.Hash, error) {
	tx := ethtypes.NewTx(&ethtypes.DynamicFeeTx{
		ChainID:   ethClient.ChainID,
		Nonce:     nonce,
		To:        &(ethClient.Accs["acc3"].Address),
		Value:     big.NewInt(1000),
		Gas:       uint64(50_000),
		GasFeeCap: gasFeeCap,
		GasTipCap: gasTipCap,
	})

	return ethClient.SendRawTransaction(nodeID, accID, tx)
}

func FutureNonces(ethClient *clients.EthClient, nodeID string, accID string, index int) ([]uint64, error) {
	currentNonce, err := NonceAt(ethClient, nodeID, accID)
	if err != nil {
		return []uint64{}, fmt.Errorf("failed to get future nonces")
	}

	nonces := []uint64{currentNonce}
	for i := 0; i < index; i++ {
		nonces = append(nonces, currentNonce+uint64(i+1))
	}

	return nonces, nil
}

func NonceAt(ethClient *clients.EthClient, nodeID string, accID string) (uint64, error) {
	ctx, cli, addr := ethClient.RequestArgs(nodeID, accID)
	blockNumber, err := ethClient.Clients[nodeID].BlockNumber(ctx)
	if err != nil {
		return uint64(0), fmt.Errorf("failed to get block number from %s", nodeID)
	}
	if int64(blockNumber) < 0 {
		return uint64(0), fmt.Errorf("invaid block number %d", blockNumber)
	}
	return cli.NonceAt(ctx, addr, big.NewInt(int64(blockNumber)))
}

func PendingNonceAt(ethClient *clients.EthClient, nodeID string, accID string) (uint64, error) {
	ctx, cli, addr := ethClient.RequestArgs(nodeID, accID)
	return cli.PendingNonceAt(ctx, addr)
}

func BaseFee(ethClient *clients.EthClient, nodeID string) (*big.Int, error) {
	ctx, cli, _ := ethClient.RequestArgs(nodeID, "acc0")
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
