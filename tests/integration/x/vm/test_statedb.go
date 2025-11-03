package vm

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"
	"cosmossdk.io/store/prefix"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

func (s *KeeperTestSuite) TestCreateAccount() {
	testCases := []struct {
		name     string
		addr     common.Address
		malleate func(vm.StateDB, common.Address)
		callback func(vm.StateDB, common.Address)
	}{
		{
			"reset account (keep balance)",
			utiltx.GenerateAddress(),
			func(vmdb vm.StateDB, addr common.Address) {
				vmdb.AddBalance(addr, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
				s.Require().NotZero(vmdb.GetBalance(addr).Uint64())
			},
			func(vmdb vm.StateDB, addr common.Address) {
				s.Require().Equal(vmdb.GetBalance(addr).Uint64(), uint64(100))
			},
		},
		{
			"create account",
			utiltx.GenerateAddress(),
			func(vmdb vm.StateDB, addr common.Address) {
				s.Require().False(vmdb.Exist(addr))
			},
			func(vmdb vm.StateDB, addr common.Address) {
				s.Require().True(vmdb.Exist(addr))
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			tc.malleate(vmdb, tc.addr)
			vmdb.CreateAccount(tc.addr)
			tc.callback(vmdb, tc.addr)
		})
	}
}

func (s *KeeperTestSuite) TestAddBalance() {
	testCases := []struct {
		name   string
		amount *uint256.Int
		isNoOp bool
	}{
		{
			"positive amount",
			uint256.NewInt(100),
			false,
		},
		{
			"zero amount",
			uint256.NewInt(0),
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			prev := vmdb.GetBalance(s.Keyring.GetAddr(0))
			vmdb.AddBalance(s.Keyring.GetAddr(0), tc.amount, tracing.BalanceChangeUnspecified)
			post := vmdb.GetBalance(s.Keyring.GetAddr(0))

			if tc.isNoOp {
				s.Require().Equal(prev, post)
			} else {
				s.Require().Equal(new(uint256.Int).Add(prev, tc.amount), post)
			}
		})
	}
}

func (s *KeeperTestSuite) TestSubBalance() {
	testCases := []struct {
		name     string
		amount   *uint256.Int
		malleate func(vm.StateDB)
		isNoOp   bool
	}{
		{
			"positive amount, below zero",
			uint256.NewInt(100),
			func(vm.StateDB) {},
			false,
		},
		{
			"positive amount, above zero",
			uint256.NewInt(50),
			func(vmdb vm.StateDB) {
				vmdb.AddBalance(s.Keyring.GetAddr(0), uint256.NewInt(100), tracing.BalanceChangeUnspecified)
			},
			false,
		},
		{
			"zero amount",
			uint256.NewInt(0),
			func(vm.StateDB) {},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			tc.malleate(vmdb)

			prev := vmdb.GetBalance(s.Keyring.GetAddr(0))
			vmdb.SubBalance(s.Keyring.GetAddr(0), tc.amount, tracing.BalanceChangeUnspecified)
			post := vmdb.GetBalance(s.Keyring.GetAddr(0))

			if tc.isNoOp {
				s.Require().Equal(prev, post)
			} else {
				s.Require().Equal(new(uint256.Int).Sub(prev, tc.amount), post)
			}
		})
	}
}

func (s *KeeperTestSuite) TestGetNonce() {
	testCases := []struct {
		name          string
		address       common.Address
		expectedNonce uint64
		malleate      func(vm.StateDB)
	}{
		{
			"account not found",
			utiltx.GenerateAddress(),
			0,
			func(vm.StateDB) {},
		},
		{
			"existing account",
			s.Keyring.GetAddr(0),
			1,
			func(vmdb vm.StateDB) {
				vmdb.SetNonce(s.Keyring.GetAddr(0), 1, tracing.NonceChangeUnspecified)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			tc.malleate(vmdb)

			nonce := vmdb.GetNonce(tc.address)
			s.Require().Equal(tc.expectedNonce, nonce)
		})
	}
}

func (s *KeeperTestSuite) TestSetNonce() {
	testCases := []struct {
		name     string
		address  common.Address
		nonce    uint64
		malleate func()
	}{
		{
			"new account",
			utiltx.GenerateAddress(),
			10,
			func() {},
		},
		{
			"existing account",
			s.Keyring.GetAddr(0),
			99,
			func() {},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			vmdb.SetNonce(tc.address, tc.nonce, tracing.NonceChangeUnspecified)
			nonce := vmdb.GetNonce(tc.address)
			s.Require().Equal(tc.nonce, nonce)
		})
	}
}

func (s *KeeperTestSuite) TestGetCodeHash() {
	addr := utiltx.GenerateAddress()
	baseAcc := &authtypes.BaseAccount{Address: sdk.AccAddress(addr.Bytes()).String()}
	newAcc := s.Network.App.GetAccountKeeper().NewAccount(s.Network.GetContext(), baseAcc)
	s.Network.App.GetAccountKeeper().SetAccount(s.Network.GetContext(), newAcc)

	testCases := []struct {
		name     string
		address  common.Address
		expHash  common.Hash
		malleate func(vm.StateDB)
	}{
		{
			"account not found",
			utiltx.GenerateAddress(),
			common.Hash{},
			func(vm.StateDB) {},
		},
		{
			"account is not a smart contract",
			addr,
			common.BytesToHash(types.EmptyCodeHash),
			func(vm.StateDB) {},
		},
		{
			"existing account",
			s.Keyring.GetAddr(0),
			crypto.Keccak256Hash([]byte("codeHash")),
			func(vmdb vm.StateDB) {
				vmdb.SetCode(s.Keyring.GetAddr(0), []byte("codeHash"))
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			tc.malleate(vmdb)

			hash := vmdb.GetCodeHash(tc.address)
			s.Require().Equal(tc.expHash, hash)
		})
	}
}

func (s *KeeperTestSuite) TestSetCode() {
	addr := utiltx.GenerateAddress()
	baseAcc := &authtypes.BaseAccount{Address: sdk.AccAddress(addr.Bytes()).String()}
	newAcc := s.Network.App.GetAccountKeeper().NewAccount(s.Network.GetContext(), baseAcc)
	s.Network.App.GetAccountKeeper().SetAccount(s.Network.GetContext(), newAcc)

	testCases := []struct {
		name    string
		address common.Address
		code    []byte
		isNoOp  bool
	}{
		{
			"account not found",
			utiltx.GenerateAddress(),
			[]byte("code"),
			false,
		},
		{
			"account not a smart contract",
			addr,
			nil,
			true,
		},
		{
			"existing account",
			s.Keyring.GetAddr(0),
			[]byte("code"),
			false,
		},
		{
			"existing account, code deleted from store",
			s.Keyring.GetAddr(0),
			nil,
			false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			prev := vmdb.GetCode(tc.address)
			vmdb.SetCode(tc.address, tc.code)
			post := vmdb.GetCode(tc.address)

			if tc.isNoOp {
				s.Require().Equal(prev, post)
			} else {
				s.Require().Equal(tc.code, post)
			}

			s.Require().Equal(len(post), vmdb.GetCodeSize(tc.address))
		})
	}
}

func (s *KeeperTestSuite) TestKeeperSetOrDeleteCode() {
	testCases := []struct {
		name     string
		codeHash []byte
		code     []byte
	}{
		{
			"set code",
			[]byte("codeHash"),
			[]byte("this is the code"),
		},
		{
			"delete code",
			[]byte("codeHash"),
			nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			addr := utiltx.GenerateAddress()
			baseAcc := s.Network.App.GetAccountKeeper().NewAccountWithAddress(s.Network.GetContext(), addr.Bytes())
			s.Network.App.GetAccountKeeper().SetAccount(s.Network.GetContext(), baseAcc)
			ctx := s.Network.GetContext()
			if len(tc.code) == 0 {
				s.Network.App.GetEVMKeeper().DeleteCode(ctx, tc.codeHash)
			} else {
				s.Network.App.GetEVMKeeper().SetCode(ctx, tc.codeHash, tc.code)
			}
			key := s.Network.App.GetKey(types.StoreKey)
			store := prefix.NewStore(ctx.KVStore(key), types.KeyPrefixCode)
			code := store.Get(tc.codeHash)

			s.Require().Equal(tc.code, code)
		})
	}
}

func (s *KeeperTestSuite) TestRefund() {
	testCases := []struct {
		name      string
		malleate  func(vm.StateDB)
		expRefund uint64
		expPanic  bool
	}{
		{
			"success - add and subtract refund",
			func(vmdb vm.StateDB) {
				vmdb.AddRefund(11)
			},
			1,
			false,
		},
		{
			"fail - subtract amount > current refund",
			func(vm.StateDB) {
			},
			0,
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			tc.malleate(vmdb)

			if tc.expPanic {
				s.Require().Panics(func() { vmdb.SubRefund(10) })
			} else {
				vmdb.SubRefund(10)
				s.Require().Equal(tc.expRefund, vmdb.GetRefund())
			}
		})
	}
}

func (s *KeeperTestSuite) TestState() {
	testCases := []struct {
		name       string
		key, value common.Hash
	}{
		{
			"set state - delete from store",
			common.BytesToHash([]byte("key")),
			common.Hash{},
		},
		{
			"set state - update value",
			common.BytesToHash([]byte("key")),
			common.BytesToHash([]byte("value")),
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			vmdb.SetState(s.Keyring.GetAddr(0), tc.key, tc.value)
			value := vmdb.GetState(s.Keyring.GetAddr(0), tc.key)
			s.Require().Equal(tc.value, value)
		})
	}
}

func (s *KeeperTestSuite) TestCommittedState() {
	key := common.BytesToHash([]byte("key"))
	value1 := common.BytesToHash([]byte("value1"))
	value2 := common.BytesToHash([]byte("value2"))

	vmdb := s.StateDB()
	vmdb.SetState(s.Keyring.GetAddr(0), key, value1)
	err := vmdb.Commit()
	s.Require().NoError(err)

	vmdb = s.StateDB()
	vmdb.SetState(s.Keyring.GetAddr(0), key, value2)
	tmp := vmdb.GetState(s.Keyring.GetAddr(0), key)
	s.Require().Equal(value2, tmp)
	tmp = vmdb.GetCommittedState(s.Keyring.GetAddr(0), key)
	s.Require().Equal(value1, tmp)
	err = vmdb.Commit()
	s.Require().NoError(err)

	vmdb = s.StateDB()
	tmp = vmdb.GetCommittedState(s.Keyring.GetAddr(0), key)
	s.Require().Equal(value2, tmp)
}

func (s *KeeperTestSuite) TestSetAndGetCodeHash() {
	s.SetupTest()
}

func (s *KeeperTestSuite) TestSuicide() {
	keyring := testkeyring.New(1)
	s.Network = network.NewUnitTestNetwork(
		s.Create,
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)

	firstAddressIndex := keyring.AddKey()
	firstAddress := keyring.GetAddr(firstAddressIndex)
	secondAddressIndex := keyring.AddKey()
	secondAddress := keyring.GetAddr(secondAddressIndex)

	code := []byte("code")
	db := s.Network.GetStateDB()
	// Add code to account
	db.SetCode(firstAddress, code)
	s.Require().Equal(code, db.GetCode(firstAddress))
	// Add state to account
	for i := 0; i < 5; i++ {
		db.SetState(
			firstAddress,
			common.BytesToHash([]byte(fmt.Sprintf("key%d", i))),
			common.BytesToHash([]byte(fmt.Sprintf("value%d", i))),
		)
	}
	s.Require().NoError(db.Commit())
	db = s.Network.GetStateDB()

	// Add code and state to account 2
	db.SetCode(secondAddress, code)
	s.Require().Equal(code, db.GetCode(secondAddress))
	for i := 0; i < 5; i++ {
		db.SetState(
			secondAddress,
			common.BytesToHash([]byte(fmt.Sprintf("key%d", i))),
			common.BytesToHash([]byte(fmt.Sprintf("value%d", i))),
		)
	}

	// Call Suicide
	db.SelfDestruct(firstAddress)

	// Check suicided is marked
	s.Require().True(db.HasSelfDestructed(firstAddress))

	// Commit state
	s.Require().NoError(db.Commit())
	db = s.Network.GetStateDB()

	// Check code is deleted
	s.Require().Nil(db.GetCode(firstAddress))

	// Check state is deleted
	var storage types.Storage
	s.Network.App.GetEVMKeeper().ForEachStorage(s.Network.GetContext(), firstAddress, func(key, value common.Hash) bool {
		storage = append(storage, types.NewState(key, value))
		return true
	})
	s.Require().Equal(0, len(storage))

	// Check account is deleted
	s.Require().Equal(common.Hash{}, db.GetCodeHash(firstAddress))

	// Check code is still present in addr2 and suicided is false
	s.Require().NotNil(db.GetCode(secondAddress))
	s.Require().False(db.HasSelfDestructed(secondAddress))
}

func (s *KeeperTestSuite) TestExist() {
	testCases := []struct {
		name     string
		address  common.Address
		malleate func(vm.StateDB)
		exists   bool
	}{
		{"success, account exists", s.Keyring.GetAddr(0), func(vm.StateDB) {}, true},
		{"success, has suicided", s.Keyring.GetAddr(0), func(vmdb vm.StateDB) {
			vmdb.SelfDestruct(s.Keyring.GetAddr(0))
		}, true},
		{"success, account doesn't exist", utiltx.GenerateAddress(), func(vm.StateDB) {}, false},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			tc.malleate(vmdb)

			s.Require().Equal(tc.exists, vmdb.Exist(tc.address))
		})
	}
}

func (s *KeeperTestSuite) TestEmpty() {
	testCases := []struct {
		name     string
		address  common.Address
		malleate func(vm.StateDB, common.Address)
		empty    bool
	}{
		{"empty, account exists", utiltx.GenerateAddress(), func(vmdb vm.StateDB, addr common.Address) { vmdb.CreateAccount(addr) }, true},
		{
			"not empty, positive balance",
			utiltx.GenerateAddress(),
			func(vmdb vm.StateDB, addr common.Address) {
				vmdb.AddBalance(addr, uint256.NewInt(100), tracing.BalanceChangeUnspecified)
			},
			false,
		},
		{"empty, account doesn't exist", utiltx.GenerateAddress(), func(vm.StateDB, common.Address) {}, true},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			vmdb := s.StateDB()
			tc.malleate(vmdb, tc.address)

			s.Require().Equal(tc.empty, vmdb.Empty(tc.address))
		})
	}
}

func (s *KeeperTestSuite) TestSnapshot() {
	key := common.BytesToHash([]byte("key"))
	value1 := common.BytesToHash([]byte("value1"))
	value2 := common.BytesToHash([]byte("value2"))

	testCases := []struct {
		name     string
		malleate func(vm.StateDB)
	}{
		{"simple revert", func(vmdb vm.StateDB) {
			revision := vmdb.Snapshot()
			s.Require().Zero(revision)

			vmdb.SetState(s.Keyring.GetAddr(0), key, value1)
			s.Require().Equal(value1, vmdb.GetState(s.Keyring.GetAddr(0), key))

			vmdb.RevertToSnapshot(revision)

			// reverted
			s.Require().Equal(common.Hash{}, vmdb.GetState(s.Keyring.GetAddr(0), key))
		}},
		{"nested snapshot/revert", func(vmdb vm.StateDB) {
			revision1 := vmdb.Snapshot()
			s.Require().Zero(revision1)

			vmdb.SetState(s.Keyring.GetAddr(0), key, value1)

			revision2 := vmdb.Snapshot()

			vmdb.SetState(s.Keyring.GetAddr(0), key, value2)
			s.Require().Equal(value2, vmdb.GetState(s.Keyring.GetAddr(0), key))

			vmdb.RevertToSnapshot(revision2)
			s.Require().Equal(value1, vmdb.GetState(s.Keyring.GetAddr(0), key))

			vmdb.RevertToSnapshot(revision1)
			s.Require().Equal(common.Hash{}, vmdb.GetState(s.Keyring.GetAddr(0), key))
		}},
		{"jump revert", func(vmdb vm.StateDB) {
			revision1 := vmdb.Snapshot()
			vmdb.SetState(s.Keyring.GetAddr(0), key, value1)
			vmdb.Snapshot()
			vmdb.SetState(s.Keyring.GetAddr(0), key, value2)
			vmdb.RevertToSnapshot(revision1)
			s.Require().Equal(common.Hash{}, vmdb.GetState(s.Keyring.GetAddr(0), key))
		}},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			vmdb := s.StateDB()
			tc.malleate(vmdb)
		})
	}
}

func (s *KeeperTestSuite) CreateTestTx(msg *types.MsgEthereumTx, priv cryptotypes.PrivKey) authsigning.Tx {
	option, err := codectypes.NewAnyWithValue(&types.ExtensionOptionsEthereumTx{})
	s.Require().NoError(err)

	clientCtx := client.Context{}.WithTxConfig(s.Network.App.GetTxConfig())
	ethSigner := ethtypes.LatestSignerForChainID(s.Network.GetEIP155ChainID())

	txBuilder := clientCtx.TxConfig.NewTxBuilder()
	builder, ok := txBuilder.(authtx.ExtensionOptionsTxBuilder)
	s.Require().True(ok)

	builder.SetExtensionOptions(option)

	err = msg.Sign(ethSigner, utiltx.NewSigner(priv))
	s.Require().NoError(err)

	err = txBuilder.SetMsgs(msg)
	s.Require().NoError(err)

	return txBuilder.GetTx()
}

func (s *KeeperTestSuite) TestAddLog() {
	addr, privKey := utiltx.NewAddrKey()
	toAddr := s.Keyring.GetAddr(0)
	ethTxParams := &types.EvmTxArgs{
		ChainID:  common.Big1,
		Nonce:    0,
		To:       &toAddr,
		Amount:   common.Big1,
		GasLimit: 100000,
		GasPrice: common.Big1,
		Input:    []byte("test"),
	}
	msg := types.NewTx(ethTxParams)
	msg.From = addr.Bytes()

	tx := s.CreateTestTx(msg, privKey)
	msg, _ = tx.GetMsgs()[0].(*types.MsgEthereumTx)
	txHash := msg.AsTransaction().Hash()

	ethTx2Params := &types.EvmTxArgs{
		ChainID:  common.Big1,
		Nonce:    2,
		To:       &toAddr,
		Amount:   common.Big1,
		GasLimit: 100000,
		GasPrice: common.Big1,
		Input:    []byte("test"),
	}
	msg2 := types.NewTx(ethTx2Params)
	msg2.From = addr.Bytes()

	ethTx3Params := &types.EvmTxArgs{
		ChainID:   big.NewInt(testconstants.EighteenDecimalsChainID),
		Nonce:     0,
		To:        &toAddr,
		Amount:    common.Big1,
		GasLimit:  100000,
		GasFeeCap: common.Big1,
		GasTipCap: common.Big1,
		Input:     []byte("test"),
	}
	msg3 := types.NewTx(ethTx3Params)
	msg3.From = addr.Bytes()

	tx3 := s.CreateTestTx(msg3, privKey)
	msg3, _ = tx3.GetMsgs()[0].(*types.MsgEthereumTx)
	txHash3 := msg3.AsTransaction().Hash()

	ethTx4Params := &types.EvmTxArgs{
		ChainID:   common.Big1,
		Nonce:     1,
		To:        &toAddr,
		Amount:    common.Big1,
		GasLimit:  100000,
		GasFeeCap: common.Big1,
		GasTipCap: common.Big1,
		Input:     []byte("test"),
	}
	msg4 := types.NewTx(ethTx4Params)
	msg4.From = addr.Bytes()

	testCases := []struct {
		name        string
		hash        common.Hash
		log, expLog *ethtypes.Log // pre and post populating log fields
		malleate    func(vm.StateDB)
	}{
		{
			"tx hash from message",
			txHash,
			&ethtypes.Log{
				Address: addr,
				Topics:  make([]common.Hash, 0),
			},
			&ethtypes.Log{
				Address: addr,
				Topics:  make([]common.Hash, 0),
			},
			func(vm.StateDB) {},
		},
		{
			"dynamicfee tx hash from message",
			txHash3,
			&ethtypes.Log{
				Address: addr,
				Topics:  make([]common.Hash, 0),
			},
			&ethtypes.Log{
				Address: addr,
				Topics:  make([]common.Hash, 0),
			},
			func(vm.StateDB) {},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			vmdb := statedb.New(s.Network.GetContext(), s.Network.App.GetEVMKeeper(), statedb.NewTxConfig(
				tc.hash,
				0,
			))
			tc.malleate(vmdb)

			vmdb.AddLog(tc.log)
			logs := vmdb.Logs()
			s.Require().Equal(1, len(logs))
			s.Require().Equal(tc.expLog, logs[0])
		})
	}
}

func (s *KeeperTestSuite) TestPrepareAccessList() {
	dest := utiltx.GenerateAddress()
	precompiles := []common.Address{utiltx.GenerateAddress(), utiltx.GenerateAddress()}
	accesses := ethtypes.AccessList{
		{Address: utiltx.GenerateAddress(), StorageKeys: []common.Hash{common.BytesToHash([]byte("key"))}},
		{Address: utiltx.GenerateAddress(), StorageKeys: []common.Hash{common.BytesToHash([]byte("key1"))}},
	}

	rules := ethparams.Rules{
		ChainID:          s.Network.GetEVMChainConfig().ChainID,
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

	vmdb := s.StateDB()
	vmdb.Prepare(rules, s.Keyring.GetAddr(0), common.Address{}, &dest, precompiles, accesses)

	s.Require().True(vmdb.AddressInAccessList(s.Keyring.GetAddr(0)))
	s.Require().True(vmdb.AddressInAccessList(dest))

	for _, precompile := range precompiles {
		s.Require().True(vmdb.AddressInAccessList(precompile))
	}

	for _, access := range accesses {
		for _, key := range access.StorageKeys {
			addrOK, slotOK := vmdb.SlotInAccessList(access.Address, key)
			s.Require().True(addrOK, access.Address.Hex())
			s.Require().True(slotOK, key.Hex())
		}
	}
}

func (s *KeeperTestSuite) TestAddAddressToAccessList() {
	testCases := []struct {
		name string
		addr common.Address
	}{
		{"new address", utiltx.GenerateAddress()},
		{"existing address", s.Keyring.GetAddr(0)},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			vmdb.AddAddressToAccessList(tc.addr)
			addrOk := vmdb.AddressInAccessList(tc.addr)
			s.Require().True(addrOk, tc.addr.Bytes())
		})
	}
}

func (s *KeeperTestSuite) TestAddSlotToAccessList() {
	testCases := []struct {
		name string
		addr common.Address
		slot common.Hash
	}{
		{"new address and slot (1)", utiltx.GenerateAddress(), common.BytesToHash([]byte("hash"))},
		{"new address and slot (2)", utiltx.GenerateAddress(), common.Hash{}},
		{"existing address and slot", s.Keyring.GetAddr(0), common.Hash{}},
		{"existing address, new slot", s.Keyring.GetAddr(0), common.BytesToHash([]byte("hash"))},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			vmdb := s.StateDB()
			vmdb.AddSlotToAccessList(tc.addr, tc.slot)
			addrOk, slotOk := vmdb.SlotInAccessList(tc.addr, tc.slot)
			s.Require().True(addrOk, tc.addr.Bytes())
			s.Require().True(slotOk, tc.slot.Hex())
		})
	}
}

// FIXME skip for now
// func (suite *KeeperTestSuite) _TestForEachStorage() {
// 	var storage types.Storage
//
// 	testCase := []struct {
// 		name      string
// 		malleate  func(vm.StateDB)
// 		callback  func(key, value common.Hash) (stop bool)
// 		expValues []common.Hash
// 	}{
// 		{
// 			"aggregate state",
// 			func(vmdb vm.StateDB) {
// 				for i := 0; i < 5; i++ {
// 					vmdb.SetState(suite.Keyring.GetAddr(0), common.BytesToHash([]byte(fmt.Sprintf("key%d", i))), common.BytesToHash([]byte(fmt.Sprintf("value%d", i))))
// 				}
// 			},
// 			func(key, value common.Hash) bool {
// 				storage = append(storage, types.NewState(key, value))
// 				return true
// 			},
// 			[]common.Hash{
// 				common.BytesToHash([]byte("value0")),
// 				common.BytesToHash([]byte("value1")),
// 				common.BytesToHash([]byte("value2")),
// 				common.BytesToHash([]byte("value3")),
// 				common.BytesToHash([]byte("value4")),
// 			},
// 		},
// 		{
// 			"filter state",
// 			func(vmdb vm.StateDB) {
// 				vmdb.SetState(suite.Keyring.GetAddr(0), common.BytesToHash([]byte("key")), common.BytesToHash([]byte("value")))
// 				vmdb.SetState(suite.Keyring.GetAddr(0), common.BytesToHash([]byte("filterkey")), common.BytesToHash([]byte("filtervalue")))
// 			},
// 			func(key, value common.Hash) bool {
// 				if value == common.BytesToHash([]byte("filtervalue")) {
// 					storage = append(storage, types.NewState(key, value))
// 					return false
// 				}
// 				return true
// 			},
// 			[]common.Hash{
// 				common.BytesToHash([]byte("filtervalue")),
// 			},
// 		},
// 	}
//
// 	for _, tc := range testCase {
// 		suite.Run(tc.name, func() {
// 			suite.SetupTest() // reset
// 			vmdb := suite.StateDB()
// 			tc.malleate(vmdb)
//
// 			err := vmdb.ForEachStorage(suite.Keyring.GetAddr(0), tc.callback)
// 			suite.Require().NoError(err)
// 			suite.Require().Equal(len(tc.expValues), len(storage), fmt.Sprintf("Expected values:\n%v\nStorage Values\n%v", tc.expValues, storage))
//
// 			vals := make([]common.Hash, len(storage))
// 			for i := range storage {
// 				vals[i] = common.HexToHash(storage[i].Value)
// 			}
//
// 			// TODO: not sure why Equals fails
// 			suite.Require().ElementsMatch(tc.expValues, vals)
// 		})
// 		storage = types.Storage{}
// 	}
// }

func (s *KeeperTestSuite) TestSetBalance() {
	amount := common.U2560
	totalBalance := common.U2560
	addr := utiltx.GenerateAddress()

	testCases := []struct {
		name           string
		addr           common.Address
		malleate       func()
		expErr         bool
		expTotalAmount func() *uint256.Int
	}{
		{
			"mint to address",
			addr,
			func() {
				amount = uint256.NewInt(100)
			},
			false,
			func() *uint256.Int {
				return uint256.NewInt(100)
			},
		},
		{
			"mint to address, vesting account",
			addr,
			func() {
				ctx := s.Network.GetContext()
				accAddr := sdk.AccAddress(addr.Bytes())
				err := s.Network.App.GetBankKeeper().SendCoins(ctx, s.Keyring.GetAccAddr(0), accAddr, sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), math.NewInt(100))))
				s.Require().NoError(err)
				// replace with vesting account
				balanceResp, err := s.Handler.GetBalanceFromEVM(accAddr)
				s.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)

				baseAccount := s.Network.App.GetAccountKeeper().GetAccount(ctx, accAddr).(*authtypes.BaseAccount)
				baseDenom := s.Network.GetBaseDenom()
				currTime := s.Network.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, balance)), s.Network.GetContext().BlockTime().Unix(), currTime+100)
				s.Require().NoError(err)
				s.Network.App.GetAccountKeeper().SetAccount(ctx, acc)

				spendable := s.Network.App.GetBankKeeper().SpendableCoin(ctx, accAddr, baseDenom).Amount
				s.Require().Equal(spendable.String(), "0")

				evmBalanceRes, err := s.Handler.GetBalanceFromEVM(accAddr)
				s.Require().NoError(err)
				evmBalance := evmBalanceRes.Balance
				s.Require().Equal(evmBalance, "0")

				tb, overflow := uint256.FromBig(s.Network.App.GetBankKeeper().GetBalance(ctx, accAddr, baseDenom).Amount.BigInt())
				s.Require().False(overflow)
				s.Require().Equal(tb.ToBig(), balance.BigInt())
				totalBalance = tb
				amount = uint256.NewInt(100)
			},
			false,
			func() *uint256.Int {
				return common.U2560.Add(totalBalance, amount)
			},
		},
		{
			"burn from address",
			addr,
			func() {
				amount = uint256.NewInt(60)
			},
			false,
			func() *uint256.Int {
				return uint256.NewInt(60)
			},
		},
		{
			"burn from address, don't burn vesting amount",
			addr,
			func() {
				ctx := s.Network.GetContext()
				accAddr := sdk.AccAddress(addr.Bytes())
				err := s.Network.App.GetBankKeeper().SendCoins(ctx, s.Keyring.GetAccAddr(0), accAddr, sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), math.NewInt(100))))
				s.Require().NoError(err)
				// replace with vesting account
				balanceResp, err := s.Handler.GetBalanceFromEVM(accAddr)
				s.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)

				baseAccount := s.Network.App.GetAccountKeeper().GetAccount(ctx, accAddr).(*authtypes.BaseAccount)
				baseDenom := s.Network.GetBaseDenom()
				currTime := s.Network.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, balance)), s.Network.GetContext().BlockTime().Unix(), currTime+100)
				s.Require().NoError(err)
				s.Network.App.GetAccountKeeper().SetAccount(ctx, acc)

				spendable := s.Network.App.GetBankKeeper().SpendableCoin(ctx, accAddr, baseDenom).Amount
				s.Require().Equal(spendable.String(), "0")

				evmBalanceRes, err := s.Handler.GetBalanceFromEVM(accAddr)
				s.Require().NoError(err)
				evmBalance := evmBalanceRes.Balance
				s.Require().Equal(evmBalance, "0")

				tb, overflow := uint256.FromBig(s.Network.App.GetBankKeeper().GetBalance(ctx, accAddr, baseDenom).Amount.BigInt())
				s.Require().False(overflow)
				s.Require().Equal(tb.ToBig(), balance.BigInt())
				totalBalance = tb
				amount = uint256.NewInt(0)
			},
			false,
			func() *uint256.Int {
				return uint256.NewInt(100)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.malleate()
			err := s.Network.App.GetEVMKeeper().SetBalance(s.Network.GetContext(), tc.addr, amount)
			if tc.expErr {
				s.Require().Error(err)
			} else {
				balance := s.Network.App.GetEVMKeeper().GetBalance(s.Network.GetContext(), tc.addr)
				s.Require().NoError(err)
				expTotalAmount := tc.expTotalAmount()
				s.Require().Equal(expTotalAmount, balance)
				spendable := s.Network.App.GetEVMKeeper().SpendableCoin(s.Network.GetContext(), tc.addr)
				s.Require().Equal(amount, spendable)
			}
		})
	}
}

func (s *KeeperTestSuite) TestDeleteAccount() {
	var (
		ctx          sdk.Context
		contractAddr common.Address
	)
	supply := big.NewInt(100)

	testCases := []struct {
		name        string
		malleate    func() common.Address
		expPass     bool
		errContains string
	}{
		{
			name:        "remove address",
			malleate:    func() common.Address { return s.Keyring.GetAddr(0) },
			errContains: "only smart contracts can be self-destructed",
		},
		{
			name: "removing vested account should remove all balance (including locked)",
			malleate: func() common.Address {
				contractAccAddr := sdk.AccAddress(contractAddr.Bytes())
				err := s.Network.App.GetBankKeeper().SendCoins(ctx, s.Keyring.GetAccAddr(0), contractAccAddr, sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), math.NewInt(100))))
				s.Require().NoError(err)
				// replace with vesting account
				balanceResp, err := s.Handler.GetBalanceFromEVM(contractAccAddr)
				s.Require().NoError(err)

				balance, ok := math.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)

				ctx := s.Network.GetContext()
				baseAccount := s.Network.App.GetAccountKeeper().GetAccount(ctx, contractAccAddr).(*authtypes.BaseAccount)
				baseDenom := s.Network.GetBaseDenom()
				currTime := s.Network.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, balance)), s.Network.GetContext().BlockTime().Unix(), currTime+100)
				s.Require().NoError(err)
				s.Network.App.GetAccountKeeper().SetAccount(ctx, acc)

				spendable := s.Network.App.GetBankKeeper().SpendableCoin(ctx, contractAccAddr, baseDenom).Amount
				s.Require().Equal(spendable.String(), "0")

				evmBalanceRes, err := s.Handler.GetBalanceFromEVM(contractAccAddr)
				s.Require().NoError(err)
				evmBalance := evmBalanceRes.Balance
				s.Require().Equal(evmBalance, "0")

				totalBalance := s.Network.App.GetBankKeeper().GetBalance(ctx, contractAccAddr, baseDenom)
				s.Require().Equal(totalBalance.Amount, balance)
				return contractAddr
			},
			expPass: true,
		},
		{
			name:     "remove unexistent address - returns nil error",
			malleate: func() common.Address { return common.HexToAddress("unexistent_address") },
			expPass:  true,
		},
		{
			name:     "remove deployed contract",
			malleate: func() common.Address { return contractAddr },
			expPass:  true,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			ctx = s.Network.GetContext()
			contractAddr = s.DeployTestContract(s.T(), ctx, s.Keyring.GetAddr(0), supply)

			addr := tc.malleate()

			err := s.Network.App.GetEVMKeeper().DeleteAccount(ctx, addr)
			if tc.expPass {
				s.Require().NoError(err, "expected deleting account to succeed")

				acc := s.Network.App.GetEVMKeeper().GetAccount(ctx, addr)
				s.Require().Nil(acc, "expected no account to be found after deleting")

				balance := s.Network.App.GetEVMKeeper().GetBalance(ctx, addr)
				s.Require().Equal(new(uint256.Int), balance, "expected balance to be zero after deleting account")
			} else {
				s.Require().ErrorContains(err, tc.errContains, "expected error to contain message")

				acc := s.Network.App.GetEVMKeeper().GetAccount(ctx, addr)
				s.Require().NotNil(acc, "expected account to still be found after failing to delete")
			}
		})
	}
}
