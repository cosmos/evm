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
		malleate func() uint64
		expErr   error
	}{
		{
			"packet data is transfer with receiver account already existing",
			func() uint64 {
				receiverAcc, err := sdk.AccAddressFromBech32(receiver)
				s.Require().NoError(err)

				// Create and set the account
				acc := s.network.App.GetAccountKeeper().NewAccountWithAddress(ctx, receiverAcc)
				s.network.App.GetAccountKeeper().SetAccount(ctx, acc)
				s.Require().True(s.network.App.GetAccountKeeper().HasAccount(ctx, receiverAcc))
				return acc.GetAccountNumber()
			},
			types.ErrContractHasNoCode,
		},
		{
			"contract code does not exist",
			func() uint64 { return 0 },
			types.ErrContractHasNoCode,
		},
		{
			"packet data is not transfer",
			func() uint64 {
				packet.Data = []byte("not a transfer packet")
				return 0
			},
			ibcerrors.ErrInvalidType, // This will be wrapped by the transfer module
		},
		{
			"packet data is transfer but receiver is not isolated address",
			func() uint64 {
				receiver = senderKey.AccAddr.String() // not an isolated address
				transferData.Receiver = receiver
				transferDataBz := transferData.GetBytes()
				packet.Data = transferDataBz
				return 0
			},
			types.ErrInvalidReceiverAddress,
		},
		{
			"packet data is transfer but callback data is not valid",
			func() uint64 {
				transferData.Memo = fmt.Sprintf(`{"dest_callback": {"address": 10, "calldata": "%x"}}`, []byte("calldata"))
				transferDataBz := transferData.GetBytes()
				packet.Data = transferDataBz
				return 0
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

		originalAccNumber := tc.malleate()
		err := s.network.App.GetCallbackKeeper().IBCReceivePacketCallback(ctx, packet, ack, contract.Hex(), transfertypes.V1)
		if originalAccNumber != 0 {
			acc := s.network.App.GetAccountKeeper().GetAccount(ctx, sdk.MustAccAddressFromBech32(receiver))
			s.Require().NotNil(acc)
			s.Require().Equal(originalAccNumber, acc.GetAccountNumber(), "account number should not be modified")
		}
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
