package keeper_test

import (
	"bytes"
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
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/contracts"
	testconstants "github.com/cosmos/evm/testutil/constants"
	testfactory "github.com/cosmos/evm/testutil/integration/os/factory"
	testhandler "github.com/cosmos/evm/testutil/integration/os/grpc"
	testkeyring "github.com/cosmos/evm/testutil/integration/os/keyring"
	"github.com/cosmos/evm/testutil/integration/os/network"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/store/prefix"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func (suite *KeeperTestSuite) TestCreateAccount() {
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
				suite.Require().NotZero(vmdb.GetBalance(addr).Uint64())
			},
			func(vmdb vm.StateDB, addr common.Address) {
				suite.Require().Equal(vmdb.GetBalance(addr).Uint64(), uint64(100))
			},
		},
		{
			"create account",
			utiltx.GenerateAddress(),
			func(vmdb vm.StateDB, addr common.Address) {
				suite.Require().False(vmdb.Exist(addr))
			},
			func(vmdb vm.StateDB, addr common.Address) {
				suite.Require().True(vmdb.Exist(addr))
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			tc.malleate(vmdb, tc.addr)
			vmdb.CreateAccount(tc.addr)
			tc.callback(vmdb, tc.addr)
		})
	}
}

func (suite *KeeperTestSuite) TestAddBalance() {
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
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			prev := vmdb.GetBalance(suite.keyring.GetAddr(0))
			vmdb.AddBalance(suite.keyring.GetAddr(0), tc.amount, tracing.BalanceChangeUnspecified)
			post := vmdb.GetBalance(suite.keyring.GetAddr(0))

			if tc.isNoOp {
				suite.Require().Equal(prev, post)
			} else {
				suite.Require().Equal(new(uint256.Int).Add(prev, tc.amount), post)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestSubBalance() {
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
				vmdb.AddBalance(suite.keyring.GetAddr(0), uint256.NewInt(100), tracing.BalanceChangeUnspecified)
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
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			tc.malleate(vmdb)

			prev := vmdb.GetBalance(suite.keyring.GetAddr(0))
			vmdb.SubBalance(suite.keyring.GetAddr(0), tc.amount, tracing.BalanceChangeUnspecified)
			post := vmdb.GetBalance(suite.keyring.GetAddr(0))

			if tc.isNoOp {
				suite.Require().Equal(prev, post)
			} else {
				suite.Require().Equal(new(uint256.Int).Sub(prev, tc.amount), post)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestGetNonce() {
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
			suite.keyring.GetAddr(0),
			1,
			func(vmdb vm.StateDB) {
				vmdb.SetNonce(suite.keyring.GetAddr(0), 1, tracing.NonceChangeUnspecified)
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			tc.malleate(vmdb)

			nonce := vmdb.GetNonce(tc.address)
			suite.Require().Equal(tc.expectedNonce, nonce)
		})
	}
}

func (suite *KeeperTestSuite) TestSetNonce() {
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
			suite.keyring.GetAddr(0),
			99,
			func() {},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			vmdb.SetNonce(tc.address, tc.nonce, tracing.NonceChangeUnspecified)
			nonce := vmdb.GetNonce(tc.address)
			suite.Require().Equal(tc.nonce, nonce)
		})
	}
}

func (suite *KeeperTestSuite) TestGetCodeHash() {
	addr := utiltx.GenerateAddress()
	baseAcc := &authtypes.BaseAccount{Address: sdk.AccAddress(addr.Bytes()).String()}
	newAcc := suite.network.App.AccountKeeper.NewAccount(suite.network.GetContext(), baseAcc)
	suite.network.App.AccountKeeper.SetAccount(suite.network.GetContext(), newAcc)

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
			suite.keyring.GetAddr(0),
			crypto.Keccak256Hash([]byte("codeHash")),
			func(vmdb vm.StateDB) {
				vmdb.SetCode(suite.keyring.GetAddr(0), []byte("codeHash"))
			},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			tc.malleate(vmdb)

			hash := vmdb.GetCodeHash(tc.address)
			suite.Require().Equal(tc.expHash, hash)
		})
	}
}

func (suite *KeeperTestSuite) TestSetCode() {
	addr := utiltx.GenerateAddress()
	baseAcc := &authtypes.BaseAccount{Address: sdk.AccAddress(addr.Bytes()).String()}
	newAcc := suite.network.App.AccountKeeper.NewAccount(suite.network.GetContext(), baseAcc)
	suite.network.App.AccountKeeper.SetAccount(suite.network.GetContext(), newAcc)

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
			suite.keyring.GetAddr(0),
			[]byte("code"),
			false,
		},
		{
			"existing account, code deleted from store",
			suite.keyring.GetAddr(0),
			nil,
			false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			prev := vmdb.GetCode(tc.address)
			vmdb.SetCode(tc.address, tc.code)
			post := vmdb.GetCode(tc.address)

			if tc.isNoOp {
				suite.Require().Equal(prev, post)
			} else {
				suite.Require().Equal(tc.code, post)
			}

			suite.Require().Equal(len(post), vmdb.GetCodeSize(tc.address))
		})
	}
}

func (suite *KeeperTestSuite) TestKeeperSetOrDeleteCode() {
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
		suite.Run(tc.name, func() {
			suite.SetupTest()
			addr := utiltx.GenerateAddress()
			baseAcc := suite.network.App.AccountKeeper.NewAccountWithAddress(suite.network.GetContext(), addr.Bytes())
			suite.network.App.AccountKeeper.SetAccount(suite.network.GetContext(), baseAcc)
			ctx := suite.network.GetContext()
			if len(tc.code) == 0 {
				suite.network.App.EVMKeeper.DeleteCode(ctx, tc.codeHash)
			} else {
				suite.network.App.EVMKeeper.SetCode(ctx, tc.codeHash, tc.code)
			}
			key := suite.network.App.GetKey(types.StoreKey)
			store := prefix.NewStore(ctx.KVStore(key), types.KeyPrefixCode)
			code := store.Get(tc.codeHash)

			suite.Require().Equal(tc.code, code)
		})
	}
}

func TestIterateContracts(t *testing.T) {
	keyring := testkeyring.New(1)
	network := network.NewUnitTestNetwork(
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)
	handler := testhandler.NewIntegrationHandler(network)
	factory := testfactory.New(network, handler)

	contractAddr, err := factory.DeployContract(
		keyring.GetPrivKey(0),
		types.EvmTxArgs{},
		testfactory.ContractDeploymentData{
			Contract:        contracts.ERC20MinterBurnerDecimalsContract,
			ConstructorArgs: []interface{}{"TestToken", "TTK", uint8(18)},
		},
	)
	require.NoError(t, err, "failed to deploy contract")
	require.NoError(t, network.NextBlock(), "failed to advance block")

	contractAddr2, err := factory.DeployContract(
		keyring.GetPrivKey(0),
		types.EvmTxArgs{},
		testfactory.ContractDeploymentData{
			Contract:        contracts.ERC20MinterBurnerDecimalsContract,
			ConstructorArgs: []interface{}{"AnotherToken", "ATK", uint8(18)},
		},
	)
	require.NoError(t, err, "failed to deploy contract")
	require.NoError(t, network.NextBlock(), "failed to advance block")

	var (
		foundAddrs  []common.Address
		foundHashes []common.Hash
	)

	network.App.EVMKeeper.IterateContracts(network.GetContext(), func(addr common.Address, codeHash common.Hash) bool {
		// NOTE: we only care about the 2 contracts deployed above, not the ERC20 native precompile for the aatom denomination
		if bytes.Equal(addr.Bytes(), common.HexToAddress(testconstants.WEVMOSContractMainnet).Bytes()) {
			return false
		}

		foundAddrs = append(foundAddrs, addr)
		foundHashes = append(foundHashes, codeHash)
		return false
	})

	require.Len(t, foundAddrs, 2, "expected 2 contracts to be found when iterating")
	require.Contains(t, foundAddrs, contractAddr, "expected contract 1 to be found when iterating")
	require.Contains(t, foundAddrs, contractAddr2, "expected contract 2 to be found when iterating")
	require.Equal(t, foundHashes[0], foundHashes[1], "expected both contracts to have the same code hash")
	require.NotEqual(t, types.EmptyCodeHash, foundHashes[0], "expected store code hash not to be the keccak256 of empty code")
}

func (suite *KeeperTestSuite) TestRefund() {
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
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			tc.malleate(vmdb)

			if tc.expPanic {
				suite.Require().Panics(func() { vmdb.SubRefund(10) })
			} else {
				vmdb.SubRefund(10)
				suite.Require().Equal(tc.expRefund, vmdb.GetRefund())
			}
		})
	}
}

func (suite *KeeperTestSuite) TestState() {
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
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			vmdb.SetState(suite.keyring.GetAddr(0), tc.key, tc.value)
			value := vmdb.GetState(suite.keyring.GetAddr(0), tc.key)
			suite.Require().Equal(tc.value, value)
		})
	}
}

func (suite *KeeperTestSuite) TestCommittedState() {
	key := common.BytesToHash([]byte("key"))
	value1 := common.BytesToHash([]byte("value1"))
	value2 := common.BytesToHash([]byte("value2"))

	vmdb := suite.StateDB()
	vmdb.SetState(suite.keyring.GetAddr(0), key, value1)
	err := vmdb.Commit()
	suite.Require().NoError(err)

	vmdb = suite.StateDB()
	vmdb.SetState(suite.keyring.GetAddr(0), key, value2)
	tmp := vmdb.GetState(suite.keyring.GetAddr(0), key)
	suite.Require().Equal(value2, tmp)
	tmp = vmdb.GetCommittedState(suite.keyring.GetAddr(0), key)
	suite.Require().Equal(value1, tmp)
	err = vmdb.Commit()
	suite.Require().NoError(err)

	vmdb = suite.StateDB()
	tmp = vmdb.GetCommittedState(suite.keyring.GetAddr(0), key)
	suite.Require().Equal(value2, tmp)
}

func (suite *KeeperTestSuite) TestSetAndGetCodeHash() {
	suite.SetupTest()
}

func (suite *KeeperTestSuite) TestSuicide() {
	keyring := testkeyring.New(1)
	unitNetwork := network.NewUnitTestNetwork(
		network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
	)

	firstAddressIndex := keyring.AddKey()
	firstAddress := keyring.GetAddr(firstAddressIndex)
	secondAddressIndex := keyring.AddKey()
	secondAddress := keyring.GetAddr(secondAddressIndex)

	code := []byte("code")
	db := unitNetwork.GetStateDB()
	// Add code to account
	db.SetCode(firstAddress, code)
	suite.Require().Equal(code, db.GetCode(firstAddress))
	// Add state to account
	for i := 0; i < 5; i++ {
		db.SetState(
			firstAddress,
			common.BytesToHash([]byte(fmt.Sprintf("key%d", i))),
			common.BytesToHash([]byte(fmt.Sprintf("value%d", i))),
		)
	}
	suite.Require().NoError(db.Commit())
	db = unitNetwork.GetStateDB()

	// Add code and state to account 2
	db.SetCode(secondAddress, code)
	suite.Require().Equal(code, db.GetCode(secondAddress))
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
	suite.Require().True(db.HasSelfDestructed(firstAddress))

	// Commit state
	suite.Require().NoError(db.Commit())
	db = unitNetwork.GetStateDB()

	// Check code is deleted
	suite.Require().Nil(db.GetCode(firstAddress))

	// Check state is deleted
	var storage types.Storage
	unitNetwork.App.EVMKeeper.ForEachStorage(unitNetwork.GetContext(), firstAddress, func(key, value common.Hash) bool {
		storage = append(storage, types.NewState(key, value))
		return true
	})
	suite.Require().Equal(0, len(storage))

	// Check account is deleted
	suite.Require().Equal(common.Hash{}, db.GetCodeHash(firstAddress))

	// Check code is still present in addr2 and suicided is false
	suite.Require().NotNil(db.GetCode(secondAddress))
	suite.Require().False(db.HasSelfDestructed(secondAddress))
}

func (suite *KeeperTestSuite) TestExist() {
	testCases := []struct {
		name     string
		address  common.Address
		malleate func(vm.StateDB)
		exists   bool
	}{
		{"success, account exists", suite.keyring.GetAddr(0), func(vm.StateDB) {}, true},
		{"success, has suicided", suite.keyring.GetAddr(0), func(vmdb vm.StateDB) {
			vmdb.SelfDestruct(suite.keyring.GetAddr(0))
		}, true},
		{"success, account doesn't exist", utiltx.GenerateAddress(), func(vm.StateDB) {}, false},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			tc.malleate(vmdb)

			suite.Require().Equal(tc.exists, vmdb.Exist(tc.address))
		})
	}
}

func (suite *KeeperTestSuite) TestEmpty() {
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
		suite.Run(tc.name, func() {
			suite.SetupTest()
			vmdb := suite.StateDB()
			tc.malleate(vmdb, tc.address)

			suite.Require().Equal(tc.empty, vmdb.Empty(tc.address))
		})
	}
}

func (suite *KeeperTestSuite) TestSnapshot() {
	key := common.BytesToHash([]byte("key"))
	value1 := common.BytesToHash([]byte("value1"))
	value2 := common.BytesToHash([]byte("value2"))

	testCases := []struct {
		name     string
		malleate func(vm.StateDB)
	}{
		{"simple revert", func(vmdb vm.StateDB) {
			revision := vmdb.Snapshot()
			suite.Require().Zero(revision)

			vmdb.SetState(suite.keyring.GetAddr(0), key, value1)
			suite.Require().Equal(value1, vmdb.GetState(suite.keyring.GetAddr(0), key))

			vmdb.RevertToSnapshot(revision)

			// reverted
			suite.Require().Equal(common.Hash{}, vmdb.GetState(suite.keyring.GetAddr(0), key))
		}},
		{"nested snapshot/revert", func(vmdb vm.StateDB) {
			revision1 := vmdb.Snapshot()
			suite.Require().Zero(revision1)

			vmdb.SetState(suite.keyring.GetAddr(0), key, value1)

			revision2 := vmdb.Snapshot()

			vmdb.SetState(suite.keyring.GetAddr(0), key, value2)
			suite.Require().Equal(value2, vmdb.GetState(suite.keyring.GetAddr(0), key))

			vmdb.RevertToSnapshot(revision2)
			suite.Require().Equal(value1, vmdb.GetState(suite.keyring.GetAddr(0), key))

			vmdb.RevertToSnapshot(revision1)
			suite.Require().Equal(common.Hash{}, vmdb.GetState(suite.keyring.GetAddr(0), key))
		}},
		{"jump revert", func(vmdb vm.StateDB) {
			revision1 := vmdb.Snapshot()
			vmdb.SetState(suite.keyring.GetAddr(0), key, value1)
			vmdb.Snapshot()
			vmdb.SetState(suite.keyring.GetAddr(0), key, value2)
			vmdb.RevertToSnapshot(revision1)
			suite.Require().Equal(common.Hash{}, vmdb.GetState(suite.keyring.GetAddr(0), key))
		}},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			vmdb := suite.StateDB()
			tc.malleate(vmdb)
		})
	}
}

func (suite *KeeperTestSuite) CreateTestTx(msg *types.MsgEthereumTx, priv cryptotypes.PrivKey) authsigning.Tx {
	option, err := codectypes.NewAnyWithValue(&types.ExtensionOptionsEthereumTx{})
	suite.Require().NoError(err)

	clientCtx := client.Context{}.WithTxConfig(suite.network.App.GetTxConfig())
	ethSigner := ethtypes.LatestSignerForChainID(types.GetEthChainConfig().ChainID)

	txBuilder := clientCtx.TxConfig.NewTxBuilder()
	builder, ok := txBuilder.(authtx.ExtensionOptionsTxBuilder)
	suite.Require().True(ok)

	builder.SetExtensionOptions(option)

	err = msg.Sign(ethSigner, utiltx.NewSigner(priv))
	suite.Require().NoError(err)

	err = txBuilder.SetMsgs(msg)
	suite.Require().NoError(err)

	return txBuilder.GetTx()
}

func (suite *KeeperTestSuite) TestAddLog() {
	addr, privKey := utiltx.NewAddrKey()
	toAddr := suite.keyring.GetAddr(0)
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
	msg.From = addr.Hex()

	tx := suite.CreateTestTx(msg, privKey)
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
	msg2.From = addr.Hex()

	ethTx3Params := &types.EvmTxArgs{
		ChainID:   big.NewInt(testconstants.ExampleEIP155ChainID),
		Nonce:     0,
		To:        &toAddr,
		Amount:    common.Big1,
		GasLimit:  100000,
		GasFeeCap: common.Big1,
		GasTipCap: common.Big1,
		Input:     []byte("test"),
	}
	msg3 := types.NewTx(ethTx3Params)
	msg3.From = addr.Hex()

	tx3 := suite.CreateTestTx(msg3, privKey)
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
	msg4.From = addr.Hex()

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
				TxHash:  txHash,
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
				TxHash:  txHash3,
				Topics:  make([]common.Hash, 0),
			},
			func(vm.StateDB) {},
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			vmdb := statedb.New(suite.network.GetContext(), suite.network.App.EVMKeeper, statedb.NewTxConfig(
				common.BytesToHash(suite.network.GetContext().HeaderHash()),
				tc.hash,
				0, 0,
			))
			tc.malleate(vmdb)

			vmdb.AddLog(tc.log)
			logs := vmdb.Logs()
			suite.Require().Equal(1, len(logs))
			suite.Require().Equal(tc.expLog, logs[0])
		})
	}
}

func (suite *KeeperTestSuite) TestPrepareAccessList() {
	dest := utiltx.GenerateAddress()
	precompiles := []common.Address{utiltx.GenerateAddress(), utiltx.GenerateAddress()}
	accesses := ethtypes.AccessList{
		{Address: utiltx.GenerateAddress(), StorageKeys: []common.Hash{common.BytesToHash([]byte("key"))}},
		{Address: utiltx.GenerateAddress(), StorageKeys: []common.Hash{common.BytesToHash([]byte("key1"))}},
	}

	rules := ethparams.Rules{
		ChainID:          suite.network.GetEVMChainConfig().ChainID,
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
	}

	vmdb := suite.StateDB()
	vmdb.Prepare(rules, suite.keyring.GetAddr(0), common.Address{}, &dest, precompiles, accesses)

	suite.Require().True(vmdb.AddressInAccessList(suite.keyring.GetAddr(0)))
	suite.Require().True(vmdb.AddressInAccessList(dest))

	for _, precompile := range precompiles {
		suite.Require().True(vmdb.AddressInAccessList(precompile))
	}

	for _, access := range accesses {
		for _, key := range access.StorageKeys {
			addrOK, slotOK := vmdb.SlotInAccessList(access.Address, key)
			suite.Require().True(addrOK, access.Address.Hex())
			suite.Require().True(slotOK, key.Hex())
		}
	}
}

func (suite *KeeperTestSuite) TestAddAddressToAccessList() {
	testCases := []struct {
		name string
		addr common.Address
	}{
		{"new address", utiltx.GenerateAddress()},
		{"existing address", suite.keyring.GetAddr(0)},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			vmdb.AddAddressToAccessList(tc.addr)
			addrOk := vmdb.AddressInAccessList(tc.addr)
			suite.Require().True(addrOk, tc.addr.Hex())
		})
	}
}

func (suite *KeeperTestSuite) TestAddSlotToAccessList() {
	testCases := []struct {
		name string
		addr common.Address
		slot common.Hash
	}{
		{"new address and slot (1)", utiltx.GenerateAddress(), common.BytesToHash([]byte("hash"))},
		{"new address and slot (2)", utiltx.GenerateAddress(), common.Hash{}},
		{"existing address and slot", suite.keyring.GetAddr(0), common.Hash{}},
		{"existing address, new slot", suite.keyring.GetAddr(0), common.BytesToHash([]byte("hash"))},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			vmdb := suite.StateDB()
			vmdb.AddSlotToAccessList(tc.addr, tc.slot)
			addrOk, slotOk := vmdb.SlotInAccessList(tc.addr, tc.slot)
			suite.Require().True(addrOk, tc.addr.Hex())
			suite.Require().True(slotOk, tc.slot.Hex())
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
// 					vmdb.SetState(suite.keyring.GetAddr(0), common.BytesToHash([]byte(fmt.Sprintf("key%d", i))), common.BytesToHash([]byte(fmt.Sprintf("value%d", i))))
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
// 				vmdb.SetState(suite.keyring.GetAddr(0), common.BytesToHash([]byte("key")), common.BytesToHash([]byte("value")))
// 				vmdb.SetState(suite.keyring.GetAddr(0), common.BytesToHash([]byte("filterkey")), common.BytesToHash([]byte("filtervalue")))
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
// 			err := vmdb.ForEachStorage(suite.keyring.GetAddr(0), tc.callback)
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

func (suite *KeeperTestSuite) TestSetBalance() {
	amount := common.U2560
	addr := utiltx.GenerateAddress()

	testCases := []struct {
		name     string
		addr     common.Address
		malleate func()
		expErr   bool
	}{
		{
			"mint to address",
			addr,
			func() {
				amount = uint256.NewInt(100)
			},
			false,
		},
		{
			"burn from address",
			addr,
			func() {
				amount = uint256.NewInt(60)
			},
			false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			suite.SetupTest()
			tc.malleate()
			err := suite.network.App.EVMKeeper.SetBalance(suite.network.GetContext(), tc.addr, amount)
			if tc.expErr {
				suite.Require().Error(err)
			} else {
				balance := suite.network.App.EVMKeeper.GetBalance(suite.network.GetContext(), tc.addr)
				suite.Require().NoError(err)
				suite.Require().Equal(amount, balance)
			}
		})
	}
}

func (suite *KeeperTestSuite) TestDeleteAccount() {
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
			malleate:    func() common.Address { return suite.keyring.GetAddr(0) },
			errContains: "only smart contracts can be self-destructed",
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
		suite.Run(tc.name, func() {
			suite.SetupTest()
			ctx = suite.network.GetContext()
			contractAddr = suite.DeployTestContract(suite.T(), ctx, suite.keyring.GetAddr(0), supply)

			addr := tc.malleate()

			err := suite.network.App.EVMKeeper.DeleteAccount(ctx, addr)
			if tc.expPass {
				suite.Require().NoError(err, "expected deleting account to succeed")

				acc := suite.network.App.EVMKeeper.GetAccount(ctx, addr)
				suite.Require().Nil(acc, "expected no account to be found after deleting")

				balance := suite.network.App.EVMKeeper.GetBalance(ctx, addr)
				suite.Require().Equal(new(uint256.Int), balance, "expected balance to be zero after deleting account")
			} else {
				suite.Require().ErrorContains(err, tc.errContains, "expected error to contain message")

				acc := suite.network.App.EVMKeeper.GetAccount(ctx, addr)
				suite.Require().NotNil(acc, "expected account to still be found after failing to delete")
			}
		})
	}
}
