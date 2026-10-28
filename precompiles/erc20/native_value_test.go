package erc20

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	cmn "github.com/cosmos/evm/precompiles/common"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func TestExecuteNativeValueRejection(t *testing.T) {
	p := Precompile{ABI: ABI}
	for _, amount := range []*big.Int{big.NewInt(1), new(big.Int).Lsh(big.NewInt(1), 128)} {
		t.Run(amount.String(), func(t *testing.T) {
			contract := vm.NewContract(common.Address{}, common.Address{}, uint256.MustFromBig(amount), 100_000, nil)
			// Invalid calldata also proves native value is rejected before ABI setup.
			contract.Input = []byte{0xff}
			output, err := p.Execute(sdk.Context{}, nil, contract, false)
			require.Nil(t, output)
			data, revertErr := cmn.ReturnRevertError(&vm.EVM{}, err)
			require.ErrorIs(t, revertErr, vm.ErrExecutionReverted)
			require.Equal(t, []byte{0xd0, 0x75, 0x77, 0x53}, data[:4])
			decoded, unpackErr := ABI.Errors[SolidityErrERC20CannotReceiveFunds].Inputs.Unpack(data[4:])
			require.NoError(t, unpackErr)
			require.Equal(t, amount, decoded[0])
		})
	}
	contract := vm.NewContract(common.Address{}, common.Address{}, uint256.NewInt(0), 100_000, nil)
	contract.Input = []byte{0xff}
	_, err := p.Execute(sdk.Context{}, nil, contract, false)
	require.Error(t, err)
	require.NotContains(t, err.Error(), SolidityErrERC20CannotReceiveFunds)
}
