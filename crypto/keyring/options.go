package keyring

import (
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/crypto/hd"
	"github.com/cosmos/evm/wallets/ledger"

	cosmoshd "github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/types"
)

// AppName is the name of the Ledger Ethereum app.
const AppName = "Ethereum"

var (
	// SupportedAlgorithms defines the list of signing algorithms used on Cosmos EVM:
	//  - eth_secp256k1 (Ethereum)
	//  - secp256k1 (CometBFT)
	SupportedAlgorithms = keyring.SigningAlgoList{hd.EthSecp256k1, cosmoshd.Secp256k1}
	// SupportedAlgorithmsLedger defines the list of signing algorithms used by Cosmos EVM for the Ledger device:
	//  - eth_secp256k1 (Ethereum app, coin type 60)
	//  - secp256k1 (Cosmos app, coin type 118)
	// The Ledger derivation function is responsible for all signing and address generation.
	SupportedAlgorithmsLedger = keyring.SigningAlgoList{hd.EthSecp256k1, cosmoshd.Secp256k1}
	// LedgerDerivation defines the Cosmos EVM Ledger Go derivation (Ethereum app with EIP-712 signing)
	LedgerDerivation = ledger.EvmLedgerDerivation()
	// CreatePubkey uses the ethsecp256k1 pubkey with Ethereum address generation and keccak hashing
	CreatePubkey = func(key []byte) types.PubKey { return &ethsecp256k1.PubKey{Key: key} }
	// SkipDERConversion represents whether the signed Ledger output should skip conversion from DER to BER.
	// This is set to true for signing performed by the Ledger Ethereum app.
	SkipDERConversion = true
)

// Option returns the Cosmos EVM keyring options: eth_secp256k1 and secp256k1
// keys, with Ledger keys signed by the app of their coin type.
func Option() keyring.Option {
	return func(options *keyring.Options) {
		options.SupportedAlgos = SupportedAlgorithms
		options.SupportedAlgosLedger = SupportedAlgorithmsLedger
		options.LedgerDerivation = DiscoverLedger
		options.LedgerCreateKey = CreatePubkey
		options.LedgerAppName = AppName
		options.LedgerSigSkipDERConv = SkipDERConversion
	}
}
