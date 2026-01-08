package grpc

import (
	"context"
	"errors"

	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
)

// ErrPreciseBankNotAvailable is returned when precisebank module is not available
var ErrPreciseBankNotAvailable = errors.New("precisebank module is not available")

func (gqh *IntegrationHandler) Remainder() (*precisebanktypes.QueryRemainderResponse, error) {
	preciseBankClient := gqh.network.GetPreciseBankClient()
	if preciseBankClient == nil {
		return nil, ErrPreciseBankNotAvailable
	}
	return preciseBankClient.Remainder(context.Background(), &precisebanktypes.QueryRemainderRequest{})
}

func (gqh *IntegrationHandler) FractionalBalance(address sdktypes.AccAddress) (*precisebanktypes.QueryFractionalBalanceResponse, error) {
	preciseBankClient := gqh.network.GetPreciseBankClient()
	if preciseBankClient == nil {
		return nil, ErrPreciseBankNotAvailable
	}
	return preciseBankClient.FractionalBalance(context.Background(), &precisebanktypes.QueryFractionalBalanceRequest{
		Address: address.String(),
	})
}
