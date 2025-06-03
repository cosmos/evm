package keeper

import (
	errorsmod "cosmossdk.io/errors"
	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/utils"
	"github.com/ethereum/go-ethereum/common"

	callbacktypes "github.com/cosmos/ibc-go/v10/modules/apps/callbacks/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	porttypes "github.com/cosmos/ibc-go/v10/modules/core/05-port/types"
	ibcexported "github.com/cosmos/ibc-go/v10/modules/core/exported"

	callbacksabi "github.com/cosmos/evm/precompiles/callbacks"

	"github.com/cosmos/evm/x/ibc/callbacks/types"
)

// ContractKeeper implements callbacktypes.ContractKeeper
var _ callbacktypes.ContractKeeper = (*ContractKeeper)(nil)

type ContractKeeper struct {
	authKeeper            types.AccountKeeper
	evmKeeper             types.EVMKeeper
	erc20Keeper           types.ERC20Keeper
	packetDataUnmarshaler porttypes.PacketDataUnmarshaler
}

func NewKeeper(authKeeper types.AccountKeeper, pdUnmarshaler porttypes.PacketDataUnmarshaler, evmKeeper types.EVMKeeper, erc20Keeper types.ERC20Keeper) ContractKeeper {
	return ContractKeeper{
		authKeeper:            authKeeper,
		packetDataUnmarshaler: pdUnmarshaler,
		evmKeeper:             evmKeeper,
		erc20Keeper:           erc20Keeper,
	}
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
	cachedCtx sdk.Context,
	packet ibcexported.PacketI,
	ack ibcexported.Acknowledgement,
	contractAddress string,
	version string,
) error {
	data, err := transfertypes.UnmarshalPacketData(packet.GetData(), version, "")
	if err != nil {
		return err
	}

	cbData, isCbPacket, err := callbacktypes.GetCallbackData(data, version, packet.GetDestPort(), cachedCtx.GasMeter().GasRemaining(), cachedCtx.GasMeter().GasRemaining(), callbacktypes.DestinationCallbackKey)
	if err != nil {
		return err
	}
	if !isCbPacket {
		return nil
	}

	// can only call callback if the receiver is the isolated address for the packet sender on this chain
	receiver := sdk.MustAccAddressFromBech32(data.Receiver)
	receiverHex, err := utils.HexAddressFromBech32String(receiver.String())
	if err != nil {
		return errorsmod.Wrapf(err, "address conversion failed for receiver address: %s", receiver)
	}

	isolatedAddr := types.GenerateIsolatedAddress(packet.GetDestChannel(), data.Sender)
	isolatedAddrHex, err := utils.HexAddressFromBech32String(isolatedAddr.String())
	if err != nil {
		return errorsmod.Wrapf(err, "address conversion failed for isolated address: %s", isolatedAddr)
	}

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

	tokenPairID := k.erc20Keeper.GetTokenPairID(cachedCtx, data.Token.Denom.IBCDenom())
	tokenPair, found := k.erc20Keeper.GetTokenPair(cachedCtx, tokenPairID)
	if !found {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "token pair for denom %s not found", data.Token.Denom.IBCDenom())
	}
	amountInt, overflow := math.NewIntFromString(data.Token.Amount)
	if overflow {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "amount overflow")
	}

	err = k.erc20Keeper.SetAllowance(cachedCtx, tokenPair.GetERC20Contract(), isolatedAddrHex, contractAddr, amountInt.BigInt())
	if err != nil {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "failed to set allowance: %v", err)
	}

	// TODO: Do something with the response
	_, err = k.evmKeeper.CallEVMWithData(cachedCtx, receiverHex, &contractAddr, cbData.Calldata, true)
	if err != nil {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "EVM returned error: %s", err.Error())
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

	abi, err := callbacksabi.LoadABI()
	if err != nil {
		return err
	}

	// TODO: Do something with the response
	// Call the onPacketAcknowledgement function in the contract
	_, err = k.evmKeeper.CallEVM(cachedCtx, *abi, sender, contractAddr, true, "onPacketAcknowledgement",
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

	abi, err := callbacksabi.LoadABI()
	if err != nil {
		return err
	}

	// TODO: Do something with the response
	_, err = k.evmKeeper.CallEVM(cachedCtx, *abi, sender, contractAddr, true, "onPacketTimeout",
		packet.GetSourceChannel(), packet.GetSourcePort(), packet.GetSequence(), packet.GetData())
	if err != nil {
		return errorsmod.Wrapf(types.ErrCallbackFailed, "EVM returned error: %s", err.Error())
	}
	return nil
}
