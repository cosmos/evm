package bank

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	_ "embed"
)

// generated with solc 0.8.30+commit.73712a01:
//go:generate solc --overwrite --optimize --optimize-runs 100000 --via-ir --bin -o . ERC20.sol

var (
	//go:embed ERC20.bin
	ERC20BinHex string

	ERC20Bin              = common.Hex2Bytes(ERC20BinHex)
	ERC20Salt             = common.FromHex("636dd1d57837e7dce61901468217da9975548dcb3ecc24d84567feb93cd11e36")
	Create2FactoryAddress = common.HexToAddress("0x4e59b44847b379578588920ca78fbf26c0b4956c")
)

// ERC20ContractAddress computes the contract address deployed with create2 factory contract.
// create2 factory: https://github.com/Arachnid/deterministic-deployment-proxy
//
// `keccak(0xff || factory || salt || keccak(bytecode || ctor))[12:]`
func ERC20ContractAddress(contract common.Address, denom string) (common.Address, error) {
	ctor, err := NewErc20ctorCall(denom, contract).Encode()
	if err != nil {
		return common.Address{}, err
	}
	bz := crypto.Keccak256(
		[]byte{0xff},
		Create2FactoryAddress.Bytes(),
		ERC20Salt,
		crypto.Keccak256(
			ERC20Bin,
			ctor,
		),
	)[12:]
	return common.BytesToAddress(bz), nil
}
