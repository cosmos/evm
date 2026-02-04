package fixtures

import (
	"encoding/hex"

	codectypes "github.com/cosmos/cosmos-sdk/codec/types"

	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"
	ibctmtypes "github.com/cosmos/ibc-go/v10/modules/light-clients/07-tendermint"
)

type UpdateClientFixture struct {
	ClientStateHex      string              `json:"client_state_hex"`
	ConsensusStateHex   string              `json:"consensus_state_hex"`
	UpdateClientMessage UpdateClientMessage `json:"update_client_message"`
}

type UpdateClientMessage struct {
	ClientMessageHex string `json:"client_message_hex"`
	TypeURL          string `json:"type_url"`
}

func LoadUpdateClientFixture(filename string) (*UpdateClientFixture, error) {
	return loadFixture[UpdateClientFixture](filename)
}

func (f *UpdateClientFixture) ClientState() (*ibctmtypes.ClientState, error) {
	clientStateBytes, err := hex.DecodeString(f.ClientStateHex)
	if err != nil {
		return nil, err
	}
	clientState := new(ibctmtypes.ClientState)
	if err := clientState.Unmarshal(clientStateBytes); err != nil {
		return nil, err
	}
	return clientState, nil
}

func (f *UpdateClientFixture) ConsensusState() (*ibctmtypes.ConsensusState, error) {
	consensusStateBytes, err := hex.DecodeString(f.ConsensusStateHex)
	if err != nil {
		return nil, err
	}
	consensusState := new(ibctmtypes.ConsensusState)
	if err := consensusState.Unmarshal(consensusStateBytes); err != nil {
		return nil, err
	}
	return consensusState, nil
}

func (f *UpdateClientFixture) ClientStateAny() (*codectypes.Any, error) {
	clientState, err := f.ClientState()
	if err != nil {
		return nil, err
	}
	return clienttypes.PackClientState(clientState)
}

func (f *UpdateClientFixture) ConsensusStateAny() (*codectypes.Any, error) {
	consensusState, err := f.ConsensusState()
	if err != nil {
		return nil, err
	}
	return clienttypes.PackConsensusState(consensusState)
}

func (f *UpdateClientFixture) UpdateClientHeader() (*ibctmtypes.Header, error) {
	clientMessageBytes, err := hex.DecodeString(f.UpdateClientMessage.ClientMessageHex)
	if err != nil {
		return nil, err
	}
	header := new(ibctmtypes.Header)
	if err := header.Unmarshal(clientMessageBytes); err != nil {
		return nil, err
	}
	return header, nil
}

func (f *UpdateClientFixture) UpdateClientMessageAny() (*codectypes.Any, error) {
	header, err := f.UpdateClientHeader()
	if err != nil {
		return nil, err
	}
	return clienttypes.PackClientMessage(header)
}
