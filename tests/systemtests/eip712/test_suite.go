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

type SystemTestSuite struct {
	*basesuite.SystemTestSuite
}

func NewSystemTestSuite(base *basesuite.SystemTestSuite) *SystemTestSuite {
	return &SystemTestSuite{SystemTestSuite: base}
}

func (s *SystemTestSuite) SetupTest(t *testing.T, nodeStartArgs ...string) {
	s.SystemTestSuite.SetupTest(t, nodeStartArgs...)
}

func (s *SystemTestSuite) BeforeEachCase(t *testing.T) {}

func (s *SystemTestSuite) AfterEachCase(t *testing.T) {
	s.AwaitNBlocks(t, 1)
}

func (s *SystemTestSuite) SendBankSendWithEIP712(
	t *testing.T,
	nodeID string,
	accID string,
	to sdk.AccAddress,
	amount *big.Int,
	nonce uint64,
	gasPrice *big.Int,
) (string, error) {
	cosmosAccount := s.CosmosAccount(accID)

	resp, err := BankSendWithEIP712(
		s.CosmosClient,
		cosmosAccount,
		nodeID,
		cosmosAccount.AccAddress,
		to,
		sdkmath.NewIntFromBigInt(amount),
		nonce,
		gasPrice,
	)
	if err != nil {
		return "", fmt.Errorf("failed to send bank send with EIP-712: %w", err)
	}

	return resp.TxHash, nil
}

func (s *SystemTestSuite) GetBalance(
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

func (s *SystemTestSuite) WaitForCommit(nodeID string, txHash string, timeout ...int) error {
	duration := 15 * time.Second
	if len(timeout) > 0 && timeout[0] > 0 {
		duration = time.Duration(timeout[0]) * time.Second
	}
	return s.SystemTestSuite.WaitForCommit(nodeID, txHash, basesuite.TxTypeCosmos, duration)
}
