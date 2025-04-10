package ibctesting

import (
	"testing"

	sdkmath "cosmossdk.io/math"
	cmtproto "github.com/cometbft/cometbft/proto/tendermint/types"
	cmttypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/evmd"
)

// NewTestChainWithValSet initializes a new TestChain instance with the given validator set
// and signer array. It also initializes 10 Sender accounts with a balance of 10000000000000000000 coins of
// bond denom to use for tests.
//
// The first block height is committed to state in order to allow for client creations on
// counterparty chains. The TestChain will return with a block height starting at 2.
//
// Time management is handled by the Coordinator in order to ensure synchrony between chains.
// Each update of any chain increments the block header time for all chains by 5 seconds.
//
// NOTE: to use a custom sender privkey and account for testing purposes, replace and modify this
// constructor function.
//
// CONTRACT: Validator array must be provided in the order expected by Tendermint.
// i.e. sorted first by power and then lexicographically by address.
func NewTestChainWithValSet(tb testing.TB, coord *ibctesting.Coordinator, chainID string, valSet *cmttypes.ValidatorSet, signers map[string]cmttypes.PrivValidator) *ibctesting.TestChain {
	tb.Helper()
	genAccs := []authtypes.GenesisAccount{}
	genBals := []banktypes.Balance{}
	senderAccs := []ibctesting.SenderAccount{}

	// generate genesis accounts
	for i := 0; i < ibctesting.MaxAccounts; i++ {
		senderPrivKey, err := ethsecp256k1.GenerateKey()
		require.NoError(tb, err)
		acc := authtypes.NewBaseAccount(senderPrivKey.PubKey().Address().Bytes(), senderPrivKey.PubKey(), uint64(i), 0)
		amount, ok := sdkmath.NewIntFromString(ibctesting.DefaultGenesisAccBalance)
		require.True(tb, ok)

		// add sender account
		balance := banktypes.Balance{
			Address: acc.GetAddress().String(),
			Coins: sdk.NewCoins(
				sdk.NewCoin(sdk.DefaultBondDenom, amount),
				sdk.NewCoin(evmd.ExampleChainDenom, amount),
			),
		}

		genAccs = append(genAccs, acc)
		genBals = append(genBals, balance)

		senderAcc := ibctesting.SenderAccount{
			SenderAccount: acc,
			SenderPrivKey: senderPrivKey,
		}

		senderAccs = append(senderAccs, senderAcc)
	}

	app := ibctesting.SetupWithGenesisValSet(tb, valSet, genAccs, chainID, sdk.DefaultPowerReduction, genBals...)
	// create current header and call begin block
	header := cmtproto.Header{
		ChainID: chainID,
		Height:  1,
		Time:    coord.CurrentTime.UTC(),
	}

	txConfig := app.GetTxConfig()

	// create an account to send transactions from
	chain := &ibctesting.TestChain{
		TB:                tb,
		Coordinator:       coord,
		ChainID:           chainID,
		App:               app,
		ProposedHeader:    header,
		TxConfig:          txConfig,
		Codec:             app.AppCodec(),
		Vals:              valSet,
		NextVals:          valSet,
		Signers:           signers,
		TrustedValidators: make(map[uint64]*cmttypes.ValidatorSet, 0),
		SenderPrivKey:     senderAccs[0].SenderPrivKey,
		SenderAccount:     senderAccs[0].SenderAccount,
		SenderAccounts:    senderAccs,
	}

	// commit genesis block
	chain.NextBlock()

	return chain
}

// NewTestChain initializes a new test chain with a default of 4 validators
// Use this function if the tests do not need custom control over the validator set
func NewTestChain(t *testing.T, coord *ibctesting.Coordinator, chainID string) *ibctesting.TestChain {
	t.Helper()
	// generate validator private/public key
	var (
		validatorsPerChain = 4
		validators         []*cmttypes.Validator
		signersByAddress   = make(map[string]cmttypes.PrivValidator, validatorsPerChain)
	)

	for i := 0; i < validatorsPerChain; i++ {
		_, privVal := cmttypes.RandValidator(false, 100)
		pubKey, err := privVal.GetPubKey()
		require.NoError(t, err)
		validators = append(validators, cmttypes.NewValidator(pubKey, 1))
		signersByAddress[pubKey.Address().String()] = privVal
	}
	// construct validator set;
	// Note that the validators are sorted by voting power
	// or, if equal, by address lexical order
	valSet := cmttypes.NewValidatorSet(validators)

	return NewTestChainWithValSet(t, coord, chainID, valSet, signersByAddress)
}
