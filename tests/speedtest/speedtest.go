package main

import (
	"context"
	"crypto/ecdsa"
	"encoding/json"
	"math/big"
	"math/rand"
	"os"
	"time"

	dbm "github.com/cosmos/cosmos-db"
	"github.com/cosmos/cosmos-sdk/baseapp"
	"github.com/cosmos/cosmos-sdk/codec"
	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
	"github.com/cosmos/cosmos-sdk/tools/speedtest"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/evmd"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	types2 "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"

	"cosmossdk.io/log"
)

var (
	r                   = rand.New(rand.NewSource(time.Now().UnixNano()))
	ERC20PrecompileAddr = common.HexToAddress("0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE")
)

func NewSpeedTestCommand() *cobra.Command {
	cmd := &cobra.Command{
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := os.MkdirTemp("", "speedtest-*")
			if err != nil {
				panic(err)
			}
			defer os.RemoveAll(dir)
			logger := log.NewNopLogger()
			db, err := dbm.NewDB("app", dbm.PebbleDBBackend, dir)
			if err != nil {
				panic(err)
			}

			chainID := "9001"
			baseAppOpts := make([]func(*baseapp.BaseApp), 0)
			baseAppOpts = append(baseAppOpts, baseapp.SetChainID(chainID))
			evmApp := evmd.NewExampleApp(logger, db, nil, true, simtestutil.NewAppOptionsWithFlagHome(dir), baseAppOpts...)
			gen := generator{
				app:      evmApp,
				accounts: make([]accountInfo, 0),
			}
			speedTestCmd := speedtest.NewCmd(
				gen.createAccount,
				gen.generateTx,
				evmApp,
				evmApp.AppCodec(),
				evmApp.DefaultGenesis(),
				chainID,
				DisableFeeMarket,
				SetERC20Precompile(ERC20PrecompileAddr.String(), sdk.DefaultBondDenom),
				BankMetadataSetter(sdk.DefaultBondDenom, 18),
			)
			speedTestCmd.SetArgs(args)
			return speedTestCmd.Execute()
		},
	}
	cmd.DisableFlagParsing = true

	return cmd
}

type generator struct {
	app      *evmd.EVMD
	accounts []accountInfo
}

type accountInfo struct {
	ecdsaKey *ecdsa.PrivateKey
	address  sdk.AccAddress
	seqNum   uint64
}

func (gen *generator) createAccount() (*types.BaseAccount, sdk.Coins) {
	privKey, err := ethsecp256k1.GenerateKey()
	if err != nil {
		panic(err)
	}
	ecsdaKey, err := crypto.ToECDSA(privKey.Key)
	if err != nil {
		panic(err)
	}
	addr := sdk.AccAddress(privKey.PubKey().Address())
	accountNum := uint64(len(gen.accounts))
	baseAcc := types.NewBaseAccount(addr, privKey.PubKey(), accountNum+1, 0)

	gen.accounts = append(gen.accounts, accountInfo{
		ecdsaKey: ecsdaKey,
		address:  addr,
		seqNum:   0,
	})
	fundingAmount := sdk.NewCoins(sdk.NewInt64Coin(sdk.DefaultBondDenom, 1_000_000_000_000_000_000))
	return baseAcc, fundingAmount
}

func (gen *generator) generateTx() []byte {
	// Select sender and recipient (ensure they're different)
	senderIdx := r.Intn(len(gen.accounts))
	recipientIdx := (senderIdx + 1 + r.Intn(len(gen.accounts)-1)) % len(gen.accounts)

	sender := gen.accounts[senderIdx]
	recipient := gen.accounts[recipientIdx]

	// Create MsgSend
	ethTx := createMsgNativeERC20Transfer(
		10_000,
		ERC20PrecompileAddr,
		common.Address(sender.address.Bytes()),
		common.Address(recipient.address.Bytes()),
		sender.seqNum,
		func(address common.Address, transaction *types2.Transaction) (*types2.Transaction, error) {
			signer := types2.NewLondonSigner(big.NewInt(int64(evmtypes.DefaultEVMChainID)))
			return types2.SignTx(transaction, signer, sender.ecdsaKey)
		})
	msg := &evmtypes.MsgEthereumTx{}
	msg.FromEthereumTx(ethTx)
	msg.From = sender.address.Bytes()
	builder := gen.app.TxConfig().NewTxBuilder()
	tx, err := msg.BuildTx(builder, sdk.DefaultBondDenom)
	if err != nil {
		panic(err)
	}
	txEncoder := gen.app.TxConfig().TxEncoder()

	// Encode transaction
	txBytes, err := txEncoder(tx)
	if err != nil {
		panic(err)
	}

	gen.accounts[senderIdx].seqNum++

	return txBytes
}

func createMsgNativeERC20Transfer(sendAmt int64, precompileAddress common.Address, fromAddr common.Address, recipientAddr common.Address, nonce uint64, signerFn bind.SignerFn) *types2.Transaction {
	// random amount. weth calls amounts wad for some reason. we continue that trend here.
	wad := big.NewInt(int64(rand.Intn(int(sendAmt))))

	// we use the weth transactor even though were interacting with the native precompile since they share the same interface,
	// and the call data constructed here will be the same.
	wethInstance, err := NewWethTransactor(precompileAddress, nil)
	if err != nil {
		panic(err)
	}
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
	if err != nil {
		panic(err)
	}
	return tx
}

var (
	BankMetadataSetter = func(denom string, exp uint32) func(cdc codec.Codec, genesisState map[string]json.RawMessage) {
		return func(cdc codec.Codec, genesisState map[string]json.RawMessage) {
			bankGenesis := genesisState[banktypes.ModuleName]
			var bankGen banktypes.GenesisState
			if err := cdc.UnmarshalJSON(bankGenesis, &bankGen); err != nil {
				panic(err)
			}
			bankGen.DenomMetadata = append(bankGen.DenomMetadata, banktypes.Metadata{
				Description: "some stuff",
				DenomUnits: []*banktypes.DenomUnit{
					{
						Denom:    denom,
						Exponent: exp,
						Aliases:  nil,
					},
				},
				Base:    denom,
				Display: denom,
				Name:    denom,
				Symbol:  denom,
			})
			bz := cdc.MustMarshalJSON(&bankGen)
			genesisState[banktypes.ModuleName] = bz
		}
	}

	SetERC20Precompile = func(addr, denom string) func(cdc codec.Codec, genesisState map[string]json.RawMessage) {
		return func(cdc codec.Codec, genesisState map[string]json.RawMessage) {
			erc20Genesis := genesisState[erc20types.ModuleName]
			var erc20Gen erc20types.GenesisState
			if err := erc20Genesis.UnmarshalJSON(erc20Genesis); err != nil {
				panic(err)
			}
			erc20Gen.NativePrecompiles = append(erc20Gen.NativePrecompiles, addr)
			erc20Gen.TokenPairs = append(erc20Gen.TokenPairs, erc20types.TokenPair{
				Erc20Address:  addr,
				Denom:         denom,
				Enabled:       true,
				ContractOwner: 1,
			})
			erc20Bz := cdc.MustMarshalJSON(&erc20Gen)

			genesisState[erc20types.ModuleName] = erc20Bz

		}
	}

	DisableFeeMarket = func(cdc codec.Codec, genesisState map[string]json.RawMessage) {
		feeMarketGenesis := genesisState[feemarkettypes.ModuleName]
		var feeMarketGen feemarkettypes.GenesisState
		if err := feeMarketGenesis.UnmarshalJSON(feeMarketGenesis); err != nil {
			panic(err)
		}
		feeMarketGen.Params.NoBaseFee = true
		bz := cdc.MustMarshalJSON(&feeMarketGen)
		genesisState[feemarkettypes.ModuleName] = bz
	}
)
