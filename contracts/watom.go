package contracts

import (
	_ "embed" // Required for the go:embed directive
	"fmt" // Added for clearer panic message

	contractutils "github.com/cosmos/evm/contracts/utils"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

var (
	// WATOMJSON embeds the raw JSON compilation artifact for the WATOM contract.
	// This JSON file contains the contract's ABI and bytecode.
	//
	//go:embed solidity/WATOM.json
	WATOMJSON []byte

	// WATOMContract holds the processed, compiled contract structure
	// ready for use by the EVM module.
	WATOMContract evmtypes.CompiledContract
)

// init runs automatically when the package is loaded. It deserializes the embedded JSON
// into a CompiledContract struct.
func init() {
	var err error
	
	// Deserialize the embedded Hardhat/Truffle JSON artifact into the EVM's internal struct.
	if WATOMContract, err = contractutils.ConvertHardhatBytesToCompiledContract(
		WATOMJSON,
	); err != nil {
		// Panic if loading or deserialization fails, as the application cannot function without core contracts.
		panic(fmt.Errorf("failed to convert embedded WATOM JSON to CompiledContract: %w", err))
	}
}
