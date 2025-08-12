// Copyright 2021 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package types

import (
	"errors"
	"fmt"
	"math"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/log"
	"github.com/holiman/uint256"
)

// TransactionArgs represents the arguments to construct a new transaction
// or a message call.
type TransactionArgs struct {
	From                 *common.Address `json:"from"`
	To                   *common.Address `json:"to"`
	Gas                  *hexutil.Uint64 `json:"gas"`
	GasPrice             *hexutil.Big    `json:"gasPrice"`
	MaxFeePerGas         *hexutil.Big    `json:"maxFeePerGas"`
	MaxPriorityFeePerGas *hexutil.Big    `json:"maxPriorityFeePerGas"`
	Value                *hexutil.Big    `json:"value"`
	Nonce                *hexutil.Uint64 `json:"nonce"`

	// We accept "data" and "input" for backwards-compatibility reasons.
	// "input" is the newer name and should be preferred by clients.
	// Issue detail: https://github.com/ethereum/go-ethereum/issues/15628
	Data  *hexutil.Bytes `json:"data"`
	Input *hexutil.Bytes `json:"input"`

	// Introduced by AccessListTxType transaction.
	AccessList *types.AccessList `json:"accessList,omitempty"`
	ChainID    *hexutil.Big      `json:"chainId,omitempty"`

	// For BlobTxType
	BlobFeeCap *hexutil.Big  `json:"maxFeePerBlobGas"`
	BlobHashes []common.Hash `json:"blobVersionedHashes,omitempty"`

	// For BlobTxType transactions with blob sidecar
	Blobs       []kzg4844.Blob       `json:"blobs"`
	Commitments []kzg4844.Commitment `json:"commitments"`
	Proofs      []kzg4844.Proof      `json:"proofs"`

	// For SetCodeTxType
	AuthorizationList []types.SetCodeAuthorization `json:"authorizationList"`
}

// GetFrom retrieves the transaction sender address.
func (args *TransactionArgs) GetFrom() common.Address {
	if args.From == nil {
		return common.Address{}
	}
	return *args.From
}

// GetData retrieves the transaction calldata. Input field is preferred.
func (args *TransactionArgs) GetData() []byte {
	if args.Input != nil {
		return *args.Input
	}
	if args.Data != nil {
		return *args.Data
	}
	return nil
}

// CallDefaults sanitizes the transaction arguments, often filling in zero values,
// for the purpose of eth_call class of RPC methods.
func (args *TransactionArgs) CallDefaults(globalGasCap uint64, baseFee *big.Int, chainID *big.Int) error {
	// Reject invalid combinations of pre- and post-1559 fee styles
	if args.GasPrice != nil && (args.MaxFeePerGas != nil || args.MaxPriorityFeePerGas != nil) {
		return errors.New("both gasPrice and (maxFeePerGas or maxPriorityFeePerGas) specified")
	}
	if args.ChainID == nil {
		args.ChainID = (*hexutil.Big)(chainID)
	} else {
		if have := (*big.Int)(args.ChainID); have.Cmp(chainID) != 0 {
			return fmt.Errorf("chainId does not match node's (have=%v, want=%v)", have, chainID)
		}
	}
	if args.Gas == nil {
		gas := globalGasCap
		if gas == 0 {
			gas = uint64(math.MaxUint64 / 2)
		}
		args.Gas = (*hexutil.Uint64)(&gas)
	} else if globalGasCap > 0 && globalGasCap < uint64(*args.Gas) {
		log.Warn("Caller gas above allowance, capping", "requested", args.Gas, "cap", globalGasCap)
		args.Gas = (*hexutil.Uint64)(&globalGasCap)
	}
	if args.Nonce == nil {
		args.Nonce = new(hexutil.Uint64)
	}
	if args.Value == nil {
		args.Value = new(hexutil.Big)
	}
	if baseFee == nil {
		// If there's no basefee, then it must be a non-1559 execution
		if args.GasPrice == nil {
			args.GasPrice = new(hexutil.Big)
		}
	} else {
		// A basefee is provided, necessitating 1559-type execution
		if args.MaxFeePerGas == nil {
			args.MaxFeePerGas = new(hexutil.Big)
		}
		if args.MaxPriorityFeePerGas == nil {
			args.MaxPriorityFeePerGas = new(hexutil.Big)
		}
	}
	if args.BlobFeeCap == nil && args.BlobHashes != nil {
		args.BlobFeeCap = new(hexutil.Big)
	}

	return nil
}

// ToMessage converts the transaction arguments to the Message type used by the
// core evm. This method is used in calls and traces that do not require a real
// live transaction.
// Assumes that fields are not nil, i.e. setDefaults or CallDefaults has been called.
func (args *TransactionArgs) ToMessage(baseFee *big.Int, skipNonceCheck, skipEoACheck bool) *core.Message {
	var (
		gasPrice  *big.Int
		gasFeeCap *big.Int
		gasTipCap *big.Int
	)
	if baseFee == nil {
		gasPrice = args.GasPrice.ToInt()
		gasFeeCap, gasTipCap = gasPrice, gasPrice
	} else {
		// A basefee is provided, necessitating 1559-type execution
		if args.GasPrice != nil {
			// User specified the legacy gas field, convert to 1559 gas typing
			gasPrice = args.GasPrice.ToInt()
			gasFeeCap, gasTipCap = gasPrice, gasPrice
		} else {
			// User specified 1559 gas fields (or none), use those
			gasFeeCap = args.MaxFeePerGas.ToInt()
			gasTipCap = args.MaxPriorityFeePerGas.ToInt()
			// Backfill the legacy gasPrice for EVM execution, unless we're all zeroes
			gasPrice = new(big.Int)
			if gasFeeCap.BitLen() > 0 || gasTipCap.BitLen() > 0 {
				gasPrice = gasPrice.Add(gasTipCap, baseFee)
				if gasPrice.Cmp(gasFeeCap) > 0 {
					gasPrice = gasFeeCap
				}
			}
		}
	}
	var accessList types.AccessList
	if args.AccessList != nil {
		accessList = *args.AccessList
	}
	return &core.Message{
		From:                  args.GetFrom(),
		To:                    args.To,
		Value:                 (*big.Int)(args.Value),
		Nonce:                 uint64(*args.Nonce),
		GasLimit:              uint64(*args.Gas),
		GasPrice:              gasPrice,
		GasFeeCap:             gasFeeCap,
		GasTipCap:             gasTipCap,
		Data:                  args.GetData(),
		AccessList:            accessList,
		BlobGasFeeCap:         (*big.Int)(args.BlobFeeCap),
		BlobHashes:            args.BlobHashes,
		SetCodeAuthorizations: args.AuthorizationList,
		SkipNonceChecks:       skipNonceCheck,
		SkipFromEOACheck:      skipEoACheck,
	}
}

// ToTransaction converts the arguments to a transaction.
// This assumes that setDefaults has been called.
func (args *TransactionArgs) ToTransaction(defaultType int) *types.Transaction {
	usedType := types.LegacyTxType
	switch {
	case args.AuthorizationList != nil || defaultType == types.SetCodeTxType:
		usedType = types.SetCodeTxType
	case args.BlobHashes != nil || defaultType == types.BlobTxType:
		usedType = types.BlobTxType
	case args.MaxFeePerGas != nil || defaultType == types.DynamicFeeTxType:
		usedType = types.DynamicFeeTxType
	case args.AccessList != nil || defaultType == types.AccessListTxType:
		usedType = types.AccessListTxType
	}
	// Make it possible to default to newer tx, but use legacy if gasprice is provided
	if args.GasPrice != nil {
		usedType = types.LegacyTxType
	}
	var data types.TxData
	switch usedType {
	case types.SetCodeTxType:
		al := types.AccessList{}
		if args.AccessList != nil {
			al = *args.AccessList
		}
		authList := []types.SetCodeAuthorization{}
		if args.AuthorizationList != nil {
			authList = args.AuthorizationList
		}
		data = &types.SetCodeTx{
			To:         *args.To,
			ChainID:    uint256.MustFromBig(args.ChainID.ToInt()),
			Nonce:      uint64(*args.Nonce),
			Gas:        uint64(*args.Gas),
			GasFeeCap:  uint256.MustFromBig((*big.Int)(args.MaxFeePerGas)),
			GasTipCap:  uint256.MustFromBig((*big.Int)(args.MaxPriorityFeePerGas)),
			Value:      uint256.MustFromBig((*big.Int)(args.Value)),
			Data:       args.GetData(),
			AccessList: al,
			AuthList:   authList,
		}

	case types.BlobTxType:
		al := types.AccessList{}
		if args.AccessList != nil {
			al = *args.AccessList
		}
		data = &types.BlobTx{
			To:         *args.To,
			ChainID:    uint256.MustFromBig((*big.Int)(args.ChainID)),
			Nonce:      uint64(*args.Nonce),
			Gas:        uint64(*args.Gas),
			GasFeeCap:  uint256.MustFromBig((*big.Int)(args.MaxFeePerGas)),
			GasTipCap:  uint256.MustFromBig((*big.Int)(args.MaxPriorityFeePerGas)),
			Value:      uint256.MustFromBig((*big.Int)(args.Value)),
			Data:       args.GetData(),
			AccessList: al,
			BlobHashes: args.BlobHashes,
			BlobFeeCap: uint256.MustFromBig((*big.Int)(args.BlobFeeCap)),
		}
		if args.Blobs != nil {
			data.(*types.BlobTx).Sidecar = &types.BlobTxSidecar{
				Blobs:       args.Blobs,
				Commitments: args.Commitments,
				Proofs:      args.Proofs,
			}
		}

	case types.DynamicFeeTxType:
		al := types.AccessList{}
		if args.AccessList != nil {
			al = *args.AccessList
		}
		data = &types.DynamicFeeTx{
			To:         args.To,
			ChainID:    (*big.Int)(args.ChainID),
			Nonce:      uint64(*args.Nonce),
			Gas:        uint64(*args.Gas),
			GasFeeCap:  (*big.Int)(args.MaxFeePerGas),
			GasTipCap:  (*big.Int)(args.MaxPriorityFeePerGas),
			Value:      (*big.Int)(args.Value),
			Data:       args.GetData(),
			AccessList: al,
		}

	case types.AccessListTxType:
		data = &types.AccessListTx{
			To:         args.To,
			ChainID:    (*big.Int)(args.ChainID),
			Nonce:      uint64(*args.Nonce),
			Gas:        uint64(*args.Gas),
			GasPrice:   (*big.Int)(args.GasPrice),
			Value:      (*big.Int)(args.Value),
			Data:       args.GetData(),
			AccessList: *args.AccessList,
		}

	default:
		data = &types.LegacyTx{
			To:       args.To,
			Nonce:    uint64(*args.Nonce),
			Gas:      uint64(*args.Gas),
			GasPrice: (*big.Int)(args.GasPrice),
			Value:    (*big.Int)(args.Value),
			Data:     args.GetData(),
		}
	}
	return types.NewTx(data)
}

// IsEIP4844 returns an indicator if the args contains EIP4844 fields.
func (args *TransactionArgs) IsEIP4844() bool {
	return args.BlobHashes != nil || args.BlobFeeCap != nil
}
