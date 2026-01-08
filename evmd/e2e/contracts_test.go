package e2e

import (
	"math/big"
	"strings"
	"testing"

	"github.com/cosmos/evm/evmd/e2e/contracts"
	"github.com/cosmos/evm/evmd/e2e/testharness"
	"github.com/cosmos/evm/evmd/e2e/utils"
	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

// TestContractDeployEvents deploys a tiny contract, mutates state, and asserts event logs.
func TestContractDeployEvents(t *testing.T) {
	t.Parallel()
	req := require.New(t)
	harness := testharness.CreateHarness(t)
	chain := harness.Chain

	// Parse ABI and load creation bytecode
	parsedABI, err := abi.JSON(strings.NewReader(contracts.CounterABIJSON()))
	req.NoError(err)
	creation := common.FromHex(contracts.CounterBinHex())

	// Deploy contract (contract creation tx)
	txHash, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, nil, big.NewInt(0), creation, 0)
	req.NoError(err)

	rec, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec)
	req.Equal(uint64(1), rec.Status)
	contractAddr := rec.ContractAddress
	req.NotEqual(common.Address{}, contractAddr)

	// Verify code exists at contract address
	code, err := chain.EthClient.CodeAt(harness.Ctx, contractAddr, nil)
	req.NoError(err)
	req.NotEmpty(code)

	// Initial state via storage and call
	slot0 := common.Hash{} // 0x00...00
	raw0, err := chain.EthClient.StorageAt(harness.Ctx, contractAddr, slot0, nil)
	req.NoError(err)
	req.Len(raw0, 32)
	req.Zero(new(big.Int).SetBytes(raw0).Cmp(big.NewInt(0)))

	callValue, err := parsedABI.Pack("value")
	req.NoError(err)
	ret0, err := chain.EthClient.CallContract(harness.Ctx, ethereum.CallMsg{To: &contractAddr, Data: callValue}, nil)
	req.NoError(err)
	outs0, err := parsedABI.Unpack("value", ret0)
	req.NoError(err)
	req.Len(outs0, 1)
	req.Zero(outs0[0].(*big.Int).Cmp(big.NewInt(0)))

	// Mutate state via set(42)
	newVal := big.NewInt(42)
	callSet, err := parsedABI.Pack("set", newVal)
	req.NoError(err)

	txHash2, err := utils.SendTx(harness.Ctx, chain.EthClient, harness.SenderKey, &contractAddr, big.NewInt(0), callSet, 0)
	req.NoError(err)

	rec2, err := utils.WaitReceipt(harness.Ctx, chain.EthClient, txHash2, utils.ReceiptTimeout)
	req.NoError(err)
	req.NotNil(rec2)
	req.Equal(uint64(1), rec2.Status)

	// Re-verify state
	raw1, err := chain.EthClient.StorageAt(harness.Ctx, contractAddr, slot0, nil)
	req.NoError(err)
	req.Len(raw1, 32)
	req.Zero(new(big.Int).SetBytes(raw1).Cmp(newVal))

	ret1, err := chain.EthClient.CallContract(harness.Ctx, ethereum.CallMsg{To: &contractAddr, Data: callValue}, nil)
	req.NoError(err)
	outs1, err := parsedABI.Unpack("value", ret1)
	req.NoError(err)
	req.Len(outs1, 1)
	req.Zero(outs1[0].(*big.Int).Cmp(newVal))

	// Assert emitted event via eth_getLogs
	topic0 := crypto.Keccak256Hash([]byte("ValueChanged(uint256,address)"))
	logs, err := chain.EthClient.FilterLogs(harness.Ctx, ethereum.FilterQuery{
		FromBlock: rec2.BlockNumber,
		ToBlock:   rec2.BlockNumber,
		Addresses: []common.Address{contractAddr},
		Topics:    [][]common.Hash{{topic0}},
	})
	req.NoError(err)
	req.NotEmpty(logs)

	var matched *types.Log
	for i := range logs {
		if logs[i].TxHash == rec2.TxHash {
			matched = &logs[i]
			break
		}
	}
	req.NotNil(matched)
	req.GreaterOrEqual(len(matched.Topics), 2)
	req.Equal(topic0, matched.Topics[0])

	// topic1 is indexed caller address
	expectedTopic1 := common.BytesToHash(common.LeftPadBytes(harness.SenderAddr.Bytes(), 32))
	req.Equal(expectedTopic1, matched.Topics[1])

	ev, ok := parsedABI.Events["ValueChanged"]
	req.True(ok)
	vals, err := ev.Inputs.NonIndexed().Unpack(matched.Data)
	req.NoError(err)
	req.Len(vals, 1)
	req.Zero(vals[0].(*big.Int).Cmp(newVal))
}
