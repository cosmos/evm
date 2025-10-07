package eip7702

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

type UserOperation struct {
	Sender               common.Address
	Nonce                *big.Int
	InitCode             []byte
	CallData             []byte
	CallGasLimit         *big.Int
	VerificationGasLimit *big.Int
	PreVerificationGas   *big.Int
	MaxFeePerGas         *big.Int
	MaxPriorityFeePerGas *big.Int
	PaymasterAndData     []byte
	Signature            []byte
}

func NewUserOperation(sender common.Address, nonce uint64, calldata []byte) *UserOperation {
	return &UserOperation{
		Sender:               sender,
		Nonce:                big.NewInt(int64(nonce)), //#nosec G115
		InitCode:             []byte{},
		CallData:             calldata,
		CallGasLimit:         big.NewInt(100000),
		VerificationGasLimit: big.NewInt(200000),
		PreVerificationGas:   big.NewInt(50000),
		MaxFeePerGas:         big.NewInt(900000000),
		MaxPriorityFeePerGas: big.NewInt(100000000),
		PaymasterAndData:     []byte{},
		Signature:            []byte{},
	}
}

func SignUserOperation(userOp *UserOperation, entryPointAddr common.Address, privKey cryptotypes.PrivKey) (*UserOperation, error) {
	chainID := new(big.Int).SetUint64(evmtypes.GetChainConfig().GetChainId())

	addressType, _ := abi.NewType("address", "", nil)
	uint256Type, _ := abi.NewType("uint256", "", nil)
	bytes32Type, _ := abi.NewType("bytes32", "", nil)

	args := abi.Arguments{
		{Type: addressType}, // sender
		{Type: uint256Type}, // nonce
		{Type: bytes32Type}, // keccak(initCode)
		{Type: bytes32Type}, // keccak(callData)
		{Type: uint256Type}, // callGasLimit
		{Type: uint256Type}, // verificationGasLimit
		{Type: uint256Type}, // preVerificationGas
		{Type: uint256Type}, // maxFeePerGas
		{Type: uint256Type}, // maxPriorityFeePerGas
		{Type: bytes32Type}, // keccak(paymasterAndData)
		{Type: addressType}, // entryPoint
		{Type: uint256Type}, // chainId
	}

	packed, err := args.Pack(
		userOp.Sender,
		userOp.Nonce,
		crypto.Keccak256Hash(userOp.InitCode),
		crypto.Keccak256Hash(userOp.CallData),
		userOp.CallGasLimit,
		userOp.VerificationGasLimit,
		userOp.PreVerificationGas,
		userOp.MaxFeePerGas,
		userOp.MaxPriorityFeePerGas,
		crypto.Keccak256Hash(userOp.PaymasterAndData),
		entryPointAddr,
		chainID,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to pack arguments of UserOperation")
	}

	userOpHash := crypto.Keccak256Hash(packed)

	ecdsaPrivKey, err := privKey.(*ethsecp256k1.PrivKey).ToECDSA()
	if err != nil {
		return nil, fmt.Errorf("failed to convert private key to ecdsa private key")
	}

	signature, err := crypto.Sign(userOpHash.Bytes(), ecdsaPrivKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign user operationHash")
	}

	// Transform V from 0/1 to 27/28 according to the yellow paper
	if signature[64] < 27 {
		signature[64] += 27
	}

	userOp.Signature = signature
	return userOp, nil
}
