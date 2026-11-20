package keyring

import (
	"errors"
	"fmt"

	"github.com/cosmos/evm/crypto/hd"
	ledger "github.com/cosmos/ledger-cosmos-go"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cosmosLedger "github.com/cosmos/cosmos-sdk/crypto/ledger"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ cosmosLedger.SECP256K1 = &ledgerRouter{}

// cosmosLedgerDerivation connects to the Ledger Cosmos app.
var cosmosLedgerDerivation = func() (cosmosLedger.SECP256K1, error) {
	device, err := ledger.FindLedgerCosmosUserApp()
	if err != nil {
		return nil, err
	}
	return device, nil
}

// ledgerApp is a Ledger app and the SDK Ledger options its keys need.
type ledgerApp struct {
	name         string
	connect      func() (cosmosLedger.SECP256K1, error)
	createPubkey func([]byte) types.PubKey
	derSignature bool
}

// ledgerApps are the supported apps by the coin type of their keys.
var ledgerApps = map[uint32]ledgerApp{
	hd.Bip44CoinType: {
		name:         AppName,
		connect:      func() (cosmosLedger.SECP256K1, error) { return LedgerDerivation() },
		createPubkey: CreatePubkey,
		derSignature: !SkipDERConversion,
	},
	sdk.CoinType: {
		name:         cosmosLedger.AppName,
		connect:      func() (cosmosLedger.SECP256K1, error) { return cosmosLedgerDerivation() },
		createPubkey: func(key []byte) types.PubKey { return &secp256k1.PubKey{Key: key} },
		derSignature: true,
	},
}

// ValidateLedgerCoinType reports whether a supported Ledger app holds keys of
// the coin type: the Ethereum app for 60 and the Cosmos app for 118.
func ValidateLedgerCoinType(coinType uint32) error {
	if _, ok := ledgerApps[coinType]; ok {
		return nil
	}
	return fmt.Errorf(
		"unsupported coin type %d for Ledger. Supported coin types: %d (Ethereum app), %d (Cosmos app)",
		coinType, hd.Bip44CoinType, sdk.CoinType,
	)
}

// DiscoverLedger returns a Ledger device that sends each call to the app of
// the path's coin type: the Ethereum app for 60, the Cosmos app for 118.
func DiscoverLedger() (cosmosLedger.SECP256K1, error) {
	return &ledgerRouter{}, nil
}

// ledgerRouter connects to the app of the requested path. The SDK Ledger
// options are process wide and read right after each device call, so the
// router sets them for that app first.
type ledgerRouter struct {
	coinType uint32
	device   cosmosLedger.SECP256K1
}

func (r *ledgerRouter) connect(hdPath []uint32) (cosmosLedger.SECP256K1, error) {
	if len(hdPath) < 2 {
		return nil, errors.New("invalid Ledger derivation path")
	}
	coinType := hdPath[1] &^ 0x80000000 // hardened or not
	if err := ValidateLedgerCoinType(coinType); err != nil {
		return nil, err
	}
	app := ledgerApps[coinType]
	cosmosLedger.SetCreatePubkey(app.createPubkey)
	cosmosLedger.SetAppName(app.name)
	cosmosLedger.SetDERConversion(app.derSignature)
	if r.device != nil && r.coinType == coinType {
		return r.device, nil
	}
	if err := r.Close(); err != nil {
		return nil, err
	}
	device, err := app.connect()
	if err != nil {
		return nil, err
	}
	r.coinType, r.device = coinType, device
	return device, nil
}

// Close closes the connected app.
func (r *ledgerRouter) Close() error {
	if r.device == nil {
		return nil
	}
	err := r.device.Close()
	r.device = nil
	return err
}

// GetPublicKeySECP256K1 returns the uncompressed public key of the path.
func (r *ledgerRouter) GetPublicKeySECP256K1(hdPath []uint32) ([]byte, error) {
	device, err := r.connect(hdPath)
	if err != nil {
		return nil, err
	}
	return device.GetPublicKeySECP256K1(hdPath)
}

// GetAddressPubKeySECP256K1 returns the public key and address of the path.
func (r *ledgerRouter) GetAddressPubKeySECP256K1(hdPath []uint32, hrp string) ([]byte, string, error) {
	device, err := r.connect(hdPath)
	if err != nil {
		return nil, "", err
	}
	return device.GetAddressPubKeySECP256K1(hdPath, hrp)
}

// SignSECP256K1 signs the message with the key of the path.
func (r *ledgerRouter) SignSECP256K1(hdPath []uint32, msg []byte, p2 byte) ([]byte, error) {
	device, err := r.connect(hdPath)
	if err != nil {
		return nil, err
	}
	return device.SignSECP256K1(hdPath, msg, p2)
}
