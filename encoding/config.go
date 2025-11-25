package encoding

import (
	"fmt" // Added for cleaner error message formatting

	"google.golang.org/protobuf/reflect/protoreflect"

	evmaddress "github.com/cosmos/evm/encoding/address"
	enccodec "github.com/cosmos/evm/encoding/codec"
	"github.com/cosmos/evm/ethereum/eip712"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"github.com/cosmos/gogoproto/proto"

	"cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/client"
	amino "github.com/cosmos/cosmos-sdk/codec"
	"github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	"github.com/cosmos/cosmos-sdk/x/auth/tx"
)

// Config specifies the concrete encoding types to use for a given app.
// This struct bridges protobuf and Amino encoding for EVM-compatible applications.
type Config struct {
	InterfaceRegistry types.InterfaceRegistry
	Codec             amino.Codec
	TxConfig          client.TxConfig
	Amino             *amino.LegacyAmino
}

// MakeConfig creates a new encoding Config, setting up the necessary codecs and
// interface registry with EVM-specific signing requirements.
func MakeConfig(evmChainID uint64) Config {
	// Initialize legacy Amino codec (required for compatibility and EIP-712)
	cdc := amino.NewLegacyAmino()

	// --- 1. Custom Address Codecs ---
	// EVM chains require a specific codec that ensures addresses are compatible 
	// with both Cosmos Bech32 and Ethereum Hex formats.
	bech32Prefix := sdk.GetConfig().GetBech32AccountAddrPrefix()
	valPrefix := sdk.GetConfig().GetBech32ValidatorAddrPrefix()

	addressCodec := evmaddress.NewEvmCodec(bech32Prefix)
	validatorCodec := evmaddress.NewEvmCodec(valPrefix)

	// --- 2. Custom GetSigners Logic ---
	// Define custom logic for extracting signers from EVM-specific messages (MsgEthereumTx, MsgConvertERC20).
	// This is necessary because these messages do not use the standard Cosmos signers function.
	signingOptions := signing.Options{
		AddressCodec:          addressCodec,
		ValidatorAddressCodec: validatorCodec,
		CustomGetSigners: map[protoreflect.FullName]signing.GetSignersFunc{
			evmtypes.MsgEthereumTxCustomGetSigner.MsgType:   evmtypes.MsgEthereumTxCustomGetSigner.Fn,
			erc20types.MsgConvertERC20CustomGetSigner.MsgType: erc20types.MsgConvertERC20CustomGetSigner.Fn,
		},
	}

	// --- 3. Interface Registry Initialization (with Error Check) ---
	interfaceRegistry, err := types.NewInterfaceRegistryWithOptions(types.InterfaceRegistryOptions{
		ProtoFiles:     proto.HybridResolver,
		SigningOptions: signingOptions,
	})
	
	// CRITICAL FIX: Ensure the interface registry is successfully created.
	if err != nil {
		panic(fmt.Sprintf("failed to create new interface registry: %v", err))
	}
	
	// Register all module interfaces and legacy Amino types
	enccodec.RegisterLegacyAminoCodec(cdc)
	enccodec.RegisterInterfaces(interfaceRegistry)
	
	// Initialize Protobuf codec
	codec := amino.NewProtoCodec(interfaceRegistry)
	
	// Configure the EIP712 handler with the encoding parameters and chain ID.
	eip712.SetEncodingConfig(cdc, interfaceRegistry, evmChainID)

	// NOTE: This links the deprecated legacy tx signing bytes to the current Amino codec.
	// This is required for EIP712 transactions which currently rely on the deprecated legacytx.StdSignBytes logic.
	legacytx.RegressionTestingAminoCodec = cdc

	// Return the final configuration bundle
	return Config{
		InterfaceRegistry: interfaceRegistry,
		Codec:             codec,
		TxConfig:          tx.NewTxConfig(codec, tx.DefaultSignModes),
		Amino:             cdc,
	}
}
