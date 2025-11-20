//go:build system_test

package eip712

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

func (s *TestSuite) SendBankSendWithEIP712(
	t *testing.T,
	nodeID string,
	accID string,
	to sdk.AccAddress,
	amount *big.Int,
	nonceIdx uint64,
	gasPrice *big.Int,
) (string, error) {
	cosmosAccount := s.CosmosAccount(accID)

	ctx := s.CosmosClient.ClientCtx.WithClient(s.CosmosClient.RpcClients[nodeID])
	account, err := ctx.AccountRetriever.GetAccount(ctx, cosmosAccount.AccAddress)
	if err != nil {
		return "", fmt.Errorf("failed to query account for nonce: %w", err)
	}

	cosmosAccount.AccountNumber = account.GetAccountNumber()
	actualNonce := account.GetSequence() + nonceIdx

	resp, err := BankSendWithEIP712(
		s.CosmosClient,
		cosmosAccount,
		nodeID,
		cosmosAccount.AccAddress,
		to,
		sdkmath.NewIntFromBigInt(amount),
		actualNonce,
		gasPrice,
	)
	if err != nil {
		return "", fmt.Errorf("failed to send bank send with EIP-712: %w", err)
	}

	return resp.TxHash, nil
}

func (s *TestSuite) GetBalance(
	t *testing.T,
	nodeID string,
	address sdk.AccAddress,
	denom string,
) (*big.Int, error) {
	balance, err := s.CosmosClient.GetBalance(nodeID, address, denom)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	return balance, nil
}

func (s *TestSuite) WaitForCommit(nodeID string, txHash string, timeout ...int) error {
	duration := 15 * time.Second
	if len(timeout) > 0 && timeout[0] > 0 {
		duration = time.Duration(timeout[0]) * time.Second
	}
	return s.BaseTestSuite.WaitForCommit(nodeID, txHash, basesuite.TxTypeCosmos, duration)
}
