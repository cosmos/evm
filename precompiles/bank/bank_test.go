package bank

import (
	"math/big"
	"slices"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
	"github.com/yihuang/go-abi"

	_ "embed"

	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/evm/precompiles/bank/erc20"
	"github.com/cosmos/evm/testutil/constants"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log"
	sdkmath "cosmossdk.io/math"
	"cosmossdk.io/store"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

//go:generate go run github.com/yihuang/go-abi/cmd -var ERC20ABI -package erc20 -output erc20/abi.go

var ERC20ABI = []string{
	"function name() view returns (string name)",
	"function symbol() view returns (string symbol)",
	"function decimals() view returns (uint8 decimals)",
	"function totalSupply() view returns (uint256 supply)",
	"function balanceOf(address account) view returns (uint256 balance)",
	"function transfer(address to, uint256 amount) returns (bool success)",
	"function transferFrom(address from, address to, uint256 amount) returns (bool success)",
	"event Transfer(address indexed from, address indexed to, uint256 amount)",
	"event Approval(address indexed owner, address indexed spender, uint256 amount)",
}

var (
	BankPrecompile = common.HexToAddress(evmtypes.BankPrecompileAddress)
	GasLimit       = uint64(100000000)
)

type TokenInfo struct {
	Denom        string
	DisplayDenom string
	Name         string
	Symbol       string
	Decimals     byte
}

func Setup(t *testing.T, token TokenInfo, mintTo common.Address, mintAmount uint64) *vm.EVM {
	t.Helper()

	chainID := uint64(constants.EighteenDecimalsChainID)
	configurator := evmtypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	// set global chain config
	ethCfg := evmtypes.DefaultChainConfig(chainID)
	if err := evmtypes.SetChainConfig(ethCfg); err != nil {
		panic(err)
	}
	err := configurator.
		WithExtendedEips(evmtypes.DefaultCosmosEVMActivators).
		// NOTE: we're using the 18 decimals default for the example chain
		WithEVMCoinInfo(constants.ChainsCoinInfo[chainID]).
		Configure()

	require.NoError(t, err)
	nativeDenom := evmtypes.GetEVMCoinDenom()

	rawdb := dbm.NewMemDB()
	logger := log.NewNopLogger()
	ms := store.NewCommitMultiStore(rawdb, logger, nil)
	ctx := sdk.NewContext(ms, cmtproto.Header{}, false, logger)
	evm := NewMockEVM(ctx)

	bankKeeper := NewMockBankKeeper()
	msgServer := NewBankMsgServer(bankKeeper)
	precompile := NewPrecompile(msgServer, bankKeeper, nil)
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
	contract := common.HexToAddress(evmtypes.BankPrecompileAddress)
	expected := common.HexToAddress("0x46514a468D158DC165192793EB8Ba44480e513e6")

	result, err := ERC20ContractAddress(contract, denom)
	require.NoError(t, err)
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
	erc20Address, err := ERC20ContractAddress(BankPrecompile, token.Denom)
	require.NoError(t, err)

	setup := func(t *testing.T) *vm.EVM {
		t.Helper()
		return Setup(t, token, user1, amount)
	}

	testCases := []struct {
		name   string
		caller common.Address
		args   abi.Method
		output abi.Encode
		expErr error
	}{
		{"name", user1, NewNameCall(token.Denom), &NameReturn{token.Name}, nil},
		{"symbol", user1, NewSymbolCall(token.Denom), &SymbolReturn{token.Symbol}, nil},
		{"decimals", user1, NewDecimalsCall(token.Denom), &DecimalsReturn{token.Decimals}, nil},
		{"supplyOf", user1, NewTotalSupply0Call(token.Denom),
			&TotalSupply0Return{new(big.Int).SetUint64(amount)},
			nil,
		},
		{
			"balanceOf",
			user1,
			NewBalanceOfCall(user1, token.Denom),
			&BalanceOfReturn{new(big.Int).SetUint64(amount)},
			nil,
		},
		{
			"balanceOf-empty", user2,
			NewBalanceOfCall(user2, token.Denom),
			&BalanceOfReturn{new(big.Int)},
			nil,
		},
		{
			"transferFrom-owner", user1,
			NewTransferFromCall(user1, user2, big.NewInt(100), token.Denom),
			&TransferFromReturn{true},
			nil,
		},
		{
			"transferFrom-erc20", erc20Address,
			NewTransferFromCall(user1, user2, big.NewInt(100), token.Denom),
			&TransferFromReturn{true},
			nil,
		},
		{
			"transferFrom-unauthorized", user2,
			NewTransferFromCall(user1, user2, big.NewInt(100), token.Denom),
			nil,
			vm.ErrExecutionReverted,
		},
		{
			"transferFrom-insufficient-balance", user2,
			NewTransferFromCall(user2, user1, big.NewInt(100), token.Denom),
			nil,
			vm.ErrExecutionReverted,
		},
		{"invalid-method", user1, erc20.NewTransferCall(user1, big.NewInt(100)), nil, vm.ErrExecutionReverted},
		{"name-invalid-denom", user1, NewNameCall("non-exist"), nil, vm.ErrExecutionReverted},
		{"symbol-invalid-denom", user1, NewSymbolCall("non-exist"), nil, vm.ErrExecutionReverted},
		{"decimals-invalid-denom", user1, NewDecimalsCall("non-exist"), nil, vm.ErrExecutionReverted},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evm := setup(t)
			input, err := tc.args.EncodeWithSelector()
			ret, _, err := evm.Call(tc.caller, BankPrecompile, input, GasLimit, uint256.NewInt(0))
			if tc.expErr != nil {
				require.Equal(t, tc.expErr, err)
			} else {
				require.NoError(t, err)
				expOutput, err := tc.output.Encode()
				require.NoError(t, err)
				require.Equal(t, expOutput, ret)
			}
		})
	}
}

// TestBankERC20 tests bank precompile through the ERC20 interface
func TestBankERC20(t *testing.T) {
	zero := common.BigToAddress(big.NewInt(0))
	user1 := common.BigToAddress(big.NewInt(1))
	user2 := common.BigToAddress(big.NewInt(2))
	info := TokenInfo{
		Denom:        "denom",
		DisplayDenom: "display",
		Symbol:       "COIN",
		Name:         "Test Coin",
		Decimals:     byte(18),
	}
	amount := uint64(1000)
	bigAmount := new(big.Int).SetUint64(amount)
	token, err := ERC20ContractAddress(BankPrecompile, info.Denom)
	require.NoError(t, err)
	nativeERC20, err := ERC20ContractAddress(BankPrecompile, evmtypes.GetEVMCoinDenom())
	require.NoError(t, err)

	setup := func(t *testing.T) *vm.EVM {
		t.Helper()
		evm := Setup(t, info, user1, amount)
		DeployERC20(t, evm, BankPrecompile, evmtypes.GetEVMCoinDenom())
		return evm
	}

	testCases := []struct {
		name   string
		caller common.Address
		token  common.Address
		input  abi.Method
		output abi.Encode
		expErr error
	}{
		{"name", zero, token, erc20.NewNameCall(), &erc20.NameReturn{Name: info.Name}, nil},
		{"symbol", zero, token, erc20.NewSymbolCall(), &erc20.SymbolReturn{Symbol: info.Symbol}, nil},
		{"decimals", zero, token, erc20.NewDecimalsCall(), &erc20.DecimalsReturn{Decimals: info.Decimals}, nil},
		{"totalSupply", zero, token, erc20.NewTotalSupplyCall(), &erc20.TotalSupplyReturn{Supply: bigAmount}, nil},
		{
			"balanceOf", zero, token,
			erc20.NewBalanceOfCall(user1),
			&erc20.BalanceOfReturn{Balance: bigAmount},
			nil,
		},
		{
			"balanceOf-empty", zero, token,
			erc20.NewBalanceOfCall(user2),
			&erc20.BalanceOfReturn{Balance: common.Big0},
			nil,
		},
		{
			"transfer", user1, token,
			erc20.NewTransferCall(user1, big.NewInt(100)),
			&erc20.TransferReturn{Success: true},
			nil,
		},
		{
			"transfer-insufficient-balance", user2, token,
			erc20.NewTransferCall(user1, big.NewInt(100)),
			nil,
			vm.ErrExecutionReverted,
		},
		{
			"native-fail", user1, nativeERC20,
			erc20.NewTransferCall(user2, big.NewInt(100)),
			nil,
			vm.ErrExecutionReverted,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			evm := setup(t)

			input, err := tc.input.EncodeWithSelector()
			require.NoError(t, err)

			ret, _, err := evm.Call(tc.caller, tc.token, input, GasLimit, uint256.NewInt(0))
			if tc.expErr != nil {
				require.Equal(t, tc.expErr, err)
				return
			}

			require.NoError(t, err)
			expOutput, err := tc.output.Encode()
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

	ctor, err := NewErc20ctorCall(denom, bank).Encode()
	require.NoError(t, err)

	input := slices.Concat(ERC20Salt, ERC20Bin, ctor)
	_, _, err = evm.Call(caller, Create2FactoryAddress, input, GasLimit, uint256.NewInt(0))
	require.NoError(t, err)

	expAddress, err := ERC20ContractAddress(bank, denom)
	require.NoError(t, err)

	require.NotEmpty(t, evm.StateDB.GetCode(expAddress))
}
