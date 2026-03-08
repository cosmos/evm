//go:build system_test

package indexer

import (
	"fmt"
	"math/big"
	"testing"
	"time"

	sdkmath "cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	basesuite "github.com/cosmos/evm/tests/systemtests/suite"
)

type TestSuite struct {
	*basesuite.BaseTestSuite
}

func NewTestSuite(base *basesuite.BaseTestSuite) *TestSuite {
	return &TestSuite{BaseTestSuite: base}
}

// SendCosmosBankSend sends a bank send transaction using Cosmos SDK.
func (s *TestSuite) SendCosmosBankSend(
	t *testing.T,
	nodeID string,
	accID string,
	to sdk.AccAddress,
	amount *big.Int,
	gasPrice *big.Int,
) (string, error) {
	cosmosAccount := s.CosmosAccount(accID)
	from := cosmosAccount.AccAddress

	ctx := s.CosmosClient.ClientCtx.WithClient(s.CosmosClient.RpcClients[nodeID])
	account, err := ctx.AccountRetriever.GetAccount(ctx, cosmosAccount.AccAddress)
	if err != nil {
		return "", fmt.Errorf("failed to query account: %w", err)
	}

	nonce := account.GetSequence()

	resp, err := s.CosmosClient.BankSend(nodeID, cosmosAccount, from, to, sdkmath.NewIntFromBigInt(amount), nonce, gasPrice)
	if err != nil {
		return "", fmt.Errorf("failed to send cosmos bank send: %w", err)
	}

	return resp.TxHash, nil
}

// WaitForCommit waits for the cosmos tx to be committed.
func (s *TestSuite) WaitForCommit(nodeID string, txHash string, timeout ...int) error {
	duration := 15 * time.Second
	if len(timeout) > 0 && timeout[0] > 0 {
		duration = time.Duration(timeout[0]) * time.Second
	}
	return s.BaseTestSuite.WaitForCommit(nodeID, txHash, basesuite.TxTypeCosmos, duration)
}

// SendCosmosDelegate sends a staking delegate transaction.
func (s *TestSuite) SendCosmosDelegate(
	t *testing.T,
	nodeID string,
	accID string,
	validator sdk.ValAddress,
	amount *big.Int,
	gasPrice *big.Int,
) (string, error) {
	cosmosAccount := s.CosmosAccount(accID)

	ctx := s.CosmosClient.ClientCtx.WithClient(s.CosmosClient.RpcClients[nodeID])
	account, err := ctx.AccountRetriever.GetAccount(ctx, cosmosAccount.AccAddress)
	if err != nil {
		return "", fmt.Errorf("failed to query account: %w", err)
	}

	nonce := account.GetSequence()

	resp, err := s.CosmosClient.Delegate(nodeID, cosmosAccount, cosmosAccount.AccAddress, validator, sdkmath.NewIntFromBigInt(amount), nonce, gasPrice)
	if err != nil {
		return "", fmt.Errorf("failed to send cosmos delegate: %w", err)
	}

	return resp.TxHash, nil
}

// SendCosmosUndelegate sends a staking undelegate transaction.
func (s *TestSuite) SendCosmosUndelegate(
	t *testing.T,
	nodeID string,
	accID string,
	validator sdk.ValAddress,
	amount *big.Int,
	gasPrice *big.Int,
) (string, error) {
	cosmosAccount := s.CosmosAccount(accID)

	ctx := s.CosmosClient.ClientCtx.WithClient(s.CosmosClient.RpcClients[nodeID])
	account, err := ctx.AccountRetriever.GetAccount(ctx, cosmosAccount.AccAddress)
	if err != nil {
		return "", fmt.Errorf("failed to query account: %w", err)
	}

	nonce := account.GetSequence()

	resp, err := s.CosmosClient.Undelegate(nodeID, cosmosAccount, cosmosAccount.AccAddress, validator, sdkmath.NewIntFromBigInt(amount), nonce, gasPrice)
	if err != nil {
		return "", fmt.Errorf("failed to send cosmos undelegate: %w", err)
	}

	return resp.TxHash, nil
}
