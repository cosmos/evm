package evm_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	"github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmsdktypes "github.com/cosmos/evm/x/vm/types"
	vmtypes "github.com/cosmos/evm/x/vm/types/mocks"

	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// adds missing methods
type ExtendedEVMKeeper struct {
	*vmtypes.EVMKeeper
}

func NewExtendedEVMKeeper() *ExtendedEVMKeeper {
	return &ExtendedEVMKeeper{
		EVMKeeper: vmtypes.NewEVMKeeper(),
	}
}

func (k *ExtendedEVMKeeper) NewEVM(_ sdk.Context, _ core.Message, _ *statedb.EVMConfig, _ *tracing.Hooks, _ vm.StateDB) *vm.EVM {
	return nil
}

func (k *ExtendedEVMKeeper) DeductTxCostsFromUserBalance(_ sdk.Context, _ sdk.Coins, _ common.Address) error {
	return nil
}

func (k *ExtendedEVMKeeper) SpendableCoin(ctx sdk.Context, addr common.Address) *uint256.Int {
	account := k.GetAccount(ctx, addr)
	if account != nil {
		return account.Balance
	}
	return uint256.NewInt(0)
}

func (k *ExtendedEVMKeeper) GetParams(_ sdk.Context) evmsdktypes.Params {
	return evmsdktypes.DefaultParams()
}
func (k *ExtendedEVMKeeper) GetBaseFee(_ sdk.Context) *big.Int           { return big.NewInt(0) }
func (k *ExtendedEVMKeeper) GetMinGasPrice(_ sdk.Context) math.LegacyDec { return math.LegacyZeroDec() }

// only methods called by EVMMonoDecorator
type MockFeeMarketKeeper struct{}

func (m MockFeeMarketKeeper) GetParams(ctx sdk.Context) feemarkettypes.Params {
	param := feemarkettypes.DefaultParams()
	param.BaseFee = m.GetBaseFee(ctx)
	return param
}

func (m MockFeeMarketKeeper) GetBaseFeeEnabled(_ sdk.Context) bool    { return true }
func (m MockFeeMarketKeeper) GetBaseFee(_ sdk.Context) math.LegacyDec { return math.LegacyZeroDec() }

// matches the actual signatures
type MockAccountKeeper struct {
	FundedAddr sdk.AccAddress
}

func (m MockAccountKeeper) GetAccount(_ context.Context, addr sdk.AccAddress) sdk.AccountI {
	if m.FundedAddr != nil && addr.Equals(m.FundedAddr) {
		return &authtypes.BaseAccount{Address: addr.String()}
	}
	return nil
}
func (m MockAccountKeeper) SetAccount(_ context.Context, _ sdk.AccountI) {}
func (m MockAccountKeeper) NewAccountWithAddress(_ context.Context, _ sdk.AccAddress) sdk.AccountI {
	return nil
}
func (m MockAccountKeeper) RemoveAccount(_ context.Context, _ sdk.AccountI) {}
func (m MockAccountKeeper) GetModuleAddress(_ string) sdk.AccAddress        { return sdk.AccAddress{} }
func (m MockAccountKeeper) GetParams(_ context.Context) authtypes.Params {
	return authtypes.DefaultParams()
}

func (m MockAccountKeeper) GetSequence(_ context.Context, _ sdk.AccAddress) (uint64, error) {
	return 0, nil
}
func (m MockAccountKeeper) RemoveExpiredUnorderedNonces(_ sdk.Context) error { return nil }
func (m MockAccountKeeper) TryAddUnorderedNonce(_ sdk.Context, _ []byte, _ time.Time) error {
	return nil
}
func (m MockAccountKeeper) UnorderedTransactionsEnabled() bool { return false }
func (m MockAccountKeeper) AddressCodec() address.Codec        { return nil }

func signMsgEthereumTx(t *testing.T, privKey *ethsecp256k1.PrivKey, args *evmsdktypes.EvmTxArgs) *evmsdktypes.MsgEthereumTx {
	t.Helper()
	msg := evmsdktypes.NewTx(args)
	fromAddr := common.BytesToAddress(privKey.PubKey().Address().Bytes())
	msg.From = fromAddr.Bytes()
	ethSigner := ethtypes.LatestSignerForChainID(evmsdktypes.GetEthChainConfig().ChainID)
	require.NoError(t, msg.Sign(ethSigner, utiltx.NewSigner(privKey)))
	return msg
}

func setupFundedKeeper(t *testing.T, privKey *ethsecp256k1.PrivKey) (*ExtendedEVMKeeper, sdk.AccAddress) {
	t.Helper()
	fromAddr := common.BytesToAddress(privKey.PubKey().Address().Bytes())
	cosmosAddr := sdk.AccAddress(fromAddr.Bytes())
	keeper := NewExtendedEVMKeeper()
	fundedAccount := statedb.NewEmptyAccount()
	fundedAccount.Balance = uint256.MustFromDecimal("1000000000000000000") // 1 eth in wei
	require.NoError(t, keeper.SetAccount(sdk.Context{}, fromAddr, *fundedAccount))
	return keeper, cosmosAddr
}

func toMsgSlice(msgs []*evmsdktypes.MsgEthereumTx) []sdk.Msg {
	out := make([]sdk.Msg, len(msgs))
	for i, m := range msgs {
		out[i] = m
	}
	return out
}

func TestMonoDecorator(t *testing.T) {
	chainID := uint64(constants.EighteenDecimalsChainID)
	cfg := encoding.MakeConfig(chainID)

	testCases := []struct {
		name      string
		simulate  bool
		buildMsgs func(privKey *ethsecp256k1.PrivKey) []*evmsdktypes.MsgEthereumTx
		expErr    string
	}{
		{
			"success with one evm tx",
			true,
			func(privKey *ethsecp256k1.PrivKey) []*evmsdktypes.MsgEthereumTx {
				args := &evmsdktypes.EvmTxArgs{
					Nonce:    0,
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
					Input:    []byte("test"),
				}
				return []*evmsdktypes.MsgEthereumTx{signMsgEthereumTx(t, privKey, args)}
			},
			"",
		},
		{
			"failure with two evm txs",
			true,
			func(privKey *ethsecp256k1.PrivKey) []*evmsdktypes.MsgEthereumTx {
				args1 := &evmsdktypes.EvmTxArgs{
					Nonce:    0,
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
					Input:    []byte("test"),
				}
				args2 := &evmsdktypes.EvmTxArgs{
					Nonce:    1,
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
					Input:    []byte("test2"),
				}
				return []*evmsdktypes.MsgEthereumTx{
					signMsgEthereumTx(t, privKey, args1),
					signMsgEthereumTx(t, privKey, args2),
				}
			},
			"expected 1 message, got 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			configurator := evmsdktypes.NewEVMConfigurator()
			configurator.ResetTestConfig()
			chainConfig := evmsdktypes.DefaultChainConfig(evmsdktypes.DefaultEVMChainID)
			err := evmsdktypes.SetChainConfig(chainConfig)
			require.NoError(t, err)
			coinInfo := evmsdktypes.EvmCoinInfo{
				Denom:         evmsdktypes.DefaultEVMExtendedDenom,
				ExtendedDenom: evmsdktypes.DefaultEVMExtendedDenom,
				DisplayDenom:  evmsdktypes.DefaultEVMDisplayDenom,
				Decimals:      18,
			}
			err = configurator.
				WithExtendedEips(evmsdktypes.DefaultCosmosEVMActivators).
				// NOTE: we're using the 18 decimals default for the example chain
				WithEVMCoinInfo(coinInfo).
				Configure()
			require.NoError(t, err)
			privKey, _ := ethsecp256k1.GenerateKey()
			keeper, cosmosAddr := setupFundedKeeper(t, privKey)
			accountKeeper := MockAccountKeeper{FundedAddr: cosmosAddr}
			feeMarketKeeper := MockFeeMarketKeeper{}
			params := keeper.GetParams(sdk.Context{})
			feemarketParams := feeMarketKeeper.GetParams(sdk.Context{})
			monoDec := evm.NewEVMMonoDecorator(accountKeeper, feeMarketKeeper, keeper, 0, &params, &feemarketParams)
			ctx := sdk.NewContext(nil, tmproto.Header{}, false, log.NewNopLogger())
			ctx = ctx.WithBlockGasMeter(storetypes.NewGasMeter(1e19))
			blockParams := tmproto.BlockParams{
				MaxBytes: 200000,
				MaxGas:   81500000, // default limit
			}
			consParams := tmproto.ConsensusParams{Block: &blockParams}
			ctx = ctx.WithConsensusParams(consParams)

			msgs := tc.buildMsgs(privKey)
			tx, err := utiltx.PrepareEthTx(cfg.TxConfig, nil, toMsgSlice(msgs)...)
			require.NoError(t, err)

			newCtx, err := monoDec.AnteHandle(ctx, tx, tc.simulate, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return ctx, nil })
			if tc.expErr == "" {
				require.NoError(t, err)
				require.NotNil(t, newCtx)
			} else {
				require.ErrorContains(t, err, tc.expErr)
			}
		})
	}
}
