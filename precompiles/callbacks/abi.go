package callbacks

import (
	"embed"

	cmn "github.com/cosmos/evm/precompiles/common"

	"github.com/ethereum/go-ethereum/accounts/abi"
)

// Embed abi json file to the executable binary. Needed when importing as dependency.
//
//go:embed abi.json
var f embed.FS

func LoadABI() (*abi.ABI, error) {
	newABI, err := cmn.LoadABI(f, "ICallbacks.json")
	if err != nil {
		return nil, err
	}

	return &newABI, nil
}
