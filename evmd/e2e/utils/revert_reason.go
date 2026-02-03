package utils

import (
	"context"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/rpc"
)

func GetRevertReasonViaEthCall(ctx context.Context, rpcURL string, addr common.Address, data []byte, blockNum *big.Int) (string, error) {
	cctx, cancel := context.WithTimeout(ctx, RevertReasonTimeout)
	defer cancel()

	rpcClient, err := rpc.DialContext(cctx, rpcURL)
	if err != nil {
		return "", err
	}
	defer rpcClient.Close()

	callArg := map[string]any{
		"to":   addr.Hex(),
		"data": "0x" + common.Bytes2Hex(data),
	}

	var blockParam any = "latest"
	if blockNum != nil {
		blockParam = "0x" + blockNum.Text(16)
	}

	var res any
	err = rpcClient.CallContext(cctx, &res, "eth_call", callArg, blockParam)
	if err == nil {
		return "", errors.New("expected revert, got success")
	}

	// Extract revert data from rpc error
	var hexData string
	var rperr rpc.DataError
	if errors.As(err, &rperr) {
		switch v := rperr.ErrorData().(type) {
		case string:
			hexData = v
		case map[string]any:
			if d, ok := v["data"].(string); ok {
				hexData = d
			}
		}
	}
	if hexData == "" {
		return "", errors.New("missing revert data in rpc error")
	}

	b := common.FromHex(hexData)
	msg, uerr := abi.UnpackRevert(b)
	if uerr != nil {
		return "", uerr
	}
	return msg, nil
}
