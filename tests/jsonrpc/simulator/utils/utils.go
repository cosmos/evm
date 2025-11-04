package utils

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rpc"

	"github.com/cosmos/evm/tests/jsonrpc/simulator/types"
)

const (
	RED    = "#FF0000"
	YELLOW = "#FFFF00"
	GREEN  = "#00FF00"
)

// MustCreateRandomAccount creates a new Ethereum account with a random private key
func MustCreateRandomAccount() types.Account {
	// Create a new account
	acc := types.Account{}
	var err error
	if acc.PrivKey, err = crypto.GenerateKey(); err != nil {
		log.Fatal(err)
	}
	acc.Address = crypto.PubkeyToAddress(acc.PrivKey.PublicKey)
	return acc
}

// MustBeautifyBlock formats and prints an Ethereum block in a readable JSON format
func MustBeautifyBlock(block *types.RpcBlock) string {
	blockJSON, err := json.MarshalIndent(block, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal block: %v", err)
	}
	return string(blockJSON)
}

// MustBeautifyReceipt formats and prints an Ethereum receipt in a readable JSON format
func MustBeautifyReceipt(receipt *gethtypes.Receipt) string {
	receiptJSON, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal receipt: %v", err)
	}
	return string(receiptJSON)
}

// MustBeautifyReceipts formats and prints a list of Ethereum receipts in a readable JSON format
func MustBeautifyReceipts(receipts gethtypes.Receipts) string {
	receiptsJSON, err := json.MarshalIndent(receipts, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal receipts: %v", err)
	}
	return string(receiptsJSON)
}

// MustBeautifyTransaction formats and prints an Ethereum transaction in a readable JSON format
func MustBeautifyTransaction(tx *gethtypes.Transaction) string {
	// First, use the default MarshalJSON to get the serialized data
	txJSON, err := tx.MarshalJSON()
	if err != nil {
		log.Fatalf("Failed to marshal transaction: %v", err)
	}

	// Then, unmarshal the serialized data into a map
	var txMap map[string]interface{}
	if err := json.Unmarshal(txJSON, &txMap); err != nil {
		log.Fatalf("Failed to unmarshal transaction: %v", err)
	}

	// Finally, marshal the map with indentation
	indentedTxJSON, err := json.MarshalIndent(txMap, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal indented transaction: %v", err)
	}

	return string(indentedTxJSON)
}

// ExtractRuntimeBytecodeHex strips the init code suffix (F3FE...) and returns the runtime bytecode as hex string with 0x prefix.
func ExtractRuntimeBytecodeHex(creation []byte) (string, error) {
	marker := []byte{0xf3, 0xfe}
	idx := bytes.Index(creation, marker)
	if idx == -1 {
		return "", fmt.Errorf("runtime marker not found in creation bytecode")
	}
	runtime := creation[idx+len(marker):]
	return hexutil.Encode(runtime), nil
}

func MustCalculateSlotKey(addr common.Address, slotIndex uint64) common.Hash {
	addressTy, err := abi.NewType("address", "", nil)
	if err != nil {
		log.Fatalf("Failed to create address type: %v", err)
	}
	uint256Ty, err := abi.NewType("uint256", "", nil)
	slotIndexBig := new(big.Int).SetUint64(slotIndex)
	packedArgs, err := abi.Arguments{
		{Type: addressTy},
		{Type: uint256Ty},
	}.Pack(addr, slotIndexBig)
	if err != nil {
		log.Fatalf("Failed to pack arguments: %v", err)
	}

	return crypto.Keccak256Hash(packedArgs)
}

// IsZeroBytes checks if a byte slice consists only of zero bytes
func IsZeroBytes(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func MustBeautifyLogs(logs []gethtypes.Log) string {
	receiptsJSON, err := json.MarshalIndent(logs, "", "  ")
	if err != nil {
		log.Fatalf("Failed to marshal receipts: %v", err)
	}
	return string(receiptsJSON)
}

func ToFilterArg(q ethereum.FilterQuery) (interface{}, error) {
	arg := map[string]interface{}{
		"address": q.Addresses,
		"topics":  q.Topics,
	}
	if q.BlockHash != nil {
		arg["blockHash"] = *q.BlockHash
		if q.FromBlock != nil || q.ToBlock != nil {
			return nil, errors.New("cannot specify both BlockHash and FromBlock/ToBlock")
		}
	} else {
		if q.FromBlock == nil {
			arg["fromBlock"] = "0x0"
		} else {
			arg["fromBlock"] = toBlockNumArg(q.FromBlock)
		}
		arg["toBlock"] = toBlockNumArg(q.ToBlock)
	}
	return arg, nil
}

func toBlockNumArg(number *big.Int) string {
	if number == nil {
		return "latest"
	}
	if number.Sign() >= 0 {
		return hexutil.EncodeBig(number)
	}
	// It's negative.
	if number.IsInt64() {
		return rpc.BlockNumber(number.Int64()).String()
	}
	// It's negative and large, which is invalid.
	return fmt.Sprintf("<invalid %d>", number)
}

// GetPrivateKeyAndAddress returns the private key and address for a given private key string
func GetPrivateKeyAndAddress(privateKeyHex string) (*ecdsa.PrivateKey, common.Address, error) {
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return nil, common.Address{}, err
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, common.Address{}, fmt.Errorf("error casting public key to ECDSA")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	return privateKey, address, nil
}
