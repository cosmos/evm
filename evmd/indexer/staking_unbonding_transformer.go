package indexer

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"

	abci "github.com/cometbft/cometbft/abci/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"

	"github.com/cosmos/evm/indexer"
)

// CompleteUnbonding event signature: CompleteUnbonding(address,address,uint256)
// Emitted when an unbonding delegation matures in BeginBlock.
// Parameters: delegator (indexed), validator (indexed), amount
var CompleteUnbondingEventSignature = crypto.Keccak256Hash([]byte("CompleteUnbonding(address,address,uint256)"))

// completeUnbonding function selector: completeUnbonding(address,uint256)
var completeUnbondingFunctionSelector = crypto.Keccak256([]byte("completeUnbonding(address,uint256)"))[:4]

// StakingUnbondingTransformer transforms staking complete_unbonding events
// into EVM-compatible CompleteUnbonding logs.
// These events are emitted in BeginBlock when unbonding delegations mature.
type StakingUnbondingTransformer struct {
	stakingPrecompileAddress common.Address
}

// NewStakingUnbondingTransformer creates a new StakingUnbondingTransformer.
func NewStakingUnbondingTransformer(stakingPrecompileAddress common.Address) *StakingUnbondingTransformer {
	return &StakingUnbondingTransformer{
		stakingPrecompileAddress: stakingPrecompileAddress,
	}
}

// CanHandle returns true for complete_unbonding events.
func (t *StakingUnbondingTransformer) CanHandle(eventType string) bool {
	return eventType == stakingtypes.EventTypeCompleteUnbonding
}

// Transform converts a complete_unbonding event to TransformedTxData with CompleteUnbonding log.
func (t *StakingUnbondingTransformer) Transform(
	event abci.Event,
	height int64,
	ethTxHash common.Hash,
) (*indexer.TransformedTxData, error) {
	delegator, validator, amount, err := t.parseCompleteUnbondingEvent(event)
	if err != nil {
		return nil, err
	}

	log := t.createCompleteUnbondingLog(delegator, validator, amount, ethTxHash, height)
	input := buildCompleteUnbondingInput(validator, amount)

	return indexer.NewTransformedTxData(
		ethTxHash,
		delegator,
		&t.stakingPrecompileAddress,
		amount,
		30000,
		1,
		[]*ethtypes.Log{log},
	).WithInput(input), nil
}

func (t *StakingUnbondingTransformer) parseCompleteUnbondingEvent(event abci.Event) (common.Address, common.Address, *big.Int, error) {
	var delegatorStr, validatorStr, amountStr string

	for _, attr := range event.Attributes {
		switch attr.Key {
		case stakingtypes.AttributeKeyDelegator:
			delegatorStr = attr.Value
		case stakingtypes.AttributeKeyValidator:
			validatorStr = attr.Value
		case sdk.AttributeKeyAmount:
			amountStr = attr.Value
		}
	}

	if delegatorStr == "" {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("missing delegator attribute in complete_unbonding event")
	}
	if validatorStr == "" {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("missing validator attribute in complete_unbonding event")
	}
	if amountStr == "" {
		return common.Address{}, common.Address{}, nil, fmt.Errorf("missing amount attribute in complete_unbonding event")
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

func (t *StakingUnbondingTransformer) createCompleteUnbondingLog(
	delegator, validator common.Address,
	amount *big.Int,
	txHash common.Hash,
	height int64,
) *ethtypes.Log {
	delegatorTopic := common.BytesToHash(common.LeftPadBytes(delegator.Bytes(), 32))
	validatorTopic := common.BytesToHash(common.LeftPadBytes(validator.Bytes(), 32))
	amountData := common.LeftPadBytes(amount.Bytes(), 32)

	return &ethtypes.Log{
		Address: t.stakingPrecompileAddress,
		Topics: []common.Hash{
			CompleteUnbondingEventSignature,
			delegatorTopic,
			validatorTopic,
		},
		Data:        amountData,
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

// buildCompleteUnbondingInput builds completeUnbonding(address,uint256) calldata.
func buildCompleteUnbondingInput(validator common.Address, amount *big.Int) []byte {
	input := make([]byte, 4+32+32)
	copy(input[:4], completeUnbondingFunctionSelector)
	copy(input[4:36], common.LeftPadBytes(validator.Bytes(), 32))
	copy(input[36:68], common.LeftPadBytes(amount.Bytes(), 32))
	return input
}
