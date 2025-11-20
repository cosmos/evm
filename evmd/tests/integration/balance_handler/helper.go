package balancehandler

import (
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	errorsmod "cosmossdk.io/errors"

	"github.com/cosmos/evm"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	testutiltypes "github.com/cosmos/evm/testutil/types"
)

// DeployContract deploys a contract to the test chain
func DeployContract(t *testing.T, chain *evmibctesting.TestChain, deploymentData testutiltypes.ContractDeploymentData) (common.Address, error) {
	t.Helper()

	// Get account's nonce to create contract hash
	from := common.BytesToAddress(chain.SenderPrivKey.PubKey().Address().Bytes())
	account := chain.App.(evm.EvmApp).GetEVMKeeper().GetAccount(chain.GetContext(), from)
	if account == nil {
		return common.Address{}, errors.New("account not found")
	}

	ctorArgs, err := deploymentData.Contract.ABI.Pack("", deploymentData.ConstructorArgs...)
	if err != nil {
		return common.Address{}, errorsmod.Wrap(err, "failed to pack constructor arguments")
	}

	data := deploymentData.Contract.Bin
	data = append(data, ctorArgs...)

	_, err = chain.App.(evm.EvmApp).GetEVMKeeper().CallEVMWithData(chain.GetContext(), from, nil, data, true, nil)
	if err != nil {
		return common.Address{}, errorsmod.Wrapf(err, "failed to deploy contract")
	}

	return crypto.CreateAddress(from, account.Nonce), nil
}
