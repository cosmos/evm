package ics02

import (
	"embed"

	"github.com/ethereum/go-ethereum/accounts/abi"

	ibcutils "github.com/cosmos/evm/ibc"
	cmn "github.com/cosmos/evm/precompiles/common"
)

const (
	// abiPath defines the path to the LightClient precompile ABI JSON file.
	abiPath = "abi.json"

	// TODO: These gas values are placeholders and should be determined through proper benchmarking.

	GasUpdateClient        = 40_000
	GasVerifyMembership    = 15_000
	GasVerifyNonMembership = 15_000
	GasMisbehaviour        = 50_000
	GasGetClientState      = 4_000
)

var (
	// Embed abi json file to the executable binary. Needed when importing as dependency.
	//
	//go:embed abi.json
	f   embed.FS
	ABI abi.ABI
)

func init() {
	var err error
	ABI, err = cmn.LoadABI(f, abiPath)
	if err != nil {
		panic(err)
	}
}

// LoadABI loads the IERC20Metadata ABI from the embedded abi.json file
// for the erc20 precompile.
func LoadABI() (abi.ABI, error) {
	return cmn.LoadABI(f, abiPath)
}

// Precompile defines the precompiled contract for ICS02.
type Precompile struct {
	cmn.Precompile

	abi.ABI
	clientKeeper ibcutils.ClientKeeper
}
