package cli

import (
	"github.com/cosmos/evm/utils"
	"github.com/spf13/cobra"

	rpctypes "github.com/cosmos/evm/rpc/types"
	"github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
)

// GetQueryCmd returns the parent command for all x/bank CLi query commands.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the evm module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetStorageCmd(),
		GetCodeCmd(),
		GetAccountCmd(),
		GetParamsCmd(),
		GetConfigCmd(),
		HexToBech32Cmd(),
		Bech32ToHexCmd(),
	)
	return cmd
}

// GetStorageCmd queries a key in an accounts storage
func GetStorageCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "storage ADDRESS KEY",
		Short: "Gets storage for an account with a given key and height",
		Long:  "Gets storage for an account with a given key and height. If the height is not provided, it will use the latest height from context.", //nolint:lll
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			address, err := accountToHex(args[0])
			if err != nil {
				return err
			}

			key := formatKeyToHash(args[1])

			req := &types.QueryStorageRequest{
				Address: address,
				Key:     key,
			}

			res, err := queryClient.Storage(rpctypes.ContextWithHeight(clientCtx.Height), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetCodeCmd queries the code field of a given address
func GetCodeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "code ADDRESS",
		Short: "Gets code from an account",
		Long:  "Gets code from an account. If the height is not provided, it will use the latest height from context.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			address, err := accountToHex(args[0])
			if err != nil {
				return err
			}

			req := &types.QueryCodeRequest{
				Address: address,
			}

			res, err := queryClient.Code(rpctypes.ContextWithHeight(clientCtx.Height), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetAccountCmd queries the account of a given address
func GetAccountCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "account ADDRESS",
		Short: "Gets account info from an address",
		Long:  "Gets account info from an address. If the height is not provided, it will use the latest height from context.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			address, err := accountToHex(args[0])
			if err != nil {
				return err
			}

			req := &types.QueryAccountRequest{
				Address: address,
			}

			res, err := queryClient.Account(rpctypes.ContextWithHeight(clientCtx.Height), req)
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetParamsCmd queries the fee market params
func GetParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Get the evm params",
		Long:  "Get the evm parameter values.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetConfigCmd queries the evm configuration
func GetConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Get the evm config",
		Long:  "Get the evm configuration values.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Config(cmd.Context(), &types.QueryConfigRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetConfigCmd queries the evm configuration
func HexToBech32Cmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "0x-to-bech32",
		Short:   "Get the evm 0x address for a given bech32 address",
		Long:    "Get the evm 0x address for a given bech32 address.",
		Example: "evmd query evm 0x-to-bech32 0x7cB61D4117AE31a12E393a1Cfa3BaC666481D02E",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cmd.Println(utils.Bech32StringFromHexAddress(args[0]))
			return nil
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetConfigCmd queries the evm configuration
func Bech32ToHexCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "bech32-to-0x",
		Short:   "Get the bech32 address for a given 0x address",
		Long:    "Get the bech32 address for a given 0x address.",
		Example: "evmd query evm bech32-to-0x cosmos10jmp6sgh4cc6zt3e8gw05wavvejgr5pwsjskvv",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			hex, err := utils.HexAddressFromBech32String(args[0])
			if err != nil {
				return err
			}
			cmd.Println(hex.String())
			return nil
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
