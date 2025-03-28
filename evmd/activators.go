package evmd

import (
	"github.com/cosmos/evm/evmd/eips"
	"github.com/cosmos/evm/x/vm/core/vm"
)

// cosmosEVMActivators defines a map of opcode modifiers associated
// with a key defining the corresponding EIP.
//
//nolint:unused
var cosmosEVMActivators = map[string]func(*vm.JumpTable){
	"evmos_0": eips.Enable0000,
	"evmos_1": eips.Enable0001,
	"evmos_2": eips.Enable0002,
}
