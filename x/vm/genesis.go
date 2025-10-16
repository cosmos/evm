package vm

import (
	"fmt"
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/x/vm/keeper"
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// InitGenesis initializes genesis state based on exported genesis
func InitGenesis(
	ctx sdk.Context,
	k *keeper.Keeper,
	accountKeeper types.AccountKeeper,
	bankKeeper types.BankKeeper,
	data types.GenesisState,
	initializer *sync.Once,
) []abci.ValidatorUpdate {
	err := k.SetParams(ctx, data.Params)
	if err != nil {
		panic(fmt.Errorf("error setting params %s", err))
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

	if err := k.InitEvmCoinInfo(ctx); err != nil {
		panic(fmt.Errorf("error initializing evm coin info: %s", err))
	}

	coinInfo := k.GetEvmCoinInfo(ctx)
	initializer.Do(func() {
		SetGlobalConfigVariables(coinInfo)
	})

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
