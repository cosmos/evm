package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"cosmossdk.io/math"

	"github.com/cosmos/evm/x/epixmint/types"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/client/flags"
	"github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/version"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

// NewTxCmd returns a root CLI command handler for epixmint transaction commands
func NewTxCmd() *cobra.Command {
	txCmd := &cobra.Command{
		Use:                        types.ModuleName,
		Short:                      "epixmint subcommands",
		DisableFlagParsing:         true,
		SuggestionsMinimumDistance: 2,
		RunE:                       client.ValidateCmd,
	}

	txCmd.AddCommand(
		NewUpdateParamsCmd(),
		NewSubmitUpdateParamsProposalCmd(),
	)
	return txCmd
}

// NewUpdateParamsCmd returns a CLI command handler for creating a MsgUpdateParams transaction.
func NewUpdateParamsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update-params [mint-denom] [initial-annual-mint-amount] [annual-reduction-rate] [block-time-seconds] [max-supply] [community-pool-rate] [staking-rewards-rate]",
		Short: "Generate a MsgUpdateParams transaction to update epixmint module parameters",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Generate a MsgUpdateParams transaction to update epixmint module parameters.
This command creates the transaction that should be submitted via governance proposal.

Example:
$ %s tx epixmint update-params aepix 10527000000000000000000000000 0.25 6 42000000000000000000000000000 0.02 0.98 --from mykey

Then submit via governance:
$ %s tx gov submit-proposal /path/to/proposal.json --from mykey

Parameters:
- mint-denom: The denomination of the coin to mint (e.g., aepix)
- initial-annual-mint-amount: The starting amount of tokens to mint in year 1 (in base units)
- annual-reduction-rate: The percentage reduction per year (decimal, e.g., 0.25 for 25%%)
- block-time-seconds: The expected block time in seconds (e.g., 6)
- max-supply: The maximum total supply that can ever be minted (in base units)
- community-pool-rate: The rate of minted tokens sent to community pool (decimal, e.g., 0.02 for 2%%)
- staking-rewards-rate: The rate of minted tokens sent to staking rewards (decimal, e.g., 0.98 for 98%%)
`,
				version.AppName,
				version.AppName,
			),
		),
		Args: cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			clientCtx, err := client.GetClientTxContext(cmd)
			if err != nil {
				return err
			}

			// Parse arguments
			mintDenom := args[0]

			initialAnnualMintAmount, ok := math.NewIntFromString(args[1])
			if !ok {
				return fmt.Errorf("invalid initial annual mint amount: %s", args[1])
			}

			annualReductionRate, err := math.LegacyNewDecFromStr(args[2])
			if err != nil {
				return fmt.Errorf("invalid annual reduction rate: %s", args[2])
			}

			blockTimeSeconds, err := strconv.ParseUint(args[3], 10, 64)
			if err != nil {
				return fmt.Errorf("invalid block time seconds: %s", args[3])
			}

			maxSupply, ok := math.NewIntFromString(args[4])
			if !ok {
				return fmt.Errorf("invalid max supply: %s", args[4])
			}

			communityPoolRate, err := math.LegacyNewDecFromStr(args[5])
			if err != nil {
				return fmt.Errorf("invalid community pool rate: %s", args[5])
			}

			stakingRewardsRate, err := math.LegacyNewDecFromStr(args[6])
			if err != nil {
				return fmt.Errorf("invalid staking rewards rate: %s", args[6])
			}

			// Create the parameters with new dynamic structure
			params := types.Params{
				MintDenom:               mintDenom,
				InitialAnnualMintAmount: initialAnnualMintAmount,
				AnnualReductionRate:     annualReductionRate,
				BlockTimeSeconds:        blockTimeSeconds,
				MaxSupply:               maxSupply,
				CommunityPoolRate:       communityPoolRate,
				StakingRewardsRate:      stakingRewardsRate,
			}

			// Validate parameters
			if err := params.Validate(); err != nil {
				return fmt.Errorf("invalid parameters: %w", err)
			}

			// Get authority (governance module account)
			authority := authtypes.NewModuleAddress(govtypes.ModuleName)

			// Create the message
			msg := &types.MsgUpdateParams{
				Authority: authority.String(),
				Params:    params,
			}

			// Validate the message
			if err := msg.ValidateBasic(); err != nil {
				return err
			}

			return tx.GenerateOrBroadcastTxCLI(clientCtx, cmd.Flags(), msg)
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}

// NewSubmitUpdateParamsProposalCmd returns a CLI command handler for submitting an update params governance proposal.
func NewSubmitUpdateParamsProposalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit-update-params-proposal [proposal-file]",
		Short: "Submit a governance proposal to update epixmint module parameters",
		Long: strings.TrimSpace(
			fmt.Sprintf(`Submit a governance proposal to update epixmint module parameters.

The proposal file should be a JSON file with the following format:
{
  "messages": [
    {
      "@type": "/epixmint.v1.MsgUpdateParams",
      "authority": "cosmos10d07y265gmmuvt4z0w9aw880jnsr700j6zn9kn",
      "params": {
        "mint_denom": "aepix",
        "annual_mint_amount": "2099000000000000000000000000",
        "max_supply": "42000000000000000000000000000",
        "blocks_per_year": "5256000",
        "community_pool_rate": "0.020000000000000000",
        "staking_rewards_rate": "0.980000000000000000"
      }
    }
  ],
  "metadata": "Update EpixMint Parameters",
  "deposit": "1000000aepix",
  "title": "Update EpixMint Parameters",
  "summary": "Update minting parameters for the epixmint module"
}

Example:
$ %s tx epixmint submit-update-params-proposal proposal.json --from mykey
`,
				version.AppName,
			),
		),
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			// This is a placeholder - in practice, you would read the proposal file
			// and submit it via the governance module
			return fmt.Errorf("please use 'tx gov submit-proposal %s' instead", args[0])
		},
	}

	flags.AddTxFlagsToCmd(cmd)

	return cmd
}
