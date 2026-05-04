package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/cosmos/evm/utils"
	"github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/core/address"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/input"
	"github.com/cosmos/cosmos-sdk/client/tx"
	sdk "github.com/cosmos/cosmos-sdk/types"
	types2 "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// NewTxCmd returns a root CLI command handler for evm module transaction commands
func NewTxCmd(ac address.Codec) *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "evm subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		NewRawTxCmd(),
		NewSendTxCmd(ac),
	)
	return txCmd
}

// NewRawTxCmd command build cosmos transaction from raw ethereum transaction
func NewRawTxCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "raw TX_HEX",
		Short: "Build cosmos transaction from raw ethereum transaction",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := hexutil.Decode(args[0])
			if err != nil {
				return errors.Wrap(err, "failed to decode ethereum tx hex bytes")
			}

			msg := &types.MsgEthereumTx{}
			if err := msg.UnmarshalBinary(data); err != nil {
				return err
			}

			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			baseDenom := types.GetEVMCoinDenom()

			tx, err := msg.BuildTx(clientCtx.TxConfig.NewTxBuilder(), baseDenom)
			if err != nil {
				return err
			}

			if clientCtx.GenerateOnly {
				json, err := clientCtx.TxConfig.TxJSONEncoder()(tx)
				if err != nil {
					return err
				}

				return clientCtx.PrintString(fmt.Sprintf("%s\n", json))
			}

			if !clientCtx.SkipConfirm {
				out, err := clientCtx.TxConfig.TxJSONEncoder()(tx)
				if err != nil {
					return err
				}

				_, _ = fmt.Fprintf(os.Stderr, "%s\n\n", out)

				buf := bufio.NewReader(os.Stdin)
				ok, err := input.GetConfirmation("confirm transaction before signing and broadcasting", buf, os.Stderr)

				if err != nil || !ok {
					_, _ = fmt.Fprintf(os.Stderr, "%s\n", "canceled transaction")
					return err
				}
			}

			txBytes, err := clientCtx.TxConfig.TxEncoder()(tx)
			if err != nil {
				return err
			}

			// broadcast to a CometBFT node
			res, err := clientCtx.BroadcastTx(txBytes)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddTxFlagsToCmd(cmd)
	return cmd
}

// NewSendTxCmd returns a CLI command handler for creating a MsgSend transaction.
func NewSendTxCmd(ac address.Codec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "send [from_key_or_address] [to_address] [amount]",
		Short: "Send funds from one account to another.",
		Long: `Send funds from one account to another. Both 0x and bech32 addresses
may be used.
Note, the '--from' flag is ignored as it is implied from [from_key_or_address].
When using '--dry-run' a key name cannot be used, only an 0x or bech32 address.
`,
		Example: "evmd tx evm send 0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E 0xA2A8B87390F8F2D188242656BFb6852914073D06 10utoken",
		Args:    cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			fromAddr := args[0]
			if strings.HasPrefix(args[0], "0x") {
				fromAddr = utils.Bech32StringFromHexAddress(args[0])
			}

			err := cmd.Flags().Set(flags.FlagFrom, fromAddr)
			if err != nil {
				return err
			}
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			toAddr, err := ac.StringToBytes(utils.Bech32StringFromHexAddress(args[1]))
			if err != nil {
				return err
			}

			coins, err := sdk.ParseCoinsNormalized(args[2])
			if err != nil {
				return err
			}

			if len(coins) == 0 {
				return fmt.Errorf("invalid coins")
			}

			msg := types2.NewMsgSend(clientCtx.GetFromAddress(), toAddr, coins)

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
