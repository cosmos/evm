package erc20

import (
	"errors"
	"fmt"
	"math/big"
	"slices"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	//nolint:revive // dot imports are fine for Gomega
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/precompiles/erc20"
	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	utiltx "github.com/cosmos/evm/testutil/tx"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// CallType indicates which type of contract call is made during the integration tests.
type CallType int

// callType constants to differentiate between direct calls and calls through a contract.
const (
	directCall CallType = iota + 1
	directCallToken2
	contractCall
	contractCallToken2
	erc20Call
	erc20CallerCall
	erc20V5Call
	erc20V5CallerCall
)

var (
	nativeCallTypes = []CallType{directCall, directCallToken2, contractCall, contractCallToken2}
	erc20CallTypes  = []CallType{erc20Call, erc20CallerCall, erc20V5Call, erc20V5CallerCall}
)

// setAllowance is a helper function to set up a SendAuthorization for
// a given owner and spender combination for a given amount.
//
// NOTE: A default expiration of 1 hour after the current block time is used.
func (s *PrecompileTestSuite) setAllowance(
	erc20Addr common.Address, ownerPriv cryptotypes.PrivKey, spender common.Address, amount *big.Int,
) {
	owner := common.BytesToAddress(ownerPriv.PubKey().Address().Bytes())
	err := s.network.App.GetErc20Keeper().SetAllowance(s.network.GetContext(), erc20Addr, owner, spender, amount)
	s.Require().NoError(err, "failed to set set allowance")
}

// setAllowanceForContract is a helper function which executes an approval
// for the given contract data.
func (is *IntegrationTestSuite) setAllowanceForContract(
	callType CallType, contractData ContractsData, ownerPriv cryptotypes.PrivKey, spender common.Address, amount *big.Int,
) {
	// NOTE: When using the caller contract, erc20 contract must be called instead of caller contract.
	// This is because caller of erc20 contract becomes the owner of allowance.
	switch callType {
	case erc20V5CallerCall:
		callType = erc20V5Call
	case contractCall:
		callType = directCall
	case contractCallToken2:
		callType = directCallToken2
	}

	abiEvents := contractData.GetContractData(callType).ABI.Events

	txArgs, callArgs := is.getTxAndCallArgs(callType, contractData, erc20.ApproveMethod, spender, amount)

	approveCheck := testutil.LogCheckArgs{
		ABIEvents: abiEvents,
		ExpEvents: []string{erc20.EventTypeApproval},
		ExpPass:   true,
	}

	_, _, err := is.factory.CallContractAndCheckLogs(ownerPriv, txArgs, callArgs, approveCheck)
	Expect(err).ToNot(HaveOccurred(), "failed to execute approve")

	// commit changes to the chain state
	err = is.network.NextBlock()
	Expect(err).ToNot(HaveOccurred(), "error while calling NextBlock")
}

// requireOut is a helper utility to reduce the amount of boilerplate code in the query tests.
//
// It requires the output bytes and error to match the expected values. Additionally, the method outputs
// are unpacked and the first value is compared to the expected value.
//
// NOTE: It's sufficient to only check the first value because all methods in the ERC20 precompile only
// return a single value.
func (s *PrecompileTestSuite) requireOut(
	bz []byte,
	err error,
	method abi.Method,
	expPass bool,
	errContains string,
	expValue interface{},
) {
	if expPass {
		s.Require().NoError(err, "expected no error")
		s.Require().NotEmpty(bz, "expected bytes not to be empty")

		// Unpack the name into a string
		out, err := method.Outputs.Unpack(bz)
		s.Require().NoError(err, "expected no error unpacking")

		// Check if expValue is a big.Int. Because of a difference in uninitialized/empty values for big.Ints,
		// this comparison is often not working as expected, so we convert to Int64 here and compare those values.
		bigExp, ok := expValue.(*big.Int)
		if ok {
			bigOut, ok := out[0].(*big.Int)
			s.Require().True(ok, "expected output to be a big.Int")
			s.Require().Zero(bigExp.Cmp(bigOut), "expected different value")
		} else {
			s.Require().Equal(expValue, out[0], "expected different value")
		}
	} else {
		s.Require().Error(err, "expected error")
		s.Require().Contains(err.Error(), errContains, "expected different error")
	}
}

// requireAllowance is a helper function to check that a SendAuthorization
// exists for a given owner and spender combination for a given amount.
func (s *PrecompileTestSuite) requireAllowance(erc20Addr, owner, spender common.Address, amount *big.Int) {
	allowance, err := s.network.App.GetErc20Keeper().GetAllowance(s.network.GetContext(), erc20Addr, owner, spender)
	s.Require().NoError(err, "expected no error unpacking the allowance")
	s.Require().Equal(allowance.String(), amount.String(), "expected different allowance")
}

// setupERC20Precompile is a helper function to set up an instance of the ERC20 precompile for
// a given token denomination, set the token pair in the ERC20 keeper and adds the precompile
// to the available and active precompiles.
func (s *PrecompileTestSuite) setupERC20Precompile(denom string) (*erc20.Precompile, error) {
	tokenPair := erc20types.NewTokenPair(utiltx.GenerateAddress(), denom, erc20types.OWNER_MODULE)
	err := s.network.App.GetErc20Keeper().SetToken(s.network.GetContext(), tokenPair)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to set token")
	}

	precompile, err := setupERC20PrecompileForTokenPair(*s.network, tokenPair)
	s.Require().NoError(err, "failed to set up %q erc20 precompile", tokenPair.Denom)

	return precompile, nil
}

// setupERC20Precompile is a helper function to set up an instance of the ERC20 precompile for
// a given token denomination, set the token pair in the ERC20 keeper and adds the precompile
// to the available and active precompiles.
func (is *IntegrationTestSuite) setupERC20Precompile(denom string, tokenPairs []erc20types.TokenPair) *erc20.Precompile {
	var tokenPair erc20types.TokenPair
	for _, tp := range tokenPairs {
		if tp.Denom != denom {
			continue
		}
		tokenPair = tp
	}

	return erc20.NewPrecompile(
		tokenPair,
		is.network.App.GetBankKeeper(),
		is.network.App.GetErc20Keeper(),
		is.network.App.GetTransferKeeper(),
	)
}

// setupERC20PrecompileForTokenPair is a helper function to set up an instance of the ERC20 precompile for
// a given token pair and adds the precompile to the available and active precompiles.
// Do not use this function for integration tests.
func setupERC20PrecompileForTokenPair(
	unitNetwork network.UnitTestNetwork, tokenPair erc20types.TokenPair,
) (*erc20.Precompile, error) {
	precompile := erc20.NewPrecompile(
		tokenPair,
		unitNetwork.App.GetBankKeeper(),
		unitNetwork.App.GetErc20Keeper(),
		unitNetwork.App.GetTransferKeeper(),
	)
	if err := unitNetwork.App.GetErc20Keeper().EnableDynamicPrecompile(
		unitNetwork.GetContext(),
		precompile.Address(),
	); err != nil {
		return nil, errorsmod.Wrapf(err, "failed to add %q erc20 precompile to EVM extensions", tokenPair.Denom)
	}

	return precompile, nil
}

// setupNewERC20PrecompileForTokenPair is a helper function to set up an instance of the ERC20 precompile for
// a given token pair and adds the precompile to the available and active precompiles.
// This function should be used for integration tests
func (is *IntegrationTestSuite) setupNewERC20PrecompileForTokenPair(
	tokenPair erc20types.TokenPair,
) (*erc20.Precompile, error) {
	precompile := erc20.NewPrecompile(
		tokenPair,
		is.network.App.GetBankKeeper(),
		is.network.App.GetErc20Keeper(),
		is.network.App.GetTransferKeeper(),
	)

	// Update the params via gov proposal
	if err := is.network.App.GetErc20Keeper().EnableDynamicPrecompile(is.network.GetContext(), precompile.Address()); err != nil {
		return nil, err
	}

	// We must directly commit keeper calls to state, otherwise they get
	// fully wiped when the next block finalizes.
	store := is.network.GetContext().MultiStore()
	if cms, ok := store.(storetypes.CacheMultiStore); ok {
		cms.Write()
	} else {
		return nil, errors.New("store is not a CacheMultiStore")
	}

	return precompile, nil
}

// getTxAndCallArgs is a helper function to return the correct call arguments for a given call type.
//
// In case of a direct call to the precompile, the precompile's ABI is used. Otherwise, the
// ERC20CallerContract's ABI is used and the given contract address.
func (is *IntegrationTestSuite) getTxAndCallArgs(
	callType CallType,
	contractData ContractsData,
	methodName string,
	args ...interface{},
) (evmtypes.EvmTxArgs, testutiltypes.CallArgs) {
	cd := contractData.GetContractData(callType)

	txArgs := evmtypes.EvmTxArgs{
		To:       &cd.Address,
		GasPrice: gasPrice,
	}

	callArgs := testutiltypes.CallArgs{
		ContractABI: cd.ABI,
		MethodName:  methodName,
		Args:        args,
	}

	return txArgs, callArgs
}

// ExpectedBalance is a helper struct to check the balances of accounts.
type ExpectedBalance struct {
	address  sdk.AccAddress
	expCoins sdk.Coins
}

// ExpectBalances is a helper function to check if the balances of the given accounts are as expected.
func (is *IntegrationTestSuite) ExpectBalances(expBalances []ExpectedBalance) {
	for _, expBalance := range expBalances {
		for _, expCoin := range expBalance.expCoins {
			coinBalance, err := is.handler.GetBalanceFromBank(expBalance.address, expCoin.Denom)
			Expect(err).ToNot(HaveOccurred(), "expected no error getting balance")
			Expect(coinBalance.Balance.Amount).To(Equal(expCoin.Amount), "expected different balance")
		}
	}
}

// ExpectBalancesForContract is a helper function to check expected balances for given accounts depending
// on the call type.
func (is *IntegrationTestSuite) ExpectBalancesForContract(callType CallType, contractData ContractsData, expBalances []ExpectedBalance) {
	switch {
	case slices.Contains(nativeCallTypes, callType):
		is.ExpectBalances(expBalances)
	case slices.Contains(erc20CallTypes, callType):
		is.ExpectBalancesForERC20(callType, contractData, expBalances)
	default:
		panic("unknown contract call type")
	}
}

// ExpectBalancesForERC20 is a helper function to check expected balances for given accounts
// when using the ERC20 contract.
func (is *IntegrationTestSuite) ExpectBalancesForERC20(callType CallType, contractData ContractsData, expBalances []ExpectedBalance) {
	contractABI := contractData.GetContractData(callType).ABI

	for _, expBalance := range expBalances {
		addr := common.BytesToAddress(expBalance.address.Bytes())
		txArgs, callArgs := is.getTxAndCallArgs(callType, contractData, "balanceOf", addr)

		passCheck := testutil.LogCheckArgs{ExpPass: true}

		_, ethRes, err := is.factory.CallContractAndCheckLogs(contractData.ownerPriv, txArgs, callArgs, passCheck)
		Expect(err).ToNot(HaveOccurred(), "expected no error getting balance")

		err = is.network.NextBlock()
		Expect(err).ToNot(HaveOccurred(), "error on NextBlock call")

		var balance *big.Int
		err = contractABI.UnpackIntoInterface(&balance, "balanceOf", ethRes.Ret)
		Expect(err).ToNot(HaveOccurred(), "expected no error unpacking balance")
		Expect(math.NewIntFromBigInt(balance)).To(Equal(expBalance.expCoins.AmountOf(is.tokenDenom)), "expected different balance")
	}
}

// ExpectAllowanceForContract is a helper function to check that a SendAuthorization
// exists for a given owner and spender combination for a given amount and optionally an access list.
func (is *IntegrationTestSuite) ExpectAllowanceForContract(
	callType CallType, contractData ContractsData, owner, spender common.Address, expAmount *big.Int,
) {
	contractABI := contractData.GetContractData(callType).ABI

	txArgs, callArgs := is.getTxAndCallArgs(callType, contractData, erc20.AllowanceMethod, owner, spender)

	passCheck := testutil.LogCheckArgs{ExpPass: true}

	_, ethRes, err := is.factory.CallContractAndCheckLogs(contractData.ownerPriv, txArgs, callArgs, passCheck)
	Expect(err).ToNot(HaveOccurred(), "expected no error getting allowance")
	// Increase block to update nonce
	Expect(is.network.NextBlock()).To(BeNil())

	var allowance *big.Int
	err = contractABI.UnpackIntoInterface(&allowance, "allowance", ethRes.Ret)
	Expect(err).ToNot(HaveOccurred(), "expected no error unpacking allowance")
	Expect(allowance.Uint64()).To(Equal(expAmount.Uint64()), "expected different allowance")
}

// ExpectTrueToBeReturned is a helper function to check that the precompile returns true
// in the ethereum transaction response.
func (is *IntegrationTestSuite) ExpectTrueToBeReturned(res *evmtypes.MsgEthereumTxResponse, methodName string) {
	var ret bool
	err := is.precompile.UnpackIntoInterface(&ret, methodName, res.Ret)
	Expect(err).ToNot(HaveOccurred(), "expected no error unpacking")
	Expect(ret).To(BeTrue(), "expected true to be returned")
}

// ContractsData is a helper struct to hold the addresses and ABIs for the
// different contract instances that are subject to testing here.
type ContractsData struct {
	contractData map[CallType]ContractData
	ownerPriv    cryptotypes.PrivKey
}

// ContractData is a helper struct to hold the address and ABI for a given contract.
type ContractData struct {
	Address common.Address
	ABI     abi.ABI
}

// GetContractData is a helper function to return the contract data for a given call type.
func (cd ContractsData) GetContractData(callType CallType) ContractData {
	data, found := cd.contractData[callType]
	if !found {
		panic(fmt.Sprintf("no contract data found for call type: %d", callType))
	}
	return data
}

// fundWithTokens is a helper function for the scope of the ERC20 integration tests.
// Depending on the passed call type, it funds the given address with tokens either
// using the Bank module or by minting straight on the ERC20 contract.
// Returns the updated balance amount of the receiver address
func (is *IntegrationTestSuite) fundWithTokens(
	callType CallType,
	contractData ContractsData,
	receiver common.Address,
	fundCoins sdk.Coins,
) math.Int {
	Expect(fundCoins).To(HaveLen(1), "expected only one coin")
	Expect(fundCoins[0].Denom).To(Equal(is.tokenDenom),
		"this helper function only supports funding with the token denom in the context of these integration tests",
	)

	var err error
	receiverBalance := fundCoins.AmountOf(is.tokenDenom)
	balanceInBankMod := slices.Contains(nativeCallTypes, callType)

	switch {
	case balanceInBankMod:
		err = is.factory.FundAccount(is.keyring.GetKey(0), receiver.Bytes(), fundCoins)
	case slices.Contains(erc20CallTypes, callType):
		err = is.MintERC20(callType, contractData, receiver, fundCoins.AmountOf(is.tokenDenom).BigInt())
	default:
		panic("unknown contract call type")
	}

	Expect(err).ToNot(HaveOccurred(), "failed to fund account")
	Expect(is.network.NextBlock()).To(BeNil())

	if balanceInBankMod {
		balRes, err := is.handler.GetBalanceFromBank(receiver.Bytes(), fundCoins.Denoms()[0])
		Expect(err).To(BeNil())
		receiverBalance = balRes.Balance.Amount
	}

	return receiverBalance
}

// MintERC20 is a helper function to mint tokens on the ERC20 contract.
//
// NOTE: we are checking that there was a Transfer event emitted (which happens on minting).
func (is *IntegrationTestSuite) MintERC20(callType CallType, contractData ContractsData, receiver common.Address, amount *big.Int) error {
	if callType == erc20V5CallerCall {
		// NOTE: When using the ERC20 caller contract, we must still mint from the actual ERC20 v5 contract.
		callType = erc20V5Call
	}
	abiEvents := contractData.GetContractData(callType).ABI.Events

	txArgs, callArgs := is.getTxAndCallArgs(callType, contractData, "mint", receiver, amount)

	mintCheck := testutil.LogCheckArgs{
		ABIEvents: abiEvents,
		ExpEvents: []string{erc20.EventTypeTransfer}, // NOTE: this event occurs when calling "mint" on ERC20s
		ExpPass:   true,
	}

	if _, _, err := is.factory.CallContractAndCheckLogs(contractData.ownerPriv, txArgs, callArgs, mintCheck); err != nil {
		return err
	}

	// commit changes to chain state
	return is.network.NextBlock()
}

// NewAddrKey generates an Ethereum address and its corresponding private key.
func NewAddrKey() (common.Address, *ethsecp256k1.PrivKey) {
	privkey, _ := ethsecp256k1.GenerateKey()
	key, err := privkey.ToECDSA()
	if err != nil {
		return common.Address{}, nil
	}

	addr := crypto.PubkeyToAddress(key.PublicKey)

	return addr, privkey
}

// GenerateAddress generates an Ethereum address.
func GenerateAddress() common.Address {
	addr, _ := NewAddrKey()
	return addr
}
