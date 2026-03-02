package indexer

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	abci "github.com/cometbft/cometbft/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"

	"github.com/cosmos/evm/indexer"
)

const (
	// EventTypeDelegate is the staking delegate event type
	EventTypeDelegate = "delegate"

	// Attribute keys for delegate event
	AttributeKeyDelegator = "delegator"
	AttributeKeyValidator = "validator"
)

// Delegate event signature: Delegate(address,address,uint256,uint256)
// Matches the precompile format from precompiles/staking/events.go
var DelegateEventSignature = crypto.Keccak256Hash([]byte("Delegate(address,address,uint256,uint256)"))

// StakingDelegateTransformer transforms staking delegate events
// into EVM-compatible Delegate logs for testing purposes.
type StakingDelegateTransformer struct {
	stakingPrecompileAddress common.Address
}

// NewStakingDelegateTransformer creates a new StakingDelegateTransformer.
func NewStakingDelegateTransformer(stakingPrecompileAddress common.Address) *StakingDelegateTransformer {
	return &StakingDelegateTransformer{
		stakingPrecompileAddress: stakingPrecompileAddress,
	}
}

// CanHandle returns true for delegate events.
func (t *StakingDelegateTransformer) CanHandle(eventType string) bool {
	return eventType == EventTypeDelegate
}

// Transform converts a delegate event to EthReceiptData with Delegate log.
func (t *StakingDelegateTransformer) Transform(
	event abci.Event,
	height int64,
	ethTxHash common.Hash,
) (*indexer.EthReceiptData, error) {
	delegator, validator, amount, err := t.parseDelegateEvent(event)
	if err != nil {
		return nil, err
	}

	log := t.createDelegateLog(delegator, validator, amount, ethTxHash, height)

	return indexer.NewEthReceiptData(
		ethTxHash,
		delegator,
		&t.stakingPrecompileAddress,
		amount,
		50000,
		1,
		[]*ethtypes.Log{log},
	), nil
}

func (t *StakingDelegateTransformer) parseDelegateEvent(event abci.Event) (common.Address, common.Address, *big.Int, error) {
	var delegatorStr, validatorStr, amountStr string

	for _, attr := range event.Attributes {
		switch attr.Key {
		case AttributeKeyDelegator:
			delegatorStr = attr.Value
		case AttributeKeyValidator:
			validatorStr = attr.Value
		case sdk.AttributeKeyAmount:
			amountStr = attr.Value
		}
	}

	if delegatorStr == "" {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("missing delegator attribute in delegate event")
	}
	if validatorStr == "" {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("missing validator attribute in delegate event")
	}
	if amountStr == "" {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("missing amount attribute in delegate event")
	}

	delegatorAddr, err := parseCosmosAddress(delegatorStr)
	if err != nil {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("invalid delegator address: %w", err)
	}

	validatorAddr, err := parseValidatorAddress(validatorStr)
	if err != nil {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("invalid validator address: %w", err)
	}

	amount, err := parseAmount(amountStr)
	if err != nil {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("invalid amount: %w", err)
	}

	return delegatorAddr, validatorAddr, amount, nil
}

func (t *StakingDelegateTransformer) createDelegateLog(
	delegator, validator common.Address,
	amount *big.Int,
	txHash common.Hash,
	height int64,
) *ethtypes.Log {
	delegatorTopic := common.BytesToHash(common.LeftPadBytes(delegator.Bytes(), 32))
	validatorTopic := common.BytesToHash(common.LeftPadBytes(validator.Bytes(), 32))

	// Data: amount (uint256) + newShares (uint256)
	// For simplicity, newShares = amount (1:1 ratio)
	amountData := common.LeftPadBytes(amount.Bytes(), 32)
	sharesData := common.LeftPadBytes(amount.Bytes(), 32)
	data := append(amountData, sharesData...)

	return &ethtypes.Log{
		Address: t.stakingPrecompileAddress,
		Topics: []common.Hash{
			DelegateEventSignature,
			delegatorTopic,
			validatorTopic,
		},
		Data:        data,
		BlockNumber: uint64(height), //#nosec G115
		TxHash:      txHash,
		Index:       0,
	}
}

func parseValidatorAddress(bech32Addr string) (common.Address, error) {
	valAddr, err := sdk.ValAddressFromBech32(bech32Addr)
	if err != nil {
		return common.Address{}, err
	}
	return common.BytesToAddress(valAddr.Bytes()), nil
}
