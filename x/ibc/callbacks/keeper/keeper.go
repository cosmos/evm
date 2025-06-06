package keeper

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/ibc"
	callbacksabi "github.com/cosmos/evm/precompiles/callbacks"
	types2 "github.com/cosmos/evm/types"
	"github.com/cosmos/evm/utils"
	erc20types "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/ibc/callbacks/types"
	evmante "github.com/cosmos/evm/x/vm/ante"
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

func NewKeeper(authKeeper types.AccountKeeper, evmKeeper types.EVMKeeper, erc20Keeper types.ERC20Keeper) ContractKeeper {
	ck := ContractKeeper{
		authKeeper:  authKeeper,
		evmKeeper:   evmKeeper,
		erc20Keeper: erc20Keeper,
	}
	ck.packetDataUnmarshaler = types.Unmarshaler{}
	return ck
}

// SendPacket callback will not supported since the contract can run custom logic before send packet is called.
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

	cachedCtx := evmante.BuildEvmExecutionCtx(ctx).
		WithGasMeter(types2.NewInfiniteGasMeterWithLimit(cbData.CommitGasLimit))

	// can only call callback if the receiver is the isolated address for the packet sender on this chain
	receiver := sdk.MustAccAddressFromBech32(data.Receiver)
	receiverHex, err := utils.HexAddressFromBech32String(receiver.String())
	if err != nil {
		return errorsmod.Wrapf(err, "address conversion failed for receiver address: %s", receiver)
	}

	isolatedAddr := types.GenerateIsolatedAddress(packet.GetDestChannel(), data.Sender)
	isolatedAddrHex := common.BytesToAddress(isolatedAddr.Bytes())

	acc := k.authKeeper.NewAccountWithAddress(cachedCtx, receiver)
	k.authKeeper.SetAccount(cachedCtx, acc)

	if receiverHex.Cmp(isolatedAddrHex) != 0 {
		return errorsmod.Wrapf(types.ErrInvalidReceiverAddress, "expected %s, got %s", isolatedAddrHex.String(), receiverHex.String())
	}

	contractAddr := common.HexToAddress(contractAddress)
	contractAccount := k.evmKeeper.GetAccountOrEmpty(cachedCtx, contractAddr)
	// this check is required because if there is no code, the call will still pass on the EVM side, but it will ignore the calldata
	// and funds may get stuck
	if !contractAccount.IsContract() {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "provided contract address is not a contract: %s", contractAddr)
	}

	token := transfertypes.Token{
		Denom:  data.Token.Denom,
		Amount: data.Token.Amount,
	}
	coin := ibc.GetReceivedCoin(packet.(channeltypes.Packet), token)

	tokenPairID := k.erc20Keeper.GetTokenPairID(cachedCtx, coin.Denom)
	tokenPair, found := k.erc20Keeper.GetTokenPair(cachedCtx, tokenPairID)
	if !found {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "token pair for denom %s not found", data.Token.Denom.IBCDenom())
	}
	amountInt, ok := math.NewIntFromString(data.Token.Amount)
	if !ok {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "amount overflow")
	}

	erc20 := contracts.ERC20MinterBurnerDecimalsContract

	remainingGas := math.NewIntFromUint64(cachedCtx.GasMeter().GasRemaining()).BigInt()

	res, err := k.evmKeeper.CallEVM(cachedCtx, erc20.ABI, receiverHex, tokenPair.GetERC20Contract(), true, remainingGas, "approve", contractAddr, amountInt.BigInt())
	if err != nil {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "failed to set allowance: %v", err)
	}

	ctx.GasMeter().ConsumeGas(res.GasUsed, "callback allowance")

	var allowance *big.Int
	err = erc20.ABI.UnpackIntoInterface(&allowance, "allowance", res.Ret)
	if err != nil {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "failed to unpack allowance: %v", err)
	}

	if allowance.Cmp(big.NewInt(1)) != 0 {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "failed to set allowance")
	}

	res, err = k.evmKeeper.CallEVMWithData(cachedCtx, receiverHex, &contractAddr, cbData.Calldata, true, math.NewIntFromUint64(cachedCtx.GasMeter().GasRemaining()).BigInt())
	if err != nil {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "EVM returned error: %s", err.Error())
	}

	ctx.GasMeter().ConsumeGas(res.GasUsed, "callback function")

	contractTokenBalance := k.erc20Keeper.BalanceOf(ctx, erc20.ABI, tokenPair.GetERC20Contract(), contractAddr)
	if contractTokenBalance.Cmp(amountInt.BigInt()) != 0 {
		return errorsmod.Wrapf(erc20types.ErrEVMCall, "contract balance %d does not equal sent amount %d", contractTokenBalance, amountInt.BigInt())
	}

	return nil
}

func (k ContractKeeper) IBCOnAcknowledgementPacketCallback(
	cachedCtx sdk.Context,
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

	cbData, isCbPacket, err := callbacktypes.GetCallbackData(data, version, packet.GetDestPort(), cachedCtx.GasMeter().GasRemaining(), cachedCtx.GasMeter().GasRemaining(), callbacktypes.SourceCallbackKey)
	if err != nil {
		return err
	}
	if !isCbPacket {
		return nil
	}

	if len(cbData.Calldata) != 0 {
		return errorsmod.Wrap(types.ErrInvalidCalldata, "acknowledgement callback data should not contain calldata")
	}

	sender := common.BytesToAddress(sdk.MustAccAddressFromBech32(packetSenderAddress))
	contractAddr := common.HexToAddress(contractAddress)
	contractAccount := k.evmKeeper.GetAccountOrEmpty(cachedCtx, contractAddr)
	// this check is required because if there is no code, the call will still pass on the EVM side, but it will ignore the calldata
	// and funds may get stuck
	if !contractAccount.IsContract() {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "provided contract address is not a contract: %s", contractAddr)
	}

	abi, err := callbacksabi.LoadABI()
	if err != nil {
		return err
	}

	// TODO: Do something with the response
	// Call the onPacketAcknowledgement function in the contract
	_, err = k.evmKeeper.CallEVM(cachedCtx, *abi, sender, contractAddr, true, nil, "onPacketAcknowledgement",
		packet.GetSourceChannel(), packet.GetSourcePort(), packet.GetSequence(), packet.GetData(), acknowledgement)
	if err != nil {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "EVM returned error: %s", err.Error())
	}
	return nil
}

func (k ContractKeeper) IBCOnTimeoutPacketCallback(
	cachedCtx sdk.Context,
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

	cbData, isCbPacket, err := callbacktypes.GetCallbackData(data, version, packet.GetDestPort(), cachedCtx.GasMeter().GasRemaining(), cachedCtx.GasMeter().GasRemaining(), callbacktypes.SourceCallbackKey)
	if err != nil {
		return err
	}
	if !isCbPacket {
		return nil
	}

	if len(cbData.Calldata) != 0 {
		return errorsmod.Wrap(types.ErrInvalidCalldata, "acknowledgement callback data should not contain calldata")
	}

	sender := common.BytesToAddress(sdk.MustAccAddressFromBech32(packetSenderAddress))
	contractAddr := common.HexToAddress(contractAddress)
	contractAccount := k.evmKeeper.GetAccountOrEmpty(cachedCtx, contractAddr)
	// this check is required because if there is no code, the call will still pass on the EVM side, but it will ignore the calldata
	// and funds may get stuck
	if !contractAccount.IsContract() {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "provided contract address is not a contract: %s", contractAddr)
	}

	abi, err := callbacksabi.LoadABI()
	if err != nil {
		return err
	}

	// TODO: Do something with the response
	_, err = k.evmKeeper.CallEVM(cachedCtx, *abi, sender, contractAddr, true, nil, "onPacketTimeout",
		packet.GetSourceChannel(), packet.GetSourcePort(), packet.GetSequence(), packet.GetData())
	if err != nil {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "EVM returned error: %s", err.Error())
	}
	return nil
}
