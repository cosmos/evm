package indexer

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	abci "github.com/cometbft/cometbft/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/cosmos/evm/indexer"
)

// ERC20 Transfer event signature: Transfer(address,address,uint256)
var TransferEventSignature = crypto.Keccak256Hash([]byte("Transfer(address,address,uint256)"))

// ERC20 transfer function selector: transfer(address,uint256)
var TransferFunctionSelector = crypto.Keccak256([]byte("transfer(address,uint256)"))[:4]

// BankTransferTransformer transforms bank coin_spent and coin_received events
// into ERC20-style Transfer logs.
type BankTransferTransformer struct {
	tokenAddress common.Address
}

// NewBankTransferTransformer creates a new BankTransferTransformer.
func NewBankTransferTransformer(tokenAddress common.Address) *BankTransferTransformer {
	return &BankTransferTransformer{
		tokenAddress: tokenAddress,
	}
}

// CanHandle returns true for coin_spent and coin_received events.
func (t *BankTransferTransformer) CanHandle(eventType string) bool {
	return eventType == banktypes.EventTypeCoinSpent || eventType == banktypes.EventTypeCoinReceived
}

// Transform converts a bank event to TransformedTxData with ERC20 Transfer log.
func (t *BankTransferTransformer) Transform(
	event abci.Event,
	height int64,
	ethTxHash common.Hash,
) (*indexer.TransformedTxData, error) {
	var from, to common.Address
	var amount *big.Int
	var err error

	switch event.Type {
	case banktypes.EventTypeCoinSpent:
		from, amount, err = t.parseCoinSpentEvent(event)
		if err != nil {
			return nil, err
		}
		to = common.Address{} // zero address for spending

	case banktypes.EventTypeCoinReceived:
		to, amount, err = t.parseCoinReceivedEvent(event)
		if err != nil {
			return nil, err
		}
		from = common.Address{} // zero address for receiving

	default:
		return nil, fmt.Errorf("unsupported event type: %s", event.Type)
	}

	log := t.createTransferLog(from, to, amount, ethTxHash, height)
	input := buildTransferInput(to, amount)

	return indexer.NewTransformedTxData(
		ethTxHash,
		from,
		&to,
		amount,
		21000,
		1,
		[]*ethtypes.Log{log},
	).WithInput(input), nil
}

func (t *BankTransferTransformer) parseCoinSpentEvent(event abci.Event) (common.Address, *big.Int, error) {
	var spender string
	var amountStr string

	for _, attr := range event.Attributes {
		switch attr.Key {
		case banktypes.AttributeKeySpender:
			spender = attr.Value
		case sdk.AttributeKeyAmount:
			amountStr = attr.Value
		}
	}

	if spender == "" {
		return common.Address{}, nil, fmt.Errorf("missing spender attribute in coin_spent event")
	}
	if amountStr == "" {
		return common.Address{}, nil, fmt.Errorf("missing amount attribute in coin_spent event")
	}

	addr, err := parseCosmosAddress(spender)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("invalid spender address: %w", err)
	}

	amount, err := parseAmount(amountStr)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("invalid amount: %w", err)
	}

	return addr, amount, nil
}

func (t *BankTransferTransformer) parseCoinReceivedEvent(event abci.Event) (common.Address, *big.Int, error) {
	var receiver string
	var amountStr string

	for _, attr := range event.Attributes {
		switch attr.Key {
		case banktypes.AttributeKeyReceiver:
			receiver = attr.Value
		case sdk.AttributeKeyAmount:
			amountStr = attr.Value
		}
	}

	if receiver == "" {
		return common.Address{}, nil, fmt.Errorf("missing receiver attribute in coin_received event")
	}
	if amountStr == "" {
		return common.Address{}, nil, fmt.Errorf("missing amount attribute in coin_received event")
	}

	addr, err := parseCosmosAddress(receiver)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("invalid receiver address: %w", err)
	}

	amount, err := parseAmount(amountStr)
	if err != nil {
		return common.Address{}, nil, fmt.Errorf("invalid amount: %w", err)
	}

	return addr, amount, nil
}

func (t *BankTransferTransformer) createTransferLog(
	from, to common.Address,
	amount *big.Int,
	txHash common.Hash,
	height int64,
) *ethtypes.Log {
	fromTopic := common.BytesToHash(common.LeftPadBytes(from.Bytes(), 32))
	toTopic := common.BytesToHash(common.LeftPadBytes(to.Bytes(), 32))
	amountData := common.LeftPadBytes(amount.Bytes(), 32)

	return &ethtypes.Log{
		Address: t.tokenAddress,
		Topics: []common.Hash{
			TransferEventSignature,
			fromTopic,
			toTopic,
		},
		Data:        amountData,
		BlockNumber: uint64(height), //#nosec G115
		TxHash:      txHash,
		Index:       0,
	}
}

func parseCosmosAddress(bech32Addr string) (common.Address, error) {
	accAddr, err := sdk.AccAddressFromBech32(bech32Addr)
	if err != nil {
		return common.Address{}, err
	}
	return common.BytesToAddress(accAddr.Bytes()), nil
}

func parseAmount(amountStr string) (*big.Int, error) {
	coins, err := sdk.ParseCoinsNormalized(amountStr)
	if err != nil {
		return nil, err
	}
	if len(coins) == 0 {
		return big.NewInt(0), nil
	}
	return coins[0].Amount.BigInt(), nil
}

// buildTransferInput builds ERC20 transfer(address,uint256) calldata.
func buildTransferInput(to common.Address, amount *big.Int) []byte {
	input := make([]byte, 4+32+32)
	copy(input[:4], TransferFunctionSelector)
	copy(input[4:36], common.LeftPadBytes(to.Bytes(), 32))
	copy(input[36:68], common.LeftPadBytes(amount.Bytes(), 32))
	return input
}
