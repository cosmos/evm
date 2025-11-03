package vm

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"math/big"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	ethlogger "github.com/ethereum/go-ethereum/eth/tracers/logger"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/server/config"
	testconstants "github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	"github.com/cosmos/evm/testutil/tx"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	types2 "github.com/cosmos/evm/x/precisebank/types"
	"github.com/cosmos/evm/x/vm/keeper/testdata"
	"github.com/cosmos/evm/x/vm/statedb"
	"github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	vestingtypes "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
)

// Not valid Ethereum address
const invalidAddress = "0x0000"

func (s *KeeperTestSuite) TestQueryAccount() {
	baseDenom := types.GetEVMCoinDenom()
	testCases := []struct {
		msg         string
		getReq      func() *types.QueryAccountRequest
		expResponse *types.QueryAccountResponse
		expPass     bool
	}{
		{
			"invalid address",
			func() *types.QueryAccountRequest {
				return &types.QueryAccountRequest{
					Address: invalidAddress,
				}
			},
			nil,
			false,
		},
		{
			"success",
			func() *types.QueryAccountRequest {
				amt := sdk.Coins{sdk.NewInt64Coin(baseDenom, 100)}

				// Add new unfunded key
				index := s.Keyring.AddKey()
				addr := s.Keyring.GetAddr(index)

				err := s.Network.App.GetBankKeeper().MintCoins(
					s.Network.GetContext(),
					types.ModuleName,
					amt,
				)
				s.Require().NoError(err)

				err = s.Network.App.GetBankKeeper().SendCoinsFromModuleToAccount(
					s.Network.GetContext(),
					types.ModuleName,
					addr.Bytes(),
					amt,
				)
				s.Require().NoError(err)

				return &types.QueryAccountRequest{
					Address: addr.String(),
				}
			},
			&types.QueryAccountResponse{
				Balance:  "100",
				CodeHash: common.BytesToHash(crypto.Keccak256(nil)).Hex(),
				Nonce:    0,
			},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			req := tc.getReq()
			expectedResponse := tc.expResponse

			ctx := s.Network.GetContext()
			// Function under test
			res, err := s.Network.GetEvmClient().Account(ctx, req)

			s.Require().Equal(expectedResponse, res)

			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestQueryCosmosAccount() {
	testCases := []struct {
		msg           string
		getReqAndResp func() (*types.QueryCosmosAccountRequest, *types.QueryCosmosAccountResponse)
		expPass       bool
	}{
		{
			"invalid address",
			func() (*types.QueryCosmosAccountRequest, *types.QueryCosmosAccountResponse) {
				req := &types.QueryCosmosAccountRequest{
					Address: invalidAddress,
				}
				return req, nil
			},
			false,
		},
		{
			"success",
			func() (*types.QueryCosmosAccountRequest, *types.QueryCosmosAccountResponse) {
				key := s.Keyring.GetKey(0)
				expAccount := &types.QueryCosmosAccountResponse{
					CosmosAddress: key.AccAddr.String(),
					Sequence:      0,
					AccountNumber: 0,
				}
				req := &types.QueryCosmosAccountRequest{
					Address: key.Addr.String(),
				}

				return req, expAccount
			},
			true,
		},
		{
			"success with seq and account number",
			func() (*types.QueryCosmosAccountRequest, *types.QueryCosmosAccountResponse) {
				index := s.Keyring.AddKey()
				newKey := s.Keyring.GetKey(index)
				accountNumber := uint64(100)
				acc := s.Network.App.GetAccountKeeper().NewAccountWithAddress(
					s.Network.GetContext(),
					newKey.AccAddr,
				)

				s.Require().NoError(acc.SetSequence(10))
				s.Require().NoError(acc.SetAccountNumber(accountNumber))
				s.Network.App.GetAccountKeeper().SetAccount(s.Network.GetContext(), acc)

				expAccount := &types.QueryCosmosAccountResponse{
					CosmosAddress: newKey.AccAddr.String(),
					Sequence:      10,
					AccountNumber: accountNumber,
				}

				req := &types.QueryCosmosAccountRequest{
					Address: newKey.Addr.String(),
				}
				return req, expAccount
			},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			req, expectedResponse := tc.getReqAndResp()

			ctx := s.Network.GetContext()

			// Function under test
			res, err := s.Network.GetEvmClient().CosmosAccount(ctx, req)

			s.Require().Equal(expectedResponse, res)

			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestQueryBalance() {
	baseDenom := types.GetEVMCoinDenom()

	testCases := []struct {
		msg           string
		getReqAndResp func() (*types.QueryBalanceRequest, *types.QueryBalanceResponse)
		expPass       bool
	}{
		{
			"invalid address",
			func() (*types.QueryBalanceRequest, *types.QueryBalanceResponse) {
				req := &types.QueryBalanceRequest{
					Address: invalidAddress,
				}
				return req, nil
			},
			false,
		},
		{
			"success",
			func() (*types.QueryBalanceRequest, *types.QueryBalanceResponse) {
				newIndex := s.Keyring.AddKey()
				addr := s.Keyring.GetAddr(newIndex)

				balance := int64(100)
				amt := sdk.Coins{sdk.NewInt64Coin(baseDenom, balance)}

				err := s.Network.App.GetBankKeeper().MintCoins(s.Network.GetContext(), types.ModuleName, amt)
				s.Require().NoError(err)
				err = s.Network.App.GetBankKeeper().SendCoinsFromModuleToAccount(s.Network.GetContext(), types.ModuleName, addr.Bytes(), amt)
				s.Require().NoError(err)

				req := &types.QueryBalanceRequest{
					Address: addr.String(),
				}
				return req, &types.QueryBalanceResponse{
					Balance: fmt.Sprint(balance),
				}
			},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			req, resp := tc.getReqAndResp()

			ctx := s.Network.GetContext()
			res, err := s.Network.GetEvmClient().Balance(ctx, req)

			s.Require().Equal(resp, res)
			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestQueryStorage() {
	testCases := []struct {
		msg           string
		getReqAndResp func() (*types.QueryStorageRequest, *types.QueryStorageResponse)
		expPass       bool
	}{
		{
			"invalid address",
			func() (*types.QueryStorageRequest, *types.QueryStorageResponse) {
				req := &types.QueryStorageRequest{
					Address: invalidAddress,
				}
				return req, nil
			},
			false,
		},
		{
			"success",
			func() (*types.QueryStorageRequest, *types.QueryStorageResponse) {
				key := common.BytesToHash([]byte("key"))
				value := []byte("value")
				expValue := common.BytesToHash(value)

				newIndex := s.Keyring.AddKey()
				addr := s.Keyring.GetAddr(newIndex)

				s.Network.App.GetEVMKeeper().SetState(
					s.Network.GetContext(),
					addr,
					key,
					value,
				)

				req := &types.QueryStorageRequest{
					Address: addr.String(),
					Key:     key.String(),
				}
				return req, &types.QueryStorageResponse{
					Value: expValue.String(),
				}
			},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			req, expectedResp := tc.getReqAndResp()

			ctx := s.Network.GetContext()
			res, err := s.Network.GetEvmClient().Storage(ctx, req)

			s.Require().Equal(expectedResp, res)

			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestQueryCode() {
	var (
		req     *types.QueryCodeRequest
		expCode []byte
	)

	testCases := []struct {
		msg           string
		getReqAndResp func() (*types.QueryCodeRequest, *types.QueryCodeResponse)
		expPass       bool
	}{
		{
			"invalid address",
			func() (*types.QueryCodeRequest, *types.QueryCodeResponse) {
				req = &types.QueryCodeRequest{
					Address: invalidAddress,
				}
				return req, nil
			},
			false,
		},
		{
			"success",
			func() (*types.QueryCodeRequest, *types.QueryCodeResponse) {
				newIndex := s.Keyring.AddKey()
				addr := s.Keyring.GetAddr(newIndex)

				expCode = []byte("code")
				stateDB := s.Network.GetStateDB()
				stateDB.SetCode(addr, expCode)
				s.Require().NoError(stateDB.Commit())

				req = &types.QueryCodeRequest{
					Address: addr.String(),
				}
				return req, &types.QueryCodeResponse{
					Code: hexutil.Bytes(expCode),
				}
			},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			req, expectedResponse := tc.getReqAndResp()

			ctx := s.Network.GetContext()
			res, err := s.Network.GetEvmClient().Code(ctx, req)

			s.Require().Equal(expectedResponse, res)
			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

// TODO: Fix this one
func (s *KeeperTestSuite) TestQueryTxLogs() {
	expLogs := []*types.Log{}
	txHash := common.BytesToHash([]byte("tx_hash"))
	txIndex := uint(1)
	logIndex := uint(0)

	testCases := []struct {
		msg      string
		malleate func(vm.StateDB)
	}{
		{
			"empty logs",
			func(vm.StateDB) {
				expLogs = nil
			},
		},
		{
			"success",
			func(vmdb vm.StateDB) {
				addr := s.Keyring.GetAddr(0)
				expLogs = []*types.Log{
					{
						Address:     addr.String(),
						Topics:      []string{common.BytesToHash([]byte("topic")).String()},
						Data:        []byte("data"),
						BlockNumber: 1,
						TxHash:      txHash.String(),
						TxIndex:     uint64(txIndex),
						BlockHash:   common.BytesToHash(s.Network.GetContext().HeaderHash()).Hex(),
						Index:       uint64(logIndex),
						Removed:     false,
					},
				}

				for _, log := range types.LogsToEthereum(expLogs) {
					vmdb.AddLog(log)
				}
			},
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			txCfg := statedb.NewTxConfig(
				txHash,
				txIndex,
			)
			vmdb := statedb.New(
				s.Network.GetContext(),
				s.Network.App.GetEVMKeeper(),
				txCfg,
			)

			tc.malleate(vmdb)
			s.Require().NoError(vmdb.Commit())

			logs := vmdb.Logs()
			s.Require().Equal(expLogs, types.NewLogsFromEth(logs))
		})
	}
}

func (s *KeeperTestSuite) TestQueryParams() {
	ctx := s.Network.GetContext()
	expParams := types.DefaultParams()
	expParams.ActiveStaticPrecompiles = types.AvailableStaticPrecompiles
	expParams.ExtraEIPs = nil

	res, err := s.Network.GetEvmClient().Params(ctx, &types.QueryParamsRequest{})
	s.Require().NoError(err)
	s.Require().Equal(expParams, res.Params)
}

func (s *KeeperTestSuite) TestQueryValidatorAccount() {
	testCases := []struct {
		msg           string
		getReqAndResp func() (*types.QueryValidatorAccountRequest, *types.QueryValidatorAccountResponse)
		expPass       bool
	}{
		{
			"invalid address",
			func() (*types.QueryValidatorAccountRequest, *types.QueryValidatorAccountResponse) {
				req := &types.QueryValidatorAccountRequest{
					ConsAddress: "",
				}
				return req, nil
			},
			false,
		},
		{
			"success",
			func() (*types.QueryValidatorAccountRequest, *types.QueryValidatorAccountResponse) {
				val := s.Network.GetValidators()[0]
				consAddr, err := val.GetConsAddr()
				s.Require().NoError(err)

				req := &types.QueryValidatorAccountRequest{
					ConsAddress: sdk.ConsAddress(consAddr).String(),
				}

				addrBz, err := s.Network.App.GetStakingKeeper().ValidatorAddressCodec().StringToBytes(val.OperatorAddress)
				s.Require().NoError(err)

				resp := &types.QueryValidatorAccountResponse{
					AccountAddress: sdk.AccAddress(addrBz).String(),
					Sequence:       0,
					AccountNumber:  2,
				}

				return req, resp
			},
			true,
		},
		{
			"success with seq and account number",
			func() (*types.QueryValidatorAccountRequest, *types.QueryValidatorAccountResponse) {
				val := s.Network.GetValidators()[0]
				consAddr, err := val.GetConsAddr()
				s.Require().NoError(err)

				// create validator account and set sequence and account number
				accNumber := uint64(100)
				accSeq := uint64(10)

				addrBz, err := s.Network.App.GetStakingKeeper().ValidatorAddressCodec().StringToBytes(val.OperatorAddress)
				s.Require().NoError(err)

				accAddrStr := sdk.AccAddress(addrBz).String()

				baseAcc := &authtypes.BaseAccount{Address: accAddrStr}
				acc := s.Network.App.GetAccountKeeper().NewAccount(s.Network.GetContext(), baseAcc)
				s.Require().NoError(acc.SetSequence(accSeq))
				s.Require().NoError(acc.SetAccountNumber(accNumber))
				s.Network.App.GetAccountKeeper().SetAccount(s.Network.GetContext(), acc)

				resp := &types.QueryValidatorAccountResponse{
					AccountAddress: accAddrStr,
					Sequence:       accSeq,
					AccountNumber:  accNumber,
				}
				req := &types.QueryValidatorAccountRequest{
					ConsAddress: sdk.ConsAddress(consAddr).String(),
				}

				return req, resp
			},
			true,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			req, resp := tc.getReqAndResp()
			ctx := s.Network.GetContext()
			res, err := s.Network.GetEvmClient().ValidatorAccount(ctx, req)

			s.Require().Equal(resp, res)
			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestEstimateGas() {
	gasHelper := hexutil.Uint64(20000)
	higherGas := hexutil.Uint64(25000)
	// Hardcode recipient address to avoid non determinism in tests
	hardcodedRecipient := common.HexToAddress("0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101")

	erc20Contract, err := testdata.LoadERC20Contract()
	s.Require().NoError(err)

	testCases := []struct {
		msg             string
		getArgs         func() types.TransactionArgs
		expPass         bool
		expGas          uint64
		EnableFeemarket bool
		gasCap          uint64
	}{
		// should success, because transfer value is zero
		{
			"success - default args - special case for ErrIntrinsicGas on contract creation, raise gas limit",
			func() types.TransactionArgs {
				return types.TransactionArgs{}
			},
			true,
			53793,
			false,
			config.DefaultGasCap,
		},
		// should success, because transfer value is zero
		{
			"success - default args with 'to' address",
			func() types.TransactionArgs {
				return types.TransactionArgs{To: &common.Address{}}
			},
			true,
			ethparams.TxGas,
			false,
			config.DefaultGasCap,
		},
		// should fail, because the default From address(zero address) don't have fund
		{
			"fail - not enough balance",
			func() types.TransactionArgs {
				return types.TransactionArgs{
					To:    &common.Address{},
					Value: (*hexutil.Big)(big.NewInt(100)),
				}
			},
			false,
			0,
			false,
			config.DefaultGasCap,
		},
		// should success, enough balance now
		{
			"success - enough balance",
			func() types.TransactionArgs {
				addr := s.Keyring.GetAddr(0)
				return types.TransactionArgs{
					To:    &common.Address{},
					From:  &addr,
					Value: (*hexutil.Big)(big.NewInt(100)),
				}
			},
			true,
			ethparams.TxGas,
			false,
			config.DefaultGasCap,
		},
		{
			"fail - not enough balance w/ gas fee cap",
			func() types.TransactionArgs {
				addr := s.Keyring.GetAddr(0)
				hexBigInt := hexutil.Big(*big.NewInt(1))
				balance := s.Network.App.GetBankKeeper().GetBalance(s.Network.GetContext(), sdk.AccAddress(addr.Bytes()), types.GetEVMCoinDenom())
				value := balance.Amount.Add(sdkmath.NewInt(1))
				return types.TransactionArgs{
					To:           &common.Address{},
					From:         &addr,
					Value:        (*hexutil.Big)(value.BigInt()),
					MaxFeePerGas: &hexBigInt,
				}
			},
			false,
			0,
			false,
			config.DefaultGasCap,
		},
		{
			"fail - insufficient funds for gas * price + value w/ gas fee cap",
			func() types.TransactionArgs {
				addr := s.Keyring.GetAddr(0)
				hexBigInt := hexutil.Big(*big.NewInt(1))
				balance := s.Network.App.GetBankKeeper().GetBalance(s.Network.GetContext(), sdk.AccAddress(addr.Bytes()), types.GetEVMCoinDenom())
				value := balance.Amount.Sub(sdkmath.NewInt(1))
				return types.TransactionArgs{
					To:           &common.Address{},
					From:         &addr,
					Value:        (*hexutil.Big)(value.BigInt()),
					MaxFeePerGas: &hexBigInt,
				}
			},
			false,
			0,
			false,
			config.DefaultGasCap,
		},
		// should success, because gas limit lower than 21000 is ignored
		{
			"gas exceed allowance",
			func() types.TransactionArgs {
				return types.TransactionArgs{To: &common.Address{}, Gas: &gasHelper}
			},
			true,
			ethparams.TxGas,
			false,
			config.DefaultGasCap,
		},
		// should fail, invalid gas cap
		{
			"gas exceed global allowance",
			func() types.TransactionArgs {
				return types.TransactionArgs{To: &common.Address{}}
			},
			false,
			0,
			false,
			20000,
		},
		// estimate gas of an erc20 contract deployment, the exact gas number is checked with geth
		{
			"contract deployment",
			func() types.TransactionArgs {
				ctorArgs, err := erc20Contract.ABI.Pack(
					"",
					&hardcodedRecipient,
					sdkmath.NewIntWithDecimal(1000, 18).BigInt(),
				)
				s.Require().NoError(err)
				data := erc20Contract.Bin
				data = append(data, ctorArgs...)

				addr := s.Keyring.GetAddr(0)
				return types.TransactionArgs{
					Data: (*hexutil.Bytes)(&data),
					From: &addr,
				}
			},
			true,
			1197697,
			false,
			config.DefaultGasCap,
		},
		// estimate gas of an erc20 transfer, the exact gas number is checked with geth
		{
			"erc20 transfer",
			func() types.TransactionArgs {
				key := s.Keyring.GetKey(0)
				contractAddr, err := deployErc20Contract(key, s.Factory)
				s.Require().NoError(err)

				err = s.Network.NextBlock()
				s.Require().NoError(err)

				transferData, err := erc20Contract.ABI.Pack(
					"transfer",
					hardcodedRecipient,
					big.NewInt(1000),
				)
				s.Require().NoError(err)
				return types.TransactionArgs{
					To:   &contractAddr,
					Data: (*hexutil.Bytes)(&transferData),
					From: &key.Addr,
				}
			},
			true,
			52669,
			false,
			config.DefaultGasCap,
		},
		// repeated tests with EnableFeemarket
		{
			"default args w/ EnableFeemarket",
			func() types.TransactionArgs {
				return types.TransactionArgs{To: &common.Address{}}
			},
			true,
			ethparams.TxGas,
			true,
			config.DefaultGasCap,
		},
		{
			"not enough balance w/ EnableFeemarket",
			func() types.TransactionArgs {
				return types.TransactionArgs{
					To:    &common.Address{},
					Value: (*hexutil.Big)(big.NewInt(100)),
				}
			},
			false,
			0,
			true,
			config.DefaultGasCap,
		},
		{
			"enough balance w/ EnableFeemarket",
			func() types.TransactionArgs {
				addr := s.Keyring.GetAddr(0)
				return types.TransactionArgs{
					To:    &common.Address{},
					From:  &addr,
					Value: (*hexutil.Big)(big.NewInt(100)),
				}
			},
			true,
			ethparams.TxGas,
			true,
			config.DefaultGasCap,
		},
		{
			"gas exceed allowance w/ EnableFeemarket",
			func() types.TransactionArgs {
				return types.TransactionArgs{To: &common.Address{}, Gas: &gasHelper}
			},
			true,
			ethparams.TxGas,
			true,
			config.DefaultGasCap,
		},
		{
			"gas exceed global allowance w/ EnableFeemarket",
			func() types.TransactionArgs {
				return types.TransactionArgs{To: &common.Address{}}
			},
			false,
			0,
			true,
			20000,
		},
		{
			"contract deployment w/ EnableFeemarket",
			func() types.TransactionArgs {
				ctorArgs, err := erc20Contract.ABI.Pack(
					"",
					&hardcodedRecipient,
					sdkmath.NewIntWithDecimal(1000, 18).BigInt(),
				)
				s.Require().NoError(err)
				data := erc20Contract.Bin
				data = append(data, ctorArgs...)

				sender := s.Keyring.GetAddr(0)
				return types.TransactionArgs{
					Data: (*hexutil.Bytes)(&data),
					From: &sender,
				}
			},
			true,
			1197697,
			true,
			config.DefaultGasCap,
		},
		{
			"erc20 transfer w/ EnableFeemarket",
			func() types.TransactionArgs {
				key := s.Keyring.GetKey(1)

				contractAddr, err := deployErc20Contract(key, s.Factory)
				s.Require().NoError(err)

				err = s.Network.NextBlock()
				s.Require().NoError(err)

				transferData, err := erc20Contract.ABI.Pack(
					"transfer",
					hardcodedRecipient,
					big.NewInt(1000),
				)
				s.Require().NoError(err)

				return types.TransactionArgs{
					To:   &contractAddr,
					From: &key.Addr,
					Data: (*hexutil.Bytes)(&transferData),
				}
			},
			true,
			52669,
			true,
			config.DefaultGasCap,
		},
		{
			"contract creation but 'create' param disabled",
			func() types.TransactionArgs {
				addr := s.Keyring.GetAddr(0)
				ctorArgs, err := erc20Contract.ABI.Pack(
					"",
					&addr,
					sdkmath.NewIntWithDecimal(1000, 18).BigInt(),
				)
				s.Require().NoError(err)

				data := erc20Contract.Bin
				data = append(data, ctorArgs...)

				args := types.TransactionArgs{
					From: &addr,
					Data: (*hexutil.Bytes)(&data),
				}
				params := s.Network.App.GetEVMKeeper().GetParams(s.Network.GetContext())
				params.AccessControl = types.AccessControl{
					Create: types.AccessControlType{
						AccessType: types.AccessTypeRestricted,
					},
				}
				err = s.Network.App.GetEVMKeeper().SetParams(
					s.Network.GetContext(),
					params,
				)
				s.Require().NoError(err)

				return args
			},
			false,
			0,
			false,
			config.DefaultGasCap,
		},
		{
			"specified gas in args higher than ethparams.TxGas (21,000)",
			func() types.TransactionArgs {
				return types.TransactionArgs{
					To:  &common.Address{},
					Gas: &higherGas,
				}
			},
			true,
			ethparams.TxGas,
			false,
			config.DefaultGasCap,
		},
		{
			"specified gas in args higher than request gasCap",
			func() types.TransactionArgs {
				return types.TransactionArgs{
					To:  &common.Address{},
					Gas: &higherGas,
				}
			},
			true,
			ethparams.TxGas,
			false,
			22_000,
		},
		{
			"invalid args - specified both gasPrice and maxFeePerGas",
			func() types.TransactionArgs {
				hexBigInt := hexutil.Big(*big.NewInt(1))

				return types.TransactionArgs{
					To:           &common.Address{},
					GasPrice:     &hexBigInt,
					MaxFeePerGas: &hexBigInt,
				}
			},
			false,
			0,
			false,
			config.DefaultGasCap,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			// Start from a clean state
			s.Require().NoError(s.Network.NextBlock())

			// Update feemarket params per test
			evmParams := feemarkettypes.DefaultParams()
			if !tc.EnableFeemarket {
				evmParams = s.Network.App.GetFeeMarketKeeper().GetParams(
					s.Network.GetContext(),
				)
				evmParams.NoBaseFee = true
			}

			err := s.Network.App.GetFeeMarketKeeper().SetParams(
				s.Network.GetContext(),
				evmParams,
			)
			s.Require().NoError(err)

			// Get call args
			args := tc.getArgs()
			marshalArgs, err := json.Marshal(args)
			s.Require().NoError(err)

			req := types.EthCallRequest{
				Args:            marshalArgs,
				GasCap:          tc.gasCap,
				ProposerAddress: s.Network.GetContext().BlockHeader().ProposerAddress,
			}

			// Function under test
			rsp, err := s.Network.GetEvmClient().EstimateGas(
				s.Network.GetContext(),
				&req,
			)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(int64(tc.expGas), int64(rsp.Gas)) //#nosec G115
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestEstimateGasWithStateOverrides() {
	// Hardcode recipient address to avoid non determinism in tests
	hardcodedRecipient := common.HexToAddress("0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101")

	erc20Contract, err := testdata.LoadERC20Contract()
	s.Require().NoError(err)

	testCases := []struct {
		msg             string
		getArgs         func() types.TransactionArgs
		getOverrides    func() string
		expPass         bool
		expGas          uint64
		EnableFeemarket bool
		gasCap          uint64
	}{
		{
			"success - native transfer with balance override",
			func() types.TransactionArgs {
				addr := s.Keyring.GetAddr(0)
				recipient := common.HexToAddress("0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17")
				return types.TransactionArgs{
					From:  &addr,
					To:    &recipient,
					Value: (*hexutil.Big)(big.NewInt(10000000000000000)), // 0.01 ether
				}
			},
			func() string {
				// Override recipient's balance to 0
				return `{
					"0x963EBDf2e1f8DB8707D05FC75bfeFFBa1B5BaC17": {
						"balance": "0x0"
					}
				}`
			},
			true,
			ethparams.TxGas,
			false,
			config.DefaultGasCap,
		},
		{
			"success - erc20 transfer with code and storage override",
			func() types.TransactionArgs {
				addr := s.Keyring.GetAddr(0)
				contractAddr := common.HexToAddress("0x5555555555555555555555555555555555555555")

				// Prepare transfer(address,uint256) call data
				// 100 TOKEN with 18 decimals
				amount := new(big.Int)
				amount.SetString("100000000000000000000", 10)
				transferData, err := erc20Contract.ABI.Pack(
					"transfer",
					hardcodedRecipient,
					amount,
				)
				s.Require().NoError(err)

				return types.TransactionArgs{
					From:  &addr,
					To:    &contractAddr,
					Input: (*hexutil.Bytes)(&transferData),
				}
			},
			func() string {
				// Override contract code and sender's balance in ERC20 contract
				// Storage slot for balances[sender] - simplified for testing
				erc20Contract, err := testdata.LoadERC20Contract()
				s.Require().NoError(err)

				sender := s.Keyring.GetAddr(0)
				slot := crypto.Keccak256Hash(
					common.LeftPadBytes(sender.Bytes(), common.HashLength),
					make([]byte, common.HashLength),
				)

				amount := new(big.Int)
				amount.SetString("100000000000000000000", 10)

				contractHex := hex.EncodeToString(erc20Contract.Bin)
				runtimeIdx := strings.Index(contractHex, "f3fe")
				s.Require().Greater(runtimeIdx, -1)
				runtimeHex := contractHex[runtimeIdx+4:]

				overrides := map[string]map[string]interface{}{
					"0x5555555555555555555555555555555555555555": {
						"code": "0x" + runtimeHex,
						"stateDiff": map[string]string{
							slot.Hex(): fmt.Sprintf("0x%064x", amount),
						},
					},
				}

				bz, err := json.Marshal(overrides)
				s.Require().NoError(err)

				return string(bz)
			},
			true,
			52114,
			false,
			config.DefaultGasCap,
		},
		{
			"success - override account nonce",
			func() types.TransactionArgs {
				addr := s.Keyring.GetAddr(0)
				return types.TransactionArgs{
					From:  &addr,
					To:    &common.Address{},
					Value: (*hexutil.Big)(big.NewInt(100)),
				}
			},
			func() string {
				addr := s.Keyring.GetAddr(0)
				return fmt.Sprintf(`{
					"%s": {
						"nonce": "0x10"
					}
				}`, addr.Hex())
			},
			true,
			ethparams.TxGas,
			false,
			config.DefaultGasCap,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			// Start from a clean state
			s.Require().NoError(s.Network.NextBlock())

			// Update feemarket params per test
			evmParams := feemarkettypes.DefaultParams()
			if !tc.EnableFeemarket {
				evmParams = s.Network.App.GetFeeMarketKeeper().GetParams(
					s.Network.GetContext(),
				)
				evmParams.NoBaseFee = true
			}

			err := s.Network.App.GetFeeMarketKeeper().SetParams(
				s.Network.GetContext(),
				evmParams,
			)
			s.Require().NoError(err)

			// Get call args
			args := tc.getArgs()
			marshalArgs, err := json.Marshal(args)
			s.Require().NoError(err)

			// Get overrides
			overrides := json.RawMessage(tc.getOverrides())

			req := types.EthCallRequest{
				Args:            marshalArgs,
				GasCap:          tc.gasCap,
				ProposerAddress: s.Network.GetContext().BlockHeader().ProposerAddress,
				Overrides:       overrides,
			}

			// Function under test
			rsp, err := s.Network.GetEvmClient().EstimateGas(
				s.Network.GetContext(),
				&req,
			)
			if tc.expPass {
				s.Require().NoError(err)
				s.Require().Equal(int64(tc.expGas), int64(rsp.Gas)) //#nosec G115
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func getDefaultTraceTxRequest(unitNetwork network.Network) *types.QueryTraceTxRequest {
	ctx := unitNetwork.GetContext()
	chainID := unitNetwork.GetEIP155ChainID().Int64()
	return &types.QueryTraceTxRequest{
		BlockMaxGas: ctx.ConsensusParams().Block.MaxGas,
		ChainId:     chainID,
		BlockTime:   ctx.BlockTime(),
		TraceConfig: &types.TraceConfig{},
	}
}

func (s *KeeperTestSuite) TestTraceTx() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	s.SetupTest()

	// Hardcode recipient address to avoid non determinism in tests
	hardcodedRecipient := common.HexToAddress("0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101")

	erc20Contract, err := testdata.LoadERC20Contract()
	s.Require().NoError(err)

	testCases := []struct {
		msg             string
		malleate        func()
		getRequest      func() *types.QueryTraceTxRequest
		getPredecessors func() []*types.MsgEthereumTx
		expPass         bool
		expPanics       bool
		expectedTrace   string
	}{
		{
			msg: "default trace",
			getRequest: func() *types.QueryTraceTxRequest {
				return getDefaultTraceTxRequest(s.Network)
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: true,
			expectedTrace: "{\"gas\":34780,\"failed\":false," +
				"\"returnValue\":\"0x0000000000000000000000000000000000000000000000000000000000000001\"," +
				"\"structLogs\":[{\"pc\":0,\"op\":\"PUSH1\",\"gas",
		},
		{
			msg: "default trace with filtered response",
			getRequest: func() *types.QueryTraceTxRequest {
				defaultRequest := getDefaultTraceTxRequest(s.Network)
				defaultRequest.TraceConfig = &types.TraceConfig{
					DisableStack:   true,
					DisableStorage: true,
					EnableMemory:   false,
				}
				return defaultRequest
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: true,
			expectedTrace: "{\"gas\":34780,\"failed\":false," +
				"\"returnValue\":\"0x0000000000000000000000000000000000000000000000000000000000000001\"," +
				"\"structLogs\":[{\"pc\":0,\"op\":\"PUSH1\",\"gas",
		},
		{
			msg: "javascript tracer",
			getRequest: func() *types.QueryTraceTxRequest {
				traceConfig := &types.TraceConfig{
					Tracer: "{data: [], fault: function(log) {}, step: function(log) { if(log.op.toString() == \"CALL\") this.data.push(log.stack.peek(0)); }, result: function() { return this.data; }}",
				}
				defaultRequest := getDefaultTraceTxRequest(s.Network)
				defaultRequest.TraceConfig = traceConfig
				return defaultRequest
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass:       true,
			expectedTrace: "[]",
		},
		{
			msg: "default tracer with predecessors",
			getRequest: func() *types.QueryTraceTxRequest {
				return getDefaultTraceTxRequest(s.Network)
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				// create predecessor tx
				// Use different address to avoid nonce collision
				senderKey := s.Keyring.GetKey(1)
				contractAddr, err := deployErc20Contract(senderKey, s.Factory)
				s.Require().NoError(err)

				err = s.Network.NextBlock()
				s.Require().NoError(err)

				txMsg, err := executeTransferCall(
					transferParams{
						senderKey:     senderKey,
						contractAddr:  contractAddr,
						recipientAddr: hardcodedRecipient,
					},
					s.Factory,
				)
				s.Require().NoError(err)

				return []*types.MsgEthereumTx{txMsg}
			},
			expPass: true,
			expectedTrace: "{\"gas\":34780,\"failed\":false," +
				"" + "\"returnValue\":\"0x0000000000000000000000000000000000000000000000000000000000000001\"," +
				"" + "\"structLogs\":[{\"pc\":0,\"op\":\"PUSH1\",\"gas",
		},
		{
			msg: "invalid too many predecessors",
			getRequest: func() *types.QueryTraceTxRequest {
				return getDefaultTraceTxRequest(s.Network)
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				pred := make([]*types.MsgEthereumTx, 10001)
				for i := 0; i < 10001; i++ {
					pred[i] = &types.MsgEthereumTx{}
				}

				return pred
			},
			expPass: false,
		},
		{
			msg: "no panic when gas limit exceeded for predecessors",
			getRequest: func() *types.QueryTraceTxRequest {
				return getDefaultTraceTxRequest(s.Network)
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				// Create predecessor tx
				// Use different address to avoid nonce collision
				senderKey := s.Keyring.GetKey(1)
				contractAddr, err := deployErc20Contract(senderKey, s.Factory)
				s.Require().NoError(err)
				s.Require().NoError(s.Network.NextBlock())
				numTxs := 1500
				txs := make([]*types.MsgEthereumTx, 0, numTxs)
				for range numTxs {
					txMsg := buildTransferTx(
						s.T(),
						transferParams{
							senderKey:     senderKey,
							contractAddr:  contractAddr,
							recipientAddr: hardcodedRecipient,
						},
						s.Factory,
					)
					txs = append(txs, txMsg)
				}
				return txs
			},
			expPanics: false,
			expPass:   true,
			expectedTrace: "{\"gas\":34780,\"failed\":false," +
				"\"returnValue\":\"0x0000000000000000000000000000000000000000000000000000000000000001\"," +
				"\"structLogs\":[{\"pc\":0,\"op\":\"PUSH1\",\"gas",
		},
		{
			msg: "error when requested block num greater than chain height",
			getRequest: func() *types.QueryTraceTxRequest {
				req := getDefaultTraceTxRequest(s.Network)
				req.BlockNumber = math.MaxInt64
				return req
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: false,
		},
		{
			msg: "invalid trace config - Negative Limit",
			getRequest: func() *types.QueryTraceTxRequest {
				defaultRequest := getDefaultTraceTxRequest(s.Network)
				defaultRequest.TraceConfig = &types.TraceConfig{
					DisableStack:   true,
					DisableStorage: true,
					EnableMemory:   false,
					Limit:          -1,
				}
				return defaultRequest
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: false,
		},
		{
			msg: "invalid trace config - Invalid Tracer",
			getRequest: func() *types.QueryTraceTxRequest {
				defaultRequest := getDefaultTraceTxRequest(s.Network)
				defaultRequest.TraceConfig = &types.TraceConfig{
					Tracer: "invalid_tracer",
				}
				return defaultRequest
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: false,
		},
		{
			msg: "invalid trace config - Invalid Timeout",
			getRequest: func() *types.QueryTraceTxRequest {
				defaultRequest := getDefaultTraceTxRequest(s.Network)
				defaultRequest.TraceConfig = &types.TraceConfig{
					DisableStack:   true,
					DisableStorage: true,
					EnableMemory:   false,
					Timeout:        "wrong_time",
				}
				return defaultRequest
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: false,
		},
		{
			msg: "default tracer with contract creation tx as predecessor but 'create' param disabled",
			getRequest: func() *types.QueryTraceTxRequest {
				return getDefaultTraceTxRequest(s.Network)
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				// use different address to avoid nonce collision
				senderKey := s.Keyring.GetKey(1)

				constructorArgs := []interface{}{
					senderKey.Addr,
					sdkmath.NewIntWithDecimal(1000, 18).BigInt(),
				}
				compiledContract := erc20Contract
				deploymentData := testutiltypes.ContractDeploymentData{
					Contract:        compiledContract,
					ConstructorArgs: constructorArgs,
				}

				txArgs, err := s.Factory.GenerateDeployContractArgs(senderKey.Addr, types.EvmTxArgs{}, deploymentData)
				s.Require().NoError(err)

				txMsg, err := s.Factory.GenerateMsgEthereumTx(senderKey.Priv, txArgs)
				s.Require().NoError(err)

				_, err = s.Factory.ExecuteEthTx(
					senderKey.Priv,
					txArgs, // Default values
				)
				s.Require().NoError(err)

				params := s.Network.App.GetEVMKeeper().GetParams(s.Network.GetContext())
				params.AccessControl = types.AccessControl{
					Create: types.AccessControlType{
						AccessType: types.AccessTypeRestricted,
					},
				}
				err = s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), params)
				s.Require().NoError(err)
				return []*types.MsgEthereumTx{&txMsg}
			},
			expPass: true,
			expectedTrace: "{\"gas\":34780,\"failed\":false," +
				"" + "\"returnValue\":\"0x0000000000000000000000000000000000000000000000000000000000000001\"," +
				"" + "\"structLogs\":[{\"pc\":0,\"op\":\"PUSH1\",\"gas",
			// expFinalGas:   26744, // gas consumed in traceTx setup (GetProposerAddr + CalculateBaseFee) + gas consumed in malleate func
		},
		{
			msg: "Empty request",
			getRequest: func() *types.QueryTraceTxRequest {
				return nil
			},
			getPredecessors: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: false,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			// Clean up per test
			defaultEvmParams := types.DefaultParams()
			err := s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), defaultEvmParams)
			s.Require().NoError(err)

			err = s.Network.NextBlock()
			s.Require().NoError(err)

			// ----- Contract Deployment -----
			senderKey := s.Keyring.GetKey(0)
			contractAddr, err := deployErc20Contract(senderKey, s.Factory)
			s.Require().NoError(err)

			err = s.Network.NextBlock()
			s.Require().NoError(err)

			// --- Add predecessor ---
			predecessors := tc.getPredecessors()

			// Get the message to trace
			msgToTrace, err := executeTransferCall(
				transferParams{
					senderKey:     senderKey,
					contractAddr:  contractAddr,
					recipientAddr: hardcodedRecipient,
				},
				s.Factory,
			)
			s.Require().NoError(err)

			s.Require().NoError(s.Network.NextBlock())

			// Get the trace request
			traceReq := tc.getRequest()
			if traceReq != nil {
				// Add predecessor to trace request
				traceReq.Predecessors = predecessors
				traceReq.Msg = msgToTrace
			}

			if tc.expPanics {
				s.Require().Panics(func() {
					//nolint:errcheck // we just want this to panic.
					s.Network.GetEvmClient().TraceTx(
						s.Network.GetContext(),
						traceReq,
					)
				})
				return
			}

			// Function under test
			res, err := s.Network.GetEvmClient().TraceTx(
				s.Network.GetContext(),
				traceReq,
			)
			if tc.expPass {
				s.Require().NoError(err)

				// if data is to big, slice the result
				if len(res.Data) > 150 {
					s.Require().Equal(tc.expectedTrace, string(res.Data[:150]))
				} else {
					s.Require().Equal(tc.expectedTrace, string(res.Data))
				}
				if traceReq.TraceConfig == nil || traceReq.TraceConfig.Tracer == "" {
					var result ethlogger.ExecutionResult
					s.Require().NoError(json.Unmarshal(res.Data, &result))
					s.Require().Positive(result.Gas)
				}
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestTraceBlock() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	s.SetupTest()

	// Hardcode recipient to make gas estimation deterministic
	hardcodedTransferRecipient := common.HexToAddress("0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101")

	testCases := []struct {
		msg              string
		getRequest       func() types.QueryTraceBlockRequest
		getAdditionalTxs func() []*types.MsgEthereumTx
		expPass          bool
		traceResponse    string
	}{
		{
			msg: "default trace",
			getRequest: func() types.QueryTraceBlockRequest {
				return getDefaultTraceBlockRequest(s.Network)
			},
			getAdditionalTxs: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: true,
			traceResponse: "[{\"result\":{\"gas\":34780,\"failed\":false," +
				"\"returnValue\":\"0x0000000000000000000000000000000000000000000000000000000000000001\"," +
				"\"structLogs\":[{\"pc\":0,\"op\":\"PU",
		},
		{
			msg: "filtered trace",
			getRequest: func() types.QueryTraceBlockRequest {
				defaultReq := getDefaultTraceBlockRequest(s.Network)
				defaultReq.TraceConfig = &types.TraceConfig{
					DisableStack:   true,
					DisableStorage: true,
					EnableMemory:   false,
				}
				return defaultReq
			},
			getAdditionalTxs: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: true,
			traceResponse: "[{\"result\":{\"gas\":34780,\"failed\":false," +
				"\"returnValue\":\"0x0000000000000000000000000000000000000000000000000000000000000001\"," +
				"\"structLogs\":[{\"pc\":0,\"op\":\"PU",
		},
		{
			msg: "javascript tracer",
			getRequest: func() types.QueryTraceBlockRequest {
				defaultReq := getDefaultTraceBlockRequest(s.Network)
				defaultReq.TraceConfig = &types.TraceConfig{
					Tracer: "{data: [], fault: function(log) {}, step: function(log) { if(log.op.toString() == \"CALL\") this.data.push(log.stack.peek(0)); }, result: function() { return this.data; }}",
				}
				return defaultReq
			},
			getAdditionalTxs: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass:       true,
			traceResponse: "[{\"result\":[]}]",
		},
		{
			msg: "tracer with multiple transactions",
			getRequest: func() types.QueryTraceBlockRequest {
				return getDefaultTraceBlockRequest(s.Network)
			},
			getAdditionalTxs: func() []*types.MsgEthereumTx {
				// create predecessor tx
				// Use different address to avoid nonce collision
				senderKey := s.Keyring.GetKey(1)
				contractAddr, err := deployErc20Contract(senderKey, s.Factory)
				s.Require().NoError(err)

				err = s.Network.NextBlock()
				s.Require().NoError(err)

				firstTransferMessage, err := executeTransferCall(
					transferParams{
						senderKey:     s.Keyring.GetKey(1),
						contractAddr:  contractAddr,
						recipientAddr: hardcodedTransferRecipient,
					},
					s.Factory,
				)
				s.Require().NoError(err)
				return []*types.MsgEthereumTx{firstTransferMessage}
			},
			expPass: true,
			traceResponse: "[{\"result\":{\"gas\":34780,\"failed\":false," +
				"\"returnValue\":\"0x0000000000000000000000000000000000000000000000000000000000000001\"," +
				"\"structLogs\":[{\"pc\":0,\"op\":\"PU",
		},
		{
			msg: "invalid trace config - Negative Limit",
			getRequest: func() types.QueryTraceBlockRequest {
				defaultReq := getDefaultTraceBlockRequest(s.Network)
				defaultReq.TraceConfig = &types.TraceConfig{
					Limit: -1,
				}
				return defaultReq
			},
			getAdditionalTxs: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: false,
		},
		{
			msg: "invalid trace config - Invalid Tracer",
			getRequest: func() types.QueryTraceBlockRequest {
				defaultReq := getDefaultTraceBlockRequest(s.Network)
				defaultReq.TraceConfig = &types.TraceConfig{
					Tracer: "invalid_tracer",
				}
				return defaultReq
			},
			getAdditionalTxs: func() []*types.MsgEthereumTx {
				return nil
			},
			expPass: true,
			traceResponse: "[{\"error\":\"rpc error: code = Internal desc = ReferenceError: invalid_tracer is not" +
				" defined",
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			// Start from fresh block
			s.Require().NoError(s.Network.NextBlock())

			// ----- Contract Deployment -----
			senderKey := s.Keyring.GetKey(0)
			contractAddr, err := deployErc20Contract(senderKey, s.Factory)
			s.Require().NoError(err)

			err = s.Network.NextBlock()
			s.Require().NoError(err)

			// --- Add predecessor ---
			txs := tc.getAdditionalTxs()

			// --- Contract Call ---
			msgToTrace, err := executeTransferCall(
				transferParams{
					senderKey:     senderKey,
					contractAddr:  contractAddr,
					recipientAddr: hardcodedTransferRecipient,
				},
				s.Factory,
			)
			s.Require().NoError(err)
			txs = append(txs, msgToTrace)

			s.Require().NoError(s.Network.NextBlock())

			// Get the trace request
			traceReq := tc.getRequest()
			// Add txs to trace request
			traceReq.Txs = txs

			res, err := s.Network.GetEvmClient().TraceBlock(s.Network.GetContext(), &traceReq)

			if tc.expPass {
				s.Require().NoError(err)
				// if data is too big, slice the result
				if len(res.Data) > 200 {
					s.Require().Contains(string(res.Data[:200]), tc.traceResponse)
				} else {
					s.Require().Contains(string(res.Data), tc.traceResponse)
				}
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestTraceCall() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	s.SetupTest()

	// Load ERC20 contract
	erc20Contract, err := testdata.LoadERC20Contract()
	s.Require().NoError(err)

	// Deploy ERC20 contract for testing
	senderKey := s.Keyring.GetKey(0)
	contractAddr, err := deployErc20Contract(senderKey, s.Factory)
	s.Require().NoError(err)
	s.Require().NoError(s.Network.NextBlock())

	testCases := []struct {
		msg            string
		getCallArgs    func() []byte
		getTraceConfig func() *types.TraceConfig
		expPass        bool
		traceResponse  string
	}{
		{
			msg: "default trace with contract call",
			getCallArgs: func() []byte {
				// Prepare transfer call data
				callArgs := testutiltypes.CallArgs{
					ContractABI: erc20Contract.ABI,
					MethodName:  "transfer",
					Args:        []interface{}{common.HexToAddress("0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101"), big.NewInt(1000)},
				}
				input, err := factory.GenerateContractCallArgs(callArgs)
				s.Require().NoError(err)
				return input
			},
			getTraceConfig: func() *types.TraceConfig {
				return nil // Use default tracer
			},
			expPass:       true,
			traceResponse: "\"gas\":",
		},
		{
			msg: "callTracer with contract call",
			getCallArgs: func() []byte {
				callArgs := testutiltypes.CallArgs{
					ContractABI: erc20Contract.ABI,
					MethodName:  "balanceOf",
					Args:        []interface{}{senderKey.Addr},
				}
				input, err := factory.GenerateContractCallArgs(callArgs)
				s.Require().NoError(err)
				return input
			},
			getTraceConfig: func() *types.TraceConfig {
				return &types.TraceConfig{
					Tracer: "callTracer",
				}
			},
			expPass:       true,
			traceResponse: "\"type\":\"CALL\"",
		},
		{
			msg: "prestateTracer with contract call",
			getCallArgs: func() []byte {
				callArgs := testutiltypes.CallArgs{
					ContractABI: erc20Contract.ABI,
					MethodName:  "balanceOf",
					Args:        []interface{}{senderKey.Addr},
				}
				input, err := factory.GenerateContractCallArgs(callArgs)
				s.Require().NoError(err)
				return input
			},
			getTraceConfig: func() *types.TraceConfig {
				return &types.TraceConfig{
					Tracer: "prestateTracer",
				}
			},
			expPass:       true,
			traceResponse: "\"balance\":",
		},
		{
			msg: "trace with filtered options",
			getCallArgs: func() []byte {
				callArgs := testutiltypes.CallArgs{
					ContractABI: erc20Contract.ABI,
					MethodName:  "balanceOf",
					Args:        []interface{}{senderKey.Addr},
				}
				input, err := factory.GenerateContractCallArgs(callArgs)
				s.Require().NoError(err)
				return input
			},
			getTraceConfig: func() *types.TraceConfig {
				return &types.TraceConfig{
					DisableStack:     true,
					DisableStorage:   true,
					EnableMemory:     false,
					EnableReturnData: true,
				}
			},
			expPass:       true,
			traceResponse: "\"returnValue\":",
		},
		{
			msg: "javascript tracer",
			getCallArgs: func() []byte {
				callArgs := testutiltypes.CallArgs{
					ContractABI: erc20Contract.ABI,
					MethodName:  "balanceOf",
					Args:        []interface{}{senderKey.Addr},
				}
				input, err := factory.GenerateContractCallArgs(callArgs)
				s.Require().NoError(err)
				return input
			},
			getTraceConfig: func() *types.TraceConfig {
				return &types.TraceConfig{
					Tracer: "{data: [], fault: function(log) {}, step: function(log) { if(log.op.toString() == \"CALL\") this.data.push(log.stack.peek(0)); }, result: function() { return this.data; }}",
				}
			},
			expPass:       true,
			traceResponse: "[",
		},
		{
			msg: "trace simple value transfer",
			getCallArgs: func() []byte {
				return nil // Simple value transfer, no data
			},
			getTraceConfig: func() *types.TraceConfig {
				return &types.TraceConfig{
					Tracer: "callTracer",
				}
			},
			expPass:       true,
			traceResponse: "\"type\":\"CALL\"",
		},
		{
			msg: "invalid trace config - Negative Limit",
			getCallArgs: func() []byte {
				return nil
			},
			getTraceConfig: func() *types.TraceConfig {
				return &types.TraceConfig{
					Limit: -1,
				}
			},
			expPass: false,
		},
		{
			msg: "invalid trace config - Invalid Tracer",
			getCallArgs: func() []byte {
				return nil
			},
			getTraceConfig: func() *types.TraceConfig {
				return &types.TraceConfig{
					Tracer: "invalid_tracer",
				}
			},
			expPass: false,
		},
		{
			msg: "invalid trace config - Invalid Timeout",
			getCallArgs: func() []byte {
				return nil
			},
			getTraceConfig: func() *types.TraceConfig {
				return &types.TraceConfig{
					Timeout: "wrong_time",
				}
			},
			expPass: false,
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.msg), func() {
			// Get current block for tracing
			currentBlock := s.Network.GetContext().BlockHeight()

			// Build transaction args for the call
			callData := tc.getCallArgs()
			var to *common.Address
			if callData != nil {
				to = &contractAddr
			} else {
				// For simple transfers, use a different recipient
				recipient := common.HexToAddress("0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101")
				to = &recipient
			}

			// Marshal transaction args with default values
			gasLimit := hexutil.Uint64(100000)
			gasPrice := hexutil.Big(*big.NewInt(1000000000)) // 1 gwei
			defaultValue := hexutil.Big(*big.NewInt(0))

			txArgs := types.TransactionArgs{
				From:     &senderKey.Addr,
				To:       to,
				Gas:      &gasLimit,
				GasPrice: &gasPrice,
				Value:    &defaultValue,
				Input:    (*hexutil.Bytes)(&callData),
			}

			// If it's a value transfer test, add value
			if tc.msg == "trace simple value transfer" {
				value := hexutil.Big(*big.NewInt(1000))
				txArgs.Value = &value
			}

			argsBytes, err := json.Marshal(txArgs)
			s.Require().NoError(err)

			// Build trace request
			ctx := s.Network.GetContext()

			traceReq := &types.QueryTraceCallRequest{
				Args:            argsBytes,
				TraceConfig:     tc.getTraceConfig(),
				BlockNumber:     currentBlock,
				BlockTime:       ctx.BlockTime(),
				BlockHash:       common.BytesToHash(ctx.HeaderHash()).Hex(),
				ProposerAddress: sdk.ConsAddress(ctx.BlockHeader().ProposerAddress),
				ChainId:         s.Network.GetEIP155ChainID().Int64(),
			}

			// Execute trace call
			res, err := s.Network.GetEvmClient().TraceCall(s.Network.GetContext(), traceReq)

			if tc.expPass {
				s.Require().NoError(err)
				s.Require().NotNil(res)

				// Verify response contains expected trace data
				if tc.traceResponse != "" {
					s.Require().Contains(string(res.Data), tc.traceResponse)
				}

				// For non-custom tracers, verify the result structure
				if tc.getTraceConfig() == nil || tc.getTraceConfig().Tracer == "" {
					var result ethlogger.ExecutionResult
					s.Require().NoError(json.Unmarshal(res.Data, &result))
					s.Require().NotNil(result.Gas)
				}
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *KeeperTestSuite) TestNonceInQuery() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	s.SetupTest()

	senderKey := s.Keyring.GetKey(0)
	nonce := s.Network.App.GetEVMKeeper().GetNonce(
		s.Network.GetContext(),
		senderKey.Addr,
	)
	s.Require().Equal(uint64(0), nonce)

	// accupy nonce 0
	contractAddr, err := deployErc20Contract(s.Keyring.GetKey(0), s.Factory)
	s.Require().NoError(err)

	erc20Contract, err := testdata.LoadERC20Contract()
	s.Require().NoError(err, "failed to load erc20 contract")

	// do an EthCall/EstimateGas with nonce 0
	ctorArgs, err := erc20Contract.ABI.Pack("", senderKey.Addr, big.NewInt(1000))
	s.Require().NoError(err)

	data := erc20Contract.Bin
	data = append(data, ctorArgs...)
	args, err := json.Marshal(&types.TransactionArgs{
		From: &senderKey.Addr,
		To:   &contractAddr,
		Data: (*hexutil.Bytes)(&data),
	})
	s.Require().NoError(err)

	proposerAddress := s.Network.GetContext().BlockHeader().ProposerAddress
	_, err = s.Network.GetEvmClient().EstimateGas(
		s.Network.GetContext(),
		&types.EthCallRequest{
			Args:            args,
			GasCap:          config.DefaultGasCap,
			ProposerAddress: proposerAddress,
		},
	)
	s.Require().NoError(err)

	_, err = s.Network.GetEvmClient().EthCall(
		s.Network.GetContext(),
		&types.EthCallRequest{
			Args:            args,
			GasCap:          config.DefaultGasCap,
			ProposerAddress: proposerAddress,
		},
	)
	s.Require().NoError(err)
}

func (s *KeeperTestSuite) TestQueryBaseFee() {
	s.EnableFeemarket = true
	defer func() { s.EnableFeemarket = false }()
	s.SetupTest()

	testCases := []struct {
		name       string
		getExpResp func() *types.QueryBaseFeeResponse
		setParams  func()
		expPass    bool
	}{
		{
			"pass - default Base Fee",
			func() *types.QueryBaseFeeResponse {
				initialBaseFee := sdkmath.NewInt(ethparams.InitialBaseFee)
				return &types.QueryBaseFeeResponse{BaseFee: &initialBaseFee}
			},
			func() {
				feemarketDefault := feemarkettypes.DefaultParams()
				s.Require().NoError(s.Network.App.GetFeeMarketKeeper().SetParams(s.Network.GetContext(), feemarketDefault))

				evmDefault := types.DefaultParams()
				s.Require().NoError(s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), evmDefault))
			},

			true,
		},
		{
			"pass - nil Base Fee when london hardfork not activated",
			func() *types.QueryBaseFeeResponse {
				return &types.QueryBaseFeeResponse{}
			},
			func() {
				feemarketDefault := feemarkettypes.DefaultParams()
				s.Require().NoError(s.Network.App.GetFeeMarketKeeper().SetParams(s.Network.GetContext(), feemarketDefault))

				chainConfig := types.DefaultChainConfig(s.Network.GetEIP155ChainID().Uint64())
				maxInt := sdkmath.NewInt(math.MaxInt64)
				chainConfig.LondonBlock = &maxInt
				chainConfig.ArrowGlacierBlock = &maxInt
				chainConfig.GrayGlacierBlock = &maxInt
				chainConfig.MergeNetsplitBlock = &maxInt
				chainConfig.ShanghaiTime = &maxInt
				chainConfig.CancunTime = &maxInt
				chainConfig.PragueTime = &maxInt

				configurator := types.NewEVMConfigurator()
				configurator.ResetTestConfig()
				err := types.SetChainConfig(chainConfig)
				s.Require().NoError(err)
				err = configurator.
					WithEVMCoinInfo(testconstants.ExampleChainCoinInfo[testconstants.ExampleChainID]).
					Configure()
				s.Require().NoError(err)
			},
			true,
		},
		{
			"pass - zero Base Fee when feemarket not activated",
			func() *types.QueryBaseFeeResponse {
				baseFee := sdkmath.ZeroInt()
				return &types.QueryBaseFeeResponse{BaseFee: &baseFee}
			},
			func() {
				feemarketDefault := feemarkettypes.DefaultParams()
				feemarketDefault.NoBaseFee = true
				s.Require().NoError(s.Network.App.GetFeeMarketKeeper().SetParams(s.Network.GetContext(), feemarketDefault))

				evmDefault := types.DefaultParams()
				s.Require().NoError(s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), evmDefault))
			},
			true,
		},
	}

	// Save initial configure to restore it between tests
	coinInfo := types.EvmCoinInfo{
		Denom:         types.GetEVMCoinDenom(),
		ExtendedDenom: types.GetEVMCoinExtendedDenom(),
		DisplayDenom:  types.GetEVMCoinDisplayDenom(),
		Decimals:      types.GetEVMCoinDecimals().Uint32(),
	}
	chainConfig := types.DefaultChainConfig(s.Network.GetEIP155ChainID().Uint64())

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			// Set necessary params
			tc.setParams()
			// Get the expected response
			expResp := tc.getExpResp()
			// Function under test
			res, err := s.Network.GetEvmClient().BaseFee(
				s.Network.GetContext(),
				&types.QueryBaseFeeRequest{},
			)
			if tc.expPass {
				s.Require().NotNil(res)
				s.Require().Equal(expResp, res, tc.name)
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
			s.Require().NoError(s.Network.NextBlock())
			configurator := types.NewEVMConfigurator()
			configurator.ResetTestConfig()
			err = types.SetChainConfig(chainConfig)
			s.Require().NoError(err)
			err = configurator.
				WithEVMCoinInfo(coinInfo).
				Configure()
			s.Require().NoError(err)
		})
	}
}

func (s *KeeperTestSuite) TestEthCall() {
	s.SetupTest()

	erc20Contract, err := testdata.LoadERC20Contract()
	s.Require().NoError(err)

	// Generate common data for requests
	sender := s.Keyring.GetAddr(0)
	supply := sdkmath.NewIntWithDecimal(1000, 18).BigInt()
	ctorArgs, err := erc20Contract.ABI.Pack("", sender, supply)
	s.Require().NoError(err)
	data := erc20Contract.Bin
	data = append(data, ctorArgs...)

	testCases := []struct {
		name       string
		getReq     func() *types.EthCallRequest
		expVMError bool
	}{
		{
			"invalid args",
			func() *types.EthCallRequest {
				return &types.EthCallRequest{Args: []byte("invalid args"), GasCap: config.DefaultGasCap}
			},
			false,
		},
		{
			"invalid args - specified both gasPrice and maxFeePerGas",
			func() *types.EthCallRequest {
				hexBigInt := hexutil.Big(*big.NewInt(1))
				args, err := json.Marshal(&types.TransactionArgs{
					From:         &sender,
					Data:         (*hexutil.Bytes)(&data),
					GasPrice:     &hexBigInt,
					MaxFeePerGas: &hexBigInt,
				})
				s.Require().NoError(err)

				return &types.EthCallRequest{Args: args, GasCap: config.DefaultGasCap}
			},
			false,
		},
		{
			"set param AccessControl - no Access",
			func() *types.EthCallRequest {
				args, err := json.Marshal(&types.TransactionArgs{
					From: &sender,
					Data: (*hexutil.Bytes)(&data),
				})

				s.Require().NoError(err)
				req := &types.EthCallRequest{Args: args, GasCap: config.DefaultGasCap}

				params := s.Network.App.GetEVMKeeper().GetParams(s.Network.GetContext())
				params.AccessControl = types.AccessControl{
					Create: types.AccessControlType{
						AccessType: types.AccessTypeRestricted,
					},
				}
				err = s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), params)
				s.Require().NoError(err)
				return req
			},
			true,
		},
		{
			"set param AccessControl = non whitelist",
			func() *types.EthCallRequest {
				args, err := json.Marshal(&types.TransactionArgs{
					From: &sender,
					Data: (*hexutil.Bytes)(&data),
				})

				s.Require().NoError(err)
				req := &types.EthCallRequest{Args: args, GasCap: config.DefaultGasCap}

				params := s.Network.App.GetEVMKeeper().GetParams(s.Network.GetContext())
				params.AccessControl = types.AccessControl{
					Create: types.AccessControlType{
						AccessType: types.AccessTypePermissioned,
					},
				}
				err = s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), params)
				s.Require().NoError(err)
				return req
			},
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			req := tc.getReq()

			res, err := s.Network.GetEvmClient().EthCall(s.Network.GetContext(), req)
			if tc.expVMError {
				s.Require().NotNil(res)
				s.Require().Contains(res.VmError, "does not have permission to deploy contracts")
			} else {
				s.Require().Error(err)
			}

			// Reset params
			defaultEvmParams := types.DefaultParams()
			err = s.Network.App.GetEVMKeeper().SetParams(s.Network.GetContext(), defaultEvmParams)
			s.Require().NoError(err)
		})
	}
}

func (s *KeeperTestSuite) TestBalance() {
	testCases := []struct {
		name        string
		returnedBal func() *uint256.Int
		expBalance  *uint256.Int
	}{
		{
			"Account method, vesting account (0 spendable, large locked balance)",
			func() *uint256.Int {
				addr := tx.GenerateAddress()
				accAddr := sdk.AccAddress(addr.Bytes())
				err := s.Network.App.GetBankKeeper().MintCoins(s.Network.GetContext(), "mint", sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), sdkmath.NewInt(100))))
				s.Require().NoError(err)
				err = s.Network.App.GetBankKeeper().SendCoinsFromModuleToAccount(s.Network.GetContext(), "mint", addr.Bytes(), sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), sdkmath.NewInt(100))))
				s.Require().NoError(err)

				// Make tx cost greater than balance
				balanceResp, err := s.Handler.GetBalanceFromEVM(accAddr)
				s.Require().NoError(err)

				balance, ok := sdkmath.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)
				balance = balance.Quo(types2.ConversionFactor())
				s.Require().NotEqual(balance.String(), "0")

				// replace with vesting account
				ctx := s.Network.GetContext()
				baseAccount := s.Network.App.GetAccountKeeper().GetAccount(ctx, accAddr).(*authtypes.BaseAccount)
				baseDenom := s.Network.GetBaseDenom()
				currTime := s.Network.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, balance)), s.Network.GetContext().BlockTime().Unix(), currTime+100)
				s.Require().NoError(err)
				s.Network.App.GetAccountKeeper().SetAccount(ctx, acc)

				spendable := s.Network.App.GetBankKeeper().SpendableCoin(ctx, accAddr, baseDenom).Amount
				s.Require().Equal(spendable.String(), "0")

				evmBalanceRes, err := s.Handler.GetBalanceFromEVM(accAddr)
				s.Require().NoError(err)
				evmBalance := evmBalanceRes.Balance
				s.Require().Equal(evmBalance, "0")

				totalBalance := s.Network.App.GetBankKeeper().GetBalance(ctx, accAddr, baseDenom)
				s.Require().Equal(totalBalance.Amount, balance)

				res, err := s.Network.App.GetEVMKeeper().Account(s.Network.GetContext(), &types.QueryAccountRequest{Address: addr.String()})
				s.Require().NoError(err)
				bal, err := uint256.FromDecimal(res.Balance)
				s.Require().NoError(err)
				return bal
			},
			&uint256.Int{0},
		},
		{
			"Balance method, vesting account (0 spendable, large locked balance)",
			func() *uint256.Int {
				addr := tx.GenerateAddress()
				accAddr := sdk.AccAddress(addr.Bytes())
				err := s.Network.App.GetBankKeeper().MintCoins(s.Network.GetContext(), "mint", sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), sdkmath.NewInt(100))))
				s.Require().NoError(err)
				err = s.Network.App.GetBankKeeper().SendCoinsFromModuleToAccount(s.Network.GetContext(), "mint", addr.Bytes(), sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), sdkmath.NewInt(100))))
				s.Require().NoError(err)

				// Make tx cost greater than balance
				balanceResp, err := s.Handler.GetBalanceFromEVM(accAddr)
				s.Require().NoError(err)

				balance, ok := sdkmath.NewIntFromString(balanceResp.Balance)
				s.Require().True(ok)
				balance = balance.Quo(types2.ConversionFactor())
				s.Require().NotEqual(balance.String(), "0")

				// replace with vesting account
				ctx := s.Network.GetContext()
				baseAccount := s.Network.App.GetAccountKeeper().GetAccount(ctx, accAddr).(*authtypes.BaseAccount)
				baseDenom := s.Network.GetBaseDenom()
				currTime := s.Network.GetContext().BlockTime().Unix()
				acc, err := vestingtypes.NewContinuousVestingAccount(baseAccount, sdk.NewCoins(sdk.NewCoin(baseDenom, balance)), s.Network.GetContext().BlockTime().Unix(), currTime+100)
				s.Require().NoError(err)
				s.Network.App.GetAccountKeeper().SetAccount(ctx, acc)

				spendable := s.Network.App.GetBankKeeper().SpendableCoin(ctx, accAddr, baseDenom).Amount
				s.Require().Equal(spendable.String(), "0")

				evmBalanceRes, err := s.Handler.GetBalanceFromEVM(accAddr)
				s.Require().NoError(err)
				evmBalance := evmBalanceRes.Balance
				s.Require().Equal(evmBalance, "0")

				totalBalance := s.Network.App.GetBankKeeper().GetBalance(ctx, accAddr, baseDenom)
				s.Require().Equal(totalBalance.Amount, balance)

				res, err := s.Network.App.GetEVMKeeper().Balance(s.Network.GetContext(), &types.QueryBalanceRequest{Address: addr.String()})
				s.Require().NoError(err)
				bal, err := uint256.FromDecimal(res.Balance)
				s.Require().NoError(err)
				return bal
			},
			&uint256.Int{0},
		},
		{
			"Account method, regular account",
			func() *uint256.Int {
				addr := tx.GenerateAddress()
				err := s.Network.App.GetBankKeeper().MintCoins(s.Network.GetContext(), "mint", sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), sdkmath.NewInt(100))))
				s.Require().NoError(err)
				err = s.Network.App.GetBankKeeper().SendCoinsFromModuleToAccount(s.Network.GetContext(), "mint", addr.Bytes(), sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), sdkmath.NewInt(100))))
				s.Require().NoError(err)
				res, err := s.Network.App.GetEVMKeeper().Account(s.Network.GetContext(), &types.QueryAccountRequest{Address: addr.String()})
				s.Require().NoError(err)
				bal, err := uint256.FromDecimal(res.Balance)
				s.Require().NoError(err)
				return bal
			},
			&uint256.Int{100},
		},
		{
			"Balance method, regular account",
			func() *uint256.Int {
				addr := tx.GenerateAddress()
				err := s.Network.App.GetBankKeeper().MintCoins(s.Network.GetContext(), "mint", sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), sdkmath.NewInt(100))))
				s.Require().NoError(err)
				err = s.Network.App.GetBankKeeper().SendCoinsFromModuleToAccount(s.Network.GetContext(), "mint", addr.Bytes(), sdk.NewCoins(sdk.NewCoin(s.Network.GetBaseDenom(), sdkmath.NewInt(100))))
				s.Require().NoError(err)
				res, err := s.Network.App.GetEVMKeeper().Balance(s.Network.GetContext(), &types.QueryBalanceRequest{Address: addr.String()})
				s.Require().NoError(err)
				bal, err := uint256.FromDecimal(res.Balance)
				s.Require().NoError(err)
				return bal
			},
			&uint256.Int{100},
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest()
			s.Require().Equal(tc.returnedBal(), tc.expBalance)
		})
	}
}

func (s *KeeperTestSuite) TestEmptyRequest() {
	s.SetupTest()
	k := s.Network.App.GetEVMKeeper()

	testCases := []struct {
		name      string
		queryFunc func() (interface{}, error)
	}{
		{
			"Account method",
			func() (interface{}, error) {
				return k.Account(s.Network.GetContext(), nil)
			},
		},
		{
			"CosmosAccount method",
			func() (interface{}, error) {
				return k.CosmosAccount(s.Network.GetContext(), nil)
			},
		},
		{
			"ValidatorAccount method",
			func() (interface{}, error) {
				return k.ValidatorAccount(s.Network.GetContext(), nil)
			},
		},
		{
			"Balance method",
			func() (interface{}, error) {
				return k.Balance(s.Network.GetContext(), nil)
			},
		},
		{
			"Storage method",
			func() (interface{}, error) {
				return k.Storage(s.Network.GetContext(), nil)
			},
		},
		{
			"Code method",
			func() (interface{}, error) {
				return k.Code(s.Network.GetContext(), nil)
			},
		},
		{
			"EthCall method",
			func() (interface{}, error) {
				return k.EthCall(s.Network.GetContext(), nil)
			},
		},
		{
			"EstimateGas method",
			func() (interface{}, error) {
				return k.EstimateGas(s.Network.GetContext(), nil)
			},
		},
		{
			"TraceTx method",
			func() (interface{}, error) {
				return k.TraceTx(s.Network.GetContext(), nil)
			},
		},
		{
			"TraceBlock method",
			func() (interface{}, error) {
				return k.TraceBlock(s.Network.GetContext(), nil)
			},
		},
	}

	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			_, err := tc.queryFunc()
			s.Require().Error(err)
		})
	}
}

func getDefaultTraceBlockRequest(unitNetwork network.Network) types.QueryTraceBlockRequest {
	ctx := unitNetwork.GetContext()
	chainID := unitNetwork.GetEIP155ChainID().Int64()
	return types.QueryTraceBlockRequest{
		BlockMaxGas: ctx.ConsensusParams().Block.MaxGas,
		ChainId:     chainID,
		BlockTime:   ctx.BlockTime(),
	}
}

func deployErc20Contract(from keyring.Key, txFactory factory.TxFactory) (common.Address, error) {
	erc20Contract, err := testdata.LoadERC20Contract()
	if err != nil {
		return common.Address{}, err
	}

	constructorArgs := []interface{}{
		from.Addr,
		sdkmath.NewIntWithDecimal(1000, 18).BigInt(),
	}
	compiledContract := erc20Contract
	contractAddr, err := txFactory.DeployContract(
		from.Priv,
		types.EvmTxArgs{}, // Default values
		testutiltypes.ContractDeploymentData{
			Contract:        compiledContract,
			ConstructorArgs: constructorArgs,
		},
	)
	if err != nil {
		return common.Address{}, err
	}
	return contractAddr, nil
}

type transferParams struct {
	senderKey     keyring.Key
	contractAddr  common.Address
	recipientAddr common.Address
}

func executeTransferCall(
	transferParams transferParams,
	txFactory factory.TxFactory,
) (msgEthereumTx *types.MsgEthereumTx, err error) {
	erc20Contract, err := testdata.LoadERC20Contract()
	if err != nil {
		return nil, err
	}

	transferArgs := types.EvmTxArgs{
		To: &transferParams.contractAddr,
	}
	callArgs := testutiltypes.CallArgs{
		ContractABI: erc20Contract.ABI,
		MethodName:  "transfer",
		Args:        []interface{}{transferParams.recipientAddr, big.NewInt(1000)},
	}

	input, err := factory.GenerateContractCallArgs(callArgs)
	if err != nil {
		return nil, err
	}
	transferArgs.Input = input

	// We need to get access to the message
	firstSignedTX, err := txFactory.GenerateSignedEthTx(transferParams.senderKey.Priv, transferArgs)
	if err != nil {
		return nil, err
	}
	txMsg, ok := firstSignedTX.GetMsgs()[0].(*types.MsgEthereumTx)
	if !ok {
		return nil, fmt.Errorf("invalid type")
	}

	result, err := txFactory.ExecuteContractCall(transferParams.senderKey.Priv, transferArgs, callArgs)
	if err != nil || !result.IsOK() {
		return nil, err
	}
	return txMsg, nil
}

func buildTransferTx(
	t *testing.T,
	transferParams transferParams,
	txFactory factory.TxFactory,
) (msgEthereumTx *types.MsgEthereumTx) {
	t.Helper()
	erc20Contract, err := testdata.LoadERC20Contract()
	require.NoError(t, err)

	transferArgs := types.EvmTxArgs{
		To: &transferParams.contractAddr,
	}
	callArgs := testutiltypes.CallArgs{
		ContractABI: erc20Contract.ABI,
		MethodName:  "transfer",
		Args:        []interface{}{transferParams.recipientAddr, big.NewInt(1000)},
	}

	input, err := factory.GenerateContractCallArgs(callArgs)
	require.NoError(t, err)
	transferArgs.Input = input

	// We need to get access to the message
	firstSignedTX, err := txFactory.GenerateSignedEthTx(transferParams.senderKey.Priv, transferArgs)
	require.NoError(t, err)
	txMsg, ok := firstSignedTX.GetMsgs()[0].(*types.MsgEthereumTx)
	require.True(t, ok, "expected MsgEthereumTx type, got type: %T", firstSignedTX.GetMsgs()[0])
	return txMsg
}
