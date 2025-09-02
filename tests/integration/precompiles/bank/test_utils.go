package bank

import (
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/precompiles/bank"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	minttypes "github.com/cosmos/cosmos-sdk/x/mint/types"
)

// setupBankPrecompile is a helper function to set up an instance of the Bank precompile for
// a given token denomination.
func (s *PrecompileTestSuite) setupBankPrecompile() *bank.Precompile {
	return bank.NewPrecompile(
		s.network.App.GetBankKeeper(),
		*s.network.App.GetErc20Keeper(),
	)
}

// setupBankPrecompile is a helper function to set up an instance of the Bank precompile for
// a given token denomination.
func (is *IntegrationTestSuite) setupBankPrecompile() *bank.Precompile {
	return bank.NewPrecompile(
		is.network.App.GetBankKeeper(),
		*is.network.App.GetErc20Keeper(),
	)
}

// mintAndSendXMPLCoin is a helper function to mint and send a coin to a given address.
func (s *PrecompileTestSuite) mintAndSendXMPLCoin(ctx sdk.Context, addr sdk.AccAddress, amount math.Int) sdk.Context {
	coins := sdk.NewCoins(sdk.NewCoin(s.tokenDenom, amount))
	err := s.network.App.GetBankKeeper().MintCoins(ctx, minttypes.ModuleName, coins)
	s.Require().NoError(err)
	err = s.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(ctx, minttypes.ModuleName, addr, coins)
	s.Require().NoError(err)
	return ctx
}

// mintAndSendXMPLCoin is a helper function to mint and send a coin to a given address.
func (is *IntegrationTestSuite) mintAndSendXMPLCoin(addr sdk.AccAddress, amount math.Int) { //nolint:unused
	coins := sdk.NewCoins(sdk.NewCoin(is.tokenDenom, amount))
	err := is.network.App.GetBankKeeper().MintCoins(is.network.GetContext(), minttypes.ModuleName, coins)
	Expect(err).ToNot(HaveOccurred())
	err = is.network.App.GetBankKeeper().SendCoinsFromModuleToAccount(is.network.GetContext(), minttypes.ModuleName, addr, coins)
	Expect(err).ToNot(HaveOccurred())
}

// callType constants to differentiate between direct calls and calls through a contract.
const (
	directCall = iota + 1
	contractCall
)

// ContractData is a helper struct to hold the addresses and ABIs for the
// different contract instances that are subject to testing here.
type ContractData struct {
	ownerPriv cryptotypes.PrivKey

	contractAddr   common.Address
	contractABI    abi.ABI
	precompileAddr common.Address
	precompileABI  abi.ABI
}

// getTxAndCallArgs is a helper function to return the correct call arguments for a given call type.
// In case of a direct call to the precompile, the precompile's ABI is used. Otherwise a caller contract is used.
func getTxAndCallArgs(
	callType int,
	contractData ContractData,
	methodName string,
	args ...interface{},
) (evmtypes.EvmTxArgs, testutiltypes.CallArgs) {
	txArgs := evmtypes.EvmTxArgs{}
	callArgs := testutiltypes.CallArgs{}

	switch callType {
	case directCall:
		txArgs.To = &contractData.precompileAddr
		callArgs.ContractABI = contractData.precompileABI
	case contractCall:
		txArgs.To = &contractData.contractAddr
		callArgs.ContractABI = contractData.contractABI
	}

	callArgs.MethodName = methodName
	callArgs.Args = args

	return txArgs, callArgs
}

func Max(x, y int) int {
	if x > y {
		return x
	}
	return y
}

// XMPL Token metadata to use on tests
const (
	xmplDenom     = "xmpl"
	xmplErc20Addr = "0x5db67696C3c088DfBf588d3dd849f44266ffffff"
)
