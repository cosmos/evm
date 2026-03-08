package clients

import (
	"crypto/ecdsa"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/ethereum/go-ethereum/common"
)

type EthAccount struct {
	Address common.Address
	PrivKey *ecdsa.PrivateKey
}

type CosmosAccount struct {
	AccAddress    sdk.AccAddress
	AccountNumber uint64
	PrivKey       *ethsecp256k1.PrivKey
}

type TxPoolResult struct {
	Pending map[string]map[string]*EthRPCTransaction `json:"pending"`
	Queued  map[string]map[string]*EthRPCTransaction `json:"queued"`
}

type EthRPCTransaction struct {
	Hash             common.Hash    `json:"hash"`
	BlockHash        *common.Hash   `json:"blockHash"`
	BlockNumber      *string        `json:"blockNumber"`
	From             common.Address `json:"from"`
	To               *common.Address `json:"to"`
	Gas              string         `json:"gas"`
	GasPrice         string         `json:"gasPrice"`
	Input            []byte         `json:"input"`
	Nonce            string         `json:"nonce"`
	TransactionIndex *string        `json:"transactionIndex"`
	Value            string         `json:"value"`
	V                string         `json:"v"`
	R                string         `json:"r"`
	S                string         `json:"s"`
}
