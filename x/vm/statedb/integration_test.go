package statedb_test

import (
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/server/config"
	"github.com/cosmos/evm/x/vm/statedb"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"io"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"

	//nolint:revive // okay to use dot imports for Ginkgo
	. "github.com/onsi/ginkgo/v2"
	//nolint:revive // okay to use dot imports for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/contracts"
	stakingprecompile "github.com/cosmos/evm/precompiles/staking"
	testcontracts "github.com/cosmos/evm/precompiles/testutil/contracts"
	testfactory "github.com/cosmos/evm/testutil/integration/os/factory"
	"github.com/cosmos/evm/testutil/integration/os/grpc"
	testkeyring "github.com/cosmos/evm/testutil/integration/os/keyring"
	testnetwork "github.com/cosmos/evm/testutil/integration/os/network"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
)

func TestNestedEVMExtensionCall(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Nested EVM Extension Call Test Suite")
}

type testCase struct {
	method                  string
	expDelegation           bool
	expSenderERC20Balance   *big.Int
	expContractERC20Balance *big.Int
}

type TrackingMultiStore struct {
	store            storetypes.CacheMultiStore
	writes           int
	historicalStores []*TrackingMultiStore
}

func (t *TrackingMultiStore) GetStoreType() storetypes.StoreType {
	return t.store.GetStoreType()
}

func (t *TrackingMultiStore) CacheWrap() storetypes.CacheWrap {
	return t.store.CacheWrap()
}

func (t *TrackingMultiStore) CacheWrapWithTrace(w io.Writer, tc storetypes.TraceContext) storetypes.CacheWrap {
	return t.store.CacheWrapWithTrace(w, tc)
}

func (t *TrackingMultiStore) CacheMultiStoreWithVersion(version int64) (storetypes.CacheMultiStore, error) {
	return t.CacheMultiStoreWithVersion(version)
}

func (t *TrackingMultiStore) GetStore(key storetypes.StoreKey) storetypes.Store {
	return t.store.GetStore(key)
}

func (t *TrackingMultiStore) GetKVStore(key storetypes.StoreKey) storetypes.KVStore {
	return t.store.GetKVStore(key)
}

func (t *TrackingMultiStore) TracingEnabled() bool {
	return t.store.TracingEnabled()
}

func (t *TrackingMultiStore) SetTracer(w io.Writer) storetypes.MultiStore {
	return t.store.SetTracer(w)
}

func (t *TrackingMultiStore) SetTracingContext(context storetypes.TraceContext) storetypes.MultiStore {
	return t.store.SetTracingContext(context)
}

func (t *TrackingMultiStore) LatestVersion() int64 {
	return t.store.LatestVersion()
}

func (t *TrackingMultiStore) Copy() storetypes.CacheMultiStore {
	return t.store.Copy()
}

func (t *TrackingMultiStore) Write() {
	t.writes++
	t.store.Write()
}

func (t *TrackingMultiStore) CacheMultiStore() storetypes.CacheMultiStore {
	cms := t.store.CacheMultiStore()
	return &TrackingMultiStore{store: cms}
}

// This test is a demonstration of the flash loan exploit that was reported.
// This happens when interacting with EVM extensions in smart contract methods,
// where a resulting state change has the same value as the original state value.
//
// Before the fix, this would result in state changes not being persisted after the EVM extension call,
// therefore leaving the loaned funds in the contract.
var _ = Describe("testing the flash loan exploit", Ordered, func() {
	var (
		keyring testkeyring.Keyring
		// NOTE: we need to use the unit test network here because we need it to instantiate the staking precompile correctly
		network *testnetwork.UnitTestNetwork
		handler grpc.Handler
		factory testfactory.TxFactory

		deployer testkeyring.Key

		erc20Addr         common.Address
		flashLoanAddr     common.Address
		flashLoanContract evmtypes.CompiledContract

		validatorToDelegateTo string

		delegatedAmountPre math.Int

		stakingPrecompile *stakingprecompile.Precompile
	)

	mintAmount := big.NewInt(2e18)
	delegateAmount := big.NewInt(1e18)

	BeforeAll(func() {
		keyring = testkeyring.New(2)
		network = testnetwork.NewUnitTestNetwork(
			testnetwork.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		)
		handler = grpc.NewIntegrationHandler(network)
		factory = testfactory.New(network, handler)

		deployer = keyring.GetKey(0)

		var err error
		stakingPrecompile, err = stakingprecompile.NewPrecompile(*network.App.StakingKeeper, network.App.AuthzKeeper)
		Expect(err).ToNot(HaveOccurred(), "failed to create staking precompile")

		// Load the flash loan contract from the compiled JSON data.
		flashLoanContract, err = testcontracts.LoadFlashLoanContract()
		Expect(err).ToNot(HaveOccurred(), "failed to load flash loan contract")
	})

	BeforeEach(func() {
		valsRes, err := handler.GetBondedValidators()
		Expect(err).ToNot(HaveOccurred(), "failed to get bonded validators")

		validatorToDelegateTo = valsRes.Validators[0].OperatorAddress
		qRes, err := handler.GetDelegation(deployer.AccAddr.String(), validatorToDelegateTo)
		Expect(err).ToNot(HaveOccurred(), "failed to get delegation")
		delegatedAmountPre = qRes.DelegationResponse.Balance.Amount

		// Deploy an ERC-20 token contract.
		erc20Addr, err = factory.DeployContract(
			deployer.Priv,
			evmtypes.EvmTxArgs{},
			testfactory.ContractDeploymentData{
				Contract:        contracts.ERC20MinterBurnerDecimalsContract,
				ConstructorArgs: []interface{}{"TestToken", "TT", uint8(18)},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to deploy ERC-20 contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Mint some tokens to the deployer.
		_, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "mint",
				Args: []interface{}{
					deployer.Addr, mintAmount,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to mint tokens")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Check the balance of the deployer on the ERC20 contract.
		res, err := factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "balanceOf",
				Args: []interface{}{
					deployer.Addr,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get balance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err := evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode balance of tx response")

		unpacked, err := contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack(
			"balanceOf",
			ethRes.Ret,
		)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack balance")

		balance, ok := unpacked[0].(*big.Int)
		Expect(ok).To(BeTrue(), "failed to convert balance to big.Int")
		Expect(balance.String()).To(Equal(mintAmount.String()), "balance is not correct")

		// Deploy the flash loan contract.
		flashLoanAddr, err = factory.DeployContract(
			deployer.Priv,
			evmtypes.EvmTxArgs{},
			testfactory.ContractDeploymentData{
				Contract: flashLoanContract,
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to deploy flash loan contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Approve the flash loan contract to spend tokens. This is required because
		// the contract will get funds from the caller to perform actions.
		_, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "approve",
				Args: []interface{}{
					flashLoanAddr, mintAmount,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to approve flash loan contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Check the allowance.
		res, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "allowance",
				Args: []interface{}{
					deployer.Addr, flashLoanAddr,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get allowance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err = evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode allowance tx response")

		unpacked, err = contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack(
			"allowance",
			ethRes.Ret,
		)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack allowance")

		var allowance *big.Int
		allowance, ok = unpacked[0].(*big.Int)
		Expect(ok).To(BeTrue(), "failed to convert allowance to big.Int")
		Expect(allowance.String()).To(Equal(mintAmount.String()), "allowance is not correct")

		// Approve the flash loan contract to delegate tokens on behalf of user.
		precompileAddr := stakingPrecompile.Address()

		_, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &precompileAddr},
			testfactory.CallArgs{
				ContractABI: stakingPrecompile.ABI,
				MethodName:  "approve",
				Args: []interface{}{
					flashLoanAddr, delegateAmount, []string{stakingprecompile.DelegateMsg},
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to approve flash loan contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Check the allowance.
		res, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &precompileAddr},
			testfactory.CallArgs{
				ContractABI: stakingPrecompile.ABI,
				MethodName:  "allowance",
				Args: []interface{}{
					deployer.Addr, flashLoanAddr, stakingprecompile.DelegateMsg,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get allowance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err = evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode allowance tx response")

		err = stakingPrecompile.UnpackIntoInterface(&allowance, "allowance", ethRes.Ret)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack allowance")
	})

	DescribeTable("call the flashLoan contract", func(tc testCase) {
		_, err := factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{
				To:       &flashLoanAddr,
				GasPrice: big.NewInt(900_000_000),
				GasLimit: 400_000,
			},
			testfactory.CallArgs{
				ContractABI: flashLoanContract.ABI,
				MethodName:  tc.method,
				Args: []interface{}{
					erc20Addr,
					validatorToDelegateTo,
					delegateAmount,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to execute flash loan")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		delRes, err := handler.GetDelegation(deployer.AccAddr.String(), validatorToDelegateTo)
		Expect(err).ToNot(HaveOccurred(), "failed to get delegation")
		delAmtPost := delRes.DelegationResponse.Balance.Amount
		if tc.expDelegation {
			Expect(delAmtPost).To(Equal(
				delegatedAmountPre.Add(math.NewIntFromBigInt(delegateAmount))),
				"delegated amount is not correct",
			)
		} else {
			Expect(delAmtPost).To(Equal(delegatedAmountPre))
		}

		// Check the ERC20 token balance of the deployer.
		res, err := factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "balanceOf",
				Args: []interface{}{
					deployer.Addr,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get balance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err := evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode balance of tx response")

		unpacked, err := contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack(
			"balanceOf",
			ethRes.Ret,
		)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack balance")

		balance, ok := unpacked[0].(*big.Int)
		Expect(ok).To(BeTrue(), "failed to convert balance to big.Int")
		Expect(balance.String()).To(Equal(tc.expSenderERC20Balance.String()), "balance is not correct")

		// Check FlashLoan smart contract ERC20 token balance.
		res, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "balanceOf",
				Args: []interface{}{
					flashLoanAddr,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get balance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err = evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode balance of tx response")

		unpacked, err = contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack(
			"balanceOf",
			ethRes.Ret,
		)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack balance")

		balance, ok = unpacked[0].(*big.Int)
		Expect(ok).To(BeTrue(), "failed to convert balance to big.Int")
		Expect(balance.String()).To(Equal(tc.expContractERC20Balance.String()), "balance is not correct")
	},
		Entry("flashLoan method & expect delegation", testCase{
			method:                  "flashLoan",
			expDelegation:           true,
			expSenderERC20Balance:   mintAmount,
			expContractERC20Balance: big.NewInt(0),
		}),
		Entry("flashLoanWithRevert method - delegation reverted", testCase{
			method:                  "flashLoanWithRevert",
			expDelegation:           false,
			expSenderERC20Balance:   delegateAmount,
			expContractERC20Balance: delegateAmount,
		}),
	)
})

var _ = Describe("testing statedb CMS branching", Ordered, func() {
	var (
		keyring testkeyring.Keyring
		// NOTE: we need to use the unit test network here because we need to retrieve the StateDB
		network *testnetwork.UnitTestNetwork
		handler grpc.Handler
		factory testfactory.TxFactory
		evm     *vm.EVM

		deployer testkeyring.Key

		erc20Addr         common.Address
		flashLoanAddr     common.Address
		flashLoanContract evmtypes.CompiledContract

		validatorToDelegateTo string

		stakingPrecompile *stakingprecompile.Precompile
	)

	mintAmount := big.NewInt(2e18)
	delegateAmount := big.NewInt(1e18)

	BeforeAll(func() {
		keyring = testkeyring.New(2)
		network = testnetwork.NewUnitTestNetwork(
			testnetwork.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		)
		handler = grpc.NewIntegrationHandler(network)
		factory = testfactory.New(network, handler)

		deployer = keyring.GetKey(0)

		var err error
		stakingPrecompile, err = stakingprecompile.NewPrecompile(*network.App.StakingKeeper, network.App.AuthzKeeper)
		Expect(err).ToNot(HaveOccurred(), "failed to create staking precompile")

		// Load the flash loan contract from the compiled JSON data.
		flashLoanContract, err = testcontracts.LoadFlashLoanContract()
		Expect(err).ToNot(HaveOccurred(), "failed to load flash loan contract")

	})

	BeforeEach(func() {
		valsRes, err := handler.GetBondedValidators()
		Expect(err).ToNot(HaveOccurred(), "failed to get bonded validators")

		validatorToDelegateTo = valsRes.Validators[0].OperatorAddress

		// Deploy an ERC-20 token contract.
		erc20Addr, err = factory.DeployContract(
			deployer.Priv,
			evmtypes.EvmTxArgs{},
			testfactory.ContractDeploymentData{
				Contract:        contracts.ERC20MinterBurnerDecimalsContract,
				ConstructorArgs: []interface{}{"TestToken", "TT", uint8(18)},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to deploy ERC-20 contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Mint some tokens to the deployer.
		_, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "mint",
				Args: []interface{}{
					deployer.Addr, mintAmount,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to mint tokens")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Check the balance of the deployer on the ERC20 contract.
		res, err := factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "balanceOf",
				Args: []interface{}{
					deployer.Addr,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get balance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err := evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode balance of tx response")

		unpacked, err := contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack(
			"balanceOf",
			ethRes.Ret,
		)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack balance")

		balance, ok := unpacked[0].(*big.Int)
		Expect(ok).To(BeTrue(), "failed to convert balance to big.Int")
		Expect(balance.String()).To(Equal(mintAmount.String()), "balance is not correct")

		// Deploy the flash loan contract.
		flashLoanAddr, err = factory.DeployContract(
			deployer.Priv,
			evmtypes.EvmTxArgs{},
			testfactory.ContractDeploymentData{
				Contract: flashLoanContract,
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to deploy flash loan contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Approve the flash loan contract to spend tokens. This is required because
		// the contract will get funds from the caller to perform actions.
		_, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "approve",
				Args: []interface{}{
					flashLoanAddr, mintAmount,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to approve flash loan contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Check the allowance.
		res, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &erc20Addr},
			testfactory.CallArgs{
				ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
				MethodName:  "allowance",
				Args: []interface{}{
					deployer.Addr, flashLoanAddr,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get allowance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err = evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode allowance tx response")

		unpacked, err = contracts.ERC20MinterBurnerDecimalsContract.ABI.Unpack(
			"allowance",
			ethRes.Ret,
		)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack allowance")

		var allowance *big.Int
		allowance, ok = unpacked[0].(*big.Int)
		Expect(ok).To(BeTrue(), "failed to convert allowance to big.Int")
		Expect(allowance.String()).To(Equal(mintAmount.String()), "allowance is not correct")

		// Approve the flash loan contract to delegate tokens on behalf of user.
		precompileAddr := stakingPrecompile.Address()

		_, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &precompileAddr},
			testfactory.CallArgs{
				ContractABI: stakingPrecompile.ABI,
				MethodName:  "approve",
				Args: []interface{}{
					flashLoanAddr, delegateAmount, []string{stakingprecompile.DelegateMsg},
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to approve flash loan contract")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		// Check the allowance.
		res, err = factory.ExecuteContractCall(
			deployer.Priv,
			evmtypes.EvmTxArgs{To: &precompileAddr},
			testfactory.CallArgs{
				ContractABI: stakingPrecompile.ABI,
				MethodName:  "allowance",
				Args: []interface{}{
					deployer.Addr, flashLoanAddr, stakingprecompile.DelegateMsg,
				},
			},
		)
		Expect(err).ToNot(HaveOccurred(), "failed to get allowance")

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")

		ethRes, err = evmtypes.DecodeTxResponse(res.Data)
		Expect(err).ToNot(HaveOccurred(), "failed to decode allowance tx response")

		err = stakingPrecompile.UnpackIntoInterface(&allowance, "allowance", ethRes.Ret)
		Expect(err).ToNot(HaveOccurred(), "failed to unpack allowance")
	})

	DescribeTable("call the flashLoan contract", func(tc testCase) {
		cms := &TrackingMultiStore{
			store:            network.App.BaseApp.CommitMultiStore().CacheMultiStore(),
			writes:           0,
			historicalStores: nil,
		}
		ctx := network.App.BaseApp.NewContext(false)
		fromAddr := common.BytesToAddress(deployer.Priv.PubKey().Address().Bytes())
		sender := vm.AccountRef(fromAddr)
		nonce, err := network.App.AccountKeeper.GetSequence(ctx, fromAddr.Bytes())
		Expect(err).ToNot(HaveOccurred())

		txCfg := statedb.NewEmptyTxConfig(common.BytesToHash(ctx.HeaderHash()))
		stateDB := statedb.New(ctx.WithMultiStore(cms), network.App.EVMKeeper, txCfg)
		evmCfg, _ := network.App.EVMKeeper.EVMConfig(ctx, sdk.ConsAddress(ctx.BlockHeader().ProposerAddress))

		/*
			args := evmtypes.EvmTxArgs{
				To:       &flashLoanAddr,
				GasPrice: big.NewInt(900_000_000),
				GasLimit: 400_000,
			}
		*/
		callArgs := []interface{}{
			erc20Addr,
			validatorToDelegateTo,
			delegateAmount,
		}
		data, err := flashLoanContract.ABI.Pack(tc.method, callArgs...)
		Expect(err).ToNot(HaveOccurred())
		msg := ethtypes.NewMessage(
			fromAddr,
			&flashLoanAddr,
			nonce,
			big.NewInt(0),        // amount
			config.DefaultGasCap, // gasLimit
			big.NewInt(0),        // gasFeeCap
			big.NewInt(0),        // gasTipCap
			big.NewInt(0),        // gasPrice
			data,
			ethtypes.AccessList{}, // AccessList
			false,                 // isFake
		)

		evm = network.App.EVMKeeper.NewEVM(ctx, msg, evmCfg, nil, stateDB)
		_, _, vmErr := evm.Call(sender, flashLoanAddr, msg.Data(), msg.Gas(), msg.Value())
		Expect(vmErr).ToNot(HaveOccurred())

		Expect(network.NextBlock()).ToNot(HaveOccurred(), "failed to commit block")
	},
		Entry("flashLoanWithRevert method -- track Writes", testCase{
			method:                  "flashLoan",
			expDelegation:           true,
			expSenderERC20Balance:   mintAmount,
			expContractERC20Balance: big.NewInt(0),
		}),
	)
})
