package utils

import "time"

const (
	// TestSetupTimeout is the maximum time allowed for test environment setup
	TestSetupTimeout = 8 * time.Minute

	// ReceiptTimeout is the maximum time to wait for transaction receipts
	ReceiptTimeout = 20 * time.Second

	// RevertReasonTimeout is the timeout for RPC calls to extract revert reasons
	RevertReasonTimeout = 5 * time.Second

	// WaitReceiptPollInterval is how often to check for transaction receipts
	WaitReceiptPollInterval = 200 * time.Millisecond

	// Extended timeouts for special cases
	ICS02ReceiptTimeout   = 12 * time.Second
	StakingReceiptTimeout = 30 * time.Second
	UnbondingWaitTime     = 12 * time.Second
	ShortReceiptTimeout   = 5 * time.Second
	FailureReceiptTimeout = 4 * time.Second

	// Gas constants
	BasicTransferGas = 21000 // Standard gas cost for basic ETH transfers

	// Precompile addresses
	BankPrecompileAddr   = "0x0000000000000000000000000000000000000804"
	Bech32PrecompileAddr = "0x0000000000000000000000000000000000000400"
	WERC20Addr           = "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"

	// Test chain configuration (shared across e2e tests)
	TestChainID             = "evmd-1"
	TestEVMChainID   uint64 = 19460
	TestBech32Prefix        = "evmd"
	TestDenom               = "astake"
	DisplayDenom            = "stake"
)
