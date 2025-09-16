package bank2

import (
	"math/big"
	"slices"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	_ "embed"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/testutil/constants"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

var (
	BankPrecompile = common.HexToAddress(evmtypes.Bank2PrecompileAddress)

	//go:embed erc20abi.json
	ERC20ABIStr string
	ERC20ABI    abi.ABI

	GasLimit = uint64(100000000)
)

func init() {
	var err error
	ERC20ABI, err = abi.JSON(strings.NewReader(ERC20ABIStr))
	if err != nil {
		panic(err)
	}

	_ = evmtypes.NewEVMConfigurator().
		WithEVMCoinInfo(constants.ExampleChainCoinInfo[constants.ExampleChainID]).
		Configure()
}

type TokenInfo struct {
	Denom        string
	DisplayDenom string
	Name         string
	Symbol       string
	Decimals     byte
}

func Setup(t *testing.T, token TokenInfo, mintTo common.Address, mintAmount uint64) *vm.EVM {
	t.Helper()
	nativeDenom := evmtypes.GetEVMCoinDenom()

	rawdb := dbm.NewMemDB()
	logger := log.NewNopLogger()
	ms := store.NewCommitMultiStore(rawdb, logger, nil)
	ctx := sdk.NewContext(ms, cmtproto.Header{}, false, logger)
	evm := NewMockEVM(ctx)

	bankKeeper := NewMockBankKeeper()
	msgServer := NewBankMsgServer(bankKeeper)
	precompile := NewPrecompile(msgServer, bankKeeper)
	evm.WithPrecompiles(map[common.Address]vm.PrecompiledContract{
		precompile.Address(): precompile,
	})

	// init token
	bankKeeper.registerDenom(token.Denom, banktypes.Metadata{
		Symbol: token.Symbol, Name: token.Name, Display: token.DisplayDenom, DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    token.Denom,
				Exponent: 0,
			},
			{
				Denom:    token.DisplayDenom,
				Exponent: uint32(token.Decimals),
			},
		},
	})
	bankKeeper.registerDenom(nativeDenom, banktypes.Metadata{
		Symbol: "NATIVE", Name: "Native Token", Display: evmtypes.GetEVMCoinDisplayDenom(), DenomUnits: []*banktypes.DenomUnit{
			{
				Denom:    nativeDenom,
				Exponent: 0,
			},
			{
				Denom:    evmtypes.GetEVMCoinDisplayDenom(),
				Exponent: 18,
			},
		},
	})
	bankKeeper.mint(mintTo.Bytes(), sdk.NewCoins(sdk.NewCoin(token.Denom, sdkmath.NewIntFromUint64(mintAmount))))
	bankKeeper.mint(mintTo.Bytes(), sdk.NewCoins(sdk.NewCoin(nativeDenom, sdkmath.NewIntFromUint64(mintAmount))))

	DeployCreate2(t, evm)
	DeployERC20(t, evm, BankPrecompile, token.Denom)

	return evm
}

func TestERC20ContractAddress(t *testing.T) {
	denom := "uatom"
	contract := common.HexToAddress(evmtypes.Bank2PrecompileAddress)
	expected := common.HexToAddress("0xdDe94B5b492d597317FD86d2A5baad9966BE2e3e")

	result := ERC20ContractAddress(contract, denom)
	require.Equal(t, expected, result)
}

// TestBankPrecompile tests calling bank precompile directly
func TestBankPrecompile(t *testing.T) {
	user1 := common.BigToAddress(big.NewInt(1))
	user2 := common.BigToAddress(big.NewInt(2))
	token := TokenInfo{
		Denom:        "denom",
		DisplayDenom: "display",
		Symbol:       "COIN",
		Name:         "Test Coin",
		Decimals:     byte(18),
	}
	amount := uint64(1000)
	erc20 := ERC20ContractAddress(BankPrecompile, token.Denom)

	setup := func(t *testing.T) *vm.EVM {
		t.Helper()
		return Setup(t, token, user1, amount)
	}

	testCases := []struct {
		name   string
		method BankMethod
		caller common.Address
		input  []byte
		output []byte
		expErr error
	}{
		{"name", MethodName, user1, []byte(token.Denom), []byte(token.Name), nil},
		{"symbol", MethodSymbol, user1, []byte(token.Denom), []byte(token.Symbol), nil},
		{"decimals", MethodDecimals, user1, []byte(token.Denom), []byte{token.Decimals}, nil},
		{
			"totalSupply", MethodTotalSupply, user1,
			[]byte(token.Denom),
			common.LeftPadBytes(new(big.Int).SetUint64(amount).Bytes(), 32),
			nil,
		},
		{
			"balanceOf", MethodBalanceOf, user1,
			slices.Concat(user1.Bytes(), []byte(token.Denom)),
			common.LeftPadBytes(new(big.Int).SetUint64(amount).Bytes(), 32),
			nil,
		},
		{
			"balanceOf-empty", MethodBalanceOf, user2,
			slices.Concat(user2.Bytes(), []byte(token.Denom)),
			common.LeftPadBytes([]byte{}, 32),
			nil,
		},
		{
			"transferFrom-owner", MethodTransferFrom, user1,
			slices.Concat(user1.Bytes(), user2.Bytes(), common.LeftPadBytes(big.NewInt(100).Bytes(), 32), []byte(token.Denom)),
			[]byte{1},
			nil,
		},
		{
			"transferFrom-erc20", MethodTransferFrom, erc20,
			slices.Concat(user1.Bytes(), user2.Bytes(), common.LeftPadBytes(big.NewInt(100).Bytes(), 32), []byte(token.Denom)),
			[]byte{1},
			nil,
		},
		{
			"transferFrom-unauthorized", MethodTransferFrom, user2,
			slices.Concat(user1.Bytes(), user2.Bytes(), common.LeftPadBytes(big.NewInt(100).Bytes(), 32), []byte(token.Denom)),
			nil,
			vm.ErrExecutionReverted,
		},
		{
			"transferFrom-insufficient-balance", MethodTransferFrom, user2,
			slices.Concat(user2.Bytes(), user1.Bytes(), common.LeftPadBytes(big.NewInt(100).Bytes(), 32), []byte(token.Denom)),
			nil,
			vm.ErrExecutionReverted,
		},
		{"invalid-method", 6, user1, nil, nil, vm.ErrExecutionReverted},
		{"name-invalid-denom", MethodName, user1, []byte("non-exist"), nil, vm.ErrExecutionReverted},
		{"symbol-invalid-denom", MethodSymbol, user1, []byte("non-exist"), nil, vm.ErrExecutionReverted},
		{"decimals-invalid-denom", MethodDecimals, user1, []byte("non-exist"), nil, vm.ErrExecutionReverted},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evm := setup(t)
			input := slices.Concat([]byte{byte(tc.method)}, tc.input)
			ret, _, err := evm.Call(tc.caller, BankPrecompile, input, GasLimit, uint256.NewInt(0))
			if tc.expErr != nil {
				require.Equal(t, tc.expErr, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.output, ret)
			}
		})
	}
}

// TestBankERC20 tests bank precompile through the ERC20 interface
func TestBankERC20(t *testing.T) {
	zero := common.BigToAddress(big.NewInt(0))
	user1 := common.BigToAddress(big.NewInt(1))
	user2 := common.BigToAddress(big.NewInt(2))
	token := TokenInfo{
		Denom:        "denom",
		DisplayDenom: "display",
		Symbol:       "COIN",
		Name:         "Test Coin",
		Decimals:     byte(18),
	}
	amount := uint64(1000)
	bigAmount := new(big.Int).SetUint64(amount)
	erc20 := ERC20ContractAddress(BankPrecompile, token.Denom)
	nativeERC20 := ERC20ContractAddress(BankPrecompile, evmtypes.GetEVMCoinDenom())

	setup := func(t *testing.T) *vm.EVM {
		t.Helper()
		evm := Setup(t, token, user1, amount)
		DeployERC20(t, evm, BankPrecompile, evmtypes.GetEVMCoinDenom())
		return evm
	}

	testCases := []struct {
		name   string
		method string
		caller common.Address
		token  common.Address
		input  []interface{}
		output []interface{}
		expErr error
	}{
		{"name", "name", zero, erc20, nil, []interface{}{token.Name}, nil},
		{"symbol", "symbol", zero, erc20, nil, []interface{}{token.Symbol}, nil},
		{"decimals", "decimals", zero, erc20, nil, []interface{}{token.Decimals}, nil},
		{"totalSupply", "totalSupply", zero, erc20, nil, []interface{}{bigAmount}, nil},
		{
			"balanceOf", "balanceOf", zero, erc20,
			[]interface{}{user1},
			[]interface{}{bigAmount},
			nil,
		},
		{
			"balanceOf-empty", "balanceOf", zero, erc20,
			[]interface{}{user2},
			[]interface{}{common.Big0},
			nil,
		},
		{
			"transfer", "transfer", user1, erc20,
			[]interface{}{user2, big.NewInt(100)},
			[]interface{}{true},
			nil,
		},
		{
			"transfer-insufficient-balance", "transfer", user2, erc20,
			[]interface{}{user1, big.NewInt(100)},
			nil,
			vm.ErrExecutionReverted,
		},
		{
			"native-fail", "transfer", user1, nativeERC20,
			[]interface{}{user2, big.NewInt(100)},
			nil,
			vm.ErrExecutionReverted,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evm := setup(t)

			method, ok := ERC20ABI.Methods[tc.method]
			require.True(t, ok, "method not found: %s", tc.method)

			input, err := method.Inputs.Pack(tc.input...)
			require.NoError(t, err)

			ret, _, err := evm.Call(tc.caller, tc.token, slices.Concat(method.ID, input), GasLimit, uint256.NewInt(0))
			if tc.expErr != nil {
				require.Equal(t, tc.expErr, err)
				return
			}

			require.NoError(t, err)
			expOutput, err := method.Outputs.Pack(tc.output...)
			require.NoError(t, err)
			require.Equal(t, expOutput, ret)
		})
	}
}

// DeployCreate2 deploys the deterministic contract factory
// https://github.com/Arachnid/deterministic-deployment-proxy
func DeployCreate2(t *testing.T, evm *vm.EVM) {
	t.Helper()
	caller := common.HexToAddress("0x3fAB184622Dc19b6109349B94811493BF2a45362")
	code := common.FromHex("604580600e600039806000f350fe7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3")
	_, address, _, err := evm.Create(caller, code, GasLimit, uint256.NewInt(0))
	require.NoError(t, err)
	require.Equal(t, Create2FactoryAddress, address)
}

func DeployERC20(t *testing.T, evm *vm.EVM, bank common.Address, denom string) {
	t.Helper()
	caller := common.BigToAddress(common.Big0)

	input := slices.Concat(ERC20Salt, ERC20Bin, ERC20Constructor(denom, bank))
	_, _, err := evm.Call(caller, Create2FactoryAddress, input, GasLimit, uint256.NewInt(0))
	require.NoError(t, err)

	expAddress := ERC20ContractAddress(bank, denom)
	require.NotEmpty(t, evm.StateDB.GetCode(expAddress))
}
