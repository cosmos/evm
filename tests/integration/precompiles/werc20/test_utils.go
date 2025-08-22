package werc20

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/keyring"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// callType constants to differentiate between
// the different types of call to the precompile.
type callType int

const (
	directCall callType = iota
	contractCall
)

// CallsData is a helper struct to hold the addresses and ABIs for the
// different contract instances that are subject to testing here.
type CallsData struct {
	// This field is used to perform transactions that are not relevant for
	// testing purposes like query to the contract.
	sender keyring.Key

	// precompileReverter is used to call into the werc20 interface and
	precompileReverterAddr common.Address
	precompileReverterABI  abi.ABI

	precompileAddr common.Address
	precompileABI  abi.ABI
}

// getTxCallArgs is a helper function to return the correct call arguments and
// transaction data for a given call type.
func (cd CallsData) getTxAndCallArgs(
	callType callType,
	methodName string,
	args ...interface{},
) (evmtypes.EvmTxArgs, testutiltypes.CallArgs) {
	txArgs := evmtypes.EvmTxArgs{}
	callArgs := testutiltypes.CallArgs{}

	switch callType {
	case directCall:
		txArgs.To = &cd.precompileAddr
		callArgs.ContractABI = cd.precompileABI
	case contractCall:
		txArgs.To = &cd.precompileReverterAddr
		callArgs.ContractABI = cd.precompileReverterABI
	}

	callArgs.MethodName = methodName
	callArgs.Args = args

	// Setting gas tip cap to zero to have zero gas price.
	txArgs.GasTipCap = new(big.Int).SetInt64(0)
	// Gas limit is added only to skip the estimate gas call
	// that makes debugging more complex.
	txArgs.GasLimit = 1_000_000_000_000

	return txArgs, callArgs
}

// -------------------------------------------------------------------------------------------------
// Balance management utilities
// -------------------------------------------------------------------------------------------------

// AccountType represents different account types in the test
type AccountType int

const (
	Sender AccountType = iota
	Receiver
	Precompile
	Contract
	PrecisebankModule
)

// String returns the string representation of AccountType
func (at AccountType) String() string {
	switch at {
	case Sender:
		return "sender"
	case Receiver:
		return "receiver"
	case Precompile:
		return "precompile"
	case Contract:
		return "contract"
	case PrecisebankModule:
		return "precisebank module"
	default:
		return "unknown"
	}
}

// BalanceSnapshot represents a snapshot of account balances for testing
type BalanceSnapshot struct {
	IntegerBalance    *big.Int
	FractionalBalance *big.Int
}

// AccountBalanceInfo holds balance tracking information for a test account
type AccountBalanceInfo struct {
	AccountType     AccountType
	Address         sdk.AccAddress
	BeforeSnapshot  *BalanceSnapshot
	IntegerDelta    *big.Int
	FractionalDelta *big.Int
}

// InitializeAccountBalances creates the account balance tracking slice with proper addresses
func InitializeAccountBalances(
	senderAddr, receiverAddr sdk.AccAddress,
	precompileAddr, contractAddr common.Address,
) []*AccountBalanceInfo {
	precisebankModuleAddr := authtypes.NewModuleAddress(precisebanktypes.ModuleName)
	return []*AccountBalanceInfo{
		{AccountType: Sender, Address: senderAddr},
		{AccountType: Receiver, Address: receiverAddr},
		{AccountType: Precompile, Address: precompileAddr.Bytes()},
		{AccountType: Contract, Address: contractAddr.Bytes()},
		{AccountType: PrecisebankModule, Address: precisebankModuleAddr},
	}
}

// ResetExpectedDeltas resets all account balance deltas to zero
func ResetExpectedDeltas(accounts []*AccountBalanceInfo) {
	for _, account := range accounts {
		account.IntegerDelta = big.NewInt(0)
		account.FractionalDelta = big.NewInt(0)
	}
}

// TakeBalanceSnapshots captures current balance states for all accounts
func TakeBalanceSnapshots(accounts []*AccountBalanceInfo, grpcHandler grpc.Handler) {
	for _, account := range accounts {
		snapshot, err := GetBalanceSnapshot(account.Address, grpcHandler)
		Expect(err).ToNot(HaveOccurred(), "failed to take balance snapshots")
		account.BeforeSnapshot = snapshot
	}
}

// VerifyBalanceChanges verifies expected balance changes for all accounts
func VerifyBalanceChanges(
	accounts []*AccountBalanceInfo,
	grpcHandler grpc.Handler,
	expectedRemainder *big.Int,
) {
	for _, account := range accounts {
		ExpectBalanceChange(account.Address, account.BeforeSnapshot,
			account.IntegerDelta, account.FractionalDelta, account.AccountType.String(), grpcHandler)
	}

	res, err := grpcHandler.Remainder()
	Expect(err).ToNot(HaveOccurred(), "failed to get precisebank module remainder")
	actualRemainder := res.Remainder.Amount.BigInt()
	Expect(actualRemainder).To(Equal(expectedRemainder))
}

// GetAccountBalance returns the AccountBalanceInfo for a given account type
func GetAccountBalance(accounts []*AccountBalanceInfo, accountType AccountType) *AccountBalanceInfo {
	for _, account := range accounts {
		if account.AccountType == accountType {
			return account
		}
	}
	return nil
}

// GetBalanceSnapshot gets complete balance information using grpcHandler
func GetBalanceSnapshot(addr sdk.AccAddress, grpcHandler grpc.Handler) (*BalanceSnapshot, error) {
	// Get integer balance (uatom)
	intRes, err := grpcHandler.GetBalanceFromBank(addr, evmtypes.GetEVMCoinDenom())
	if err != nil {
		return nil, fmt.Errorf("failed to get integer balance: %w", err)
	}

	// Get fractional balance using the new grpcHandler method
	fracRes, err := grpcHandler.FractionalBalance(addr)
	if err != nil {
		return nil, fmt.Errorf("failed to get fractional balance: %w", err)
	}

	return &BalanceSnapshot{
		IntegerBalance:    intRes.Balance.Amount.BigInt(),
		FractionalBalance: fracRes.FractionalBalance.Amount.BigInt(),
	}, nil
}

// ExpectBalanceChange verifies expected balance changes after operations
func ExpectBalanceChange(
	addr sdk.AccAddress,
	beforeSnapshot *BalanceSnapshot,
	expectedIntegerDelta *big.Int,
	expectedFractionalDelta *big.Int,
	description string,
	grpcHandler grpc.Handler,
) {
	afterSnapshot, err := GetBalanceSnapshot(addr, grpcHandler)
	Expect(err).ToNot(HaveOccurred(), "failed to get balance snapshot for %s", description)

	actualIntegerDelta := new(big.Int).Sub(afterSnapshot.IntegerBalance, beforeSnapshot.IntegerBalance)
	actualFractionalDelta := new(big.Int).Sub(afterSnapshot.FractionalBalance, beforeSnapshot.FractionalBalance)

	Expect(actualIntegerDelta.Cmp(expectedIntegerDelta)).To(Equal(0),
		"integer balance delta mismatch for %s: expected %s, got %s",
		description, expectedIntegerDelta.String(), actualIntegerDelta.String())

	Expect(actualFractionalDelta.Cmp(expectedFractionalDelta)).To(Equal(0),
		"fractional balance delta mismatch for %s: expected %s, got %s",
		description, expectedFractionalDelta.String(), actualFractionalDelta.String())
}
