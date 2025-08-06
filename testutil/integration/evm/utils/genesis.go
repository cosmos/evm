package utils

import (
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	utiltx "github.com/cosmos/evm/testutil/tx"
	erc20types "github.com/cosmos/evm/x/erc20/types"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// CreateGenesisWithTokenPairs creates a genesis that includes
// the WEVMOS and the provided denoms.
// If no denoms provided, creates only one dynamic precompile with the 'xmpl' denom.
func CreateGenesisWithTokenPairs(keyring testkeyring.Keyring, denoms ...string) network.CustomGenesisState {
	// Add all keys from the keyring to the genesis accounts as well.
	//
	// NOTE: This is necessary to enable the account to send EVM transactions,
	// because the Mono ante handler checks the account balance by querying the
	// account from the account keeper first. If these accounts are not in the genesis
	// state, the ante handler finds a zero balance because of the missing account.

	// if denom not provided, defaults to create only one dynamic erc20
	// precompile with the 'xmpl' denom
	if len(denoms) == 0 {
		denoms = []string{"xmpl"}
	}

	accs := keyring.GetAllAccAddrs()
	genesisAccounts := make([]*authtypes.BaseAccount, len(accs))
	for i, addr := range accs {
		genesisAccounts[i] = &authtypes.BaseAccount{
			Address:       addr.String(),
			PubKey:        nil,
			AccountNumber: uint64(i + 1), //nolint:gosec // G115
			Sequence:      1,
		}
	}

	accGenesisState := authtypes.DefaultGenesisState()
	for _, genesisAccount := range genesisAccounts {
		// NOTE: This type requires to be packed into a *types.Any as seen on SDK tests,
		// e.g. https://github.com/evmos/cosmos-sdk/blob/v0.47.5-evmos.2/x/auth/keeper/keeper_test.go#L193-L223
		accGenesisState.Accounts = append(accGenesisState.Accounts, codectypes.UnsafePackAny(genesisAccount))
	}

	// Add token pairs to genesis
	tokenPairs := make([]erc20types.TokenPair, 0, len(denoms)+1)
	tokenPairs = append(tokenPairs,
		// NOTE: the example token pairs are being added in the integration test utils
		testconstants.ExampleTokenPairs...,
	)

	dynPrecAddr := make([]string, 0, len(denoms))
	for _, denom := range denoms {
		addr := utiltx.GenerateAddress().Hex()
		tp := erc20types.TokenPair{
			Erc20Address:  addr,
			Denom:         denom,
			Enabled:       true,
			ContractOwner: erc20types.OWNER_MODULE, // NOTE: Owner is the module account since it's a native token and was registered through governance
		}
		tokenPairs = append(tokenPairs, tp)
		dynPrecAddr = append(dynPrecAddr, addr)
	}

	// STR v2: update the NativePrecompiles and DynamicPrecompiles
	// with the WEVMOS (default is mainnet) and 'xmpl' tokens in the erc20 params
	erc20GenesisState := erc20types.DefaultGenesisState()
	erc20GenesisState.TokenPairs = tokenPairs
	erc20GenesisState.NativePrecompiles = []string{testconstants.WEVMOSContractMainnet}
	erc20GenesisState.DynamicPrecompiles = dynPrecAddr

	// Combine module genesis states
	return network.CustomGenesisState{
		authtypes.ModuleName:  accGenesisState,
		erc20types.ModuleName: erc20GenesisState,
	}
}

// NewErc20GenesisState returns the default genesis state for the ERC20 module.
//
// NOTE: for the example chain implementation we are also adding a default token pair,
// which is the base denomination of the chain (i.e. the WEVMOS contract).
func NewErc20GenesisState() *erc20types.GenesisState {
	erc20GenState := erc20types.DefaultGenesisState()
	erc20GenState.TokenPairs = testconstants.ExampleTokenPairs
	erc20GenState.NativePrecompiles = []string{testconstants.WEVMOSContractMainnet}

	return erc20GenState
}
