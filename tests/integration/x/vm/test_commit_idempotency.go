package vm

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/holiman/uint256"

	vmkeeper "github.com/cosmos/evm/x/vm/keeper"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TestCommitIdempotency verifies that calling FlushToCacheCtx multiple times
// without any state changes produces identical results across all branches in commitWithCtx:
// - DeleteAccount (self-destructed)
// - SetCode/DeleteCode (code operations)
// - SetAccount (account updates)
// - SetState/DeleteState (storage operations)
func (s *KeeperTestSuite) TestCommitIdempotency() {
	s.SetupTest()
	evmKeeper := s.Network.App.GetEVMKeeper()

	// Setup test accounts
	addr1 := common.BytesToAddress([]byte("addr1"))
	addr2 := common.BytesToAddress([]byte("addr2"))
	addr3 := common.BytesToAddress([]byte("addr3"))
	addr4 := common.BytesToAddress([]byte("addr4"))

	// Test data
	code := []byte{0x60, 0x80, 0x60, 0x40, 0x52} // sample bytecode
	emptyCodeHash := crypto.Keccak256Hash(nil)
	storageKey1 := common.HexToHash("0x1")
	storageKey2 := common.HexToHash("0x2")

	// Setup state to cover all branches
	db := s.StateDB()
	cacheCtx, err := db.GetCacheContext()
	s.Require().NoError(err)

	// addr1: Account with code and storage (tests SetCode + SetState)
	db.CreateAccount(addr1)
	code1 := []byte{0x60, 0x42}
	db.SetCode(addr1, code1)
	db.SetState(addr1, storageKey1, common.HexToHash("0x456"))
	db.AddBalance(addr1, uint256ToInt(big.NewInt(1000)), 0)

	// addr2: Account with empty/deleted code (tests DeleteCode)
	db.CreateAccount(addr2)
	db.SetCode(addr2, nil)
	db.AddBalance(addr2, uint256ToInt(big.NewInt(2000)), 0)

	// addr3: Self-destructed account (tests DeleteAccount)
	db.CreateAccount(addr3)
	db.SetCode(addr3, code)
	db.SelfDestruct(addr3)

	// addr4: Account with storage deleted (tests DeleteState)
	db.CreateAccount(addr4)
	db.SetState(addr4, storageKey1, common.Hash{}) // deleted storage
	db.SetState(addr4, storageKey2, common.HexToHash("0x789"))
	db.AddBalance(addr4, uint256ToInt(big.NewInt(4000)), 0)

	// Commit once to persist state
	err = db.FlushToCacheCtx()
	s.Require().NoError(err)

	// Capture state after first commit
	snapshot1 := captureState(cacheCtx, evmKeeper, []common.Address{addr1, addr2, addr3, addr4})

	err = db.FlushToCacheCtx()
	s.Require().NoError(err)

	snapshot2 := captureState(cacheCtx, evmKeeper, []common.Address{addr1, addr2, addr3, addr4})

	err = db.FlushToCacheCtx()
	s.Require().NoError(err)

	snapshot3 := captureState(cacheCtx, evmKeeper, []common.Address{addr1, addr2, addr3, addr4})

	// All snapshots should be identical
	s.Require().Equal(snapshot1, snapshot2, "First and second commit should produce identical state")
	s.Require().Equal(snapshot2, snapshot3, "Second and third commit should produce identical state")

	// Verify specific invariants
	s.Require().Equal(code1, evmKeeper.GetCode(cacheCtx, crypto.Keccak256Hash(code1)))
	s.Require().Empty(evmKeeper.GetCode(cacheCtx, emptyCodeHash))
	s.Require().Nil(evmKeeper.GetAccount(cacheCtx, addr3))
	s.Require().Equal(common.Hash{}, evmKeeper.GetState(cacheCtx, addr4, storageKey1))
}

// TestCommitIdempotencyWithStorage tests idempotency of storage operations
func (s *KeeperTestSuite) TestCommitIdempotencyWithStorage() {
	s.SetupTest()
	evmKeeper := s.Network.App.GetEVMKeeper()

	addr := common.BytesToAddress([]byte("testaddr"))
	storageKey := common.HexToHash("0x1")
	targetValue := common.HexToHash("0x222")

	// Setup: Create account and set storage
	db := s.StateDB()
	cacheCtx, err := db.GetCacheContext()
	s.Require().NoError(err)
	db.CreateAccount(addr)
	db.SetState(addr, storageKey, targetValue)
	err = db.FlushToCacheCtx()
	s.Require().NoError(err)

	snapshot1 := evmKeeper.GetState(cacheCtx, addr, storageKey)

	// Multiple commits without changes should be idempotent
	err = db.FlushToCacheCtx()
	s.Require().NoError(err)
	snapshot2 := evmKeeper.GetState(cacheCtx, addr, storageKey)

	err = db.FlushToCacheCtx()
	s.Require().NoError(err)
	snapshot3 := evmKeeper.GetState(cacheCtx, addr, storageKey)

	s.Require().Equal(snapshot1, snapshot2)
	s.Require().Equal(snapshot2, snapshot3)
	s.Require().Equal(targetValue, snapshot3)
}

// TestCommitIdempotencyWithCodeDeletion tests idempotency of code deletion
func (s *KeeperTestSuite) TestCommitIdempotencyWithCodeDeletion() {
	s.SetupTest()
	evmKeeper := s.Network.App.GetEVMKeeper()

	addr := common.BytesToAddress([]byte("testaddr"))

	// Setup: Create account and delete code
	db := s.StateDB()
	cacheCtx, err := db.GetCacheContext()
	s.Require().NoError(err)
	db.CreateAccount(addr)
	db.SetCode(addr, nil)
	err = db.FlushToCacheCtx()
	s.Require().NoError(err)

	codeHash1 := evmKeeper.GetCodeHash(cacheCtx, addr)

	// Multiple commits without changes should be idempotent
	err = db.FlushToCacheCtx()
	s.Require().NoError(err)
	codeHash2 := evmKeeper.GetCodeHash(cacheCtx, addr)

	err = db.FlushToCacheCtx()
	s.Require().NoError(err)
	codeHash3 := evmKeeper.GetCodeHash(cacheCtx, addr)

	s.Require().Equal(codeHash1, codeHash2)
	s.Require().Equal(codeHash2, codeHash3)
}

// TestCommitIdempotencyWithSelfDestruct tests idempotency of account deletion
func (s *KeeperTestSuite) TestCommitIdempotencyWithSelfDestruct() {
	s.SetupTest()
	evmKeeper := s.Network.App.GetEVMKeeper()

	addr := common.BytesToAddress([]byte("testaddr"))

	// Setup: Create account and self-destruct
	db := s.StateDB()
	cacheCtx, err := db.GetCacheContext()
	s.Require().NoError(err)
	db.CreateAccount(addr)
	db.SelfDestruct(addr)
	err = db.FlushToCacheCtx()
	s.Require().NoError(err)

	account1 := evmKeeper.GetAccount(cacheCtx, addr)

	// Multiple commits without changes should be idempotent
	err = db.FlushToCacheCtx()
	s.Require().NoError(err)
	account2 := evmKeeper.GetAccount(cacheCtx, addr)

	err = db.FlushToCacheCtx()
	s.Require().NoError(err)
	account3 := evmKeeper.GetAccount(cacheCtx, addr)

	s.Require().Nil(account1)
	s.Require().Nil(account2)
	s.Require().Nil(account3)
}

// accountState captures relevant account state for comparison
type accountState struct {
	Exists       bool
	Balance      *big.Int
	Nonce        uint64
	CodeHash     common.Hash
	StorageState map[common.Hash]common.Hash
}

// stateSnapshot captures the state of multiple accounts
type stateSnapshot map[common.Address]accountState

// captureState reads and captures the current state of given addresses
func captureState(ctx sdk.Context, evmKeeper *vmkeeper.Keeper, addrs []common.Address) stateSnapshot {
	snapshot := make(stateSnapshot)

	for _, addr := range addrs {
		account := evmKeeper.GetAccount(ctx, addr)
		if account == nil {
			snapshot[addr] = accountState{Exists: false}
			continue
		}

		storage := make(map[common.Hash]common.Hash)
		// Capture all storage keys for this address
		evmKeeper.ForEachStorage(ctx, addr, func(key, value common.Hash) bool {
			storage[key] = value
			return true
		})

		snapshot[addr] = accountState{
			Exists:       true,
			Balance:      account.Balance.ToBig(),
			Nonce:        account.Nonce,
			CodeHash:     common.BytesToHash(account.CodeHash),
			StorageState: storage,
		}
	}

	return snapshot
}

func uint256ToInt(i *big.Int) *uint256.Int {
	u, _ := uint256.FromBig(i)
	return u
}
