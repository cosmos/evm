package keeper

import (
	"errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
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
	packetDataUnmarshaler porttypes.PacketDataUnmarshaler
}

func NewKeeper(authKeeper types.AccountKeeper, pdUnmarshaler porttypes.PacketDataUnmarshaler, evmKeeper types.EVMKeeper) ContractKeeper {
	// ensure evm callbacks module account is set
	if addr := authKeeper.GetModuleAddress(types.ModuleName); addr == nil {
		panic(errors.New("the EVM callbacks module account has not been set"))
	}

	return ContractKeeper{
		authKeeper:            authKeeper,
		packetDataUnmarshaler: pdUnmarshaler,
		evmKeeper:             evmKeeper,
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

	// can only call callback if the receiver is the module address
	receiver := sdk.MustAccAddressFromBech32(data.Receiver)
	if !receiver.Equals(k.authKeeper.GetModuleAddress(types.ModuleName)) {
		return nil
	}

	cbData, isCbPacket, err := callbacktypes.GetCallbackData(data, version, packet.GetDestPort(), cachedCtx.GasMeter().GasRemaining(), cachedCtx.GasMeter().GasRemaining(), callbacktypes.DestinationCallbackKey)
	if err != nil {
		return err
	}
	if !isCbPacket {
		return nil
	}

	ethReceiver := common.BytesToAddress(receiver)
	contractAddr := common.HexToAddress(contractAddress)

	// TODO: Approve the ERC20 tokens in the transfer packet for the contract on behalf of the receiver
	// before calling the callback.

	// TODO: Do something with the response
	_, err = k.evmKeeper.CallEVMWithData(cachedCtx, ethReceiver, &contractAddr, cbData.Calldata, true)
	if err != nil {
		return err
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
		return err
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
		return err
	}
	return nil
}
