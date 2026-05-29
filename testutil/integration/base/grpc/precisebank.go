package grpc

import (
	"context"

	precisebanktypes "github.com/cosmos/evm/x/precisebank/types"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
)

func (gqh *IntegrationHandler) Remainder() (*precisebanktypes.QueryRemainderResponse, error) {
	preciseBankClient := gqh.network.GetPreciseBankClient()
	return preciseBankClient.Remainder(context.Background(), &precisebanktypes.QueryRemainderRequest{})
}

func (gqh *IntegrationHandler) FractionalBalance(address sdktypes.AccAddress) (*precisebanktypes.QueryFractionalBalanceResponse, error) {
	preciseBankClient := gqh.network.GetPreciseBankClient()
	return preciseBankClient.FractionalBalance(context.Background(), &precisebanktypes.QueryFractionalBalanceRequest{
		Address: address.String(),
	})
}
