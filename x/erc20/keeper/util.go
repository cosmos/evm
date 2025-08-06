package keeper

import (
	"github.com/ethereum/go-ethereum/common"

	types2 "github.com/cosmos/evm/x/erc20/types"
	"github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/errors"
)

// validateApprovalEventDoesNotExist returns an error if the given transactions logs include
// an unexpected `Approval` event
func validateApprovalEventDoesNotExist(logs []*types.Log) error {
	for _, log := range logs {
		if log.Topics[0] == logApprovalSigHash.Hex() {
			return errors.Wrapf(
				types2.ErrUnexpectedEvent, "unexpected Approval event",
			)
		}
	}

	return nil
}

// validateTransferEventExists returns an error if the given transactions logs DO NOT include
// an expected `Transfer` event from the expected address
func validateTransferEventExists(logs []*types.Log, tokenAddress common.Address) error {
	if len(logs) == 0 {
		return errors.Wrapf(
			types2.ErrExpectedEvent, "expected Transfer event",
		)
	}
	found := false
	for _, log := range logs {
		if log.Topics[0] == logTransferSigHash.Hex() {
			if log.Address != tokenAddress.Hex() {
				return errors.Wrapf(
					types2.ErrUnexpectedEvent, "Transfer event from unexpected address",
				)
			}
			if found {
				return errors.Wrapf(
					types2.ErrUnexpectedEvent, "duplicate Transfer event",
				)
			}
			found = true
		}
	}

	if !found {
		return errors.Wrapf(
			types2.ErrExpectedEvent, "expected Transfer event",
		)
	}

	return nil
}
