package ibc

import (
	"errors"
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

	evmCtx := chain.GetContext()
	evmApp := chain.App.(evm.EvmApp)

	// Deploy new ERC20 contract with default metadata
	stateDB := statedb.New(chain.GetContext(), chain.App.(evm.EvmApp).GetEVMKeeper(), statedb.NewEmptyTxConfig())
	contractAddr, err := DeployERC20Contract(evmCtx, stateDB, evmApp.GetAccountKeeper(), evmApp.GetEVMKeeper(), banktypes.Metadata{
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
	_, err = evmApp.GetErc20Keeper().RegisterERC20(evmCtx, &erc20types.MsgRegisterERC20{
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

	stateDB = statedb.New(evmCtx, evmApp.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	_, err = evmApp.GetEVMKeeper().CallEVM(
		evmCtx,
		stateDB,
		contractAbi,
		erc20types.ModuleAddress,
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
	stateDB = statedb.New(evmCtx, evmApp.GetEVMKeeper(), statedb.NewEmptyTxConfig())
	ethRes, err := evmApp.GetEVMKeeper().CallEVM(
		evmCtx,
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

	return &NativeErc20Info{
		Denom:        nativeDenom,
		ContractAbi:  contractAbi,
		ContractAddr: contractAddr,
		Account:      common.BytesToAddress(senderAddr),
		InitialBal:   big.NewInt(sendAmt.Int64()),
	}
}

// SetupNativeErc20 deploys, registers, and mints a native ERC20 token on an EVM-based chain.
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

	stateDB := statedb.New(chain.GetContext(), chain.App.(evm.EvmApp).GetEVMKeeper(), statedb.NewEmptyTxConfig())

	_, err = chain.App.(evm.EvmApp).GetEVMKeeper().CallEVMWithData(chain.GetContext(), stateDB, from, nil, data, true, false, nil)
	if err != nil {
		return common.Address{}, errorsmod.Wrapf(err, "failed to deploy contract")
	}

	return crypto.CreateAddress(from, account.Nonce), nil
}

// DeployERC20Contract creates and deploys an ERC20 contract on the EVM with the
// erc20 module account as owner.
func DeployERC20Contract(
	ctx sdk.Context,
	stateDB *statedb.StateDB,
	accountKeeper erc20types.AccountKeeper,
	evmKeeper erc20types.EVMKeeper,
	coinMetadata banktypes.Metadata,
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

	nonce, err := accountKeeper.GetSequence(ctx, erc20types.ModuleAddress.Bytes())
	if err != nil {
		return common.Address{}, err
	}

	contractAddr := crypto.CreateAddress(erc20types.ModuleAddress, nonce)
	_, err = evmKeeper.CallEVMWithData(ctx, stateDB, erc20types.ModuleAddress, nil, data, true, false, nil)
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
