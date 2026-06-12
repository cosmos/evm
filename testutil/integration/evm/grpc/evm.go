package grpc

import (
	"context"
	"errors"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
)

// GetEvmAccount returns the EVM account for the given address.
func (gqh *IntegrationHandler) GetEvmAccount(address common.Address) (*evmtypes.QueryAccountResponse, error) {
	evmClient := gqh.network.GetEvmClient()
	return evmClient.Account(context.Background(), &evmtypes.QueryAccountRequest{
		Address: address.String(),
	})
}

// EstimateGas returns the estimated gas for the given call args.
func (gqh *IntegrationHandler) EstimateGas(args []byte, gasCap uint64) (*evmtypes.EstimateGasResponse, error) {
	evmClient := gqh.network.GetEvmClient()
	res, err := evmClient.EstimateGas(context.Background(), &evmtypes.EthCallRequest{
		Args:   args,
		GasCap: gasCap,
	})
	if err != nil {
		return nil, err
	}

	// handle case where there's a revert related error
	if res.Failed() {
		if (res.VmError != vm.ErrExecutionReverted.Error()) || len(res.Ret) == 0 {
			return nil, errors.New(res.VmError)
		}
		return nil, evmtypes.NewExecErrorWithReason(res.Ret)
	}

	return res, err
}

// EthCall executes a read-only call against the EVM without modifying state.
func (gqh *IntegrationHandler) EthCall(args []byte, gasCap uint64) (*evmtypes.MsgEthereumTxResponse, error) {
	evmClient := gqh.network.GetEvmClient()
	res, err := evmClient.EthCall(context.Background(), &evmtypes.EthCallRequest{
		Args:   args,
		GasCap: gasCap,
	})
	if err != nil {
		return nil, err
	}

	if res.Failed() {
		if (res.VmError != vm.ErrExecutionReverted.Error()) || len(res.Ret) == 0 {
			return nil, errors.New(res.VmError)
		}
		return nil, evmtypes.NewExecErrorWithReason(res.Ret)
	}

	return res, nil
}

// GetEvmParams returns the EVM module params.
func (gqh *IntegrationHandler) GetEvmParams() (*evmtypes.QueryParamsResponse, error) {
	evmClient := gqh.network.GetEvmClient()
	return evmClient.Params(context.Background(), &evmtypes.QueryParamsRequest{})
}

// GetEvmParams returns the EVM module params.
func (gqh *IntegrationHandler) GetEvmBaseFee() (*evmtypes.QueryBaseFeeResponse, error) {
	evmClient := gqh.network.GetEvmClient()
	return evmClient.BaseFee(context.Background(), &evmtypes.QueryBaseFeeRequest{})
}

// GetBalanceFromEVM returns the balance for the given address.
func (gqh *IntegrationHandler) GetBalanceFromEVM(address sdktypes.AccAddress) (*evmtypes.QueryBalanceResponse, error) {
	evmClient := gqh.network.GetEvmClient()
	return evmClient.Balance(context.Background(), &evmtypes.QueryBalanceRequest{
		Address: common.BytesToAddress(address).Hex(),
	})
}
