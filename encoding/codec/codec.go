package codec

import (
	"sync"

	cryptocodec "github.com/cosmos/evm/crypto/codec"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/ethereum/eip712"
	"github.com/cosmos/gogoproto/proto"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/std"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var (
	emTypesRegistered bool
	emTypesMutex      sync.Mutex
)

// RegisterLegacyAminoCodec registers Interfaces from types, crypto, and SDK std.
func RegisterLegacyAminoCodec(cdc *codec.LegacyAmino) {
	sdk.RegisterLegacyAminoCodec(cdc)
	cryptocodec.RegisterCrypto(cdc)
	codec.RegisterEvidences(cdc)
}

// RegisterInterfaces registers Interfaces from types, crypto, and SDK std.
func RegisterInterfaces(interfaceRegistry codectypes.InterfaceRegistry) {
	std.RegisterInterfaces(interfaceRegistry)
	cryptocodec.RegisterInterfaces(interfaceRegistry)
	eip712.RegisterInterfaces(interfaceRegistry)

	// ETHERMINT COMPATIBILITY: Register Ethermint type URLs (only once)
	emTypesMutex.Lock()
	defer emTypesMutex.Unlock()

	if !emTypesRegistered {
		// Try multiple registration methods to ensure compatibility
		proto.RegisterType((*ethsecp256k1.PubKey)(nil), "ethermint.crypto.v1.ethsecp256k1.PubKey")
		proto.RegisterType((*ethsecp256k1.PrivKey)(nil), "ethermint.crypto.v1.ethsecp256k1.PrivKey")

		// Also register in the interface registry with Ethermint type URLs
		interfaceRegistry.RegisterImplementations((*cryptotypes.PubKey)(nil), &ethsecp256k1.PubKey{})
		interfaceRegistry.RegisterImplementations((*cryptotypes.PrivKey)(nil), &ethsecp256k1.PrivKey{})

		emTypesRegistered = true
	}
}
