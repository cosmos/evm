package vmv1

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	protov2 "google.golang.org/protobuf/proto"
)

// supportedTxs holds the Ethereum transaction types
// supported by Cosmos EVM
//
// Use a function to return a new pointer and avoid
// possible reuse or racing conditions when using the same pointer
var supportedTxs = map[string]func() TxDataV2{
	"/cosmos.evm.vm.v1.DynamicFeeTx": func() TxDataV2 { return &DynamicFeeTx{} },
	"/cosmos.evm.vm.v1.AccessListTx": func() TxDataV2 { return &AccessListTx{} },
	"/cosmos.evm.vm.v1.LegacyTx":     func() TxDataV2 { return &LegacyTx{} },
}

// getSender extracts the sender address from the signature values using the latest signer for the given chainID.
func getSender(txData TxDataV2) (common.Address, error) {
	signer := ethtypes.LatestSignerForChainID(txData.GetChainID())
	from, err := signer.Sender(ethtypes.NewTx(txData.AsEthereumData()))
	if err != nil {
		return common.Address{}, err
	}
	return from, nil
}

// GetSigners is the custom function to get signers on Ethereum transactions
// Gets the signer's address from the Ethereum tx signature
func GetSigners(msg protov2.Message) ([][]byte, error) {
	msgEthTx, ok := msg.(*MsgEthereumTx)
	if !ok {
		return nil, fmt.Errorf("invalid type, expected MsgEthereumTx and got %T", msg)
	}

	return [][]byte{msgEthTx.From}, nil
}
