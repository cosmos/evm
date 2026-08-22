package common

import (
	"fmt"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"

	_ "embed"
)

//go:embed interfaces/abi.json
var sharedErrorABIJSON string

var SharedErrorABI abi.ABI

func init() {
	var err error
	SharedErrorABI, err = abi.JSON(strings.NewReader(sharedErrorABIJSON))
	if err != nil {
		panic(fmt.Errorf("parse shared precompile ABI: %w", err))
	}
	reviewedGRPCErrorRegistry = MustNewGRPCErrorRegistry(reviewedGRPCErrorDispositionDeclarations)
}
