package factory

import (
	gethtypes "github.com/ethereum/go-ethereum/core/types"

	"github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

// SignMsgEthereumTx signs a MsgEthereumTx with the provided private key and chainID.
func (tf *IntegrationTxFactory) SignMsgEthereumTx(privKey cryptotypes.PrivKey, msgEthereumTx evmtypes.MsgEthereumTx) (evmtypes.MsgEthereumTx, error) {
	ethChainID := tf.network.GetEIP155ChainID()
	signer := gethtypes.LatestSignerForChainID(ethChainID)
	err := msgEthereumTx.Sign(signer, tx.NewSigner(privKey))
	if err != nil {
		return evmtypes.MsgEthereumTx{}, errorsmod.Wrap(err, "failed to sign transaction")
	}
	return msgEthereumTx, nil
}
