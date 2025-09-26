package factory

import (
	"encoding/json"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	abcitypes "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/server/config"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"

	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
)

// ExecuteEthTx executes an Ethereum transaction - contract call with the provided private key and txArgs
// It first builds a MsgEthereumTx and then broadcasts it to the network.
func (tf *IntegrationTxFactory) ExecuteEthTx(
	priv cryptotypes.PrivKey,
	txArgs evmtypes.EvmTxArgs,
) (abcitypes.ExecTxResult, error) {
	signedMsg, err := tf.GenerateSignedEthTx(priv, txArgs)
	if err != nil {
		return abcitypes.ExecTxResult{}, errorsmod.Wrap(err, "failed to generate signed ethereum tx")
	}

	txBytes, err := tf.encodeTx(signedMsg)
	if err != nil {
		return abcitypes.ExecTxResult{}, errorsmod.Wrap(err, "failed to encode ethereum tx")
	}

	res, err := tf.network.BroadcastTxSync(txBytes)
	if err != nil {
		return abcitypes.ExecTxResult{}, errorsmod.Wrap(err, "failed to broadcast ethereum tx")
	}

	if err := tf.checkEthTxResponse(&res); err != nil {
		return res, errorsmod.Wrap(err, "failed ETH tx")
	}
	return res, nil
}

// ExecuteContractCall executes a contract call with the provided private key.
func (tf *IntegrationTxFactory) ExecuteContractCall(privKey cryptotypes.PrivKey, txArgs evmtypes.EvmTxArgs, callArgs testutiltypes.CallArgs) (abcitypes.ExecTxResult, error) {
	input, err := GenerateContractCallArgs(callArgs)
	if err != nil {
		return abcitypes.ExecTxResult{}, errorsmod.Wrap(err, "failed to generate contract call args")
	}
	txArgs.Input = input
	return tf.ExecuteEthTx(privKey, txArgs)
}

// DeployContract deploys a contract with the provided private key,
// compiled contract data and constructor arguments.
// TxArgs Input and Nonce fields are overwritten.
func (tf *IntegrationTxFactory) DeployContract(
	priv cryptotypes.PrivKey,
	txArgs evmtypes.EvmTxArgs,
	deploymentData testutiltypes.ContractDeploymentData,
) (common.Address, error) {
	// Get account's nonce to create contract hash
	from := common.BytesToAddress(priv.PubKey().Address().Bytes())
	completeTxArgs, err := tf.GenerateDeployContractArgs(from, txArgs, deploymentData)
	if err != nil {
		return common.Address{}, errorsmod.Wrap(err, "failed to generate contract call args")
	}

	res, err := tf.ExecuteEthTx(priv, completeTxArgs)
	if err != nil || !res.IsOK() {
		return common.Address{}, errorsmod.Wrap(err, "failed to execute eth tx")
	}
	return crypto.CreateAddress(from, completeTxArgs.Nonce), nil
}

// CallContractAndCheckLogs is a helper function to call a contract and check the logs using
// the integration test utilities.
//
// It returns the Cosmos Tx response, the decoded Ethereum Tx response and an error. This error value
// is nil, if the expected logs are found and the VM error is the expected one, should one be expected.
func (tf *IntegrationTxFactory) CallContractAndCheckLogs(
	priv cryptotypes.PrivKey,
	txArgs evmtypes.EvmTxArgs,
	callArgs testutiltypes.CallArgs,
	logCheckArgs testutil.LogCheckArgs,
) (abcitypes.ExecTxResult, *evmtypes.MsgEthereumTxResponse, error) {
	res, err := tf.ExecuteContractCall(priv, txArgs, callArgs)
	logCheckArgs.Res = res
	if err != nil {
		// NOTE: here we are still passing the response to the log check function,
		// because we want to check the logs and expected error in case of a VM error.
		return res, nil, CheckError(err, logCheckArgs)
	}

	ethRes, err := evmtypes.DecodeTxResponse(res.Data)
	if err != nil {
		return res, nil, err
	}

	return res, ethRes, testutil.CheckLogs(logCheckArgs)
}

// QueryContract executes a read-only contract call using eth_call without affecting account nonces.
func (tf *IntegrationTxFactory) QueryContract(
	txArgs evmtypes.EvmTxArgs,
	callArgs testutiltypes.CallArgs,
	gasCap uint64,
) (*evmtypes.MsgEthereumTxResponse, error) {
	input, err := GenerateContractCallArgs(callArgs)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to generate contract call args")
	}

	txArgs.Input = input
	if gasCap == 0 {
		gasCap = config.DefaultGasCap
	}

	// ensure the call has enough intrinsic gas by setting the tx gas limit
	callArgsWithGas := txArgs
	callArgsWithGas.GasLimit = gasCap
	txData := callArgsWithGas.ToTxData()
	args, err := json.Marshal(txData)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to marshal eth_call arguments")
	}

	res, err := tf.grpcHandler.EthCall(args, gasCap)
	if err != nil {
		return nil, errorsmod.Wrap(err, "failed to execute eth_call")
	}

	return res, nil
}
