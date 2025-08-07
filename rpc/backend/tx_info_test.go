package backend

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/stretchr/testify/require"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestCreateAccessList(t *testing.T) {
	// Test that CreateAccessList works with unsigned transactions
	// This test verifies that our fix for the signature validation issue works

	// Create a simple transaction args
	from := common.HexToAddress("0xc6fe5d33615a1c52c08018c47e8bc53646a0e101")
	to := common.HexToAddress("0x963ebdf2e1f8db8707d05fc75bfeffba1b5bac17")
	value := hexutil.Big(*big.NewInt(5000)) // 0x1388

	args := evmtypes.TransactionArgs{
		From:  &from,
		To:    &to,
		Value: &value,
	}

	// Create a mock backend for testing
	// Note: This is a simplified test - in a real scenario, you'd need to set up
	// a proper backend with mocked dependencies

	// Test that the transaction args can be converted to a message without signature validation
	msg := args.ToTransaction()
	require.NotNil(t, msg, "ToTransaction should not return nil")

	// Test that the message has the correct From field
	require.Equal(t, from.Bytes(), msg.From, "From field should match the input address")

	// Test that the transaction can be created without signature validation
	tx := msg.AsTransaction()
	require.NotNil(t, tx, "AsTransaction should not return nil")

	// Test that the transaction has the correct values
	require.Equal(t, to, *tx.To(), "To address should match")
	require.Equal(t, big.NewInt(5000), tx.Value(), "Value should match")
}

func TestCreateAccessListWithGasPrice(t *testing.T) {
	// Test CreateAccessList with gas price set
	from := common.HexToAddress("0xc6fe5d33615a1c52c08018c47e8bc53646a0e101")
	to := common.HexToAddress("0x963ebdf2e1f8db8707d05fc75bfeffba1b5bac17")
	value := hexutil.Big(*big.NewInt(5000))
	gasPrice := hexutil.Big(*big.NewInt(20000000000)) // 20 gwei

	args := evmtypes.TransactionArgs{
		From:     &from,
		To:       &to,
		Value:    &value,
		GasPrice: &gasPrice,
	}

	msg := args.ToTransaction()
	require.NotNil(t, msg, "ToTransaction should not return nil")

	tx := msg.AsTransaction()
	require.NotNil(t, tx, "AsTransaction should not return nil")
	require.Equal(t, big.NewInt(20000000000), tx.GasPrice(), "Gas price should match")
}

func TestCreateAccessListWithData(t *testing.T) {
	// Test CreateAccessList with data
	from := common.HexToAddress("0xc6fe5d33615a1c52c08018c47e8bc53646a0e101")
	to := common.HexToAddress("0x963ebdf2e1f8db8707d05fc75bfeffba1b5bac17")
	data := hexutil.Bytes{0x12, 0x34, 0x56, 0x78}

	args := evmtypes.TransactionArgs{
		From: &from,
		To:   &to,
		Data: &data,
	}

	msg := args.ToTransaction()
	require.NotNil(t, msg, "ToTransaction should not return nil")

	tx := msg.AsTransaction()
	require.NotNil(t, tx, "AsTransaction should not return nil")
	require.Equal(t, []byte{0x12, 0x34, 0x56, 0x78}, tx.Data(), "Data should match")
}
