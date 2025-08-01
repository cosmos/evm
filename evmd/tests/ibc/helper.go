package ibc

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/evmd"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	errorsmod "cosmossdk.io/errors"

	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

// NativeErc20Info holds details about a deployed ERC20 token.
type NativeErc20Info struct {
	Denom        string
	ContractAbi  abi.ABI
	ContractAddr common.Address
	Account      common.Address // The address of the minter on the EVM chain
	InitialBal   *big.Int
}

// SetupNativeErc20 deploys, registers, and mints a native ERC20 token on an EVM-based chain.
func SetupNativeErc20(t *testing.T, chain *evmibctesting.TestChain) *NativeErc20Info {
	t.Helper()

	evmCtx := chain.GetContext()
	evmApp := chain.App.(*evmd.EVMD)

	// Deploy new ERC20 contract with default metadata
	contractAddr, err := evmApp.Erc20Keeper.DeployERC20Contract(evmCtx, banktypes.Metadata{
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: "example", Exponent: 18},
		},
		Name:   "Example",
		Symbol: "Ex",
	})
	if err != nil {
		t.Fatalf("ERC20 deployment failed: %v", err)
	}
	chain.NextBlock()

	// Register the contract
	_, err = evmApp.Erc20Keeper.RegisterERC20(evmCtx, &erc20types.MsgRegisterERC20{
		Signer:         authtypes.NewModuleAddress(govtypes.ModuleName).String(), // does not have to be gov
		Erc20Addresses: []string{contractAddr.Hex()},
	})
	if err != nil {
		t.Fatalf("RegisterERC20 failed: %v", err)
	}

	// Mint tokens to default sender
	contractAbi := contracts.ERC20MinterBurnerDecimalsContract.ABI
	nativeDenom := erc20types.CreateDenom(contractAddr.String())
	sendAmt := ibctesting.DefaultCoinAmount
	senderAcc := chain.SenderAccount.GetAddress()

	_, err = evmApp.EVMKeeper.CallEVM(
		evmCtx,
		contractAbi,
		erc20types.ModuleAddress,
		contractAddr,
		true,
		nil,
		"mint",
		common.BytesToAddress(senderAcc),
		big.NewInt(sendAmt.Int64()),
	)
	if err != nil {
		t.Fatalf("mint call failed: %v", err)
	}

	// Verify minted balance
	bal := evmApp.Erc20Keeper.BalanceOf(evmCtx, contractAbi, contractAddr, common.BytesToAddress(senderAcc))
	if bal.Cmp(big.NewInt(sendAmt.Int64())) != 0 {
		t.Fatalf("unexpected ERC20 balance; got %s, want %s", bal.String(), sendAmt.String())
	}

	return &NativeErc20Info{
		Denom:        nativeDenom,
		ContractAbi:  contractAbi,
		ContractAddr: contractAddr,
		Account:      common.BytesToAddress(senderAcc),
		InitialBal:   big.NewInt(sendAmt.Int64()),
	}
}

// SetupNativeErc20 deploys, registers, and mints a native ERC20 token on an EVM-based chain.
func DeployContract(t *testing.T, chain *evmibctesting.TestChain, deploymentData testutiltypes.ContractDeploymentData) (common.Address, error) {
	t.Helper()

	// Get account's nonce to create contract hash
	from := common.BytesToAddress(chain.SenderPrivKey.PubKey().Address().Bytes())
	account := chain.App.(*evmd.EVMD).EVMKeeper.GetAccount(chain.GetContext(), from)
	if account == nil {
		return common.Address{}, errors.New("account not found")
	}

	ctorArgs, err := deploymentData.Contract.ABI.Pack("", deploymentData.ConstructorArgs...)
	if err != nil {
		return common.Address{}, errorsmod.Wrap(err, "failed to pack constructor arguments")
	}

	data := deploymentData.Contract.Bin
	data = append(data, ctorArgs...)

	_, err = chain.App.(*evmd.EVMD).EVMKeeper.CallEVMWithData(chain.GetContext(), from, nil, data, true, nil)
	if err != nil {
		return common.Address{}, errorsmod.Wrapf(err, "failed to deploy contract")
	}

	return crypto.CreateAddress(from, account.Nonce), nil
}
