package callbacks

import (
	"bytes"

	"github.com/ethereum/go-ethereum/accounts/abi"

	_ "embed"
)

// Embed abi json file to the executable binary. Needed when importing as dependency.
var (
	//go:embed abi.json
	f   []byte
	ABI abi.ABI
)

func init() {
	var err error
	ABI, err = abi.JSON(bytes.NewReader(f))
	if err != nil {
		panic(err)
	}
}
