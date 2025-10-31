package keys

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"

	"github.com/spf13/cobra"

	cryptohd "github.com/cosmos/evm/crypto/hd"
	evmkeyring "github.com/cosmos/evm/crypto/keyring"
	bip39 "github.com/cosmos/go-bip39"
	ledger "github.com/cosmos/ledger-cosmos-go"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/client/keys"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	cosmosLedger "github.com/cosmos/cosmos-sdk/crypto/ledger"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	flagInteractive       = "interactive"
	flagRecover           = "recover"
	flagNoBackup          = "no-backup"
	flagCoinType          = "coin-type"
	flagAccount           = "account"
	flagIndex             = "index"
	flagMultisig          = "multisig"
	flagMultiSigThreshold = "multisig-threshold"
	flagNoSort            = "nosort"
	flagHDPath            = "hd-path"
	flagMnemonicSrc       = "source"

	mnemonicEntropySize = 256
)

/*
RunAddCmd
input
  - bip39 mnemonic
  - bip39 passphrase
  - bip44 path
  - local encryption password

output
  - armor encrypted private key (saved to file)
*/
func RunAddCmd(ctx client.Context, cmd *cobra.Command, args []string, inBuf *bufio.Reader) error {
	var (
		algo keyring.SignatureAlgo
		err  error
	)

	name := args[0]

	interactive, _ := cmd.Flags().GetBool(flagInteractive)
	noBackup, _ := cmd.Flags().GetBool(flagNoBackup)
	useLedger, _ := cmd.Flags().GetBool(flags.FlagUseLedger)
	algoStr, _ := cmd.Flags().GetString(flags.FlagKeyType)

	showMnemonic := !noBackup
	kb := ctx.Keyring
	outputFormat := ctx.OutputFormat

	keyringAlgos, ledgerAlgos := kb.SupportedAlgorithms()

	// check if the provided signing algorithm is supported by the keyring or
	// ledger
	if useLedger {
		algo, err = keyring.NewSigningAlgoFromString(algoStr, ledgerAlgos)
	} else {
		algo, err = keyring.NewSigningAlgoFromString(algoStr, keyringAlgos)
	}

	if err != nil {
		return err
	}

	if dryRun, _ := cmd.Flags().GetBool(flags.FlagDryRun); dryRun {
		// use in memory keybase
		kb = keyring.NewInMemory(ctx.Codec, cryptohd.EthSecp256k1Option())
	} else {
		_, err = kb.Key(name)
		if err == nil {
			// account exists, ask for user confirmation
			response, err2 := input.GetConfirmation(fmt.Sprintf("override the existing name %s", name), inBuf, cmd.ErrOrStderr())
			if err2 != nil {
				return err2
			}

			if !response {
				return fmt.Errorf("key overwrite confirmation for %q aborted", name)
			}

			err2 = kb.Delete(name)
			if err2 != nil {
				return err2
			}
		}

		multisigKeys, _ := cmd.Flags().GetStringSlice(flagMultisig)
		if len(multisigKeys) != 0 {
			pks := make([]cryptotypes.PubKey, len(multisigKeys))
			multisigThreshold, _ := cmd.Flags().GetInt(flagMultiSigThreshold)
			if err := validateMultisigThreshold(multisigThreshold, len(multisigKeys)); err != nil {
				return err
			}

			for i, keyname := range multisigKeys {
				k, err := kb.Key(keyname)
				if err != nil {
					return err
				}

				key, err := k.GetPubKey()
				if err != nil {
					return err
				}
				pks[i] = key
			}

			if noSort, _ := cmd.Flags().GetBool(flagNoSort); !noSort {
				sort.Slice(pks, func(i, j int) bool {
					return bytes.Compare(pks[i].Address(), pks[j].Address()) < 0
				})
			}

			pk := multisig.NewLegacyAminoPubKey(multisigThreshold, pks)
			k, err := kb.SaveMultisig(name, pk)
			if err != nil {
				return err
			}

			return printCreate(cmd, k, false, "", outputFormat)
		}
	}

	pubKey, _ := cmd.Flags().GetString(keys.FlagPublicKey)
	if pubKey != "" {
		var pk cryptotypes.PubKey
		if err = ctx.Codec.UnmarshalInterfaceJSON([]byte(pubKey), &pk); err != nil {
			return err
		}

		k, err := kb.SaveOfflineKey(name, pk)
		if err != nil {
			return err
		}

		return printCreate(cmd, k, false, "", outputFormat)
	}

	coinType, _ := cmd.Flags().GetUint32(flagCoinType)
	account, _ := cmd.Flags().GetUint32(flagAccount)
	index, _ := cmd.Flags().GetUint32(flagIndex)
	hdPath, _ := cmd.Flags().GetString(flagHDPath)

	if len(hdPath) == 0 {
		hdPath = hd.CreateHDPath(coinType, account, index).String()
	} else if useLedger {
		return errors.New("cannot set custom bip32 path with ledger")
	}

	// If we're using ledger, only thing we need is the path and the bech32 prefix.
	if useLedger {
		bech32PrefixAccAddr := sdk.GetConfig().GetBech32AccountAddrPrefix()
		needHardware := flag.Lookup("test.v") == nil && os.Getenv("GO_TEST") != "1"
		if needHardware {
			switch coinType {
			case 60:
				cosmosLedger.SetDiscoverLedger(func() (cosmosLedger.SECP256K1, error) {
					return evmkeyring.LedgerDerivation()
				})
				cosmosLedger.SetCreatePubkey(func(key []byte) cryptotypes.PubKey {
					return evmkeyring.CreatePubkey(key)
				})
				cosmosLedger.SetAppName(evmkeyring.AppName)
				cosmosLedger.SetDERConversion(false)
			case 118:
				cosmosLedger.SetDiscoverLedger(func() (cosmosLedger.SECP256K1, error) {
					device, err := ledger.FindLedgerCosmosUserApp()
					if err != nil {
						return nil, err
					}
					return device, nil
				})
				cosmosLedger.SetCreatePubkey(func(key []byte) cryptotypes.PubKey {
					return &secp256k1.PubKey{Key: key}
				})
				cosmosLedger.SetAppName(cosmosLedger.AppName)
				cosmosLedger.SetDERConversion(true)
			default:
				return fmt.Errorf(
					"unsupported coin type %d for Ledger. Supported coin types: 60 (Ethereum app), 118 (Cosmos app)", coinType,
				)
			}
		}
		// use the provided algo to save the ledger key
		k, err := kb.SaveLedgerKey(name, algo, bech32PrefixAccAddr, coinType, account, index)
		if err != nil {
			return err
		}

		return printCreate(cmd, k, false, "", outputFormat)
	}

	// Get bip39 mnemonic
	var mnemonic, bip39Passphrase string

	recoverKey, _ := cmd.Flags().GetBool(flagRecover)
	mnemonicSrc, _ := cmd.Flags().GetString(flagMnemonicSrc)
	if recoverKey {
		if mnemonicSrc != "" {
			mnemonic, err = readMnemonicFromFile(mnemonicSrc)
			if err != nil {
				return err
			}
		} else {
			mnemonic, err = input.GetString("Enter your bip39 mnemonic", inBuf)
			if err != nil {
				return err
			}
		}

		if !bip39.IsMnemonicValid(mnemonic) {
			return errors.New("invalid mnemonic")
		}
	} else if interactive {
		if mnemonicSrc != "" {
			mnemonic, err = readMnemonicFromFile(mnemonicSrc)
			if err != nil {
				return err
			}
		} else {
			mnemonic, err = input.GetString("Enter your bip39 mnemonic, or hit enter to generate one.", inBuf)
			if err != nil {
				return err
			}
		}

		if !bip39.IsMnemonicValid(mnemonic) && mnemonic != "" {
			return errors.New("invalid mnemonic")
		}
	}

	if len(mnemonic) == 0 {
		// read entropy seed straight from tmcrypto.Rand and convert to mnemonic
		entropySeed, err := bip39.NewEntropy(mnemonicEntropySize)
		if err != nil {
			return err
		}

		mnemonic, err = bip39.NewMnemonic(entropySeed)
		if err != nil {
			return err
		}
	}

	// override bip39 passphrase
	if interactive {
		bip39Passphrase, err = input.GetString(
			"Enter your bip39 passphrase. This is combined with the mnemonic to derive the seed. "+
				"Most users should just hit enter to use the default, \"\"", inBuf)
		if err != nil {
			return err
		}

		// if they use one, make them re-enter it
		if len(bip39Passphrase) != 0 {
			p2, err := input.GetString("Repeat the passphrase:", inBuf)
			if err != nil {
				return err
			}

			if bip39Passphrase != p2 {
				return errors.New("passphrases don't match")
			}
		}
	}

	k, err := kb.NewAccount(name, mnemonic, bip39Passphrase, hdPath, algo)
	if err != nil {
		return err
	}

	// Recover key from seed passphrase
	if recoverKey {
		// Hide mnemonic from output
		showMnemonic = false
		mnemonic = ""
	}

	return printCreate(cmd, k, showMnemonic, mnemonic, outputFormat)
}

func printCreate(cmd *cobra.Command, k *keyring.Record, showMnemonic bool, mnemonic, outputFormat string) error {
	switch outputFormat {
	case OutputFormatText:
		cmd.PrintErrln()
		if err := printKeyringRecord(cmd.OutOrStdout(), k, keys.MkAccKeyOutput, outputFormat); err != nil {
			return err
		}

		// print mnemonic unless requested not to.
		if showMnemonic {
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(),
				"\n**Important** write this mnemonic phrase in a safe place.\nIt is the only way to recover your account if you ever forget your password.\n\n%s\n\n", //nolint:lll
				mnemonic); err != nil {
				return fmt.Errorf("failed to print mnemonic: %v", err)
			}
		}
	case OutputFormatJSON:
		out, err := keys.MkAccKeyOutput(k)
		if err != nil {
			return err
		}

		if showMnemonic {
			out.Mnemonic = mnemonic
		}

		jsonString, err := json.Marshal(out)
		if err != nil {
			return err
		}

		cmd.Println(string(jsonString))

	default:
		return fmt.Errorf("invalid output format %s", outputFormat)
	}

	return nil
}

func validateMultisigThreshold(k, nKeys int) error {
	if k <= 0 {
		return fmt.Errorf("threshold must be a positive integer")
	}
	if nKeys < k {
		return fmt.Errorf(
			"threshold k of n multisignature: %d < %d", nKeys, k)
	}
	return nil
}

func readMnemonicFromFile(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	bz, err := io.ReadAll(file)
	if err != nil {
		return "", err
	}
	return string(bz), nil
}
