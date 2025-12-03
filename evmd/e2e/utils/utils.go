package utils

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"google.golang.org/grpc"

	"github.com/cosmos/cosmos-sdk/types/bech32"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func WaitForCondition(ctx context.Context, cond func() (bool, string, error), interval time.Duration, timeout time.Duration) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		ok, reason, err := cond()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-timeoutCtx.Done():
			return fmt.Errorf("timeout waiting for condition: %s", reason)
		case <-time.After(interval):
		}
	}
}

func WaitForBlocks(ctx context.Context, cli *ethclient.Client, n uint64) error {
	return WaitForCondition(ctx, func() (bool, string, error) {
		block, err := cli.BlockNumber(ctx)
		if err != nil {
			return false, err.Error(), nil // only return error as reason, so we can use this while chain is starting up
		}
		return block >= n, fmt.Sprintf("block number %d >= %d", block, n), nil
	}, 500*time.Millisecond, 60*time.Second)
}

func GetBankBalance(ctx context.Context, grpcClient *grpc.ClientConn, address string, denom string) (*banktypes.QueryBalanceResponse, error) {
	return banktypes.NewQueryClient(grpcClient).Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: address,
		Denom:   denom,
	})
}

func AddressToBech32(address common.Address) (string, error) {
	return bech32.ConvertAndEncode(TestBech32Prefix, address.Bytes())
}
