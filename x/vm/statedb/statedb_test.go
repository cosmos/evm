package statedb_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types/mocks"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	address       common.Address   = common.BigToAddress(big.NewInt(101))
	address2      common.Address   = common.BigToAddress(big.NewInt(102))
	address3      common.Address   = common.BigToAddress(big.NewInt(103))
	emptyTxConfig statedb.TxConfig = statedb.NewEmptyTxConfig()
)

type StateDBTestSuite struct {
	suite.Suite
}

func (suite *StateDBTestSuite) TestAccount() {
	key1 := common.BigToHash(big.NewInt(1))
	value1 := common.BigToHash(big.NewInt(2))
	key2 := common.BigToHash(big.NewInt(3))
	value2 := common.BigToHash(big.NewInt(4))
	testCases := []struct {
		name     string
		malleate func(sdk.Context, *statedb.StateDB)
	}{
		{"non-exist account", func(_ sdk.Context, db *statedb.StateDB) {
			suite.Require().Equal(false, db.Exist(address))
			suite.Require().Equal(true, db.Empty(address))
			suite.Require().Equal(common.U2560, db.GetBalance(address))
			suite.Require().Equal([]byte(nil), db.GetCode(address))
			suite.Require().Equal(common.Hash{}, db.GetCodeHash(address))
			suite.Require().Equal(uint64(0), db.GetNonce(address))
		}},
		{"empty account", func(ctx sdk.Context, db *statedb.StateDB) {
			db.CreateAccount(address)
			suite.Require().NoError(db.Commit())

			keeper := db.Keeper().(*mocks.EVMKeeper)
			acct := keeper.GetAccount(ctx, address)
			suite.Require().Equal(statedb.NewEmptyAccount(), acct)
			suite.Require().Empty(acct.Balance)
			suite.Require().False(acct.HasCodeHash())

			db = statedb.New(sdk.Context{}, keeper, emptyTxConfig)
			suite.Require().Equal(true, db.Exist(address))
			suite.Require().Equal(true, db.Empty(address))
			suite.Require().Equal(common.U2560, db.GetBalance(address))
			suite.Require().Equal([]byte(nil), db.GetCode(address))
			suite.Require().Equal(common.BytesToHash(mocks.EmptyCodeHash), db.GetCodeHash(address))
			suite.Require().Equal(uint64(0), db.GetNonce(address))
		}},
		{"self-destruct", func(ctx sdk.Context, db *statedb.StateDB) {
			// non-exist account.
			db.SelfDestruct(address)
			suite.Require().False(db.HasSelfDestructed(address))

			// create a contract account
			db.CreateAccount(address)
			db.SetCode(address, []byte("hello world"))
			db.AddBalance(address, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
			db.CreateContract(address)
			db.SetState(address, key1, value1)
			db.SetState(address, key2, value2)
			suite.Require().NoError(db.Commit())

			// SelfDestruct
			db = statedb.New(sdk.Context{}, db.Keeper(), emptyTxConfig)
			suite.Require().False(db.HasSelfDestructed(address))
			db.SelfDestruct(address)

			// check dirty state
			suite.Require().True(db.HasSelfDestructed(address))
			// balance is cleared
			suite.Require().Equal(common.U2560, db.GetBalance(address))
			// but code and state are still accessible in dirty state
			suite.Require().Equal(value1, db.GetState(address, key1))
			suite.Require().Equal([]byte("hello world"), db.GetCode(address))

			suite.Require().NoError(db.Commit())

			// not accessible from StateDB anymore
			db = statedb.New(sdk.Context{}, db.Keeper(), emptyTxConfig)
			suite.Require().False(db.Exist(address))

			// and cleared in keeper too
			keeper := db.Keeper().(*mocks.EVMKeeper)
			keeper.ForEachStorage(ctx, address, func(key, value common.Hash) bool {
				suite.Require().Equal(0, len(value.Bytes()))
				return true
			})
		}},
		{"self-destruct-6780 same tx", func(ctx sdk.Context, db *statedb.StateDB) {
			// non-exist account.
			db.SelfDestruct(address)
			suite.Require().False(db.HasSelfDestructed(address))

			// create a contract account
			db.CreateAccount(address)
			db.SetCode(address, []byte("hello world"))
			db.AddBalance(address, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
			db.CreateContract(address)
			db.SetState(address, key1, value1)
			db.SetState(address, key2, value2)

			// SelfDestruct
			suite.Require().False(db.HasSelfDestructed(address))
			_, _ = db.SelfDestruct6780(address)

			// check dirty state
			suite.Require().True(db.HasSelfDestructed(address))
			// balance is cleared
			suite.Require().Equal(common.U2560, db.GetBalance(address))
			// but code and state are still accessible in dirty state
			suite.Require().Equal(value1, db.GetState(address, key1))
			suite.Require().Equal([]byte("hello world"), db.GetCode(address))

			suite.Require().NoError(db.Commit())

			// not accessible from StateDB anymore
			db = statedb.New(sdk.Context{}, db.Keeper(), emptyTxConfig)
			suite.Require().False(db.Exist(address))

			// and cleared in keeper too
			keeper := db.Keeper().(*mocks.EVMKeeper)
			keeper.ForEachStorage(ctx, address, func(key, value common.Hash) bool {
				suite.Require().Equal(0, len(value.Bytes()))
				return true
			})
		}},
		{"self-destruct-6780 different tx", func(ctx sdk.Context, db *statedb.StateDB) {
			// non-exist account.
			db.SelfDestruct(address)
			suite.Require().False(db.HasSelfDestructed(address))

			// create a contract account
			db.CreateAccount(address)
			db.SetCode(address, []byte("hello world"))
			db.AddBalance(address, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
			db.CreateContract(address)
			db.SetState(address, key1, value1)
			db.SetState(address, key2, value2)
			suite.Require().NoError(db.Commit())

			// SelfDestruct
			db = statedb.New(sdk.Context{}, db.Keeper(), emptyTxConfig)
			suite.Require().False(db.HasSelfDestructed(address))
			_, _ = db.SelfDestruct6780(address)

			// Same-tx is not marked as self-destructed
			suite.Require().False(db.HasSelfDestructed(address))
			// code and state are still accessible in dirty state
			suite.Require().Equal(value1, db.GetState(address, key1))
			suite.Require().Equal([]byte("hello world"), db.GetCode(address))

			suite.Require().NoError(db.Commit())

			// Same-tx maintains state
			db = statedb.New(sdk.Context{}, db.Keeper(), emptyTxConfig)
			suite.Require().True(db.Exist(address))
			suite.Require().False(db.HasSelfDestructed(address))
			// but code and state are still accessible in dirty state
			suite.Require().Equal(value1, db.GetState(address, key1))
			suite.Require().Equal([]byte("hello world"), db.GetCode(address))

			// and not cleared in keeper too
			keeper := db.Keeper().(*mocks.EVMKeeper)
			acc := keeper.GetAccount(ctx, address)
			suite.Require().NotNil(acc)
			keeper.ForEachStorage(ctx, address, func(key, value common.Hash) bool {
				suite.Require().Greater(len(value.Bytes()), 0)
				return len(value) == 0
			})
		}},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			ctx := sdk.Context{}.WithEventManager(sdk.NewEventManager())
			keeper := mocks.NewEVMKeeper()
			db := statedb.New(sdk.Context{}, keeper, emptyTxConfig)
			tc.malleate(ctx, db)
		})
	}
}

func (suite *StateDBTestSuite) TestAccountOverride() {
	keeper := mocks.NewEVMKeeper()
	db := statedb.New(sdk.Context{}, keeper, emptyTxConfig)
	// test balance carry over when overwritten
	amount := uint256.NewInt(1)

	// init an EOA account, account overridden only happens on EOA account.
	db.AddBalance(address, amount, tracing.BalanceChangeUnspecified)
	db.SetNonce(address, 1, tracing.NonceChangeUnspecified)

	// override
	db.CreateAccount(address)

	// check balance is not lost
	suite.Require().Equal(amount, db.GetBalance(address))
	// but nonce is reset
	suite.Require().Equal(uint64(0), db.GetNonce(address))
}

func (suite *StateDBTestSuite) TestDBError() {
	testCases := []struct {
		name     string
		malleate func(vm.StateDB)
	}{
		{"set account", func(db vm.StateDB) {
			db.SetNonce(mocks.ErrAddress, 1, tracing.NonceChangeUnspecified)
		}},
		{"delete account", func(db vm.StateDB) {
			db.SetNonce(mocks.ErrAddress, 1, tracing.NonceChangeUnspecified)
			db.SelfDestruct(mocks.ErrAddress)
			suite.Require().True(db.HasSelfDestructed(mocks.ErrAddress))
		}},
	}
	for _, tc := range testCases {
		db := statedb.New(sdk.Context{}, mocks.NewEVMKeeper(), emptyTxConfig)
		tc.malleate(db)
		suite.Require().Error(db.Commit())
	}
}

func (suite *StateDBTestSuite) TestBalance() {
	// NOTE: no need to test overflow/underflow, that is guaranteed by evm implementation.
	testCases := []struct {
		name       string
		malleate   func(*statedb.StateDB)
		expBalance *uint256.Int
	}{
		{"add balance", func(db *statedb.StateDB) {
			db.AddBalance(address, uint256.NewInt(10), tracing.BalanceChangeUnspecified)
		}, uint256.NewInt(10)},
		{"sub balance", func(db *statedb.StateDB) {
			db.AddBalance(address, uint256.NewInt(10), tracing.BalanceChangeUnspecified)
			// get dirty balance
			suite.Require().Equal(uint256.NewInt(10), db.GetBalance(address))
			db.SubBalance(address, uint256.NewInt(2), tracing.BalanceChangeUnspecified)
		}, uint256.NewInt(8)},
		{"add zero balance", func(db *statedb.StateDB) {
			db.AddBalance(address, uint256.NewInt(0), tracing.BalanceChangeUnspecified)
		}, uint256.NewInt(0)},
		{"sub zero balance", func(db *statedb.StateDB) {
			db.SubBalance(address, uint256.NewInt(0), tracing.BalanceChangeUnspecified)
		}, uint256.NewInt(0)},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			ctx := sdk.Context{}.WithEventManager(sdk.NewEventManager())
			keeper := mocks.NewEVMKeeper()
			db := statedb.New(sdk.Context{}, keeper, emptyTxConfig)
			tc.malleate(db)

			// check dirty state
			suite.Require().Equal(tc.expBalance, db.GetBalance(address))
			suite.Require().NoError(db.Commit())
			// check committed balance too
			suite.Require().Equal(tc.expBalance, keeper.GetAccount(ctx, address).Balance)
		})
	}
}

func (suite *StateDBTestSuite) TestState() {
	key1 := common.BigToHash(big.NewInt(1))
	value1 := common.BigToHash(big.NewInt(1))
	testCases := []struct {
		name      string
		malleate  func(*statedb.StateDB)
		expStates statedb.Storage
	}{
		{"empty state", func(_ *statedb.StateDB) {
		}, nil},
		{"set empty value", func(db *statedb.StateDB) {
			db.SetState(address, key1, common.Hash{})
		}, statedb.Storage{}},
		{"set state even if same as original value (due to possible reverts within precompile calls)", func(db *statedb.StateDB) {
			db.SetState(address, key1, value1)
			db.SetState(address, key1, common.Hash{})
		}, statedb.Storage{}},
		{"set state", func(db *statedb.StateDB) {
			// check empty initial state
			suite.Require().Equal(common.Hash{}, db.GetState(address, key1))
			suite.Require().Equal(common.Hash{}, db.GetCommittedState(address, key1))

			// set state
			db.SetState(address, key1, value1)
			// query dirty state
			suite.Require().Equal(value1, db.GetState(address, key1))
			// check committed state is still not exist
			suite.Require().Equal(common.Hash{}, db.GetCommittedState(address, key1))

			// set same value again, should be noop
			db.SetState(address, key1, value1)
			suite.Require().Equal(value1, db.GetState(address, key1))
		}, statedb.Storage{
			key1: value1,
		}},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			ctx := sdk.Context{}.WithEventManager(sdk.NewEventManager())
			keeper := mocks.NewEVMKeeper()
			db := statedb.New(sdk.Context{}, keeper, emptyTxConfig)
			tc.malleate(db)
			suite.Require().NoError(db.Commit())

			// check committed states in keeper
			for _, key := range tc.expStates.SortedKeys() {
				suite.Require().Equal(tc.expStates[key], keeper.GetState(ctx, address, key))
			}

			// check ForEachStorage
			db = statedb.New(sdk.Context{}, keeper, emptyTxConfig)
			collected := CollectContractStorage(db)
			if len(tc.expStates) > 0 {
				suite.Require().Equal(tc.expStates, collected)
			} else {
				suite.Require().Empty(collected)
			}
		})
	}
}

func (suite *StateDBTestSuite) TestCode() {
	code := []byte("hello world")
	codeHash := crypto.Keccak256Hash(code)

	testCases := []struct {
		name        string
		malleate    func(vm.StateDB)
		expCode     []byte
		expCodeHash common.Hash
	}{
		{"non-exist account", func(vm.StateDB) {}, nil, common.Hash{}},
		{"empty account", func(db vm.StateDB) {
			db.CreateAccount(address)
		}, nil, common.BytesToHash(mocks.EmptyCodeHash)},
		{"set code", func(db vm.StateDB) {
			db.SetCode(address, code)
		}, code, codeHash},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			keeper := mocks.NewEVMKeeper()
			db := statedb.New(sdk.Context{}, keeper, emptyTxConfig)
			tc.malleate(db)

			// check dirty state
			suite.Require().Equal(tc.expCode, db.GetCode(address))
			suite.Require().Equal(len(tc.expCode), db.GetCodeSize(address))
			suite.Require().Equal(tc.expCodeHash, db.GetCodeHash(address))

			suite.Require().NoError(db.Commit())

			// check again
			db = statedb.New(sdk.Context{}, keeper, emptyTxConfig)
			suite.Require().Equal(tc.expCode, db.GetCode(address))
			suite.Require().Equal(len(tc.expCode), db.GetCodeSize(address))
			suite.Require().Equal(tc.expCodeHash, db.GetCodeHash(address))
		})
	}
}

func (suite *StateDBTestSuite) TestRevertSnapshot() {
	v1 := common.BigToHash(big.NewInt(1))
	v2 := common.BigToHash(big.NewInt(2))
	v3 := common.BigToHash(big.NewInt(3))
	testCases := []struct {
		name     string
		malleate func(vm.StateDB)
	}{
		{"set state", func(db vm.StateDB) {
			db.SetState(address, v1, v3)
		}},
		{"set nonce", func(db vm.StateDB) {
			db.SetNonce(address, 10, tracing.NonceChangeUnspecified)
		}},
		{"change balance", func(db vm.StateDB) {
			db.AddBalance(address, uint256.NewInt(10), tracing.BalanceChangeUnspecified)
			db.SubBalance(address, uint256.NewInt(5), tracing.BalanceChangeUnspecified)
		}},
		{"override account", func(db vm.StateDB) {
			db.CreateAccount(address)
		}},
		{"set code", func(db vm.StateDB) {
			db.SetCode(address, []byte("hello world"))
		}},
		{"suicide", func(db vm.StateDB) {
			db.SetState(address, v1, v2)
			db.SetCode(address, []byte("hello world"))
			db.SelfDestruct(address)
			suite.Require().True(db.HasSelfDestructed(address))
		}},
		{"add log", func(db vm.StateDB) {
			db.AddLog(&ethtypes.Log{
				Address: address,
			})
		}},
		{"add refund", func(db vm.StateDB) {
			db.AddRefund(10)
			db.SubRefund(5)
		}},
		{"access list", func(db vm.StateDB) {
			db.AddAddressToAccessList(address)
			db.AddSlotToAccessList(address, v1)
		}},
	}
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			ctx := sdk.Context{}.WithEventManager(sdk.NewEventManager())
			keeper := mocks.NewEVMKeeper()

			{
				// do some arbitrary changes to the storage
				db := statedb.New(ctx, keeper, emptyTxConfig)
				db.SetNonce(address, 1, tracing.NonceChangeUnspecified)
				db.AddBalance(address, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
				db.SetCode(address, []byte("hello world"))
				db.SetState(address, v1, v2)
				db.SetNonce(address2, 1, tracing.NonceChangeUnspecified)
				suite.Require().NoError(db.Commit())
			}

			originalKeeper := keeper.Clone()

			// run test
			db := statedb.New(ctx, keeper, emptyTxConfig)
			rev := db.Snapshot()
			tc.malleate(db)
			db.RevertToSnapshot(rev)

			// check empty states after revert
			suite.Require().Zero(db.GetRefund())
			suite.Require().Empty(db.Logs())

			suite.Require().NoError(db.Commit())

			// check keeper should stay the same
			suite.Require().Equal(originalKeeper, keeper)
		})
	}
}

func (suite *StateDBTestSuite) TestNestedSnapshot() {
	key := common.BigToHash(big.NewInt(1))
	value1 := common.BigToHash(big.NewInt(1))
	value2 := common.BigToHash(big.NewInt(2))

	db := statedb.New(sdk.Context{}.WithEventManager(sdk.NewEventManager()), mocks.NewEVMKeeper(), emptyTxConfig)

	rev1 := db.Snapshot()
	db.SetState(address, key, value1)

	rev2 := db.Snapshot()
	db.SetState(address, key, value2)
	suite.Require().Equal(value2, db.GetState(address, key))

	db.RevertToSnapshot(rev2)
	suite.Require().Equal(value1, db.GetState(address, key))

	db.RevertToSnapshot(rev1)
	suite.Require().Equal(common.Hash{}, db.GetState(address, key))
}

func (suite *StateDBTestSuite) TestInvalidSnapshotId() {
	db := statedb.New(sdk.Context{}, mocks.NewEVMKeeper(), emptyTxConfig)
	suite.Require().Panics(func() {
		db.RevertToSnapshot(1)
	})
}

func (suite *StateDBTestSuite) TestAccessList() {
	value1 := common.BigToHash(big.NewInt(1))
	value2 := common.BigToHash(big.NewInt(2))

	testCases := []struct {
		name     string
		malleate func(vm.StateDB)
	}{
		{"add address", func(db vm.StateDB) {
			suite.Require().False(db.AddressInAccessList(address))
			db.AddAddressToAccessList(address)
			suite.Require().True(db.AddressInAccessList(address))

			addrPresent, slotPresent := db.SlotInAccessList(address, value1)
			suite.Require().True(addrPresent)
			suite.Require().False(slotPresent)

			// add again, should be no-op
			db.AddAddressToAccessList(address)
			suite.Require().True(db.AddressInAccessList(address))
		}},
		{"add slot", func(db vm.StateDB) {
			addrPresent, slotPresent := db.SlotInAccessList(address, value1)
			suite.Require().False(addrPresent)
			suite.Require().False(slotPresent)
			db.AddSlotToAccessList(address, value1)
			addrPresent, slotPresent = db.SlotInAccessList(address, value1)
			suite.Require().True(addrPresent)
			suite.Require().True(slotPresent)

			// add another slot
			db.AddSlotToAccessList(address, value2)
			addrPresent, slotPresent = db.SlotInAccessList(address, value2)
			suite.Require().True(addrPresent)
			suite.Require().True(slotPresent)

			// add again, should be noop
			db.AddSlotToAccessList(address, value2)
			addrPresent, slotPresent = db.SlotInAccessList(address, value2)
			suite.Require().True(addrPresent)
			suite.Require().True(slotPresent)
		}},
		{"prepare access list", func(db vm.StateDB) {
			al := ethtypes.AccessList{{
				Address:     address3,
				StorageKeys: []common.Hash{value1},
			}}

			rules := ethparams.Rules{
				ChainID:          big.NewInt(1000),
				IsHomestead:      true,
				IsEIP150:         true,
				IsEIP155:         true,
				IsEIP158:         true,
				IsByzantium:      true,
				IsConstantinople: true,
				IsPetersburg:     true,
				IsIstanbul:       true,
				IsBerlin:         true,
				IsLondon:         true,
				IsMerge:          true,
				IsShanghai:       true,
				IsCancun:         true,
				IsEIP2929:        true,
				IsPrague:         true,
			}
			db.Prepare(rules, address, common.Address{}, &address2, vm.PrecompiledAddressesPrague, al)

			// check sender and dst
			suite.Require().True(db.AddressInAccessList(address))
			suite.Require().True(db.AddressInAccessList(address2))
			// check precompiles
			suite.Require().True(db.AddressInAccessList(common.BytesToAddress([]byte{1})))
			// check AccessList
			suite.Require().True(db.AddressInAccessList(address3))
			addrPresent, slotPresent := db.SlotInAccessList(address3, value1)
			suite.Require().True(addrPresent)
			suite.Require().True(slotPresent)
			addrPresent, slotPresent = db.SlotInAccessList(address3, value2)
			suite.Require().True(addrPresent)
			suite.Require().False(slotPresent)
		}},
	}

	for _, tc := range testCases {
		db := statedb.New(sdk.Context{}, mocks.NewEVMKeeper(), emptyTxConfig)
		tc.malleate(db)
	}
}

func (suite *StateDBTestSuite) TestLog() {
	txHash := common.BytesToHash([]byte("tx"))
	// use a non-default tx config
	txConfig := statedb.NewTxConfig(
		txHash,
		1,
	)
	db := statedb.New(sdk.Context{}, mocks.NewEVMKeeper(), txConfig)
	data := []byte("hello world")
	db.AddLog(&ethtypes.Log{
		Address:     address,
		Topics:      []common.Hash{},
		Data:        data,
		BlockNumber: 1,
	})
	suite.Require().Equal(1, len(db.Logs()))
	expecedLog := &ethtypes.Log{
		Address:     address,
		Topics:      []common.Hash{},
		Data:        data,
		BlockNumber: 1,
		TxIndex:     1,
		Index:       0,
	}
	suite.Require().Equal(expecedLog, db.Logs()[0])

	db.AddLog(&ethtypes.Log{
		Address:     address,
		Topics:      []common.Hash{},
		Data:        data,
		BlockNumber: 1,
	})
	suite.Require().Equal(2, len(db.Logs()))
	expecedLog.Index++
	suite.Require().Equal(expecedLog, db.Logs()[1])
}

func (suite *StateDBTestSuite) TestRefund() {
	testCases := []struct {
		name      string
		malleate  func(vm.StateDB)
		expRefund uint64
		expPanic  bool
	}{
		{"add refund", func(db vm.StateDB) {
			db.AddRefund(uint64(10))
		}, 10, false},
		{"sub refund", func(db vm.StateDB) {
			db.AddRefund(uint64(10))
			db.SubRefund(uint64(5))
		}, 5, false},
		{"negative refund counter", func(db vm.StateDB) {
			db.AddRefund(uint64(5))
			db.SubRefund(uint64(10))
		}, 0, true},
	}
	for _, tc := range testCases {
		db := statedb.New(sdk.Context{}, mocks.NewEVMKeeper(), emptyTxConfig)
		if !tc.expPanic {
			tc.malleate(db)
			suite.Require().Equal(tc.expRefund, db.GetRefund())
		} else {
			suite.Require().Panics(func() {
				tc.malleate(db)
			})
		}
	}
}

func (suite *StateDBTestSuite) TestIterateStorage() {
	ctx := sdk.Context{}.WithEventManager(sdk.NewEventManager())

	key1 := common.BigToHash(big.NewInt(1))
	value1 := common.BigToHash(big.NewInt(2))
	key2 := common.BigToHash(big.NewInt(3))
	value2 := common.BigToHash(big.NewInt(4))

	keeper := mocks.NewEVMKeeper()
	db := statedb.New(sdk.Context{}, keeper, emptyTxConfig)
	db.SetState(address, key1, value1)
	db.SetState(address, key2, value2)

	// ForEachStorage only iterate committed state
	suite.Require().Empty(CollectContractStorage(db))

	suite.Require().NoError(db.Commit())

	storage := CollectContractStorage(db)
	suite.Require().Equal(2, len(storage))
	for _, key := range storage.SortedKeys() {
		suite.Require().Equal(keeper.GetState(ctx, address, key), storage[key])
	}

	// break early iteration
	storage = make(statedb.Storage)
	err := db.ForEachStorage(address, func(k, v common.Hash) bool {
		storage[k] = v
		// return false to break early
		return false
	})
	suite.Require().NoError(err)
	suite.Require().Equal(1, len(storage))
}

func (suite *StateDBTestSuite) TestSetStorage() {
	contract := common.BigToAddress(big.NewInt(101))

	testCases := []struct {
		name     string
		prestate map[common.Hash]common.Hash
		assert   func(*statedb.StateDB)
	}{
		{
			"set storage",
			map[common.Hash]common.Hash{
				common.BigToHash(big.NewInt(0)): common.BigToHash(big.NewInt(0)),
				common.BigToHash(big.NewInt(1)): common.BigToHash(big.NewInt(1)),
				common.BigToHash(big.NewInt(2)): common.BigToHash(big.NewInt(2)),
			},
			func(db *statedb.StateDB) {
				db.SetStorage(contract, map[common.Hash]common.Hash{
					common.BigToHash(big.NewInt(1)): common.BigToHash(big.NewInt(3)),
				})

				suite.Require().Equal(common.Hash{}, db.GetState(contract, common.BigToHash(big.NewInt(0))))
				suite.Require().Equal(common.BigToHash(big.NewInt(3)), db.GetState(contract, common.BigToHash(big.NewInt(1))))
				suite.Require().Equal(common.Hash{}, db.GetState(contract, common.BigToHash(big.NewInt(2))))
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			keeper := mocks.NewEVMKeeper()
			db := statedb.New(sdk.Context{}, keeper, emptyTxConfig)
			for k, v := range tc.prestate {
				db.SetState(contract, k, v)
			}
			tc.assert(db)
		})
	}
}

func CollectContractStorage(db vm.StateDB) statedb.Storage {
	storage := make(statedb.Storage)
	stDB, ok := db.(*statedb.StateDB)
	if !ok {
		panic(fmt.Sprintf("invalid stateDB type %T", db))
	}
	err := stDB.ForEachStorage(address, func(k, v common.Hash) bool {
		storage[k] = v
		return true
	})
	if err != nil {
		return nil
	}

	return storage
}

func TestStateDBTestSuite(t *testing.T) {
	suite.Run(t, &StateDBTestSuite{})
}
