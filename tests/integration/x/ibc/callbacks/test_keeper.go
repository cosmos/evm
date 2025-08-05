package callbacks

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"

	"github.com/cosmos/evm/testutil/keyring"
	"github.com/cosmos/evm/x/ibc/callbacks/types"
	cbtypes "github.com/cosmos/ibc-go/v10/modules/apps/callbacks/types"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	channeltypes "github.com/cosmos/ibc-go/v10/modules/core/04-channel/types"
	ibcerrors "github.com/cosmos/ibc-go/v10/modules/core/errors"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperTestSuite) TestOnRecvPacket() {
	var (
		contract     common.Address
		ctx          sdk.Context
		senderKey    keyring.Key
		receiver     string
		transferData transfertypes.FungibleTokenPacketData
		packet       channeltypes.Packet
	)
	testCases := []struct {
		name     string
		malleate func()
		expErr   error
	}{
		{
			"contract code does not exist",
			func() {},
			types.ErrContractHasNoCode,
		},
		{
			"packet data is not transfer",
			func() {
				packet.Data = []byte("not a transfer packet")
			},
			ibcerrors.ErrInvalidType,
		},
		{
			"packet data is transfer but receiver is not isolated address",
			func() {
				receiver = senderKey.AccAddr.String() // not an isolated address
				transferData.Receiver = receiver
				transferDataBz := transferData.GetBytes()
				packet.Data = transferDataBz
			},
			types.ErrInvalidReceiverAddress,
		},
		{
			"packet data is transfer but callback data is not valid",
			func() {
				transferData.Memo = fmt.Sprintf(`{"dest_callback": {"address": 10, "calldata": "%x"}}`, []byte("calldata"))
				transferDataBz := transferData.GetBytes()
				packet.Data = transferDataBz
			},
			cbtypes.ErrInvalidCallbackData,
		},
	}

	for _, tc := range testCases {
		s.SetupTest() // reset
		ctx = s.network.GetContext()

		senderKey = s.keyring.GetKey(0)
		receiverBz := types.GenerateIsolatedAddress("channel-1", senderKey.AccAddr.String())
		receiver = sdk.AccAddress(receiverBz.Bytes()).String()
		contract = common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678") // Example contract address

		transferData = transfertypes.NewFungibleTokenPacketData(
			"uatom",
			"100",
			senderKey.AccAddr.String(),
			receiver,
			fmt.Sprintf(`{"dest_callback": {"address": "%s", "calldata": "%x"}}`, contract.Hex(), []byte("calldata")),
		)
		transferDataBz := transferData.GetBytes()

		packet = channeltypes.NewPacket(
			transferDataBz,
			1,
			transfertypes.PortID,
			"channel-0",
			transfertypes.PortID,
			"channel-1",
			clienttypes.ZeroHeight(),
			10000000,
		)
		ack := channeltypes.NewResultAcknowledgement([]byte{1})

		tc.malleate()

		err := s.network.App.GetCallbackKeeper().IBCReceivePacketCallback(ctx, packet, ack, contract.Hex(), transfertypes.V1)
		if tc.expErr != nil {
			s.Require().Contains(err.Error(), tc.expErr.Error(), "expected error: %s, got: %s", tc.expErr.Error(), err.Error())
		} else {
			s.Require().NoError(err)
		}
	}
}

func (s *KeeperTestSuite) TestOnAcknowledgementPacket() {
	var (
		contract     common.Address
		ctx          sdk.Context
		senderKey    keyring.Key
		receiver     string
		transferData transfertypes.FungibleTokenPacketData
		packet       channeltypes.Packet
	)
	testCases := []struct {
		name     string
		malleate func()
		expErr   error
	}{
		{
			"success",
			func() {},
			types.ErrCallbackFailed,
		},
		{
			"packet data is not transfer",
			func() {
				packet.Data = []byte("not a transfer packet")
			},
			ibcerrors.ErrInvalidType,
		},
		{
			"packet data is transfer but callback data is not valid",
			func() {
				transferData.Memo = fmt.Sprintf(`{"src_callback": {"address": 10, "calldata": "%x"}}`, []byte("calldata"))
				transferDataBz := transferData.GetBytes()
				packet.Data = transferDataBz
			},
			cbtypes.ErrInvalidCallbackData,
		},
		{
			"packet data is transfer but custom calldata is set",
			func() {
				transferData.Memo = fmt.Sprintf(`{"src_callback": {"address": "%s", "calldata": "%x"}}`, contract.Hex(), []byte("calldata"))
				transferDataBz := transferData.GetBytes()
				packet.Data = transferDataBz
			},
			types.ErrInvalidCalldata,
		},
	}

	for _, tc := range testCases {
		s.SetupTest() // reset
		ctx = s.network.GetContext()

		senderKey = s.keyring.GetKey(0)
		receiver = types.GenerateIsolatedAddress("channel-1", senderKey.AccAddr.String()).String()

		transferData = transfertypes.NewFungibleTokenPacketData(
			"uatom",
			"100",
			senderKey.AccAddr.String(),
			receiver,
			fmt.Sprintf(`{"src_callback": {"address": "%s"}}`, contract.Hex()),
		)
		transferDataBz := transferData.GetBytes()

		packet = channeltypes.NewPacket(
			transferDataBz,
			1,
			transfertypes.PortID,
			"channel-0",
			transfertypes.PortID,
			"channel-1",
			clienttypes.ZeroHeight(),
			10000000,
		)
		ack := channeltypes.NewResultAcknowledgement([]byte{1})

		tc.malleate()

		err := s.network.App.GetCallbackKeeper().IBCOnAcknowledgementPacketCallback(
			ctx, packet, ack.Acknowledgement(), senderKey.AccAddr, contract.Hex(), senderKey.AccAddr.String(), transfertypes.V1,
		)
		if tc.expErr != nil {
			s.Require().Contains(err.Error(), tc.expErr.Error(), "expected error: %s, got: %s", tc.expErr.Error(), err.Error())
		} else {
			s.Require().NoError(err)
		}
	}
}

func (s *KeeperTestSuite) TestOnTimeoutPacket() {
	var (
		contract     common.Address
		ctx          sdk.Context
		senderKey    keyring.Key
		receiver     string
		transferData transfertypes.FungibleTokenPacketData
		packet       channeltypes.Packet
	)
	testCases := []struct {
		name     string
		malleate func()
		expErr   error
	}{
		{
			"success",
			func() {},
			types.ErrCallbackFailed,
		},
		{
			"packet data is not transfer",
			func() {
				packet.Data = []byte("not a transfer packet")
			},
			ibcerrors.ErrInvalidType,
		},
		{
			"packet data is transfer but callback data is not valid",
			func() {
				transferData.Memo = fmt.Sprintf(`{"src_callback": {"address": 10, "calldata": "%x"}}`, []byte("calldata"))
				transferDataBz := transferData.GetBytes()
				packet.Data = transferDataBz
			},
			cbtypes.ErrInvalidCallbackData,
		},
		{
			"packet data is transfer but custom calldata is set",
			func() {
				transferData.Memo = fmt.Sprintf(`{"src_callback": {"address": "%s", "calldata": "%x"}}`, contract.Hex(), []byte("calldata"))
				transferDataBz := transferData.GetBytes()
				packet.Data = transferDataBz
			},
			types.ErrInvalidCalldata,
		},
	}

	for _, tc := range testCases {
		s.SetupTest() // reset
		ctx = s.network.GetContext()

		senderKey = s.keyring.GetKey(0)
		receiver = types.GenerateIsolatedAddress("channel-1", senderKey.AccAddr.String()).String()

		transferData = transfertypes.NewFungibleTokenPacketData(
			"uatom",
			"100",
			senderKey.AccAddr.String(),
			receiver,
			fmt.Sprintf(`{"src_callback": {"address": "%s"}}`, contract.Hex()),
		)
		transferDataBz := transferData.GetBytes()

		packet = channeltypes.NewPacket(
			transferDataBz,
			1,
			transfertypes.PortID,
			"channel-0",
			transfertypes.PortID,
			"channel-1",
			clienttypes.ZeroHeight(),
			10000000,
		)

		tc.malleate()

		err := s.network.App.GetCallbackKeeper().IBCOnTimeoutPacketCallback(
			ctx, packet, senderKey.AccAddr, contract.Hex(), senderKey.AccAddr.String(), transfertypes.V1,
		)
		if tc.expErr != nil {
			s.Require().Contains(err.Error(), tc.expErr.Error(), "expected error: %s, got: %s", tc.expErr.Error(), err.Error())
		} else {
			s.Require().NoError(err)
		}
	}
}
