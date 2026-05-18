package vm

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
)

var templateAccessListTx = &ethtypes.AccessListTx{
	GasPrice: big.NewInt(1),
	Gas:      21000,
	To:       &common.Address{},
	Value:    big.NewInt(0),
	Data:     []byte{},
}

var templateLegacyTx = &ethtypes.LegacyTx{
	GasPrice: big.NewInt(1),
	Gas:      21000,
	To:       &common.Address{},
	Value:    big.NewInt(0),
	Data:     []byte{},
}

var templateDynamicFeeTx = &ethtypes.DynamicFeeTx{
	GasFeeCap: big.NewInt(10),
	GasTipCap: big.NewInt(2),
	Gas:       21000,
	To:        &common.Address{},
	Value:     big.NewInt(0),
	Data:      []byte{},
}

var templateSetCodeTx = &ethtypes.SetCodeTx{
	GasFeeCap: uint256.NewInt(10),
	GasTipCap: uint256.NewInt(2),
	Gas:       21000,
	To:        common.Address{},
	Value:     uint256.NewInt(0),
	Data:      []byte{},
	AuthList:  []ethtypes.SetCodeAuthorization{},
}

func newSignedEthTx(
	txData ethtypes.TxData,
	nonce uint64,
	addr sdk.Address,
	krSigner keyring.Signer,
	ethSigner ethtypes.Signer,
) (*evmtypes.MsgEthereumTx, error) {
	var ethTx *ethtypes.Transaction
	switch txData := txData.(type) {
	case *ethtypes.AccessListTx:
		txData.Nonce = nonce
		ethTx = ethtypes.NewTx(txData)
	case *ethtypes.LegacyTx:
		txData.Nonce = nonce
		ethTx = ethtypes.NewTx(txData)
	case *ethtypes.DynamicFeeTx:
		txData.Nonce = nonce
		ethTx = ethtypes.NewTx(txData)
	default:
		return nil, errors.New("unknown transaction type")
	}

	sig, _, err := krSigner.SignByAddress(addr, ethTx.Hash().Bytes(), signingtypes.SignMode_SIGN_MODE_TEXTUAL)
	if err != nil {
		return nil, err
	}

	ethTx, err = ethTx.WithSignature(ethSigner, sig)
	if err != nil {
		return nil, err
	}

	var msg evmtypes.MsgEthereumTx
	if err := msg.FromSignedEthereumTx(ethTx, ethSigner); err != nil {
		return nil, err
	}
	return &msg, nil
}

func newEthMsgTx(
	nonce uint64,
	address common.Address,
	krSigner keyring.Signer,
	ethSigner ethtypes.Signer,
	txType byte,
	data []byte,
	accessList ethtypes.AccessList,
	authList []ethtypes.SetCodeAuthorization,
) (*evmtypes.MsgEthereumTx, *big.Int, error) {
	var (
		ethTx   *ethtypes.Transaction
		baseFee *big.Int
	)
	switch txType {
	case ethtypes.LegacyTxType:
		templateLegacyTx.Nonce = nonce
		if data != nil {
			templateLegacyTx.Data = data
		}
		ethTx = ethtypes.NewTx(templateLegacyTx)
	case ethtypes.AccessListTxType:
		templateAccessListTx.Nonce = nonce
		if data != nil {
			templateAccessListTx.Data = data
		} else {
			templateAccessListTx.Data = []byte{}
		}

		templateAccessListTx.AccessList = accessList
		ethTx = ethtypes.NewTx(templateAccessListTx)
	case ethtypes.DynamicFeeTxType:
		templateDynamicFeeTx.Nonce = nonce

		if data != nil {
			templateAccessListTx.Data = data
		} else {
			templateAccessListTx.Data = []byte{}
		}
		templateAccessListTx.AccessList = accessList
		ethTx = ethtypes.NewTx(templateDynamicFeeTx)
		baseFee = big.NewInt(3)
	case ethtypes.SetCodeTxType:
		templateSetCodeTx.Nonce = nonce

		if data != nil {
			templateSetCodeTx.Data = data
		} else {
			templateSetCodeTx.Data = []byte{}
		}
		templateSetCodeTx.AuthList = authList
		ethTx = ethtypes.NewTx(templateSetCodeTx)
		baseFee = big.NewInt(3)
	default:
		return nil, baseFee, errors.New("unsupported tx type")
	}

	msg := &evmtypes.MsgEthereumTx{}
	msg.FromEthereumTx(ethTx)
	msg.From = address.Bytes()

	return msg, baseFee, msg.Sign(ethSigner, krSigner)
}

func newNativeMessage(
	nonce uint64,
	address common.Address,
	krSigner keyring.Signer,
	ethSigner ethtypes.Signer,
	txType byte,
	data []byte,
	accessList ethtypes.AccessList,
	authorizationList []ethtypes.SetCodeAuthorization, //nolint:unparam
) (*core.Message, error) {
	msg, baseFee, err := newEthMsgTx(nonce, address, krSigner, ethSigner, txType, data, accessList, authorizationList)
	if err != nil {
		return nil, err
	}

	return msg.AsMessage(baseFee), nil
}

func BenchmarkApplyTransaction(b *testing.B) { //nolint:dupl
	suite := KeeperTestSuite{EnableLondonHF: true}
	suite.SetupTest()

	ethSigner := ethtypes.LatestSignerForChainID(evmtypes.GetEthChainConfig().ChainID)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		addr := suite.Keyring.GetAddr(0)
		krSigner := utiltx.NewSigner(suite.Keyring.GetPrivKey(0))
		tx, err := newSignedEthTx(templateAccessListTx,
			suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), addr),
			sdk.AccAddress(addr.Bytes()),
			krSigner,
			ethSigner,
		)
		require.NoError(b, err)

		b.StartTimer()
		resp, err := suite.Network.App.GetEVMKeeper().ApplyTransaction(suite.Network.GetContext(), tx.AsTransaction())
		b.StopTimer()

		require.NoError(b, err)
		require.False(b, resp.Failed())
	}
}

func BenchmarkApplyTransactionWithLegacyTx(b *testing.B) { //nolint:dupl
	suite := KeeperTestSuite{EnableLondonHF: true}
	suite.SetupTest()

	ethSigner := ethtypes.LatestSignerForChainID(evmtypes.GetEthChainConfig().ChainID)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		addr := suite.Keyring.GetAddr(0)
		krSigner := utiltx.NewSigner(suite.Keyring.GetPrivKey(0))
		tx, err := newSignedEthTx(templateLegacyTx,
			suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), addr),
			sdk.AccAddress(addr.Bytes()),
			krSigner,
			ethSigner,
		)
		require.NoError(b, err)

		b.StartTimer()
		resp, err := suite.Network.App.GetEVMKeeper().ApplyTransaction(suite.Network.GetContext(), tx.AsTransaction())
		b.StopTimer()

		require.NoError(b, err)
		require.False(b, resp.Failed())
	}
}

func BenchmarkApplyTransactionWithDynamicFeeTx(b *testing.B) {
	suite := KeeperTestSuite{EnableFeemarket: true, EnableLondonHF: true}
	suite.SetupTest()

	ethSigner := ethtypes.LatestSignerForChainID(evmtypes.GetEthChainConfig().ChainID)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		addr := suite.Keyring.GetAddr(0)
		krSigner := utiltx.NewSigner(suite.Keyring.GetPrivKey(0))
		tx, err := newSignedEthTx(templateDynamicFeeTx,
			suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), addr),
			sdk.AccAddress(addr.Bytes()),
			krSigner,
			ethSigner,
		)
		require.NoError(b, err)

		b.StartTimer()
		resp, err := suite.Network.App.GetEVMKeeper().ApplyTransaction(suite.Network.GetContext(), tx.AsTransaction())
		b.StopTimer()

		require.NoError(b, err)
		require.False(b, resp.Failed())
	}
}

func BenchmarkApplyMessage(b *testing.B) {
	suite := KeeperTestSuite{EnableLondonHF: true}
	suite.SetupTest()

	ethCfg := evmtypes.GetEthChainConfig()
	signer := ethtypes.LatestSignerForChainID(ethCfg.ChainID)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		addr := suite.Keyring.GetAddr(0)
		krSigner := utiltx.NewSigner(suite.Keyring.GetPrivKey(0))
		m, err := newNativeMessage(
			suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), addr),
			addr,
			krSigner,
			signer,
			ethtypes.AccessListTxType,
			nil,
			nil,
			nil,
		)
		require.NoError(b, err)

		b.StartTimer()
		resp, err := suite.Network.App.GetEVMKeeper().ApplyMessage(suite.Network.GetContext(), *m, nil, true, false)
		b.StopTimer()

		require.NoError(b, err)
		require.False(b, resp.Failed())
	}
}

func BenchmarkApplyMessageWithLegacyTx(b *testing.B) {
	suite := KeeperTestSuite{EnableLondonHF: true}
	suite.SetupTest()

	ethCfg := evmtypes.GetEthChainConfig()
	signer := ethtypes.LatestSignerForChainID(ethCfg.ChainID)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		addr := suite.Keyring.GetAddr(0)
		krSigner := utiltx.NewSigner(suite.Keyring.GetPrivKey(0))
		m, err := newNativeMessage(
			suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), addr),
			addr,
			krSigner,
			signer,
			ethtypes.AccessListTxType,
			nil,
			nil,
			nil,
		)
		require.NoError(b, err)

		b.StartTimer()
		resp, err := suite.Network.App.GetEVMKeeper().ApplyMessage(suite.Network.GetContext(), *m, nil, true, false)
		b.StopTimer()

		require.NoError(b, err)
		require.False(b, resp.Failed())
	}
}

func BenchmarkApplyMessageWithDynamicFeeTx(b *testing.B) {
	suite := KeeperTestSuite{EnableFeemarket: true, EnableLondonHF: true}
	suite.SetupTest()

	ethCfg := evmtypes.GetEthChainConfig()
	signer := ethtypes.LatestSignerForChainID(ethCfg.ChainID)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		addr := suite.Keyring.GetAddr(0)
		krSigner := utiltx.NewSigner(suite.Keyring.GetPrivKey(0))
		m, err := newNativeMessage(
			suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), addr),
			addr,
			krSigner,
			signer,
			ethtypes.DynamicFeeTxType,
			nil,
			nil,
			nil,
		)
		require.NoError(b, err)

		b.StartTimer()
		resp, err := suite.Network.App.GetEVMKeeper().ApplyMessage(suite.Network.GetContext(), *m, nil, true, false)
		b.StopTimer()

		require.NoError(b, err)
		require.False(b, resp.Failed())
	}
}
