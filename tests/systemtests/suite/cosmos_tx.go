package suite

import (
	"fmt"
	"math/big"

	sdkmath "cosmossdk.io/math"
)

func (s *SystemTestSuite) SendCosmosTx(
	nodeID string,
	accID string,
	nonce uint64,
	gasPrice *big.Int,
	_ *big.Int,
) (string, error) {
	from := s.CosmosClient.Accs[accID].AccAddress
	to := s.CosmosClient.Accs["acc3"].AccAddress
	amount := sdkmath.NewInt(1000)

	resp, err := s.CosmosClient.BankSend(nodeID, accID, from, to, amount, nonce, gasPrice)
	if err != nil {
		return "", fmt.Errorf("failed to cosmos tx bank send: %v", err)
	}
	return resp.TxHash, nil
}
