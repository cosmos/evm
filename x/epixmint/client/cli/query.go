package cli

import (
	"github.com/spf13/cobra"

	"github.com/cosmos/evm/x/epixmint/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
)

// GetQueryCmd returns the parent command for all x/epixmint CLI query commands.
func GetQueryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "Querying commands for the epixmint module",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	cmd.AddCommand(
		GetParamsCmd(),
		GetInflationCmd(),
		GetAnnualProvisionsCmd(),
		GetCurrentSupplyCmd(),
		GetMaxSupplyCmd(),
	)
	return cmd
}

// GetParamsCmd returns the command for querying epixmint parameters.
func GetParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "params",
		Short: "Query the current epixmint parameters",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Params(cmd.Context(), &types.QueryParamsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(&res.Params)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetInflationCmd returns the command for querying the current inflation rate.
func GetInflationCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "inflation",
		Short: "Query the current inflation rate",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.Inflation(cmd.Context(), &types.QueryInflationRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetAnnualProvisionsCmd returns the command for querying the current annual provisions.
func GetAnnualProvisionsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "annual-provisions",
		Short: "Query the current annual provisions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.AnnualProvisions(cmd.Context(), &types.QueryAnnualProvisionsRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetCurrentSupplyCmd returns the command for querying the current supply.
func GetCurrentSupplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "current-supply",
		Short: "Query the current total supply of the mint denomination",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.CurrentSupply(cmd.Context(), &types.QueryCurrentSupplyRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}

// GetMaxSupplyCmd returns the command for querying the maximum supply.
func GetMaxSupplyCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "max-supply",
		Short: "Query the maximum total supply that can ever be minted",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientQueryContext(cmd)
			if err != nil {
				return err
			}

			queryClient := types.NewQueryClient(clientCtx)

			res, err := queryClient.MaxSupply(cmd.Context(), &types.QueryMaxSupplyRequest{})
			if err != nil {
				return err
			}

			return clientCtx.PrintProto(res)
		},
	}

	flags.AddQueryFlagsToCmd(cmd)

	return cmd
}
