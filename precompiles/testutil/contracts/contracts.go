package contracts

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/crypto"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/testutil/integration"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// Call is a helper function to call any arbitrary smart contract.
func Call(ctx sdk.Context, app evm.EvmApp, args CallArgs) (res abci.ExecTxResult, ethRes *evmtypes.MsgEthereumTxResponse, err error) {
	var (
		nonce    uint64
		gasLimit = args.GasLimit
	)

	if args.PrivKey == nil {
		return abci.ExecTxResult{}, nil, fmt.Errorf("private key is required; got: %v", args.PrivKey)
	}

	pk, ok := args.PrivKey.(*ethsecp256k1.PrivKey)
	if !ok {
		return abci.ExecTxResult{}, nil, errors.New("error while casting type ethsecp256k1.PrivKey on provided private key")
	}

	key, err := pk.ToECDSA()
	if err != nil {
		return abci.ExecTxResult{}, nil, fmt.Errorf("error while converting private key to ecdsa: %v", err)
	}

	addr := crypto.PubkeyToAddress(key.PublicKey)

	if args.Nonce == nil {
		nonce = app.GetEVMKeeper().GetNonce(ctx, addr)
	} else {
		nonce = args.Nonce.Uint64()
	}

	// if gas limit not provided
	// use default
	if args.GasLimit == 0 {
		gasLimit = 1000000
	}

	// if gas price not provided
	var gasPrice *big.Int
	if args.GasPrice == nil {
		baseFeeRes, err := app.GetEVMKeeper().BaseFee(ctx, &evmtypes.QueryBaseFeeRequest{})
		if err != nil {
			return abci.ExecTxResult{}, nil, err
		}
		gasPrice = baseFeeRes.BaseFee.BigInt() // default gas price == block base fee
	} else {
		gasPrice = args.GasPrice
	}
	// create MsgEthereumTx that calls the contract
	input, err := args.ContractABI.Pack(args.MethodName, args.Args...)
	if err != nil {
		return abci.ExecTxResult{}, nil, fmt.Errorf("error while packing the input: %v", err)
	}

	// create MsgEthereumTx that calls the contract
	msg := evmtypes.NewTx(&evmtypes.EvmTxArgs{
		ChainID:   evmtypes.GetEthChainConfig().ChainID,
		Nonce:     nonce,
		To:        &args.ContractAddr,
		Amount:    args.Amount,
		GasLimit:  gasLimit,
		GasPrice:  gasPrice,
		GasFeeCap: args.GasFeeCap,
		GasTipCap: args.GasTipCap,
		Input:     input,
		Accesses:  args.AccessList,
	})
	msg.From = addr.Hex()

	res, err = integration.DeliverEthTx(app, args.PrivKey, msg)
	if err != nil {
		return res, nil, fmt.Errorf("error during deliver tx: %s", err)
	}
	if !res.IsOK() {
		return res, nil, fmt.Errorf("error during deliver tx: %v", res.Log)
	}

	ethRes, err = evmtypes.DecodeTxResponse(res.Data)
	if err != nil {
		return res, nil, fmt.Errorf("error while decoding tx response: %v", err)
	}

	return res, ethRes, nil
}
