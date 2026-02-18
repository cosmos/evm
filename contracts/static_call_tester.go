package contracts

import (
	_ "embed"

	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

var (
	// StaticCallTesterJSON are the compiled bytes of the StaticCallTesterContract
	//
	//go:embed solidity/StaticCallTester.json
	StaticCallTesterJSON []byte

	// StaticCallTesterContract is the compiled StaticCallTester contract
	StaticCallTesterContract evmtypes.CompiledContract

	// StaticCallbackTargetJSON are the compiled bytes of the StaticCallbackTarget contract
	//
	//go:embed solidity/StaticCallbackTarget.json
	StaticCallbackTargetJSON []byte

	// StaticCallbackTargetContract is the compiled contract for cross-contract static call testing
	StaticCallbackTargetContract evmtypes.CompiledContract

	// ViewCallerJSON are the compiled bytes of the ViewCaller contract
	//
	//go:embed solidity/ViewCaller.json
	ViewCallerJSON []byte

	// ViewCallerContract is the compiled contract for invoking view functions
	ViewCallerContract evmtypes.CompiledContract
)

func init() {
	var err error
	if StaticCallTesterContract, err = contractutils.ConvertHardhatBytesToCompiledContract(
		StaticCallTesterJSON,
	); err != nil {
		panic(err)
	}

	if StaticCallbackTargetContract, err = contractutils.ConvertHardhatBytesToCompiledContract(
		StaticCallbackTargetJSON,
	); err != nil {
		panic(err)
	}

	if ViewCallerContract, err = contractutils.ConvertHardhatBytesToCompiledContract(
		ViewCallerJSON,
	); err != nil {
		panic(err)
	}
}
