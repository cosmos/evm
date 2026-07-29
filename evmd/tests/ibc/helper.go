package ibc

import (
	"fmt"
	"math/big"
	"testing"

	abci "github.com/cometbft/cometbft/abci/types"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	"github.com/cosmos/evm"
	"github.com/cosmos/evm/contracts"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"
	ibctesting "github.com/cosmos/ibc-go/v11/testing"

	errorsmod "cosmossdk.io/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

// Event count constants for test assertions
const (
	DelegationEventCount          = 3  // Staking delegation events
	EVMEventCount                 = 5  // EVM transaction wrapper events
	WithdrawalNoTokensEventCount  = 1  // Withdrawal with no tokens
	ICS20WithConversionEventCount = 15 // ERC20 conversion (7) + IBC packet (8)
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
func SetupNativeErc20(t *testing.T, chain *evmibctesting.TestChain, senderAcc evmibctesting.SenderAccount) *NativeErc20Info {
	t.Helper()

	evmApp := chain.App.(evm.EvmApp)
	ctx := chain.GetContext()

	// Deploy new ERC20 contract with default metadata.
	// The deployer is a dedicated EOA.
	deployer := common.BytesToAddress([]byte("erc20-test-deployer"))
	deployerAccAddr := sdk.AccAddress(deployer.Bytes())
	if evmApp.GetAccountKeeper().GetAccount(ctx, deployerAccAddr) == nil {
		evmApp.GetAccountKeeper().SetAccount(ctx, evmApp.GetAccountKeeper().NewAccountWithAddress(ctx, deployerAccAddr))
	}

	stateDB := statedb.New(ctx, evmApp.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	contractAddr, err := DeployERC20Contract(ctx, stateDB, evmApp.GetAccountKeeper(), evmApp.GetEVMKeeper(), banktypes.Metadata{
		DenomUnits: []*banktypes.DenomUnit{
			{Denom: "example", Exponent: 18},
		},
		Name:   "Example",
		Symbol: "Ex",
	}, deployer)
	if err != nil {
		t.Fatalf("ERC20 deployment failed: %v", err)
	}

	// Register the contract
	_, err = evmApp.GetErc20Keeper().RegisterERC20(ctx, &erc20types.MsgRegisterERC20{
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
	senderAddr := senderAcc.SenderAccount.GetAddress()

	stateDB = statedb.New(ctx, evmApp.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmApp.GetEVMKeeper().CallEVM(
		ctx,
		stateDB,
		contractAbi,
		deployer,
		contractAddr,
		true,
		false,
		nil,
		"mint",
		common.BytesToAddress(senderAddr),
		big.NewInt(sendAmt.Int64()),
	)
	if err != nil {
		t.Fatalf("mint call failed: %v", err)
	}

	// Verify minted balance
	stateDB = statedb.New(ctx, evmApp.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	ethRes, err := evmApp.GetEVMKeeper().CallEVM(
		ctx,
		stateDB,
		contractAbi,
		common.BytesToAddress(senderAddr),
		contractAddr,
		false,
		false,
		nil,
		"balanceOf",
		common.BytesToAddress(senderAddr),
	)
	if err != nil {
		t.Fatalf("balanceOf call failed: %v", err)
	}
	var bal *big.Int
	err = contractAbi.UnpackIntoInterface(&bal, "balanceOf", ethRes.Ret)
	if err != nil {
		t.Fatalf("balanceOf call failed: %v", err)
	}
	if bal.Cmp(big.NewInt(sendAmt.Int64())) != 0 {
		t.Fatalf("unexpected ERC20 balance; got %s, want %s", bal.String(), sendAmt.String())
	}
	chain.NextBlock()

	return &NativeErc20Info{
		Denom:        nativeDenom,
		ContractAbi:  contractAbi,
		ContractAddr: contractAddr,
		Account:      common.BytesToAddress(senderAddr),
		InitialBal:   big.NewInt(sendAmt.Int64()),
	}
}

// DeployContract deploys an arbitrary contract on an EVM-based chain.
// Like DeployERC20Contract, the sender is an EOA rather than a module account.
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

// DeployERC20Contract creates and deploys an ERC20 contract on the EVM with
// deployer as owner.
//
// deployer must be an EOA. Do not pass a module account.
// It is also required in practice. Contract creation bumps the sender's nonce,
// SetAccount persists nonce and balance together, and the EVM commit path is
// not allowed to write a module account's balance.
func DeployERC20Contract(
	ctx sdk.Context,
	stateDB *statedb.StateDB,
	accountKeeper erc20types.AccountKeeper,
	evmKeeper erc20types.EVMKeeper,
	coinMetadata banktypes.Metadata,
	deployer common.Address,
) (common.Address, error) {
	decimals := uint8(0)
	if len(coinMetadata.DenomUnits) > 0 {
		decimalsIdx := len(coinMetadata.DenomUnits) - 1
		decimals = uint8(coinMetadata.DenomUnits[decimalsIdx].Exponent) //#nosec G115 // exponent will not exceed uint8
	}
	ctorArgs, err := contracts.ERC20MinterBurnerDecimalsContract.ABI.Pack(
		"",
		coinMetadata.Name,
		coinMetadata.Symbol,
		decimals,
	)
	if err != nil {
		return common.Address{}, errorsmod.Wrapf(types.ErrABIPack, "coin metadata is invalid %s: %s", coinMetadata.Name, err.Error())
	}

	data := make([]byte, len(contracts.ERC20MinterBurnerDecimalsContract.Bin)+len(ctorArgs))
	copy(data[:len(contracts.ERC20MinterBurnerDecimalsContract.Bin)], contracts.ERC20MinterBurnerDecimalsContract.Bin)
	copy(data[len(contracts.ERC20MinterBurnerDecimalsContract.Bin):], ctorArgs)

	nonce, err := accountKeeper.GetSequence(ctx, deployer.Bytes())
	if err != nil {
		return common.Address{}, err
	}

	contractAddr := crypto.CreateAddress(deployer, nonce)
	_, err = evmKeeper.CallEVMWithData(ctx, stateDB, deployer, nil, data, true, false, nil)
	if err != nil {
		return common.Address{}, errorsmod.Wrapf(err, "failed to deploy contract for %s", coinMetadata.Name)
	}

	return contractAddr, nil
}

// PrintEvents prints all events with their attributes for debugging
func PrintEvents(label string, events []abci.Event) {
	fmt.Printf("\n========== Events for %s ==========\n", label)
	fmt.Printf("Total Event Count: %d\n\n", len(events))

	for i, event := range events {
		fmt.Printf("[%d] Type: %s\n", i, event.Type)
		for _, attr := range event.Attributes {
			fmt.Printf("    %s: %s\n", attr.Key, attr.Value)
		}
		fmt.Println()
	}
	fmt.Printf("========================================\n\n")
}
