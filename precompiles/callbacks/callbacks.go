// Code generated - DO NOT EDIT.
// This file is a generated binding and any manual changes will be lost.

package callbacks

import (
	"errors"
	"math/big"
	"strings"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

// Reference imports to suppress errors if they are not otherwise used.
var (
	_ = errors.New
	_ = big.NewInt
	_ = strings.NewReader
	_ = ethereum.NotFound
	_ = bind.Bind
	_ = common.Big1
	_ = types.BloomLookup
	_ = event.NewSubscription
	_ = abi.ConvertType
)

// PrecompileMetaData contains all meta data concerning the Precompile contract.
var PrecompileMetaData = &bind.MetaData{
	ABI: "[{\"inputs\":[{\"internalType\":\"string\",\"name\":\"channelId\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"portId\",\"type\":\"string\"},{\"internalType\":\"uint64\",\"name\":\"sequence\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"},{\"internalType\":\"bytes\",\"name\":\"acknowledgement\",\"type\":\"bytes\"}],\"name\":\"onPacketAcknowledgement\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"},{\"inputs\":[{\"internalType\":\"string\",\"name\":\"channelId\",\"type\":\"string\"},{\"internalType\":\"string\",\"name\":\"portId\",\"type\":\"string\"},{\"internalType\":\"uint64\",\"name\":\"sequence\",\"type\":\"uint64\"},{\"internalType\":\"bytes\",\"name\":\"data\",\"type\":\"bytes\"}],\"name\":\"onPacketTimeout\",\"outputs\":[],\"stateMutability\":\"nonpayable\",\"type\":\"function\"}]",
}

// PrecompileABI is the input ABI used to generate the binding from.
// Deprecated: Use PrecompileMetaData.ABI instead.
var PrecompileABI = PrecompileMetaData.ABI

// Precompile is an auto generated Go binding around an Ethereum contract.
type Precompile struct {
	PrecompileCaller     // Read-only binding to the contract
	PrecompileTransactor // Write-only binding to the contract
	PrecompileFilterer   // Log filterer for contract events
}

// PrecompileCaller is an auto generated read-only Go binding around an Ethereum contract.
type PrecompileCaller struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PrecompileTransactor is an auto generated write-only Go binding around an Ethereum contract.
type PrecompileTransactor struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PrecompileFilterer is an auto generated log filtering Go binding around an Ethereum contract events.
type PrecompileFilterer struct {
	contract *bind.BoundContract // Generic contract wrapper for the low level calls
}

// PrecompileSession is an auto generated Go binding around an Ethereum contract,
// with pre-set call and transact options.
type PrecompileSession struct {
	Contract     *Precompile       // Generic contract binding to set the session for
	CallOpts     bind.CallOpts     // Call options to use throughout this session
	TransactOpts bind.TransactOpts // Transaction auth options to use throughout this session
}

// PrecompileCallerSession is an auto generated read-only Go binding around an Ethereum contract,
// with pre-set call options.
type PrecompileCallerSession struct {
	Contract *PrecompileCaller // Generic contract caller binding to set the session for
	CallOpts bind.CallOpts     // Call options to use throughout this session
}

// PrecompileTransactorSession is an auto generated write-only Go binding around an Ethereum contract,
// with pre-set transact options.
type PrecompileTransactorSession struct {
	Contract     *PrecompileTransactor // Generic contract transactor binding to set the session for
	TransactOpts bind.TransactOpts     // Transaction auth options to use throughout this session
}

// PrecompileRaw is an auto generated low-level Go binding around an Ethereum contract.
type PrecompileRaw struct {
	Contract *Precompile // Generic contract binding to access the raw methods on
}

// PrecompileCallerRaw is an auto generated low-level read-only Go binding around an Ethereum contract.
type PrecompileCallerRaw struct {
	Contract *PrecompileCaller // Generic read-only contract binding to access the raw methods on
}

// PrecompileTransactorRaw is an auto generated low-level write-only Go binding around an Ethereum contract.
type PrecompileTransactorRaw struct {
	Contract *PrecompileTransactor // Generic write-only contract binding to access the raw methods on
}

// NewPrecompile creates a new instance of Precompile, bound to a specific deployed contract.
func NewPrecompile(address common.Address, backend bind.ContractBackend) (*Precompile, error) {
	contract, err := bindPrecompile(address, backend, backend, backend)
	if err != nil {
		return nil, err
	}
	return &Precompile{PrecompileCaller: PrecompileCaller{contract: contract}, PrecompileTransactor: PrecompileTransactor{contract: contract}, PrecompileFilterer: PrecompileFilterer{contract: contract}}, nil
}

// NewPrecompileCaller creates a new read-only instance of Precompile, bound to a specific deployed contract.
func NewPrecompileCaller(address common.Address, caller bind.ContractCaller) (*PrecompileCaller, error) {
	contract, err := bindPrecompile(address, caller, nil, nil)
	if err != nil {
		return nil, err
	}
	return &PrecompileCaller{contract: contract}, nil
}

// NewPrecompileTransactor creates a new write-only instance of Precompile, bound to a specific deployed contract.
func NewPrecompileTransactor(address common.Address, transactor bind.ContractTransactor) (*PrecompileTransactor, error) {
	contract, err := bindPrecompile(address, nil, transactor, nil)
	if err != nil {
		return nil, err
	}
	return &PrecompileTransactor{contract: contract}, nil
}

// NewPrecompileFilterer creates a new log filterer instance of Precompile, bound to a specific deployed contract.
func NewPrecompileFilterer(address common.Address, filterer bind.ContractFilterer) (*PrecompileFilterer, error) {
	contract, err := bindPrecompile(address, nil, nil, filterer)
	if err != nil {
		return nil, err
	}
	return &PrecompileFilterer{contract: contract}, nil
}

// bindPrecompile binds a generic wrapper to an already deployed contract.
func bindPrecompile(address common.Address, caller bind.ContractCaller, transactor bind.ContractTransactor, filterer bind.ContractFilterer) (*bind.BoundContract, error) {
	parsed, err := PrecompileMetaData.GetAbi()
	if err != nil {
		return nil, err
	}
	return bind.NewBoundContract(address, *parsed, caller, transactor, filterer), nil
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Precompile *PrecompileRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Precompile.Contract.PrecompileCaller.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Precompile *PrecompileRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Precompile.Contract.PrecompileTransactor.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Precompile *PrecompileRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Precompile.Contract.PrecompileTransactor.contract.Transact(opts, method, params...)
}

// Call invokes the (constant) contract method with params as input values and
// sets the output to result. The result type might be a single field for simple
// returns, a slice of interfaces for anonymous returns and a struct for named
// returns.
func (_Precompile *PrecompileCallerRaw) Call(opts *bind.CallOpts, result *[]interface{}, method string, params ...interface{}) error {
	return _Precompile.Contract.contract.Call(opts, result, method, params...)
}

// Transfer initiates a plain transaction to move funds to the contract, calling
// its default method if one is available.
func (_Precompile *PrecompileTransactorRaw) Transfer(opts *bind.TransactOpts) (*types.Transaction, error) {
	return _Precompile.Contract.contract.Transfer(opts)
}

// Transact invokes the (paid) contract method with params as input values.
func (_Precompile *PrecompileTransactorRaw) Transact(opts *bind.TransactOpts, method string, params ...interface{}) (*types.Transaction, error) {
	return _Precompile.Contract.contract.Transact(opts, method, params...)
}

// OnPacketAcknowledgement is a paid mutator transaction binding the contract method 0x39b4073a.
//
// Solidity: function onPacketAcknowledgement(string channelId, string portId, uint64 sequence, bytes data, bytes acknowledgement) returns()
func (_Precompile *PrecompileTransactor) OnPacketAcknowledgement(opts *bind.TransactOpts, channelId string, portId string, sequence uint64, data []byte, acknowledgement []byte) (*types.Transaction, error) {
	return _Precompile.contract.Transact(opts, "onPacketAcknowledgement", channelId, portId, sequence, data, acknowledgement)
}

// OnPacketAcknowledgement is a paid mutator transaction binding the contract method 0x39b4073a.
//
// Solidity: function onPacketAcknowledgement(string channelId, string portId, uint64 sequence, bytes data, bytes acknowledgement) returns()
func (_Precompile *PrecompileSession) OnPacketAcknowledgement(channelId string, portId string, sequence uint64, data []byte, acknowledgement []byte) (*types.Transaction, error) {
	return _Precompile.Contract.OnPacketAcknowledgement(&_Precompile.TransactOpts, channelId, portId, sequence, data, acknowledgement)
}

// OnPacketAcknowledgement is a paid mutator transaction binding the contract method 0x39b4073a.
//
// Solidity: function onPacketAcknowledgement(string channelId, string portId, uint64 sequence, bytes data, bytes acknowledgement) returns()
func (_Precompile *PrecompileTransactorSession) OnPacketAcknowledgement(channelId string, portId string, sequence uint64, data []byte, acknowledgement []byte) (*types.Transaction, error) {
	return _Precompile.Contract.OnPacketAcknowledgement(&_Precompile.TransactOpts, channelId, portId, sequence, data, acknowledgement)
}

// OnPacketTimeout is a paid mutator transaction binding the contract method 0x1f8ee603.
//
// Solidity: function onPacketTimeout(string channelId, string portId, uint64 sequence, bytes data) returns()
func (_Precompile *PrecompileTransactor) OnPacketTimeout(opts *bind.TransactOpts, channelId string, portId string, sequence uint64, data []byte) (*types.Transaction, error) {
	return _Precompile.contract.Transact(opts, "onPacketTimeout", channelId, portId, sequence, data)
}

// OnPacketTimeout is a paid mutator transaction binding the contract method 0x1f8ee603.
//
// Solidity: function onPacketTimeout(string channelId, string portId, uint64 sequence, bytes data) returns()
func (_Precompile *PrecompileSession) OnPacketTimeout(channelId string, portId string, sequence uint64, data []byte) (*types.Transaction, error) {
	return _Precompile.Contract.OnPacketTimeout(&_Precompile.TransactOpts, channelId, portId, sequence, data)
}

// OnPacketTimeout is a paid mutator transaction binding the contract method 0x1f8ee603.
//
// Solidity: function onPacketTimeout(string channelId, string portId, uint64 sequence, bytes data) returns()
func (_Precompile *PrecompileTransactorSession) OnPacketTimeout(channelId string, portId string, sequence uint64, data []byte) (*types.Transaction, error) {
	return _Precompile.Contract.OnPacketTimeout(&_Precompile.TransactOpts, channelId, portId, sequence, data)
}
