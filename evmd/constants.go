package evmd

import (
	erc20types "github.com/cosmos/evm/x/erc20/types"
)

// Chain denomination constants for EpixChain
const (
	// BaseDenom is the base denomination for the EpixChain (atto-epix)
	BaseDenom = "aepix"

	// DisplayDenom is the display denomination for the EpixChain
	DisplayDenom = "epix"

	// Decimals is the number of decimals for the display denomination
	Decimals = 18

	// WEPIXContract is the WEPIX contract address
	WEPIXContract = "0xD4949664cD82660AaE99bEdc034a0deA8A0bd517"
)

var (
	// TokenPairs creates a slice of token pairs for the native denom of EpixChain
	TokenPairs = []erc20types.TokenPair{
		{
			Erc20Address:  WEPIXContract,
			Denom:         BaseDenom,
			Enabled:       true,
			ContractOwner: erc20types.OWNER_MODULE,
		},
	}
)

