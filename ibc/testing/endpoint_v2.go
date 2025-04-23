package ibctesting

import (
	"encoding/hex"

	"github.com/pkg/errors"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/gogoproto/proto"
	clientv2types "github.com/cosmos/ibc-go/v10/modules/core/02-client/v2/types"
	channeltypesv2 "github.com/cosmos/ibc-go/v10/modules/core/04-channel/v2/types"
	hostv2 "github.com/cosmos/ibc-go/v10/modules/core/24-host/v2"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// RegisterCounterparty will construct and execute a MsgRegisterCounterparty on the associated endpoint.
func (endpoint *Endpoint) RegisterCounterparty() (err error) {
	msg := clientv2types.NewMsgRegisterCounterparty(endpoint.ClientID, endpoint.Counterparty.MerklePathPrefix.KeyPath, endpoint.Counterparty.ClientID, endpoint.Chain.SenderAccount.GetAddress().String())

	// setup counterparty
	_, err = endpoint.Chain.SendMsgs(msg)

	return err
}

// MsgSendPacket sends a packet on the associated endpoint using a predefined sender. The constructed packet is returned.
func (endpoint *Endpoint) MsgSendPacket(timeoutTimestamp uint64, payload channeltypesv2.Payload) (channeltypesv2.Packet, error) {
	senderAccount := SenderAccount{
		SenderPrivKey: endpoint.Chain.SenderPrivKey,
		SenderAccount: endpoint.Chain.SenderAccount,
	}

	return endpoint.MsgSendPacketWithSender(timeoutTimestamp, payload, senderAccount)
}

// MsgSendPacketWithSender sends a packet on the associated endpoint using the provided sender. The constructed packet is returned.
func (endpoint *Endpoint) MsgSendPacketWithSender(timeoutTimestamp uint64, payload channeltypesv2.Payload, sender SenderAccount) (channeltypesv2.Packet, error) {
	msgSendPacket := channeltypesv2.NewMsgSendPacket(endpoint.ClientID, timeoutTimestamp, sender.SenderAccount.GetAddress().String(), payload)

	res, err := endpoint.Chain.SendMsgsWithSender(sender, msgSendPacket)
	if err != nil {
		return channeltypesv2.Packet{}, err
	}

	if err := endpoint.Counterparty.UpdateClient(); err != nil {
		return channeltypesv2.Packet{}, err
	}

	// TODO: parse the packet from events instead of from the response. https://github.com/cosmos/ibc-go/issues/7459
	// get sequence from msg response
	var msgData sdk.TxMsgData
	err = proto.Unmarshal(res.Data, &msgData)
	if err != nil {
		return channeltypesv2.Packet{}, err
	}
	msgResponse := msgData.MsgResponses[0]
	var sendResponse channeltypesv2.MsgSendPacketResponse
	err = proto.Unmarshal(msgResponse.Value, &sendResponse)
	if err != nil {
		return channeltypesv2.Packet{}, err
	}
	packet := channeltypesv2.NewPacket(sendResponse.Sequence, endpoint.ClientID, endpoint.Counterparty.ClientID, timeoutTimestamp, payload)

	return packet, nil
}

// ParseV2PacketFromEvent parses result from a send packet and returns
func (endpoint *Endpoint) ParseV2PacketFromEvent(events []abci.Event) ([]channeltypesv2.Packet, error) {
	var packets []channeltypesv2.Packet
	for _, event := range events {
		if event.Type == "send_packet" {
			for _, attribute := range event.Attributes {
				if attribute.Key == "encoded_packet_hex" {
					data, err := hex.DecodeString(attribute.Value)
					if err != nil {
						return nil, errors.Wrap(err, "failed to decode encoded_packet_hex string")
					}

					var ibcPacket channeltypesv2.Packet
					if err := proto.Unmarshal(data, &ibcPacket); err != nil {
						return nil, errors.Wrap(err, "failed to unmarshal IBC packet")
					}

					packets = append(packets, ibcPacket)
				}
			}
		}
	}

	if len(packets) == 0 {
		return nil, errors.New("no IBC v2 packets found in events")
	}

	return packets, nil
}

// MsgRecvPacket sends a MsgRecvPacket on the associated endpoint with the provided packet.
func (endpoint *Endpoint) MsgRecvPacketWithResult(packet channeltypesv2.Packet) (*abci.ExecTxResult, error) {
	// get proof of packet commitment from chainA
	packetKey := hostv2.PacketCommitmentKey(packet.SourceClient, packet.Sequence)
	proof, proofHeight := endpoint.Counterparty.QueryProof(packetKey)

	msg := channeltypesv2.NewMsgRecvPacket(packet, proof, proofHeight, endpoint.Chain.SenderAccount.GetAddress().String())

	res, err := endpoint.Chain.SendMsgs(msg)
	if err != nil {
		return nil, err
	}

	if err := endpoint.Counterparty.UpdateClient(); err != nil {
		return nil, err
	}

	return res, nil
}

// MsgAcknowledgePacket sends a MsgAcknowledgement on the associated endpoint with the provided packet and ack.
func (endpoint *Endpoint) MsgAcknowledgePacket(packet channeltypesv2.Packet, ack channeltypesv2.Acknowledgement) error {
	packetKey := hostv2.PacketAcknowledgementKey(packet.DestinationClient, packet.Sequence)
	proof, proofHeight := endpoint.Counterparty.QueryProof(packetKey)

	msg := channeltypesv2.NewMsgAcknowledgement(packet, ack, proof, proofHeight, endpoint.Chain.SenderAccount.GetAddress().String())

	if err := endpoint.Chain.sendMsgs(msg); err != nil {
		return err
	}

	return endpoint.Counterparty.UpdateClient()
}

// MsgTimeoutPacket sends a MsgTimeout on the associated endpoint with the provided packet.
func (endpoint *Endpoint) MsgTimeoutPacket(packet channeltypesv2.Packet) error {
	packetKey := hostv2.PacketReceiptKey(packet.DestinationClient, packet.Sequence)
	proof, proofHeight := endpoint.Counterparty.QueryProof(packetKey)

	msg := channeltypesv2.NewMsgTimeout(packet, proof, proofHeight, endpoint.Chain.SenderAccount.GetAddress().String())

	if err := endpoint.Chain.sendMsgs(msg); err != nil {
		return err
	}

	return endpoint.Counterparty.UpdateClient()
}
