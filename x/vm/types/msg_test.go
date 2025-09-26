package types_test

import (
	"fmt"
	"math"
	"math/big"
	"reflect"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/encoding"
	testconstants "github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type MsgsTestSuite struct {
	suite.Suite

	signer        keyring.Signer
	from          common.Address
	to            common.Address
	chainID       *big.Int
	hundredBigInt *big.Int

	clientCtx client.Context
}

func TestMsgsTestSuite(t *testing.T) {
	suite.Run(t, new(MsgsTestSuite))
}

func (suite *MsgsTestSuite) SetupTest() {
	from, privFrom := utiltx.NewAddrKey()

	suite.signer = utiltx.NewSigner(privFrom)
	suite.from = from
	suite.to = utiltx.GenerateAddress()
	suite.chainID = big.NewInt(1)
	suite.hundredBigInt = big.NewInt(100)

	encodingConfig := encoding.MakeConfig(suite.chainID.Uint64())
	suite.clientCtx = client.Context{}.WithTxConfig(encodingConfig.TxConfig)

	configurator := types.NewEVMConfigurator()
	configurator.ResetTestConfig()
}

func (suite *MsgsTestSuite) TestMsgEthereumTx_Constructor() {
	evmTx := &types.EvmTxArgs{
		Nonce:    0,
		To:       &suite.to,
		GasLimit: 100000,
		Input:    []byte("test"),
	}
	msg := types.NewTx(evmTx)

	// suite.Require().Equal(msg.Data.To, suite.to.Hex())
	suite.Require().Equal(msg.Route(), types.RouterKey)
	suite.Require().Equal(msg.Type(), types.TypeMsgEthereumTx)
	// suite.Require().NotNil(msg.To())
	suite.Require().Equal(msg.GetMsgs(), []sdk.Msg{msg})
	suite.Require().Empty(msg.GetSigners())

	evmTx2 := &types.EvmTxArgs{
		Nonce:    0,
		GasLimit: 100000,
		Input:    []byte("test"),
	}
	msg = types.NewTx(evmTx2)
	suite.Require().NotNil(msg)
	// suite.Require().Empty(msg.Data.To)
	// suite.Require().Nil(msg.To())
}

func (suite *MsgsTestSuite) TestMsgEthereumTx_BuildTx() {
	evmTx := &types.EvmTxArgs{
		Nonce:     0,
		To:        &suite.to,
		GasLimit:  100000,
		GasPrice:  big.NewInt(1e18),
		GasFeeCap: big.NewInt(1e18),
		GasTipCap: big.NewInt(0),
		Input:     []byte("test"),
	}
	testCases := []struct {
		name     string
		msg      *types.MsgEthereumTx
		expError bool
	}{
		{
			"build tx - pass",
			types.NewTx(evmTx),
			false,
		},
		{
			"build tx - nil data",
			types.NewTx(evmTx),
			false,
		},
	}
	for _, coinInfo := range []types.EvmCoinInfo{
		testconstants.ExampleChainCoinInfo[testconstants.SixDecimalsChainID],
		testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID],
	} {
		for _, tc := range testCases {
			configurator := types.NewEVMConfigurator()
			configurator.ResetTestConfig()
			suite.Require().NoError(configurator.WithEVMCoinInfo(coinInfo).Configure())

			baseDenom := types.GetEVMCoinDenom()
			extendedDenom := types.GetEVMCoinExtendedDenom()

			tx, err := tc.msg.BuildTx(suite.clientCtx.TxConfig.NewTxBuilder(), baseDenom)
			if tc.expError {
				suite.Require().Error(err)
			} else {
				suite.Require().NoError(err)

				suite.Require().Empty(tx.GetMemo())
				suite.Require().Empty(tx.GetTimeoutHeight())
				suite.Require().Equal(uint64(100000), tx.GetGas())

				expFeeAmt := sdkmath.NewIntFromBigInt(evmTx.GasPrice).MulRaw(int64(evmTx.GasLimit)) //#nosec
				expFee := sdk.NewCoins(sdk.NewCoin(extendedDenom, expFeeAmt))
				suite.Require().Equal(expFee, tx.GetFee())
			}
		}
	}
}

func (suite *MsgsTestSuite) TestMsgEthereumTx_ValidateBasic() {
	var (
		hundredInt   = big.NewInt(100)
		validChainID = big.NewInt(9000)
		zeroInt      = big.NewInt(0)
		minusOneInt  = big.NewInt(-1)
		//nolint:all
		exp_2_255 = new(big.Int).Exp(big.NewInt(2), big.NewInt(255), nil)
	)
	testCases := []struct {
		msg        string
		to         string
		amount     *big.Int
		gasLimit   uint64
		gasPrice   *big.Int
		gasFeeCap  *big.Int
		gasTipCap  *big.Int
		from       string
		accessList *ethtypes.AccessList
		chainID    *big.Int
		expectPass bool
		errMsg     string
	}{
		{
			msg:        "pass with recipient - Legacy Tx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   hundredInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "pass with recipient - AccessList Tx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   zeroInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			accessList: &ethtypes.AccessList{},
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "pass with recipient - DynamicFee Tx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   zeroInt,
			gasFeeCap:  hundredInt,
			gasTipCap:  zeroInt,
			accessList: &ethtypes.AccessList{},
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "pass contract - Legacy Tx",
			to:         "",
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   hundredInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "maxInt64 gas limit overflow",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   math.MaxInt64 + 1,
			gasPrice:   hundredInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "nil amount - Legacy Tx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     nil,
			gasLimit:   21000,
			gasPrice:   hundredInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "negative amount - Legacy Tx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     minusOneInt,
			gasLimit:   21000,
			gasPrice:   hundredInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			chainID:    validChainID,
			expectPass: false,
			errMsg:     "negative value",
		},
		{
			msg:        "zero gas limit - Legacy Tx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   0,
			gasPrice:   hundredInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			chainID:    validChainID,
			expectPass: false,
			errMsg:     "intrinsic gas too low",
		},
		{
			msg:        "negative gas price - Legacy Tx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   minusOneInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			chainID:    validChainID,
			expectPass: false,
			errMsg:     "transaction gas price below minimum",
		},
		{
			msg:        "zero gas price - Legacy Tx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   zeroInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "out of bound gas fee - Legacy Tx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   exp_2_255,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "nil amount - AccessListTx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     nil,
			gasLimit:   21000,
			gasPrice:   hundredInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			accessList: &ethtypes.AccessList{},
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "negative amount - AccessListTx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     minusOneInt,
			gasLimit:   21000,
			gasPrice:   hundredInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			accessList: &ethtypes.AccessList{},
			chainID:    validChainID,
			expectPass: false,
			errMsg:     "negative value",
		},
		{
			msg:        "zero gas limit - AccessListTx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   0,
			gasPrice:   big.NewInt(1),
			gasFeeCap:  nil,
			gasTipCap:  nil,
			accessList: &ethtypes.AccessList{},
			chainID:    validChainID,
			expectPass: false,
			errMsg:     "intrinsic gas too low",
		},
		{
			msg:        "nil gas price - AccessListTx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   nil,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			accessList: &ethtypes.AccessList{},
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "negative gas price - AccessListTx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   minusOneInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			accessList: &ethtypes.AccessList{},
			chainID:    validChainID,
			expectPass: false,
			errMsg:     "transaction gas price below minimum",
		},
		{
			msg:        "zero gas price - AccessListTx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   zeroInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			accessList: &ethtypes.AccessList{},
			chainID:    validChainID,
			expectPass: true,
		},
		{
			msg:        "chain ID not set on AccessListTx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   zeroInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			accessList: &ethtypes.AccessList{},
			chainID:    nil,
			expectPass: true,
		},
		{
			msg:        "nil tx.Data - AccessList Tx",
			to:         suite.to.Hex(),
			from:       suite.from.Hex(),
			amount:     hundredInt,
			gasLimit:   21000,
			gasPrice:   zeroInt,
			gasFeeCap:  nil,
			gasTipCap:  nil,
			accessList: &ethtypes.AccessList{},
			expectPass: true,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.msg, func() {
			to := common.HexToAddress(tc.to)
			evmTx := &types.EvmTxArgs{
				ChainID:   tc.chainID,
				Nonce:     1,
				To:        &to,
				Amount:    tc.amount,
				GasLimit:  tc.gasLimit,
				GasPrice:  tc.gasPrice,
				GasFeeCap: tc.gasFeeCap,
				Accesses:  tc.accessList,
			}
			tx := types.NewTx(evmTx)
			tx.From = common.HexToAddress(tc.from).Bytes()

			// for legacy_Tx need to sign tx because the chainID is derived
			// from signature
			if tc.accessList == nil && tc.from == suite.from.Hex() {
				ethSigner := ethtypes.LatestSignerForChainID(tc.chainID)
				err := tx.Sign(ethSigner, suite.signer)
				suite.Require().NoError(err)
			}

			err := tx.ValidateBasic()

			if tc.expectPass {
				suite.Require().NoError(err)
			} else {
				suite.Require().Error(err)
				suite.Require().Contains(err.Error(), tc.errMsg)
			}
		})
	}
}

func (suite *MsgsTestSuite) TestMsgEthereumTx_Sign() {
	testCases := []struct {
		msg        string
		txParams   *types.EvmTxArgs
		ethSigner  ethtypes.Signer
		malleate   func(tx *types.MsgEthereumTx)
		expectPass bool
	}{
		{
			"pass - EIP2930 signer",
			&types.EvmTxArgs{
				ChainID:  suite.chainID,
				Nonce:    0,
				To:       &suite.to,
				GasLimit: 100000,
				Input:    []byte("test"),
				Accesses: &ethtypes.AccessList{},
			},
			ethtypes.NewEIP2930Signer(suite.chainID),
			func(tx *types.MsgEthereumTx) { tx.From = suite.from.Bytes() },
			true,
		},
		{
			"pass - EIP155 signer",
			&types.EvmTxArgs{
				ChainID:  suite.chainID,
				Nonce:    0,
				To:       &suite.to,
				GasLimit: 100000,
				Input:    []byte("test"),
			},
			ethtypes.NewEIP155Signer(suite.chainID),
			func(tx *types.MsgEthereumTx) { tx.From = suite.from.Bytes() },
			true,
		},
		{
			"pass - Homestead signer",
			&types.EvmTxArgs{
				ChainID:  suite.chainID,
				Nonce:    0,
				To:       &suite.to,
				GasLimit: 100000,
				Input:    []byte("test"),
			},
			ethtypes.HomesteadSigner{},
			func(tx *types.MsgEthereumTx) { tx.From = suite.from.Bytes() },
			true,
		},
		{
			"pass - Frontier signer",
			&types.EvmTxArgs{
				ChainID:  suite.chainID,
				Nonce:    0,
				To:       &suite.to,
				GasLimit: 100000,
				Input:    []byte("test"),
			},
			ethtypes.FrontierSigner{},
			func(tx *types.MsgEthereumTx) { tx.From = suite.from.Bytes() },
			true,
		},
		{
			"no from address ",
			&types.EvmTxArgs{
				ChainID:  suite.chainID,
				Nonce:    0,
				To:       &suite.to,
				GasLimit: 100000,
				Input:    []byte("test"),
				Accesses: &ethtypes.AccessList{},
			},
			ethtypes.NewEIP2930Signer(suite.chainID),
			func(tx *types.MsgEthereumTx) { tx.From = []byte{} },
			false,
		},
		{
			"from address ≠ signer address",
			&types.EvmTxArgs{
				ChainID:  suite.chainID,
				Nonce:    0,
				To:       &suite.to,
				GasLimit: 100000,
				Input:    []byte("test"),
				Accesses: &ethtypes.AccessList{},
			},
			ethtypes.NewEIP2930Signer(suite.chainID),
			func(tx *types.MsgEthereumTx) { tx.From = suite.to.Bytes() },
			false,
		},
	}

	for i, tc := range testCases {
		tx := types.NewTx(tc.txParams)
		tc.malleate(tx)
		err := tx.Sign(tc.ethSigner, suite.signer)
		if tc.expectPass {
			suite.Require().NoError(err, "valid test %d failed: %s", i, tc.msg)
			sender, err := tx.GetSenderLegacy(ethtypes.LatestSignerForChainID(suite.chainID))
			suite.Require().NoError(err, tc.msg)
			suite.Require().Equal(tx.From, sender.Bytes(), tc.msg)
		} else {
			suite.Require().Error(err, "invalid test %d passed: %s", i, tc.msg)
		}
	}
}

func (suite *MsgsTestSuite) TestMsgEthereumTx_Getters() {
	evmTx := &types.EvmTxArgs{
		ChainID:  suite.chainID,
		Nonce:    0,
		To:       &suite.to,
		GasLimit: 50,
		GasPrice: suite.hundredBigInt,
		Accesses: &ethtypes.AccessList{},
	}
	testCases := []struct {
		name      string
		ethSigner ethtypes.Signer
		exp       *big.Int
	}{
		{
			"get fee - pass",
			ethtypes.NewEIP2930Signer(suite.chainID),
			big.NewInt(5000),
		},
		{
			"get fee - nil data",
			ethtypes.NewEIP2930Signer(suite.chainID),
			big.NewInt(5000),
		},
		{
			"get effective fee - pass",
			ethtypes.NewEIP2930Signer(suite.chainID),
			big.NewInt(5000),
		},
		{
			"get effective fee - nil data",
			ethtypes.NewEIP2930Signer(suite.chainID),
			big.NewInt(5000),
		},
		{
			"get gas - pass",
			ethtypes.NewEIP2930Signer(suite.chainID),
			big.NewInt(50),
		},
		{
			"get gas - nil data",
			ethtypes.NewEIP2930Signer(suite.chainID),
			big.NewInt(50),
		},
	}

	var fee, effFee *big.Int
	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			tx := types.NewTx(evmTx)
			switch {
			case strings.Contains(tc.name, "get fee"):
				fee = tx.GetFee()
				suite.Require().Equal(tc.exp, fee)
			case strings.Contains(tc.name, "get effective fee"):
				effFee = tx.GetEffectiveFee(big.NewInt(0))
				suite.Require().Equal(tc.exp, effFee)
			case strings.Contains(tc.name, "get gas"):
				gas := tx.GetGas()
				suite.Require().Equal(tc.exp.Uint64(), gas)
			}
		})
	}
}

// TestTransactionCoding tests serializing/de-serializing to/from rlp and JSON.
// adapted from go-ethereum
func (suite *MsgsTestSuite) TestTransactionCoding() {
	key, err := crypto.GenerateKey()
	if err != nil {
		suite.T().Fatalf("could not generate key: %v", err)
	}
	var (
		signer    = ethtypes.NewEIP2930Signer(common.Big1)
		addr      = common.HexToAddress("0x0000000000000000000000000000000000000001")
		recipient = common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87")
		accesses  = ethtypes.AccessList{{Address: addr, StorageKeys: []common.Hash{{0}}}}
	)
	for i := uint64(0); i < 500; i++ {
		var txdata ethtypes.TxData
		switch i % 5 {
		case 0:
			// Legacy tx.
			txdata = &ethtypes.LegacyTx{
				Nonce:    i,
				To:       &recipient,
				Gas:      1,
				GasPrice: big.NewInt(2),
				Data:     []byte("abcdef"),
			}
		case 1:
			// Legacy tx contract creation.
			txdata = &ethtypes.LegacyTx{
				Nonce:    i,
				Gas:      1,
				GasPrice: big.NewInt(2),
				Data:     []byte("abcdef"),
			}
		case 2:
			// Tx with non-zero access list.
			txdata = &ethtypes.AccessListTx{
				ChainID:    big.NewInt(1),
				Nonce:      i,
				To:         &recipient,
				Gas:        123457,
				GasPrice:   big.NewInt(10),
				AccessList: accesses,
				Data:       []byte("abcdef"),
			}
		case 3:
			// Tx with empty access list.
			txdata = &ethtypes.AccessListTx{
				ChainID:  big.NewInt(1),
				Nonce:    i,
				To:       &recipient,
				Gas:      123457,
				GasPrice: big.NewInt(10),
				Data:     []byte("abcdef"),
			}
		case 4:
			// Contract creation with access list.
			txdata = &ethtypes.AccessListTx{
				ChainID:    big.NewInt(1),
				Nonce:      i,
				Gas:        123457,
				GasPrice:   big.NewInt(10),
				AccessList: accesses,
			}
		}
		tx, err := ethtypes.SignNewTx(key, signer, txdata)
		if err != nil {
			suite.T().Fatalf("could not sign transaction: %v", err)
		}
		// RLP
		parsedTx, err := encodeDecodeBinary(tx, signer.ChainID())
		if err != nil {
			suite.T().Fatal(err)
		}
		err = assertEqual(parsedTx.AsTransaction(), tx)
		suite.Require().NoError(err)
	}
}

func encodeDecodeBinary(tx *ethtypes.Transaction, chainID *big.Int) (*types.MsgEthereumTx, error) {
	data, err := tx.MarshalBinary()
	if err != nil {
		return nil, fmt.Errorf("rlp encoding failed: %v", err)
	}
	parsedTx := &types.MsgEthereumTx{}
	if err := parsedTx.UnmarshalBinary(data, ethtypes.LatestSignerForChainID(chainID)); err != nil {
		return nil, fmt.Errorf("rlp decoding failed: %v", err)
	}
	return parsedTx, nil
}

func assertEqual(orig *ethtypes.Transaction, cpy *ethtypes.Transaction) error {
	// compare nonce, price, gaslimit, recipient, amount, payload, V, R, S
	if want, got := orig.Hash(), cpy.Hash(); want != got {
		return fmt.Errorf("parsed tx differs from original tx, want %v, got %v", want, got)
	}
	if want, got := orig.ChainId(), cpy.ChainId(); want.Cmp(got) != 0 {
		return fmt.Errorf("invalid chain id, want %d, got %d", want, got)
	}
	if orig.AccessList() != nil {
		if !reflect.DeepEqual(orig.AccessList(), cpy.AccessList()) {
			return fmt.Errorf("access list wrong")
		}
	}
	return nil
}
