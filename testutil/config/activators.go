package config

import (
	"github.com/ethereum/go-ethereum/core/vm"
)

// cosmosEVMActivators defines a map of opcode modifiers associated
// with a key defining the corresponding EIP.
var cosmosEVMActivators = map[int]func(*vm.JumpTable){
	0o000: Enable0000,
	0o001: Enable0001,
	0o002: Enable0002,
}
