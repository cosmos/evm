package vm

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/x/vm/keeper"
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// InitGenesis initializes genesis state based on exported genesis
func InitGenesis(
	ctx sdk.Context,
	k *keeper.Keeper,
	accountKeeper types.AccountKeeper,
	data types.GenesisState,
) []abci.ValidatorUpdate {
	err := k.SetParams(ctx, data.Params)
	if err != nil {
		panic(fmt.Errorf("error setting params %s", err))
	}

	// Derive and set evmCoinInfo from bank metadata and VM params
	if err := deriveAndSetEvmCoinInfo(ctx, k, data.Params); err != nil {
		panic(fmt.Errorf("error deriving EVM coin info from genesis: %w", err))
	}

	// ensure evm module account is set
	if addr := accountKeeper.GetModuleAddress(types.ModuleName); addr == nil {
		panic("the EVM module account has not been set")
	}

	for _, account := range data.Accounts {
		address := common.HexToAddress(account.Address)
		accAddress := sdk.AccAddress(address.Bytes())

		// check that the account is actually found in the account keeper
		acc := accountKeeper.GetAccount(ctx, accAddress)
		if acc == nil {
			panic(fmt.Errorf("account not found for address %s", account.Address))
		}

		code := common.Hex2Bytes(account.Code)
		codeHash := crypto.Keccak256Hash(code).Bytes()

		if !types.IsEmptyCodeHash(codeHash) {
			k.SetCodeHash(ctx, address.Bytes(), codeHash)
		}

		if len(code) != 0 {
			k.SetCode(ctx, codeHash, code)
		}

		for _, storage := range account.Storage {
			k.SetState(ctx, address, common.HexToHash(storage.Key), common.HexToHash(storage.Value).Bytes())
		}
	}

	if err := k.AddPreinstalls(ctx, data.Preinstalls); err != nil {
		panic(fmt.Errorf("error adding preinstalls: %s", err))
	}

	return []abci.ValidatorUpdate{}
}

// ExportGenesis exports genesis state of the EVM module
func ExportGenesis(ctx sdk.Context, k *keeper.Keeper) *types.GenesisState {
	var ethGenAccounts []types.GenesisAccount
	k.IterateContracts(ctx, func(address common.Address, codeHash common.Hash) (stop bool) {
		storage := k.GetAccountStorage(ctx, address)

		genAccount := types.GenesisAccount{
			Address: address.String(),
			Code:    common.Bytes2Hex(k.GetCode(ctx, codeHash)),
			Storage: storage,
		}

		ethGenAccounts = append(ethGenAccounts, genAccount)
		return false
	})

	return &types.GenesisState{
		Accounts: ethGenAccounts,
		Params:   k.GetParams(ctx),
	}
}

// deriveAndSetEvmCoinInfo derives the EVM coin info from bank metadata and VM params and sets it
func deriveAndSetEvmCoinInfo(ctx sdk.Context, k *keeper.Keeper, params types.Params) error {
	evmDenom := params.EvmDenom
	if evmDenom == "" {
		return fmt.Errorf("evm_denom parameter is empty")
	}

	bankKeeper := k.GetBankKeeper()
	metadata, found := bankKeeper.GetDenomMetaData(ctx, evmDenom)
	if !found {
		return fmt.Errorf("bank metadata not found for evm_denom: %s", evmDenom)
	}

	coinInfo, err := types.DeriveCoinInfoFromMetadata(metadata, evmDenom)
	if err != nil {
		return fmt.Errorf("failed to derive coin info from bank metadata: %w", err)
	}

	// Set the evmCoinInfo globally
	if err := types.SetEVMCoinInfo(*coinInfo); err != nil {
		return fmt.Errorf("failed to set EVM coin info: %w", err)
	}

	return nil
}

// ValidateStakingBondDenomWithBankMetadata validates that the required staking bond denom
// is included in the bank metadata and has proper EVM compatibility.
// This function can be called at the app level to ensure proper configuration.
func ValidateStakingBondDenomWithBankMetadata(stakingBondDenom string, bankMetadata []banktypes.Metadata) error {
	// Find the bank metadata for the staking bond denom
	var bondMetadata *banktypes.Metadata
	for _, metadata := range bankMetadata {
		if metadata.Base == stakingBondDenom {
			bondMetadata = &metadata
			break
		}
	}

	if bondMetadata == nil {
		return fmt.Errorf("bank metadata not found for staking bond denom: %s. "+
			"The bank module genesis must include metadata for the staking bond denomination", stakingBondDenom)
	}

	// For staking bond denom, we need to ensure it has an 18-decimal variant for EVM compatibility
	found18DecimalVariant := false
	for _, unit := range bondMetadata.DenomUnits {
		if unit.Exponent == 18 {
			found18DecimalVariant = true
			break
		}
		// Check aliases for 18-decimal variants (like "atto" prefix)
		for _, alias := range unit.Aliases {
			if strings.HasPrefix(alias, "atto") || strings.Contains(alias, "18") {
				found18DecimalVariant = true
				break
			}
		}
		if found18DecimalVariant {
			break
		}
	}

	if !found18DecimalVariant {
		return fmt.Errorf(
			"staking bond denom %s requires an 18-decimal variant in bank metadata for EVM compatibility, "+
				"but none found. This is required for proper EVM gas token handling",
			stakingBondDenom,
		)
	}

	return nil
}
