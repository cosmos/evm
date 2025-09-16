package eips

import (
	"github.com/ethereum/go-ethereum/core/vm"
)

var (
	// CosmosEVMActivators defines a map of opcode modifiers associated
	// with a key defining the corresponding EIP.
	CosmosEVMActivators = map[int]func(*vm.JumpTable){
		0o000: Enable0000,
		0o001: Enable0001,
		0o002: Enable0002,
	}
)
