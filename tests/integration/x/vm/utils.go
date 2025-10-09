package vm

import (
	"encoding/json"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	servercfg "github.com/cosmos/evm/server/config"
	testKeyring "github.com/cosmos/evm/testutil/keyring"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/vm/keeper/testdata"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func (s *KeeperTestSuite) EvmDenom() string {
	return evmtypes.GetEVMCoinDenom()
}

func (s *KeeperTestSuite) StateDB() *statedb.StateDB {
	return statedb.New(s.Network.GetContext(), s.Network.App.GetEVMKeeper(), statedb.NewEmptyTxConfig())
}

// DeployTestContract deploy a test erc20 contract and returns the contract address
func (s *KeeperTestSuite) DeployTestContract(t require.TestingT, ctx sdk.Context, owner common.Address, supply *big.Int) common.Address {
	chainID := evmtypes.GetEthChainConfig().ChainID

	erc20Contract, err := testdata.LoadERC20Contract()
	require.NoError(t, err, "failed to load contract")

	ctorArgs, err := erc20Contract.ABI.Pack("", owner, supply)
	require.NoError(t, err)

	addr := s.Keyring.GetAddr(0)
	nonce := s.Network.App.GetEVMKeeper().GetNonce(s.Network.GetContext(), addr)

	data := erc20Contract.Bin
	data = append(data, ctorArgs...)
	args, err := json.Marshal(&evmtypes.TransactionArgs{
		From: &addr,
		Data: (*hexutil.Bytes)(&data),
	})
	require.NoError(t, err)
	res, err := s.Network.GetEvmClient().EstimateGas(ctx, &evmtypes.EthCallRequest{
		Args:            args,
		GasCap:          servercfg.DefaultGasCap,
		ProposerAddress: s.Network.GetContext().BlockHeader().ProposerAddress,
	})
	require.NoError(t, err)

	baseFeeRes, err := s.Network.GetEvmClient().BaseFee(ctx, &evmtypes.QueryBaseFeeRequest{})
	require.NoError(t, err)

	var erc20DeployTx *evmtypes.MsgEthereumTx
	if s.EnableFeemarket {
		ethTxParams := &evmtypes.EvmTxArgs{
			ChainID:   chainID,
			Nonce:     nonce,
			GasLimit:  res.Gas,
			GasFeeCap: baseFeeRes.BaseFee.BigInt(),
			GasTipCap: big.NewInt(1),
			Input:     data,
			Accesses:  &ethtypes.AccessList{},
		}
		erc20DeployTx = evmtypes.NewTx(ethTxParams)
	} else {
		ethTxParams := &evmtypes.EvmTxArgs{
			ChainID:  chainID,
			Nonce:    nonce,
			GasLimit: res.Gas,
			Input:    data,
		}
		erc20DeployTx = evmtypes.NewTx(ethTxParams)
	}

	krSigner := utiltx.NewSigner(s.Keyring.GetPrivKey(0))
	erc20DeployTx.From = addr.Bytes()
	err = erc20DeployTx.Sign(ethtypes.LatestSignerForChainID(chainID), krSigner)
	require.NoError(t, err)
	rsp, err := s.Network.App.GetEVMKeeper().EthereumTx(ctx, erc20DeployTx)
	require.NoError(t, err)
	require.Empty(t, rsp.VmError)
	return crypto.CreateAddress(addr, nonce)
}

func (s *KeeperTestSuite) TransferERC20Token(t require.TestingT, contractAddr, from, to common.Address, amount *big.Int) *evmtypes.MsgEthereumTx {
	ctx := s.Network.GetContext()
	chainID := evmtypes.GetEthChainConfig().ChainID

	erc20Contract, err := testdata.LoadERC20Contract()
	require.NoError(t, err, "failed to load contract")

	transferData, err := erc20Contract.ABI.Pack("transfer", to, amount)
	require.NoError(t, err)
	args, err := json.Marshal(&evmtypes.TransactionArgs{To: &contractAddr, From: &from, Data: (*hexutil.Bytes)(&transferData)})
	require.NoError(t, err)
	res, err := s.Network.GetEvmClient().EstimateGas(ctx, &evmtypes.EthCallRequest{
		Args:            args,
		GasCap:          25_000_000,
		ProposerAddress: s.Network.GetContext().BlockHeader().ProposerAddress,
	})
	require.NoError(t, err)

	nonce := s.Network.App.GetEVMKeeper().GetNonce(s.Network.GetContext(), s.Keyring.GetAddr(0))
	baseFeeRes, err := s.Network.GetEvmClient().BaseFee(ctx, &evmtypes.QueryBaseFeeRequest{})
	require.NoError(t, err, "failed to get base fee")

	var ercTransferTx *evmtypes.MsgEthereumTx
	if s.EnableFeemarket {
		ethTxParams := &evmtypes.EvmTxArgs{
			ChainID:   chainID,
			Nonce:     nonce,
			To:        &contractAddr,
			GasLimit:  res.Gas,
			GasFeeCap: baseFeeRes.BaseFee.BigInt(),
			GasTipCap: big.NewInt(1),
			Input:     transferData,
			Accesses:  &ethtypes.AccessList{},
		}
		ercTransferTx = evmtypes.NewTx(ethTxParams)
	} else {
		ethTxParams := &evmtypes.EvmTxArgs{
			ChainID:  chainID,
			Nonce:    nonce,
			To:       &contractAddr,
			GasLimit: res.Gas,
			Input:    transferData,
		}
		ercTransferTx = evmtypes.NewTx(ethTxParams)
	}

	addr := s.Keyring.GetAddr(0)
	krSigner := utiltx.NewSigner(s.Keyring.GetPrivKey(0))
	ercTransferTx.From = addr.Bytes()
	err = ercTransferTx.Sign(ethtypes.LatestSignerForChainID(chainID), krSigner)
	require.NoError(t, err)
	rsp, err := s.Network.App.GetEVMKeeper().EthereumTx(ctx, ercTransferTx)
	require.NoError(t, err)
	require.Empty(t, rsp.VmError)
	return ercTransferTx
}

// DeployTestMessageCall deploy a test erc20 contract and returns the contract address
func (s *KeeperTestSuite) DeployTestMessageCall(t require.TestingT) common.Address {
	ctx := s.Network.GetContext()
	chainID := evmtypes.GetEthChainConfig().ChainID

	testMsgCall, err := testdata.LoadMessageCallContract()
	require.NoError(t, err)

	data := testMsgCall.Bin
	addr := s.Keyring.GetAddr(0)
	args, err := json.Marshal(&evmtypes.TransactionArgs{
		From: &addr,
		Data: (*hexutil.Bytes)(&data),
	})
	require.NoError(t, err)

	res, err := s.Network.GetEvmClient().EstimateGas(ctx, &evmtypes.EthCallRequest{
		Args:            args,
		GasCap:          servercfg.DefaultGasCap,
		ProposerAddress: s.Network.GetContext().BlockHeader().ProposerAddress,
	})
	require.NoError(t, err)

	nonce := s.Network.App.GetEVMKeeper().GetNonce(s.Network.GetContext(), addr)
	baseFeeRes, err := s.Network.GetEvmClient().BaseFee(ctx, &evmtypes.QueryBaseFeeRequest{})
	require.NoError(t, err, "failed to get base fee")

	var erc20DeployTx *evmtypes.MsgEthereumTx
	if s.EnableFeemarket {
		ethTxParams := &evmtypes.EvmTxArgs{
			ChainID:   chainID,
			Nonce:     nonce,
			GasLimit:  res.Gas,
			Input:     data,
			GasFeeCap: baseFeeRes.BaseFee.BigInt(),
			Accesses:  &ethtypes.AccessList{},
			GasTipCap: big.NewInt(1),
		}
		erc20DeployTx = evmtypes.NewTx(ethTxParams)
	} else {
		ethTxParams := &evmtypes.EvmTxArgs{
			ChainID:  chainID,
			Nonce:    nonce,
			GasLimit: res.Gas,
			Input:    data,
		}
		erc20DeployTx = evmtypes.NewTx(ethTxParams)
	}

	krSigner := utiltx.NewSigner(s.Keyring.GetPrivKey(0))
	erc20DeployTx.From = addr.Bytes()
	err = erc20DeployTx.Sign(ethtypes.LatestSignerForChainID(chainID), krSigner)
	require.NoError(t, err)
	rsp, err := s.Network.App.GetEVMKeeper().EthereumTx(ctx, erc20DeployTx)
	require.NoError(t, err)
	require.Empty(t, rsp.VmError)
	return crypto.CreateAddress(addr, nonce)
}

func (s *KeeperTestSuite) SignSetCodeAuthorization(authority testKeyring.Key, auth ethtypes.SetCodeAuthorization) ethtypes.SetCodeAuthorization {
	s.T().Helper()

	privKey, ok := authority.Priv.(*ethsecp256k1.PrivKey)
	s.Require().True(ok)

	ecdsaPriv, err := privKey.ToECDSA()
	s.Require().NoError(err)

	signedAuth, err := ethtypes.SignSetCode(ecdsaPriv, auth)
	s.Require().NoError(err)

	return signedAuth
}
