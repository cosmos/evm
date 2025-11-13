package cmd

import (
	"crypto/ecdsa"
	"math/big"
	"math/rand"
	"os"
	"testing"
	"time"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/tools/speedtest"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/evmd"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
)

var (
	r = rand.New(rand.NewSource(time.Now().UnixNano()))
)

type Application struct {
	*evmd.EVMD
}

func (a Application) Codec() codec.Codec {
	return a.AppCodec()
}

func NewSpeedTestCommand() *cobra.Command {
	logger := log.NewNopLogger()
	dir := os.TempDir()

	db, err := dbm.NewDB("app", dbm.GoLevelDBBackend, dir)
	if err != nil {
		panic(err)
	}

	baseAppOpts := make([]func(*baseapp.BaseApp), 0)
	evmd := evmd.NewExampleApp(logger, db, nil, true, simtestutil.NewAppOptionsWithFlagHome(dir), baseAppOpts...)
	app := Application{evmd}
	ac := accountCreator{}
	cmd := speedtest.SpeedTestCmd(ac.createAccount, nil, app, "9001")
	return cmd
}

type accountCreator struct {
	accounts []accountInfo
}

type accountInfo struct {
	privKey  cryptotypes.PrivKey
	ecdsaKey *ecdsa.PrivateKey
	address  sdk.AccAddress
	accNum   uint64
	seqNum   uint64
}

func (ac *accountCreator) createAccount() (*types.BaseAccount, sdk.Coins) {
	privKey, err := ethsecp256k1.GenerateKey()
	if err != nil {
		panic(err)
	}
	ecsdaKey, err := crypto.ToECDSA(privKey.Key)
	addr := sdk.AccAddress(privKey.PubKey().Address())
	accountNum := uint64(len(ac.accounts))
	baseAcc := types.NewBaseAccount(addr, privKey.PubKey(), accountNum, 0)

	ac.accounts = append(ac.accounts, accountInfo{
		privKey:  privKey,
		ecdsaKey: ecsdaKey,
		address:  addr,
		accNum:   accountNum,
		seqNum:   0,
	})
	fundingAmount := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1_000_000_000_000_000_000))
	return baseAcc, fundingAmount
}

func (ac *accountCreator) generateTx() []byte {
	// Select sender and recipient (ensure they're different)
	senderIdx := r.Intn(len(ac.accounts))
	recipientIdx := (senderIdx + 1 + r.Intn(len(ac.accounts)-1)) % len(ac.accounts)

	sender := ac.accounts[senderIdx]
	recipient := ac.accounts[recipientIdx]

	// Create MsgSend
	ethTx := createMsgNativeERC20Transfer(
		b,
		config.SendAmount,
		ERC20PrecompileAddr,
		common.Address(sender.address.Bytes()),
		common.Address(recipient.address.Bytes()),
		sender.seqNum,
		func(address common.Address, transaction *types.Transaction) (*types.Transaction, error) {
			signer := types.NewLondonSigner(big.NewInt(262144))
			return types.SignTx(transaction, signer, sender.ecdsaKey)
		})
	msg := &evmtypes.MsgEthereumTx{}
	msg.FromEthereumTx(ethTx)
	msg.From = sender.address.Bytes()
	builder := app.TxConfig().NewTxBuilder()
	tx, err := msg.BuildTx(builder, sdk.DefaultBondDenom)
	require.NoError(b, err)

	// Encode transaction
	txBytes, err := txEncoder(tx)
	require.NoError(b, err)

	blockTxs[txIdx] = txBytes

	// Update sequence number for next transaction
	accounts[senderIdx].seqNum++
}

func createMsgNativeERC20Transfer(sendAmt int64, precompileAddress common.Address, fromAddr common.Address, recipientAddr common.Address, nonce uint64, signerFn bind.SignerFn) *types.Transaction {
	// random amount. weth calls amounts wad for some reason. we continue that trend here.
	wad := big.NewInt(int64(rand.Intn(int(sendAmt))))

	// we use the weth transactor even though were interacting with the native precompile since they share the same interface,
	// and the call data constructed here will be the same.
	wethInstance, err := NewWethTransactor(precompileAddress, nil)
	txOpts := &bind.TransactOpts{
		From:      fromAddr,
		Signer:    signerFn,
		Nonce:     big.NewInt(int64(nonce)), //nolint:gosec // G115: overflow unlikely in practice
		GasTipCap: big.NewInt(25_000),
		GasFeeCap: big.NewInt(25_000),
		Context:   context.Background(),
		GasLimit:  250_000,
		NoSend:    true,
	}
	tx, err := wethInstance.Transfer(txOpts, recipientAddr, wad)
	require.NoError(t, err)
	return tx
}
