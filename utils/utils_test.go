package utils

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	cryptocodec "github.com/cosmos/evm/crypto/codec"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/crypto/hd"
	"github.com/cosmos/evm/types"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdkhd "github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	"github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestIsSupportedKeys(t *testing.T) {
	testCases := []struct {
		name        string
		pk          cryptotypes.PubKey
		isSupported bool
	}{
		{
			"nil key",
			nil,
			false,
		},
		{
			"ethsecp256k1 key",
			&ethsecp256k1.PubKey{},
			true,
		},
		{
			"ed25519 key",
			&ed25519.PubKey{},
			true,
		},
		{
			"multisig key - no pubkeys",
			&multisig.LegacyAminoPubKey{},
			false,
		},
		{
			"multisig key - valid pubkeys",
			multisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{&ed25519.PubKey{}, &ed25519.PubKey{}, &ed25519.PubKey{}}),
			true,
		},
		{
			"multisig key - nested multisig",
			multisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{&ed25519.PubKey{}, &ed25519.PubKey{}, &multisig.LegacyAminoPubKey{}}),
			false,
		},
		{
			"multisig key - invalid pubkey",
			multisig.NewLegacyAminoPubKey(2, []cryptotypes.PubKey{&ed25519.PubKey{}, &ed25519.PubKey{}, &secp256k1.PubKey{}}),
			false,
		},
		{
			"cosmos secp256k1",
			&secp256k1.PubKey{},
			false,
		},
	}

	for _, tc := range testCases {
		require.Equal(t, tc.isSupported, IsSupportedKey(tc.pk), tc.name)
	}
}

func TestGetAccAddressFromBech32(t *testing.T) {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("cosmos", "cosmospub")

	testCases := []struct {
		name       string
		address    string
		expAddress string
		expError   bool
	}{
		{
			"blank bech32 address",
			" ",
			"",
			true,
		},
		{
			"invalid bech32 address",
			"evmos",
			"",
			true,
		},
		{
			"invalid address bytes",
			"cosmos1123",
			"",
			true,
		},
		{
			"evmos address",
			"cosmos1qql8ag4cluz6r4dz28p3w00dnc9w8ueulg2gmc",
			"cosmos1qql8ag4cluz6r4dz28p3w00dnc9w8ueulg2gmc",
			false,
		},
		{
			"cosmos address",
			"cosmos1qql8ag4cluz6r4dz28p3w00dnc9w8ueulg2gmc",
			"cosmos1qql8ag4cluz6r4dz28p3w00dnc9w8ueulg2gmc",
			false,
		},
		{
			"osmosis address",
			"osmo1qql8ag4cluz6r4dz28p3w00dnc9w8ueuhnecd2",
			"cosmos1qql8ag4cluz6r4dz28p3w00dnc9w8ueulg2gmc",
			false,
		},
	}

	for _, tc := range testCases {
		addr, err := GetAccAddressFromBech32(tc.address)
		if tc.expError {
			require.Error(t, err, tc.name)
		} else {
			require.NoError(t, err, tc.name)
			require.Equal(t, tc.expAddress, addr.String(), tc.name)
		}
	}
}

func TestEvmosCoinDenom(t *testing.T) {
	testCases := []struct {
		name     string
		denom    string
		expError bool
	}{
		{
			"valid denom - native coin",
			"aatom",
			false,
		},
		{
			"valid denom - ibc coin",
			"ibc/7B2A4F6E798182988D77B6B884919AF617A73503FDAC27C916CD7A69A69013CF",
			false,
		},
		{
			"valid denom - ethereum address (ERC-20 contract)",
			"erc20:0x52908400098527886e0f7030069857D2E4169EE7",
			false,
		},
		{
			"invalid denom - only one character",
			"a",
			true,
		},
		{
			"invalid denom - too large (> 127 chars)",
			"ibc/7B2A4F6E798182988D77B6B884919AF617A73503FDAC27C916CD7A69A69013CF7B2A4F6E798182988D77B6B884919AF617A73503FDAC27C916CD7A69A69013CF",
			true,
		},
		{
			"invalid denom - starts with 0 but not followed by 'x'",
			"0a52908400098527886E0F7030069857D2E4169EE7",
			true,
		},
		{
			"invalid denom - hex address but 19 bytes long",
			"0x52908400098527886E0F7030069857D2E4169E",
			true,
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("Case %s", tc.name), func(t *testing.T) {
			err := sdk.ValidateDenom(tc.denom)
			if tc.expError {
				require.Error(t, err, tc.name)
			} else {
				require.NoError(t, err, tc.name)
			}
		})
	}
}

func TestAccAddressFromBech32(t *testing.T) {
	testCases := []struct {
		address      string
		bech32Prefix string
		expErr       bool
		errContains  string
	}{
		{
			"",
			"",
			true,
			"empty address string is not allowed",
		},
		{
			"cosmos1xv9tklw7d82sezh9haa573wufgy59vmwe6xxe5",
			"stride",
			true,
			"invalid Bech32 prefix; expected stride, got cosmos",
		},
		{
			"cosmos1xv9tklw7d82sezh9haa573wufgy59vmw5",
			"cosmos",
			true,
			"decoding bech32 failed: invalid checksum",
		},
		{
			"stride1mdna37zrprxl7kn0rj4e58ndp084fzzwcxhrh2",
			"stride",
			false,
			"",
		},
	}

	for _, tc := range testCases {
		tc := tc //nolint:copyloopvar // Needed to work correctly with concurrent tests

		t.Run(tc.address, func(t *testing.T) {
			t.Parallel()

			_, err := CreateAccAddressFromBech32(tc.address, tc.bech32Prefix)
			if tc.expErr {
				require.Error(t, err, "expected error while creating AccAddress")
				require.Contains(t, err.Error(), tc.errContains, "expected different error")
			} else {
				require.NoError(t, err, "expected no error while creating AccAddress")
			}
		})
	}
}

func TestAddressConversion(t *testing.T) {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("cosmos", "cosmospub")

	hex := "0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E"
	bech32 := "cosmos10jmp6sgh4cc6zt3e8gw05wavvejgr5pwsjskvv"

	require.Equal(t, bech32, Bech32StringFromHexAddress(hex))
	gotAddr, err := HexAddressFromBech32String(bech32)
	require.NoError(t, err)
	require.Equal(t, hex, gotAddr.Hex())
}

func TestGetIBCDenomAddress(t *testing.T) {
	testCases := []struct {
		name        string
		denom       string
		expErr      bool
		expectedRes string
	}{
		{
			"",
			"test",
			true,
			"does not have 'ibc/' prefix",
		},
		{
			"",
			"ibc/",
			true,
			"is not a valid IBC voucher hash",
		},
		{
			"",
			"ibc/qqqqaaaaaa",
			true,
			"invalid denomination for cross-chain transfer",
		},
		{
			"",
			"ibc/DF63978F803A2E27CA5CC9B7631654CCF0BBC788B3B7F0A10200508E37C70992",
			false,
			"0x631654CCF0BBC788b3b7F0a10200508e37c70992",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			address, err := GetIBCDenomAddress(tc.denom)
			if tc.expErr {
				require.Error(t, err, "expected error while get ibc denom address")
				require.Contains(t, err.Error(), tc.expectedRes, "expected different error")
			} else {
				require.NoError(t, err, "expected no error while get ibc denom address")
				require.Equal(t, address.Hex(), tc.expectedRes)
			}
		})
	}
}

// TestAccountEquivalence tests and demonstrates the equivalence of accounts
func TestAccountEquivalence(t *testing.T) {
	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	uid := "inMemory"
	mnemonic := "aunt imitate maximum student guard unhappy guard rotate marine panel negative merit record priority zoo voice mixture boost describe fruit often occur expect teach"

	// create a keyring with support for ethsecp and secp (default supported)
	kb, err := keyring.New("keybasename", keyring.BackendMemory, t.TempDir(), nil, cdc, hd.EthSecp256k1Option())
	require.NoError(t, err)

	// get the proper signing algorithms
	keyringAlgos, _ := kb.SupportedAlgorithms()
	algoEvm, err := keyring.NewSigningAlgoFromString(string(hd.EthSecp256k1Type), keyringAlgos)
	require.NoError(t, err)
	legacyAlgo, err := keyring.NewSigningAlgoFromString(string(sdkhd.Secp256k1Type), keyringAlgos)
	require.NoError(t, err)

	// legacy account using "regular" cosmos secp
	// and coin type 118
	legacyCosmosKey, err := kb.NewAccount(uid, mnemonic, keyring.DefaultBIP39Passphrase, sdk.FullFundraiserPath, legacyAlgo)
	require.NoError(t, err)

	// account using ethsecp
	// and coin type 118
	cosmsosKey, err := kb.NewAccount(uid, mnemonic, keyring.DefaultBIP39Passphrase, sdk.FullFundraiserPath, algoEvm)
	require.NoError(t, err)

	// account using ethsecp
	// and coin type 60
	evmKey, err := kb.NewAccount(uid, mnemonic, keyring.DefaultBIP39Passphrase, types.BIP44HDPath, algoEvm)
	require.NoError(t, err)

	// verify that none of these three keys are equal
	require.NotEqual(t, legacyCosmosKey, cosmsosKey)
	require.NotEqual(t, legacyCosmosKey.String(), cosmsosKey.String())
	require.NotEqual(t, legacyCosmosKey.PubKey.String(), cosmsosKey.PubKey.String())

	require.NotEqual(t, legacyCosmosKey, evmKey)
	require.NotEqual(t, legacyCosmosKey.String(), evmKey.String())
	require.NotEqual(t, legacyCosmosKey.PubKey.String(), evmKey.PubKey.String())

	require.NotEqual(t, cosmsosKey, evmKey)
	require.NotEqual(t, cosmsosKey.String(), evmKey.String())
	require.NotEqual(t, cosmsosKey.PubKey.String(), evmKey.PubKey.String())

	// calls:
	// sha := sha256.Sum256(pubKey.Key)
	// hasherRIPEMD160 := ripemd160.New()
	// hasherRIPEMD160.Write(sha[:])
	//
	// one way sha256 -> ripeMD160
	// this is the actual bech32 algorithm
	legacyAddress, err := legacyCosmosKey.GetAddress() //
	require.NoError(t, err)

	legacyPubKey, err := legacyCosmosKey.GetPubKey()
	require.NoError(t, err)

	// create an ethsecp key from the same exact pubkey bytes
	// this will mean that calling `Address()` will use the Keccack hash of the pubkey
	ethSecpPubkey := ethsecp256k1.PubKey{Key: legacyPubKey.Bytes()}

	// calls:
	// 	pubBytes := FromECDSAPub(&p)
	//	return common.BytesToAddress(Keccak256(pubBytes[1:])[12:])
	//
	// one way keccak hash
	// because the key implementation points to it to call the EVM methods
	ethSecpAddress := ethSecpPubkey.Address().Bytes()
	require.False(t, bytes.Equal(legacyAddress.Bytes(), ethSecpAddress))
	trueHexLegacy, err := HexAddressFromBech32String(sdk.AccAddress(ethSecpAddress).String())
	require.NoError(t, err)

	// deriving a legacy bech32 from the legacy address
	legacyBech32Address := legacyAddress.String()

	// this just converts the ripeMD(sha(pubkey)) from bech32 formatting style to hex
	gotHexLegacy, err := HexAddressFromBech32String(legacyBech32Address)
	require.NoError(t, err)
	require.NotEqual(t, trueHexLegacy.Hex(), gotHexLegacy.Hex())

	fmt.Println("\nLegacy Ethereum address:\t\t", gotHexLegacy.Hex()) //
	fmt.Println("True Legacy Ethereum address:\t", trueHexLegacy.Hex())
	fmt.Println("Legacy Bech32 address:\t\t\t", legacyBech32Address)
	fmt.Println()

	// calls:
	// 	pubBytes := FromECDSAPub(&p)
	//	return common.BytesToAddress(Keccak256(pubBytes[1:])[12:])
	//
	// one way keccak hash
	// because the key implementation points to it to call the EVM methods
	cosmosAddress, err := cosmsosKey.GetAddress() //
	require.NoError(t, err)
	require.NotEqual(t, legacyAddress, cosmosAddress)
	require.False(t, legacyAddress.Equals(cosmosAddress))

	// calls:
	// 	pubBytes := FromECDSAPub(&p)
	//	return common.BytesToAddress(Keccak256(pubBytes[1:])[12:])
	//
	// one way keccak hash
	evmAddress, err := evmKey.GetAddress()
	require.NoError(t, err)
	require.NotEqual(t, cosmosAddress, evmAddress)
	require.False(t, cosmosAddress.Equals(evmAddress))

	// we have verified that two privkeys generated from the same mnemonic (on different HD paths) are different
	// now, let's derive the 0x and bech32 addresses of our EVM key
	t.Run("verify 0x and cosmos formatted address string is the same for an EVM key", func(t *testing.T) {
		addr := evmAddress
		require.NoError(t, err)
		_, err = kb.KeyByAddress(addr)
		require.NoError(t, err)

		bech32 := addr.String()
		// Decode from hex to bytes

		// Convert to Ethereum address
		address := common.BytesToAddress(addr)

		fmt.Println("\nEthereum address:", address.Hex())
		fmt.Println("Bech32 address:", bech32)

		require.Equal(t, bech32, Bech32StringFromHexAddress(address.Hex()))
		gotAddr, err := HexAddressFromBech32String(bech32)
		require.NoError(t, err)
		require.Equal(t, address.Hex(), gotAddr.Hex())
	})
}
