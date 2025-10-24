package keeper

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/ibc"
	callbacksabi "github.com/cosmos/evm/precompiles/callbacks"
	"github.com/cosmos/evm/utils"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/ibc/callbacks/types"
	evmante "github.com/cosmos/evm/x/vm/ante"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	callbacktypes "github.com/cosmos/ibc-go/v10/modules/apps/callbacks/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"

	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ContractKeeper implements callbacktypes.ContractKeeper
var _ callbacktypes.ContractKeeper = (*ContractKeeper)(nil)

type ContractKeeper struct {
	authKeeper            types.AccountKeeper
	evmKeeper             types.EVMKeeper
	erc20Keeper           types.ERC20Keeper
	packetDataUnmarshaler porttypes.PacketDataUnmarshaler
}

// NewKeeper creates and initializes a new ContractKeeper instance.
//
// The ContractKeeper manages cross-chain contract execution and handles IBC packet
// callbacks for smart contract interactions.
func NewKeeper(authKeeper types.AccountKeeper, evmKeeper types.EVMKeeper, erc20Keeper types.ERC20Keeper) ContractKeeper {
	ck := ContractKeeper{
		authKeeper:  authKeeper,
		evmKeeper:   evmKeeper,
		erc20Keeper: erc20Keeper,
	}
	ck.packetDataUnmarshaler = types.Unmarshaler{}
	return ck
}

// IBCSendPacketCallback handles IBC packet send callbacks for cross-chain operations.
//
// IMPORTANT: This callback is currently not supported and always returns nil.
// The rationale is that contracts can implement custom logic before the send packet
// operation is called, making this callback redundant or potentially conflicting
// with contract-defined behavior.
func (k ContractKeeper) IBCSendPacketCallback(
	cachedCtx sdk.Context,
	sourcePort string,
	sourceChannel string,
	timeoutHeight clienttypes.Height,
	timeoutTimestamp uint64,
	packetData []byte,
	contractAddress,
	packetSenderAddress string,
	version string,
) error {
	return nil
}

// IBCReceivePacketCallback handles IBC packet callbacks for cross-chain contract execution.
// This function processes incoming IBC packets that contain callback data and executes
// the specified contract with the transferred tokens.
//
// The function performs the following operations:
// 1. Unmarshals and validates the IBC packet data
// 2. Extracts callback data from the packet
// 3. Generates an isolated address for security
// 4. Validates the receiver address matches the isolated address
// 5. Verifies the target contract exists and contains code
// 6. Sets up ERC20 token allowance for the contract
// 7. Executes the callback function on the target contract
// 8. Validates that all tokens were successfully transferred to the contract
//
// Returns:
//   - error: Returns nil on success, or an error if any step fails including:
//   - Packet data unmarshaling errors
//   - Invalid callback data
//   - Address validation failures
//   - Contract validation failures (non-existent or no code)
//   - Token pair registration errors
//   - EVM execution errors
//   - Gas limit exceeded errors
//   - Token transfer validation failures
//
// Security Notes:
//   - Uses isolated addresses to prevent unauthorized access
//   - Validates contract existence to prevent fund loss
//   - Enforces gas limits to prevent DoS attacks
//   - Requires contracts to implement proper token transfer logic
//   - Validates final token balances to ensure successful transfers
func (k ContractKeeper) IBCReceivePacketCallback(
	ctx sdk.Context,
	packet ibcexported.PacketI,
	ack ibcexported.Acknowledgement,
	contractAddress string,
	version string,
) error {
	data, err := transfertypes.UnmarshalPacketData(packet.GetData(), version, "")
	if err != nil {
		return err
	}

	cbData, isCbPacket, err := callbacktypes.GetCallbackData(data, version, packet.GetDestPort(), ctx.GasMeter().GasRemaining(), ctx.GasMeter().GasRemaining(), callbacktypes.DestinationCallbackKey)
	if err != nil {
		return err
	}
	if !isCbPacket {
		return nil
	}

	// `ProcessCallback` in IBC-Go overrides the infinite gas meter with a basic gas meter,
	// so we need to generate a new infinite gas meter to run the EVM executions on.
	// Skipping this causes the EVM gas estimation function to deplete all Cosmos gas.
	// We re-add the actual EVM call gas used to the original context after the call is complete
	// with the gas retrieved from the EVM message result.
	cachedCtx, writeFn := ctx.CacheContext()
	cachedCtx = evmante.BuildEvmExecutionCtx(cachedCtx).
		WithGasMeter(evmtypes.NewInfiniteGasMeterWithLimit(cbData.CommitGasLimit))

	// receiver := sdk.MustAccAddressFromBech32(data.Receiver)
	receiver, err := sdk.AccAddressFromBech32(data.Receiver)
	if err != nil {
		return errorsmod.Wrapf(types.ErrInvalidReceiverAddress,
			"acc addr from bech32 conversion failed for receiver address: %s", data.Receiver)
	}
	receiverHex, err := utils.HexAddressFromBech32String(receiver.String())
	if err != nil {
		return errorsmod.Wrapf(types.ErrInvalidReceiverAddress,
			"hex address conversion failed for receiver address: %s", receiver)
	}

	// Generate secure isolated address from sender.
	isolatedAddr := types.GenerateIsolatedAddress(packet.GetDestChannel(), data.Sender)
	isolatedAddrHex := common.BytesToAddress(isolatedAddr.Bytes())

	acc := k.authKeeper.NewAccountWithAddress(ctx, receiver)
	k.authKeeper.SetAccount(ctx, acc)

	// Ensure receiver address is equal to the isolated address.
	if receiverHex.Cmp(isolatedAddrHex) != 0 {
		return errorsmod.Wrapf(types.ErrInvalidReceiverAddress, "expected %s, got %s", isolatedAddrHex.String(), receiverHex.String())
	}

	contractAddr := common.HexToAddress(contractAddress)

	// Check if the contract address contains code.
	// This check is required because if there is no code, the call will still pass on the EVM side,
	// but it will ignore the calldata and funds may get stuck.
	if !k.evmKeeper.IsContract(ctx, contractAddr) {
		return errorsmod.Wrapf(types.ErrContractHasNoCode, "provided contract address is not a contract: %s", contractAddr)
	}

	// Check if the token pair exists and get the ERC20 contract address
	// for the native ERC20 or the precompile.
	// This call fails if the token does not exist or is not registered.
	token := transfertypes.Token{
		Denom:  data.Token.Denom,
		Amount: data.Token.Amount,
	}
	coin := ibc.GetReceivedCoin(packet.(channeltypes.Packet), token)

	tokenPairID := k.erc20Keeper.GetTokenPairID(ctx, coin.Denom)
	tokenPair, found := k.erc20Keeper.GetTokenPair(ctx, tokenPairID)
	if !found {
		return errorsmod.Wrapf(types.ErrTokenPairNotFound, "token pair for denom %s not found", data.Token.Denom.IBCDenom())
	}
	amountInt, ok := math.NewIntFromString(data.Token.Amount)
	if !ok {
		return errorsmod.Wrapf(types.ErrNumberOverflow, "amount overflow")
	}

	erc20 := contracts.ERC20MinterBurnerDecimalsContract

	remainingGas := math.NewIntFromUint64(cachedCtx.GasMeter().GasRemaining()).BigInt()

	// Call the EVM with the remaining gas as the maximum gas limit.
	// Up to now, the remaining gas is equal to the callback gas limit set by the user.
	// NOTE: use the cached ctx for the EVM calls.
	res, err := k.evmKeeper.CallEVM(cachedCtx, erc20.ABI, receiverHex, tokenPair.GetERC20Contract(), true, remainingGas, "approve", contractAddr, amountInt.BigInt())
	if err != nil {
		return errorsmod.Wrapf(types.ErrAllowanceFailed, "failed to set allowance: %v", err)
	}

	// Consume the actual used gas on the original callback context.
	ctx.GasMeter().ConsumeGas(res.GasUsed, "callback allowance")
	remainingGas = remainingGas.Sub(remainingGas, math.NewIntFromUint64(res.GasUsed).BigInt())
	if ctx.GasMeter().IsOutOfGas() || remainingGas.Cmp(big.NewInt(0)) < 0 {
		return errorsmod.Wrapf(types.ErrOutOfGas, "out of gas")
	}

	var approveSuccess bool
	err = erc20.ABI.UnpackIntoInterface(&approveSuccess, "approve", res.Ret)
	if err != nil {
		return errorsmod.Wrapf(types.ErrAllowanceFailed, "failed to unpack approve return: %v", err)
	}

	if !approveSuccess {
		return errorsmod.Wrapf(types.ErrAllowanceFailed, "failed to set allowance")
	}

	// NOTE: use the cached ctx for the EVM calls.
	res, err = k.evmKeeper.CallEVMWithData(cachedCtx, receiverHex, &contractAddr, cbData.Calldata, true, remainingGas)
	if err != nil {
		return errorsmod.Wrapf(types.ErrEVMCallFailed, "EVM returned error: %s", err.Error())
	}

	// Consume the actual gas used on the original callback context.
	ctx.GasMeter().ConsumeGas(res.GasUsed, "callback function")
	if ctx.GasMeter().IsOutOfGas() {
		return errorsmod.Wrapf(types.ErrOutOfGas, "out of gas")
	}

	// Write cachedCtx events back to ctx.
	writeFn()

	// Check that the sender no longer has tokens after the callback.
	// NOTE: contracts must implement an IERC20(token).transferFrom(msg.sender, address(this), amount)
	// for the total amount, or the callback will fail.
	// This check is here to prevent funds from getting stuck in the isolated address,
	// since they would become irretrievable.
	receiverTokenBalance := k.erc20Keeper.BalanceOf(ctx, erc20.ABI, tokenPair.GetERC20Contract(), receiverHex) // here,
	// we can use the original ctx and skip manually adding the gas
	if receiverTokenBalance.Cmp(big.NewInt(0)) != 0 {
		return errorsmod.Wrapf(erc20types.ErrEVMCall,
			"receiver has %d unrecoverable tokens after callback", receiverTokenBalance)
	}

	return nil
}

// IBCOnAcknowledgementPacketCallback handles IBC packet acknowledgement callbacks for cross-chain contract execution.
// This function is triggered when an IBC packet receives an acknowledgement from the destination chain,
// allowing contracts to react to successful or failed packet delivery.
//
// The function performs the following operations:
// 1. Unmarshals and validates the IBC packet data
// 2. Extracts callback data from the packet (source-side callback)
// 3. Validates that no calldata is present (acknowledgement callbacks should not contain calldata)
// 4. Verifies the target contract exists and contains code
// 5. Calls the contract's onPacketAcknowledgement function with packet details
// 6. Manages gas consumption and validates gas limits
//
// Returns:
//   - error: Returns nil on success, or an error if any step fails including:
//   - Packet data unmarshaling errors
//   - Invalid callback data or unexpected calldata presence
//   - Address parsing failures
//   - Contract validation failures (non-existent or no code)
//   - ABI loading errors
//   - EVM execution errors
//   - Gas limit exceeded errors
//
// Contract Requirements:
//   - Must implement onPacketAcknowledgement(string calldata sourceChannel, string calldata sourcePort,
//     uint64 sequence, bytes calldata data, bytes calldata acknowledgement) function
//   - Should handle both successful and failed acknowledgements appropriately
func (k ContractKeeper) IBCOnAcknowledgementPacketCallback(
	ctx sdk.Context,
	packet channeltypes.Packet,
	acknowledgement []byte,
	relayer sdk.AccAddress,
	contractAddress,
	packetSenderAddress string,
	version string,
) error {
	data, err := transfertypes.UnmarshalPacketData(packet.GetData(), version, "")
	if err != nil {
		return err
	}

	cbData, isCbPacket, err := callbacktypes.GetCallbackData(data, version, packet.GetDestPort(), ctx.GasMeter().GasRemaining(), ctx.GasMeter().GasRemaining(), callbacktypes.SourceCallbackKey)
	if err != nil {
		return err
	}
	if !isCbPacket {
		return nil
	}

	// `ProcessCallback` in IBC-Go overrides the infinite gas meter with a basic gas meter,
	// so we need to generate a new infinite gas meter to run the EVM executions on.
	// Skipping this causes the EVM gas estimation function to deplete all Cosmos gas.
	// We re-add the actual EVM call gas used to the original context after the call is complete
	// with the gas retrieved from the EVM message result.
	cachedCtx, writeFn := ctx.CacheContext()
	cachedCtx = evmante.BuildEvmExecutionCtx(cachedCtx).
		WithGasMeter(evmtypes.NewInfiniteGasMeterWithLimit(cbData.CommitGasLimit))

	if len(cbData.Calldata) != 0 {
		return errorsmod.Wrap(types.ErrInvalidCalldata, "acknowledgement callback data should not contain calldata")
	}

	sender, err := utils.HexAddressFromBech32String(packetSenderAddress)
	if err != nil {
		return errorsmod.Wrapf(err, "unable to parse packet sender address %s", packetSenderAddress)
	}

	contractAddr := common.HexToAddress(contractAddress)

	// Check if the contract address contains code.
	// This check is required because if there is no code, the call will still pass on the EVM side,
	// but it will ignore the calldata and funds may get stuck.
	if !k.evmKeeper.IsContract(ctx, contractAddr) {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "provided contract address is not a contract: %s", contractAddr)
	}

	// Call the onPacketAcknowledgement function in the contract
	// NOTE: use the cached ctx for the EVM calls.
	res, err := k.evmKeeper.CallEVM(cachedCtx, callbacksabi.ABI, sender, contractAddr, true, math.NewIntFromUint64(cachedCtx.GasMeter().GasRemaining()).BigInt(), "onPacketAcknowledgement",
		packet.GetSourceChannel(), packet.GetSourcePort(), packet.GetSequence(), packet.GetData(), acknowledgement)
	if err != nil {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "EVM returned error: %s", err.Error())
	}

	// Consume the actual gas used on the original callback context.
	ctx.GasMeter().ConsumeGas(res.GasUsed, "callback onPacketAcknowledgement")
	if ctx.GasMeter().IsOutOfGas() {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "out of gas")
	}

	writeFn()

	return nil
}

// IBCOnTimeoutPacketCallback handles IBC packet timeout callbacks for cross-chain contract execution.
// This function is triggered when an IBC packet times out without receiving an acknowledgement,
// allowing contracts to handle timeout scenarios and perform cleanup or rollback operations.
//
// The function performs the following operations:
// 1. Unmarshals and validates the IBC packet data
// 2. Extracts callback data from the packet (source-side callback)
// 3. Validates that no calldata is present (timeout callbacks should not contain calldata)
// 4. Sets up a cached context with proper gas metering for EVM execution
// 5. Verifies the target contract exists and contains code
// 6. Calls the contract's onPacketTimeout function with packet details
// 7. Manages gas consumption and validates gas limits
// 8. Commits the cached context changes back to the original context
//
// Returns:
//   - error: Returns nil on success, or an error if any step fails including:
//   - Packet data unmarshaling errors
//   - Invalid callback data or unexpected calldata presence
//   - Address parsing failures
//   - Contract validation failures (non-existent or no code)
//   - ABI loading errors
//   - EVM execution errors
//   - Gas limit exceeded errors
//
// Contract Requirements:
//   - Must implement onPacketTimeout(string calldata sourceChannel, string calldata sourcePort,
//     uint64 sequence, bytes calldata data) function
//   - Should handle timeout scenarios appropriately (e.g., refunds, state rollbacks)
func (k ContractKeeper) IBCOnTimeoutPacketCallback(
	ctx sdk.Context,
	packet channeltypes.Packet,
	relayer sdk.AccAddress,
	contractAddress,
	packetSenderAddress string,
	version string,
) error {
	data, err := transfertypes.UnmarshalPacketData(packet.GetData(), version, "")
	if err != nil {
		return err
	}

	cbData, isCbPacket, err := callbacktypes.GetCallbackData(data, version, packet.GetDestPort(), ctx.GasMeter().GasRemaining(), ctx.GasMeter().GasRemaining(), callbacktypes.SourceCallbackKey)
	if err != nil {
		return err
	}
	if !isCbPacket {
		return nil
	}

	// `ProcessCallback` in IBC-Go overrides the infinite gas meter with a basic gas meter,
	// so we need to generate a new infinite gas meter to run the EVM executions on.
	// Skipping this causes the EVM gas estimation function to deplete all Cosmos gas.
	// We re-add the actual EVM call gas used to the original context after the call is complete
	// with the gas retrieved from the EVM message result.
	cachedCtx, writeFn := ctx.CacheContext()
	cachedCtx = evmante.BuildEvmExecutionCtx(cachedCtx).
		WithGasMeter(evmtypes.NewInfiniteGasMeterWithLimit(cbData.CommitGasLimit))

	if len(cbData.Calldata) != 0 {
		return errorsmod.Wrap(types.ErrInvalidCalldata, "timeout callback data should not contain calldata")
	}

	senderAccount, err := sdk.AccAddressFromBech32(packetSenderAddress)
	if err != nil {
		return errorsmod.Wrapf(err, "unable to parse packet sender address %s", packetSenderAddress)
	}
	sender := common.BytesToAddress(senderAccount.Bytes())
	contractAddr := common.HexToAddress(contractAddress)

	// Check if the contract address contains code.
	// This check is required because if there is no code, the call will still pass on the EVM side,
	// but it will ignore the calldata and funds may get stuck.
	if !k.evmKeeper.IsContract(ctx, contractAddr) {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "provided contract address is not a contract: %s", contractAddr)
	}

	res, err := k.evmKeeper.CallEVM(ctx, callbacksabi.ABI, sender, contractAddr, true, math.NewIntFromUint64(cachedCtx.GasMeter().GasRemaining()).BigInt(), "onPacketTimeout",
		packet.GetSourceChannel(), packet.GetSourcePort(), packet.GetSequence(), packet.GetData())
	if err != nil {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "EVM returned error: %s", err.Error())
	}

	// Consume the actual gas used on the original callback context.
	ctx.GasMeter().ConsumeGas(res.GasUsed, "callback onPacketAcknowledgement")
	if ctx.GasMeter().IsOutOfGas() {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "out of gas")
	}

	writeFn()
	return nil
}
