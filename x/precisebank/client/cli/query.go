package cli

import (
	"github.com/spf13/cobra"

	"github.com/cosmos/evm/x/precisebank/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
)

// GetQueryCmd returns the parent command for all x/precisebank CLI query commands.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the precise bank module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetRemainderCmd(),
		GetFractionalBalanceCmd(),
	)
	return cmd
}

// GetRemainderCmd queries the remainder amount
func GetRemainderCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "remainder",
		Short: "Get the remainder amount",
		Long:  "Get the remainder amount in the precise bank module",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			ctx := cmd.Context()
			res, err := queryClient.Remainder(ctx, &types.QueryRemainderRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}

// GetFractionalBalanceCmd queries the fractional balance of an account
func GetFractionalBalanceCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fractional-balance [address]",
		Short: "Get the fractional balance of an account",
		Long:  "Get the fractional balance of an account at the specified address",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			ctx := cmd.Context()
			res, err := queryClient.FractionalBalance(ctx, &types.QueryFractionalBalanceRequest{
				Address: args[0],
			})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)
	return cmd
}
