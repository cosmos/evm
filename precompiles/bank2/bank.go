package bank2

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"

	_ "embed"

	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

var (
	// generated with solc 0.8.30+commit.73712a01:
	// solc --overwrite --optimize --optimize-runs 100000 --via-ir --bin -o . ERC20.sol
	//
	//go:embed ERC20.bin
	ERC20BinHex string

	ERC20Bin              []byte
	ERC20Salt             = common.FromHex("636dd1d57837e7dce61901468217da9975548dcb3ecc24d84567feb93cd11e36")
	Create2FactoryAddress = common.HexToAddress("0x4e59b44847b379578588920ca78fbf26c0b4956c")
)

var (
	ErrInputTooShort = errors.New("input too short")
	ErrDenomNotFound = errors.New("denom not found")
	ErrUnauthorized  = errors.New("unauthorized")

	ErrUnknownMethod = "unknown method: %d"
)

func init() {
	var err error
	ERC20Bin, err = hex.DecodeString(ERC20BinHex)
	if err != nil {
		panic(err)
	}
}

type BankMethod uint8

const (
	MethodName BankMethod = iota
	MethodSymbol
	MethodDecimals
	MethodTotalSupply
	MethodBalanceOf
	MethodTransferFrom
)

var _ vm.PrecompiledContract = &Precompile{}

type Precompile struct {
	cmn.Precompile

	msgServer  BankMsgServer
	bankKeeper BankKeeper
}

func NewPrecompile(msgServer BankMsgServer, bankKeeper BankKeeper) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:          storetypes.KVGasConfig(),
			TransientKVGasConfig: storetypes.TransientGasConfig(),
			ContractAddress:      common.HexToAddress(evmtypes.Bank2PrecompileAddress),
		},
		msgServer:  msgServer,
		bankKeeper: bankKeeper,
	}
}

func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 1 {
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(BankMethod(input[0])))
}

// IsTransaction checks if the given method name corresponds to a transaction or query.
// It returns false since all bank methods are queries.
func (Precompile) IsTransaction(method BankMethod) bool {
	return method == MethodTransferFrom
}

func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, contract, readonly)
	})
}

// Name
// input format: abi.encodePacked(string denom)
// output format: abi.encodePacked(string)
func (p Precompile) Name(ctx sdk.Context, input []byte) ([]byte, error) {
	metadata, found := p.bankKeeper.GetDenomMetaData(ctx, string(input))
	if !found {
		return nil, ErrDenomNotFound
	}

	return []byte(metadata.Name), nil
}

// Symbol
//
// input format: abi.encodePacked(string denom)
// output format: abi.encodePacked(string)
func (p Precompile) Symbol(ctx sdk.Context, input []byte) ([]byte, error) {
	metadata, found := p.bankKeeper.GetDenomMetaData(ctx, string(input))
	if !found {
		return nil, ErrDenomNotFound
	}

	return []byte(metadata.Symbol), nil
}

// Decimals returns the exponent of the display denom unit
//
// input format: abi.encodePacked(string denom)
// output format: abi.encodePacked(uint8)
func (p Precompile) Decimals(ctx sdk.Context, input []byte) ([]byte, error) {
	m, found := p.bankKeeper.GetDenomMetaData(ctx, string(input))
	if !found {
		return nil, ErrDenomNotFound
	}

	if len(m.DenomUnits) == 0 {
		return []byte{0}, nil
	}

	// look up Display denom unit
	index := -1
	for i, denomUnit := range m.DenomUnits {
		if denomUnit.Denom == m.Display {
			index = i
			break
		}
	}

	var exponent uint32
	if index == -1 {
		exponent = 0
	} else {
		exponent = m.DenomUnits[index].Exponent
	}

	if exponent > math.MaxUint8 {
		return nil, errors.New("exponent too large")
	}

	return []byte{uint8(exponent)}, nil
}

// TotalSupply
// input format: abi.encodePacked(string denom)
// output format: abi.encodePacked(uint256)
func (p Precompile) TotalSupply(ctx sdk.Context, input []byte) ([]byte, error) {
	supply := p.bankKeeper.GetSupply(ctx, string(input)).Amount
	return common.LeftPadBytes(supply.BigInt().Bytes(), 32), nil
}

// BalanceOf
// input format: abi.encodePacked(address account, string denom)
func (p Precompile) BalanceOf(ctx sdk.Context, input []byte) ([]byte, error) {
	if len(input) < 20 {
		return nil, ErrInputTooShort
	}
	account := common.BytesToAddress(input[:20])
	denom := string(input[20:])
	balance := p.bankKeeper.GetBalance(ctx, account.Bytes(), denom).Amount
	return common.LeftPadBytes(balance.BigInt().Bytes(), 32), nil
}

// TransferFrom
// input format: abi.encodePacked(address from, address to, uint256 amount, string denom)
func (p Precompile) TransferFrom(ctx sdk.Context, caller common.Address, input []byte) ([]byte, error) {
	if len(input) < 20*2+32 {
		return nil, ErrInputTooShort
	}

	from := common.BytesToAddress(input[:20])
	to := common.BytesToAddress(input[20 : 20+20])
	amount := new(big.Int).SetBytes(input[40 : 40+32])
	denom := string(input[72:])

	// don't handle gas token here
	if denom == evmtypes.GetEVMCoinDenom() {
		return nil, errors.New("cannot transfer gas token with bank precompile")
	}

	// authorization: only from address or deterministic erc20 contract address can call this method
	if caller != from && caller != ERC20ContractAddress(p.Address(), denom) {
		return nil, ErrUnauthorized
	}

	coins := sdk.Coins{{Denom: denom, Amount: sdkmath.NewIntFromBigInt(amount)}}
	if err := coins.Validate(); err != nil {
		return nil, fmt.Errorf("invalid coins: %w", err)
	}

	// execute the transfer with bank keeper
	msg := banktypes.NewMsgSend(from.Bytes(), to.Bytes(), coins)
	if _, err := p.msgServer.Send(ctx, msg); err != nil {
		return nil, fmt.Errorf("failed to send coins: %w", err)
	}

	return []byte{1}, nil
}

func (p Precompile) Execute(ctx sdk.Context, contract *vm.Contract, readonly bool) ([]byte, error) {
	// 1 byte method selector
	if len(contract.Input) == 0 {
		return nil, ErrInputTooShort
	}

	method := BankMethod(contract.Input[0])
	if readonly && p.IsTransaction(method) {
		return nil, vm.ErrWriteProtection
	}

	input := contract.Input[1:]
	switch method {
	case MethodName:
		return p.Name(ctx, input)
	case MethodSymbol:
		return p.Symbol(ctx, input)
	case MethodDecimals:
		return p.Decimals(ctx, input)
	case MethodTotalSupply:
		return p.TotalSupply(ctx, input)
	case MethodBalanceOf:
		return p.BalanceOf(ctx, input)
	case MethodTransferFrom:
		return p.TransferFrom(ctx, contract.Caller(), input)
	}

	return nil, fmt.Errorf(ErrUnknownMethod, method)
}

// ERC20ContractAddress computes the contract address deployed with create2 factory contract.
// create2 factory: https://github.com/Arachnid/deterministic-deployment-proxy
//
// `keccak(0xff || factory || salt || keccak(bytecode || ctor))[12:]`
func ERC20ContractAddress(contract common.Address, denom string) common.Address {
	bz := crypto.Keccak256(
		[]byte{0xff},
		Create2FactoryAddress.Bytes(),
		ERC20Salt,
		crypto.Keccak256(
			ERC20Bin,
			ERC20Constructor(denom, contract),
		),
	)[12:]
	return common.BytesToAddress(bz)
}

// ERC20Constructor builds the constructor args for the ERC20 contract,
// equivalent to `abi.encode(string denom, address bank)`
func ERC20Constructor(denom string, bank common.Address) []byte {
	paddedDenomLen := padTo32(len(denom))
	bufSize := 32*3 + paddedDenomLen // string offset + bank + string length + denom

	buf := make([]byte, bufSize)
	buf[31] = 32 * 2                // string offset
	copy(buf[32+12:], bank.Bytes()) // bank contract
	binary.BigEndian.PutUint64(     // string length
		buf[32*2+24:],
		uint64(len(denom)),
	)
	copy(buf[32*3:], []byte(denom)) // string data
	return buf
}

func padTo32(size int) int {
	remainder := size % 32
	if remainder == 0 {
		return size
	}
	return size + 32 - remainder
}
