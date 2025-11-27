package client

import (
	"bufio"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/spf13/cobra"

	// Using EVM-specific crypto implementations for Ethereum-style keys
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/crypto/hd"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/crypto"
)

// EthPrivateKeyLength is the expected byte length of an Ethereum private key.
const EthPrivateKeyLength = 32

// NewUnsafeImportKeyCommand defines the CLI command to import Ethereum private keys.
// This command is explicitly marked as unsafe because it handles raw private keys via CLI args.
func NewUnsafeImportKeyCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "unsafe-import-eth-key <name> <pk>",
		Short: "**UNSAFE** Import Ethereum private keys into the local keybase",
		Long:  "**UNSAFE** Import a hex-encoded Ethereum private key into the local keybase. The private key must be a 32-byte hex string (e.g., 64 characters, excluding 0x prefix).",
		Args:  cobra.ExactArgs(2),
		RunE:  runImportCmd,
	}
}

func runImportCmd(cmd *cobra.Command, args []string) error {
	// Configure client context to use the Ethereum key type option.
	clientCtx := client.GetClientContextFromCmd(cmd).WithKeyringOptions(hd.EthSecp256k1Option())
	clientCtx, err := client.ReadPersistentCommandFlags(clientCtx, cmd.Flags())
	if err != nil {
		return err
	}

	keyName := args[0]
	pkHex := args[1]

	// 1. Decode the hex-encoded private key string.
	keyBytes := common.FromHex(pkHex)
	
	// 2. Critical: Validate the key length to prevent invalid key generation or crashes.
	if len(keyBytes) != EthPrivateKeyLength {
		return fmt.Errorf(
			"invalid private key length; expected %d bytes (64 hex characters), got %d bytes",
			EthPrivateKeyLength,
			len(keyBytes),
		)
	}

	// 3. Prompt for passphrase to encrypt the key in the local store.
	inBuf := bufio.NewReader(cmd.InOrStdin())
	passphrase, err := input.GetPassword("Enter passphrase to encrypt your key:", inBuf)
	if err != nil {
		return err
	}

	// 4. Construct the private key object.
	privKey := &ethsecp256k1.PrivKey{
		Key: keyBytes,
	}

	// 5. Encrypt the private key with the passphrase using the specified type.
	keyType := "eth_secp256k1"
	armor := crypto.EncryptArmorPrivKey(privKey, passphrase, keyType)

	// 6. Import the encrypted key (armor) into the keyring.
	if err := clientCtx.Keyring.ImportPrivKey(keyName, armor, passphrase); err != nil {
		return fmt.Errorf("failed to import private key: %w", err)
	}

	cmd.Printf("Successfully imported key '%s'\n", keyName)
	return nil
}
