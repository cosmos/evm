package utils_test

import (
	"bytes"
	"fmt"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"

	cryptocodec "github.com/cosmos/evm/crypto/codec"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/crypto/hd"
	"github.com/cosmos/evm/rpc/types"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/utils"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

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

const (
	hex    = "0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E"
	bech32 = "cosmos10jmp6sgh4cc6zt3e8gw05wavvejgr5pwsjskvv"
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
		require.Equal(t, tc.isSupported, utils.IsSupportedKey(tc.pk), tc.name)
	}
}

func TestIsBech32Address(t *testing.T) {
	config := sdk.GetConfig()
	config.SetBech32PrefixForAccount("cosmos", "cosmospub")

	testCases := []struct {
		name    string
		address string
		expResp bool
	}{
		{
			"blank bech32 address",
			" ",
			false,
		},
		{
			"invalid bech32 address",
			"evmos",
			false,
		},
		{
			"invalid address bytes",
			"cosmos1123",
			false,
		},
		{
			"evmos address",
			"evmos1ltzy54ms24v590zz37r2q9hrrdcc8eslndsqwv",
			true,
		},
		{
			"cosmos address",
			"cosmos1qql8ag4cluz6r4dz28p3w00dnc9w8ueulg2gmc",
			true,
		},
		{
			"osmosis address",
			"osmo1qql8ag4cluz6r4dz28p3w00dnc9w8ueuhnecd2",
			true,
		},
	}

	for _, tc := range testCases {
		isValid := utils.IsBech32Address(tc.address)
		require.Equal(t, tc.expResp, isValid, tc.name)
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
			"evmos1ltzy54ms24v590zz37r2q9hrrdcc8eslndsqwv",
			"cosmos1ltzy54ms24v590zz37r2q9hrrdcc8esl3vpw5y",
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
		addr, err := utils.GetAccAddressFromBech32(tc.address)
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

			_, err := utils.CreateAccAddressFromBech32(tc.address, tc.bech32Prefix)
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

	require.Equal(t, bech32, utils.Bech32StringFromHexAddress(hex))
	gotAddr, err := utils.HexAddressFromBech32String(bech32)
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
			address, err := utils.GetIBCDenomAddress(tc.denom)
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

// TestBytes32ToString tests the Bytes32ToString helper function
func TestBytes32ToString(t *testing.T) {
	testCases := []struct {
		name     string
		input    [32]byte
		expected string
	}{
		{
			name:     "Full string - no null bytes",
			input:    [32]byte{'M', 'a', 'k', 'e', 'r', ' ', 'T', 'o', 'k', 'e', 'n'},
			expected: "Maker Token",
		},
		{
			name:     "Short string - with null bytes",
			input:    [32]byte{'M', 'K', 'R'},
			expected: "MKR",
		},
		{
			name:     "Empty string",
			input:    [32]byte{},
			expected: "",
		},
		{
			name:     "Single character",
			input:    [32]byte{'A'},
			expected: "A",
		},
		{
			name:     "String with special characters",
			input:    [32]byte{'T', 'e', 's', 't', '-', '1', '2', '3'},
			expected: "Test-123",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := utils.Bytes32ToString(tc.input)
			require.Equal(t, tc.expected, result)
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
	evmKey, err := kb.NewAccount(uid, mnemonic, keyring.DefaultBIP39Passphrase, hd.BIP44HDPath, algoEvm)
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
	trueHexLegacy, err := utils.HexAddressFromBech32String(sdk.AccAddress(ethSecpAddress).String())
	require.NoError(t, err)

	// deriving a legacy bech32 from the legacy address
	legacyBech32Address := legacyAddress.String()

	// this just converts the ripeMD(sha(pubkey)) from bech32 formatting style to hex
	gotHexLegacy, err := utils.HexAddressFromBech32String(legacyBech32Address)
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

		require.Equal(t, bech32, utils.Bech32StringFromHexAddress(address.Hex()))
		gotAddr, err := utils.HexAddressFromBech32String(bech32)
		require.NoError(t, err)
		require.Equal(t, address.Hex(), gotAddr.Hex())
	})
}

func TestCalcBaseFee(t *testing.T) {
	for _, chainID := range []constants.ChainID{constants.ExampleChainID, constants.TwelveDecimalsChainID, constants.SixDecimalsChainID} {
		t.Run(chainID.ChainID, func(t *testing.T) {
			evmConfigurator := evmtypes.NewEVMConfigurator().
				WithEVMCoinInfo(constants.ExampleChainCoinInfo[chainID])
			evmConfigurator.ResetTestConfig()
			err := evmConfigurator.Configure()
			require.NoError(t, err)

			config := &params.ChainConfig{
				LondonBlock: big.NewInt(0),
			}

			testCases := []struct {
				name           string
				config         *params.ChainConfig
				parent         *ethtypes.Header
				params         feemarkettypes.Params
				expectedResult *big.Int
				expectedError  string
				checkFunc      func(t *testing.T, result *big.Int, parent *ethtypes.Header)
			}{
				{
					name: "pre-London block - returns InitialBaseFee",
					config: &params.ChainConfig{
						LondonBlock: big.NewInt(100), // London activated at block 100
					},
					parent: &ethtypes.Header{
						Number:  big.NewInt(50), // Block 50 (pre-London)
						BaseFee: big.NewInt(1000000000),
					},
					params: feemarkettypes.Params{
						ElasticityMultiplier:     2,
						BaseFeeChangeDenominator: 8,
						MinGasPrice:              sdkmath.LegacyZeroDec(),
					},
					expectedResult: big.NewInt(params.InitialBaseFee), // 1000000000
					expectedError:  "",
				},
				{
					name: "ElasticityMultiplier is zero - returns error",
					config: &params.ChainConfig{
						LondonBlock: big.NewInt(0), // London activated from genesis
					},
					parent: &ethtypes.Header{
						Number:   big.NewInt(10),
						BaseFee:  big.NewInt(1000000000),
						GasLimit: 10000000,
						GasUsed:  5000000,
					},
					params: feemarkettypes.Params{
						ElasticityMultiplier:     0, // Invalid - zero
						BaseFeeChangeDenominator: 8,
						MinGasPrice:              sdkmath.LegacyZeroDec(),
					},
					expectedResult: nil,
					expectedError:  "ElasticityMultiplier cannot be 0 as it's checked in the params validation",
				},
				{
					name:   "gas used equals target - base fee unchanged",
					config: config,
					parent: &ethtypes.Header{
						Number:   big.NewInt(10),
						BaseFee:  big.NewInt(1000000000),
						GasLimit: 10000000,
						GasUsed:  5000000, // Target = 10000000 / 2 = 5000000
					},
					params: feemarkettypes.Params{
						ElasticityMultiplier:     2,
						BaseFeeChangeDenominator: 8,
						MinGasPrice:              sdkmath.LegacyZeroDec(),
					},
					expectedResult: big.NewInt(1000000000), // Unchanged
					expectedError:  "",
				},
				{
					name:   "gas used > target - base fee increases",
					config: config,
					parent: &ethtypes.Header{
						Number:   big.NewInt(10),
						BaseFee:  big.NewInt(1000000000),
						GasLimit: 10000000,
						GasUsed:  7500000, // Target = 5000000, used > target
					},
					params: feemarkettypes.Params{
						ElasticityMultiplier:     2,
						BaseFeeChangeDenominator: 8,
						MinGasPrice:              sdkmath.LegacyZeroDec(),
					},
					expectedResult: func() *big.Int {
						// gasUsedDelta = 7500000 - 5000000 = 2500000
						// baseFeeDelta = max(1, 1000000000 * 2500000 / 5000000 / 8)
						// baseFeeDelta = max(1, 62500000)
						// result = 1000000000 + 62500000 = 1062500000
						return big.NewInt(1062500000)
					}(),
					expectedError: "",
				},
				{
					name:   "gas used < target - base fee decreases",
					config: config,
					parent: &ethtypes.Header{
						Number:   big.NewInt(10),
						BaseFee:  big.NewInt(1000000000),
						GasLimit: 10000000,
						GasUsed:  2500000, // Target = 5000000, used < target
					},
					params: feemarkettypes.Params{
						ElasticityMultiplier:     2,
						BaseFeeChangeDenominator: 8,
						MinGasPrice:              sdkmath.LegacyNewDec(1_000_000_000), // 1 minimum gas unit
					},
					expectedResult: func() *big.Int {
						// gasUsedDelta = 5000000 - 2500000 = 2500000
						// baseFeeDelta = 1000000000 * 2500000 / 5000000 / 8 = 62500000
						// result = max(1000000000 - 62500000, minGasPrice)
						// result = max(937500000, 1000000000) = 1000000000 (minGasPrice wins)
						factor := sdkmath.LegacyNewDecFromInt(evmtypes.GetEVMCoinDecimals().ConversionFactor())
						return factor.Mul(sdkmath.LegacyNewDec(1_000_000_000)).TruncateInt().BigInt()
					}(),
					expectedError: "",
				},
				{
					name:   "base fee decrease with low min gas price",
					config: config,
					parent: &ethtypes.Header{
						Number:   big.NewInt(10),
						BaseFee:  big.NewInt(1000000000),
						GasLimit: 10000000,
						GasUsed:  2500000,
					},
					params: feemarkettypes.Params{
						ElasticityMultiplier:     2,
						BaseFeeChangeDenominator: 8,
						MinGasPrice:              sdkmath.LegacyNewDecWithPrec(1, 12), // Very low
					},
					expectedResult: func() *big.Int {
						// result = 1000000000 - 62500000 = 937500000
						// minGasPrice is very low, so doesn't affect result
						return big.NewInt(937500000)
					}(),
					expectedError: "",
				},
				{
					name:   "small base fee delta gets clamped to 1",
					config: config,
					parent: &ethtypes.Header{
						Number:   big.NewInt(10),
						BaseFee:  big.NewInt(1000),
						GasLimit: 10000000,
						GasUsed:  5000001, // Tiny increase
					},
					params: feemarkettypes.Params{
						ElasticityMultiplier:     2,
						BaseFeeChangeDenominator: 8,
						MinGasPrice:              sdkmath.LegacyZeroDec(),
					},
					expectedResult: func() *big.Int {
						// gasUsedDelta = 1
						// baseFeeDelta = max(1, 1000 * 1 / 5000000 / 8) = max(1, 0) = 1
						// result = 1000 + 1 = 1001
						return big.NewInt(1001)
					}(),
					expectedError: "",
				},
				{
					name:   "very high gas usage",
					config: config,
					parent: &ethtypes.Header{
						Number:   big.NewInt(10),
						BaseFee:  big.NewInt(1000000000),
						GasLimit: 30000000,
						GasUsed:  29000000, // Nearly full block
					},
					params: feemarkettypes.Params{
						ElasticityMultiplier:     2,
						BaseFeeChangeDenominator: 8,
						MinGasPrice:              sdkmath.LegacyZeroDec(),
					},
					expectedResult: nil,
					expectedError:  "",
					checkFunc: func(t *testing.T, result *big.Int, parent *ethtypes.Header) {
						t.Helper()
						require.True(t, result.Cmp(parent.BaseFee) > 0, "Base fee should increase significantly")
					},
				},
				{
					name:   "very low gas usage",
					config: config,
					parent: &ethtypes.Header{
						Number:   big.NewInt(10),
						BaseFee:  big.NewInt(1000000000),
						GasLimit: 30000000,
						GasUsed:  1000000, // Very low usage
					},
					params: feemarkettypes.Params{
						ElasticityMultiplier:     2,
						BaseFeeChangeDenominator: 8,
						MinGasPrice:              sdkmath.LegacyZeroDec(),
					},
					expectedResult: nil,
					expectedError:  "",
					checkFunc: func(t *testing.T, result *big.Int, parent *ethtypes.Header) {
						t.Helper()
						require.True(t, result.Cmp(parent.BaseFee) < 0, "Base fee should decrease significantly")
					},
				},
				{
					name:   "zero gas used",
					config: config,
					parent: &ethtypes.Header{
						Number:   big.NewInt(10),
						BaseFee:  big.NewInt(1000000000),
						GasLimit: 30000000,
						GasUsed:  0, // No gas used
					},
					params: feemarkettypes.Params{
						ElasticityMultiplier:     2,
						BaseFeeChangeDenominator: 8,
						MinGasPrice:              sdkmath.LegacyNewDec(50_000_000_000), // 50 minimum gas unit
					},
					expectedResult: nil,
					expectedError:  "",
					checkFunc: func(t *testing.T, result *big.Int, parent *ethtypes.Header) {
						t.Helper()
						// Should be at least the minimum gas price
						factor := sdkmath.LegacyNewDecFromInt(evmtypes.GetEVMCoinDecimals().ConversionFactor())
						expectedMinGasPrice := sdkmath.LegacyNewDec(50_000_000_000).Mul(factor).TruncateInt().BigInt()
						require.True(t, result.Cmp(expectedMinGasPrice) >= 0, "Result should be at least min gas price")
					},
				},
			}

			for _, tc := range testCases {
				t.Run(tc.name, func(t *testing.T) {
					result, err := types.CalcBaseFee(tc.config, tc.parent, tc.params)

					if tc.expectedError != "" {
						require.Error(t, err)
						require.Contains(t, err.Error(), tc.expectedError)
						require.Nil(t, result)
					} else {
						require.NoError(t, err)
						require.NotNil(t, result)
						if tc.checkFunc != nil {
							tc.checkFunc(t, result, tc.parent)
						} else {
							require.Equal(t, tc.expectedResult, result,
								"Expected: %s, Got: %s", tc.expectedResult.String(), result.String())
						}
					}
				})
			}
		})
	}
}

func TestHexAddressFromBech32String(t *testing.T) {
	accAddr := "cosmos16val7w9lc7wltqvpt0kscaul4xd6l2l43nhcq4"
	valAddr := "cosmosvaloper16val7w9lc7wltqvpt0kscaul4xd6l2l458rdvx"
	consAddr := "cosmosvalcons16val7w9lc7wltqvpt0kscaul4xd6l2l4q5s3q8"
	invalidAddr := "invalid1address"
	expectedHex := "0xd33bFF38Bfc79df581815BED0c779FA99BaFAbf5"

	testCases := []struct {
		name      string
		input     string
		wantHex   string
		wantError bool
	}{
		{"account address", accAddr, expectedHex, false},
		{"validator address", valAddr, expectedHex, false},
		{"consensus address", consAddr, expectedHex, false},
		{"invalid address", invalidAddr, "", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			addr, err := utils.HexAddressFromBech32String(tc.input)
			if tc.wantError {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				require.Equal(t, tc.wantHex, addr.Hex())
			}
		})
	}
}
