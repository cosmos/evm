package v2

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/precompiles/bank"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// EVMKeeper defines the expected interface for the EVM keeper.
type EVMKeeper interface {
	CallEVMWithData(ctx sdk.Context, from common.Address, contract *common.Address, data []byte, commit bool, gasCap *big.Int) (*evmtypes.MsgEthereumTxResponse, error)
}

// ERC20Keeper defines the expected interface for the ERC20 keeper.
type ERC20Keeper interface {
	GetDynamicPrecompiles(ctx sdk.Context) []string
	DeleteDynamicPrecompile(ctx sdk.Context, precompile common.Address)
	GetTokenPairID(ctx sdk.Context, token string) []byte
	GetTokenPair(ctx sdk.Context, id []byte) (erc20types.TokenPair, bool)
	SetTokenPair(ctx sdk.Context, tokenPair erc20types.TokenPair)
	SetERC20Map(ctx sdk.Context, erc20 common.Address, id []byte)
	DeleteERC20Map(ctx sdk.Context, erc20 common.Address)
	SetDenomMap(ctx sdk.Context, denom string, id []byte)
	DeleteTokenPairByID(ctx sdk.Context, id []byte)
	DeleteDenomMap(ctx sdk.Context, denom string)
}

// BankPrecompileAddress is the address of the bank precompile (0x804).
var BankPrecompileAddress = common.HexToAddress(evmtypes.BankPrecompileAddress)

// deployERC20Contract deploys an ERC20 contract using the CREATE2 factory.
func deployERC20Contract(
	ctx sdk.Context,
	evmKeeper EVMKeeper,
	deployer common.Address,
	denom string,
) (common.Address, error) {
	expectedAddr, err := bank.ERC20ContractAddress(BankPrecompileAddress, denom)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to compute ERC20 address: %w", err)
	}

	deployData, err := bank.Create2DeployData(denom, BankPrecompileAddress)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to create deploy data: %w", err)
	}

	res, err := evmKeeper.CallEVMWithData(
		ctx,
		deployer,
		&bank.Create2FactoryAddress,
		deployData,
		true,
		nil,
	)
	if err != nil {
		return common.Address{}, fmt.Errorf("failed to deploy ERC20: %w", err)
	}
	if res.Failed() {
		return common.Address{}, fmt.Errorf("ERC20 deployment failed")
	}

	return expectedAddr, nil
}

// MigrateDynamicPrecompilesToERC20 migrates dynamic precompiles to canonical
// ERC20 wrapper contracts deployed via CREATE2. The new contracts delegate to
// the bank precompile (0x804) and have deterministic addresses based on denom.
//
// For each dynamic precompile:
//  1. Compute the deterministic CREATE2 address for the new ERC20 wrapper
//  2. Deploy the canonical ERC20 wrapper contract via CREATE2 factory
//  3. Update state mappings with the new ERC20 address
//  4. Remove the dynamic precompile registration
func MigrateDynamicPrecompilesToERC20(
	ctx sdk.Context,
	evmKeeper EVMKeeper,
	erc20Keeper ERC20Keeper,
	deployer common.Address,
) error {
	logger := ctx.Logger().With("module", "erc20-migration")

	dynamicPrecompiles := erc20Keeper.GetDynamicPrecompiles(ctx)
	if len(dynamicPrecompiles) == 0 {
		logger.Info("No dynamic precompiles to migrate")
		return nil
	}

	logger.Info("Starting migration of dynamic precompiles", "count", len(dynamicPrecompiles))

	for _, precompileAddr := range dynamicPrecompiles {
		addr := common.HexToAddress(precompileAddr)

		pairID := erc20Keeper.GetTokenPairID(ctx, precompileAddr)
		if len(pairID) == 0 {
			logger.Warn("No token pair found for precompile", "address", precompileAddr)
			continue
		}

		pair, found := erc20Keeper.GetTokenPair(ctx, pairID)
		if !found {
			logger.Warn("Token pair not found", "address", precompileAddr)
			continue
		}

		denom := pair.GetDenom()
		if denom == "" {
			logger.Warn("Empty denom for token pair", "address", precompileAddr)
			continue
		}

		expectedAddr, err := deployERC20Contract(ctx, evmKeeper, deployer, denom)
		if err != nil {
			return fmt.Errorf("failed to deploy ERC20 for denom %s: %w", denom, err)
		}

		logger.Info("Deployed canonical ERC20 wrapper",
			"denom", denom,
			"old_address", precompileAddr,
			"new_address", expectedAddr.Hex(),
		)

		oldERC20Addr := pair.GetERC20Contract()
		oldPairID := pairID

		// Delete old mappings (required because TokenPair.GetID() changes with new address)
		erc20Keeper.DeleteTokenPairByID(ctx, oldPairID)
		erc20Keeper.DeleteERC20Map(ctx, oldERC20Addr)
		erc20Keeper.DeleteDenomMap(ctx, denom)

		// Create new mappings with updated address
		pair.Erc20Address = expectedAddr.Hex()
		newPairID := pair.GetID()
		erc20Keeper.SetTokenPair(ctx, pair)
		erc20Keeper.SetERC20Map(ctx, expectedAddr, newPairID)
		erc20Keeper.SetDenomMap(ctx, denom, newPairID)

		erc20Keeper.DeleteDynamicPrecompile(ctx, addr)
	}

	logger.Info("Migration completed", "migrated", len(dynamicPrecompiles))
	return nil
}

// DeployCanonicalERC20 deploys a canonical ERC20 wrapper for a denom via CREATE2.
func DeployCanonicalERC20(
	ctx sdk.Context,
	evmKeeper EVMKeeper,
	deployer common.Address,
	denom string,
) (common.Address, error) {
	return deployERC20Contract(ctx, evmKeeper, deployer, denom)
}
