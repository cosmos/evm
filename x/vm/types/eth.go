package types

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"

	errorsmod "cosmossdk.io/errors"

	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

type EthereumTx struct {
	*ethtypes.Transaction
}

func (tx EthereumTx) Size() int {
	if tx.Transaction == nil {
		return 0
	}
	return int(tx.Transaction.Size()) //nolint:gosec
}

func (tx EthereumTx) MarshalTo(dst []byte) (int, error) {
	if tx.Transaction == nil {
		return 0, nil
	}
	bz, err := tx.MarshalBinary()
	if err != nil {
		return 0, err
	}
	copy(dst, bz)
	return len(bz), nil
}

func (tx *EthereumTx) Unmarshal(dst []byte) error {
	if len(dst) == 0 {
		tx.Transaction = nil
		return nil
	}
	if tx.Transaction == nil {
		tx.Transaction = new(ethtypes.Transaction)
	}
	return tx.UnmarshalBinary(dst)
}

func (tx *EthereumTx) UnmarshalJSON(bz []byte) error {
	var data hexutil.Bytes
	if err := json.Unmarshal(bz, &data); err != nil {
		return err
	}
	return tx.Unmarshal(data)
}

func (tx EthereumTx) MarshalJSON() ([]byte, error) {
	if tx.Transaction == nil {
		return []byte("null"), nil
	}
	bz, err := tx.MarshalBinary()
	if err != nil {
		return nil, err
	}
	return json.Marshal(hexutil.Bytes(bz))
}

func (tx EthereumTx) Validate() error {
	if tx.Transaction == nil {
		return errorsmod.Wrapf(errortypes.ErrInvalidRequest, "raw tx is missing")
	}

	// prevent txs with 0 gas to fill up the mempool
	if tx.Gas() == 0 {
		return errorsmod.Wrap(ErrInvalidGasLimit, "gas limit must not be zero")
	}
	if tx.GasPrice().BitLen() > 256 {
		return errorsmod.Wrap(ErrInvalidGasPrice, "out of bound")
	}
	if tx.GasFeeCap().BitLen() > 256 {
		return errorsmod.Wrap(ErrInvalidGasPrice, "out of bound")
	}
	if tx.GasTipCap().BitLen() > 256 {
		return errorsmod.Wrap(ErrInvalidGasPrice, "out of bound")
	}
	if tx.Cost().BitLen() > 256 {
		return errorsmod.Wrap(ErrInvalidGasFee, "out of bound")
	}
	if tx.GasFeeCapIntCmp(tx.GasTipCap()) < 0 {
		return errorsmod.Wrapf(
			ErrInvalidGasCap,
			"max priority fee per gas higher than max fee per gas (%s > %s)",
			tx.GasTipCap(), tx.GasFeeCap(),
		)
	}
	return nil
}
