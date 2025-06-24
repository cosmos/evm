package vmv1

import (
	"fmt"

	protov2 "google.golang.org/protobuf/proto"
)

// GetSigners is the custom function to get signers on Ethereum transactions
// Gets the signer's address from the Ethereum tx signature
func GetSigners(msg protov2.Message) ([][]byte, error) {
	msgEthTx, ok := msg.(*MsgEthereumTx)
	if !ok {
		return nil, fmt.Errorf("invalid type, expected MsgEthereumTx and got %T", msg)
	}

	return [][]byte{msgEthTx.From}, nil
}
