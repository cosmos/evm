package balancehandler

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	"github.com/cosmos/evm/x/vm/statedb"

	errorsmod "cosmossdk.io/errors"
)

// DeployContract deploys a contract to the test chain
func DeployContract(t *testing.T, chain *evmibctesting.TestChain, deploymentData testutiltypes.ContractDeploymentData) (common.Address, error) {
	t.Helper()

	// Keep address derivation aligned with CallEVMWithData, which uses account sequence as nonce.
	from := common.BytesToAddress(chain.SenderPrivKey.PubKey().Address().Bytes())
	ctx := chain.GetContext()
	nonce, err := chain.App.(evm.EvmApp).GetAccountKeeper().GetSequence(ctx, from.Bytes())
	if err != nil {
		return common.Address{}, errorsmod.Wrap(err, "failed to get account sequence")
	}

	ctorArgs, err := deploymentData.Contract.ABI.Pack("", deploymentData.ConstructorArgs...)
	if err != nil {
		return common.Address{}, errorsmod.Wrap(err, "failed to pack constructor arguments")
	}

	data := deploymentData.Contract.Bin
	data = append(data, ctorArgs...)
	stateDB := statedb.New(ctx, chain.App.(evm.EvmApp).GetEVMKeeper(), statedb.NewEmptyTxConfig())

	_, err = chain.App.(evm.EvmApp).GetEVMKeeper().CallEVMWithData(ctx, stateDB, from, nil, data, true, false, nil)
	if err != nil {
		return common.Address{}, errorsmod.Wrapf(err, "failed to deploy contract")
	}

	return crypto.CreateAddress(from, nonce), nil
}
