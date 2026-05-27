package evm_test

import (
	"context"
	"errors"
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
	"cosmossdk.io/log/v2"
	"cosmossdk.io/math"

	storetypes "github.com/cosmos/cosmos-sdk/store/v2/types"
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

type monoTestEnv struct {
	dec     evm.MonoDecorator
	privKey *ethsecp256k1.PrivKey
	cfg     encoding.Config
}

func setupMonoEnv(t *testing.T) monoTestEnv {
	t.Helper()
	configurator := evmsdktypes.NewEVMConfigurator()
	configurator.ResetTestConfig()
	require.NoError(t, evmsdktypes.SetChainConfig(evmsdktypes.DefaultChainConfig(evmsdktypes.DefaultEVMChainID)))
	require.NoError(t, configurator.
		WithExtendedEips(evmsdktypes.DefaultCosmosEVMActivators).
		WithEVMCoinInfo(evmsdktypes.EvmCoinInfo{
			Denom:         evmsdktypes.DefaultEVMExtendedDenom,
			ExtendedDenom: evmsdktypes.DefaultEVMExtendedDenom,
			DisplayDenom:  evmsdktypes.DefaultEVMDisplayDenom,
			Decimals:      18,
		}).
		Configure())

	privKey, _ := ethsecp256k1.GenerateKey()
	keeper, cosmosAddr := setupFundedKeeper(t, privKey)
	accountKeeper := MockAccountKeeper{FundedAddr: cosmosAddr}
	feeMarketKeeper := MockFeeMarketKeeper{}
	params := keeper.GetParams(sdk.Context{})
	feemarketParams := feeMarketKeeper.GetParams(sdk.Context{})

	return monoTestEnv{
		dec:     evm.NewEVMMonoDecorator(accountKeeper, feeMarketKeeper, keeper, 0, &params, &feemarketParams),
		privKey: privKey,
		cfg:     encoding.MakeConfig(uint64(constants.EighteenDecimalsChainID)),
	}
}

func newMonoCtx() sdk.Context {
	return sdk.NewContext(nil, tmproto.Header{}, false, log.NewNopLogger()).
		WithBlockGasMeter(storetypes.NewGasMeter(1e19)).
		WithConsensusParams(tmproto.ConsensusParams{Block: &tmproto.BlockParams{MaxBytes: 200000, MaxGas: 81500000}})
}

func defaultEthTxArgs() *evmsdktypes.EvmTxArgs {
	return &evmsdktypes.EvmTxArgs{
		Nonce:    0,
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
		Input:    []byte("test"),
	}
}

func TestMonoDecorator(t *testing.T) {
	testCases := []struct {
		name      string
		buildMsgs func(env monoTestEnv) []*evmsdktypes.MsgEthereumTx
		expErr    string
	}{
		{
			"success with one evm tx",
			func(env monoTestEnv) []*evmsdktypes.MsgEthereumTx {
				return []*evmsdktypes.MsgEthereumTx{signMsgEthereumTx(t, env.privKey, defaultEthTxArgs())}
			},
			"",
		},
		{
			"failure with two evm txs",
			func(env monoTestEnv) []*evmsdktypes.MsgEthereumTx {
				args2 := defaultEthTxArgs()
				args2.Nonce = 1
				args2.Input = []byte("test2")
				return []*evmsdktypes.MsgEthereumTx{
					signMsgEthereumTx(t, env.privKey, defaultEthTxArgs()),
					signMsgEthereumTx(t, env.privKey, args2),
				}
			},
			"expected 1 message, got 2",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			env := setupMonoEnv(t)
			tx, err := utiltx.PrepareEthTx(env.cfg.TxConfig, nil, toMsgSlice(tc.buildMsgs(env))...)
			require.NoError(t, err)

			newCtx, err := env.dec.AnteHandle(newMonoCtx(), tx, true, func(ctx sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return ctx, nil })
			if tc.expErr == "" {
				require.NoError(t, err)
				require.NotNil(t, newCtx)
			} else {
				require.ErrorContains(t, err, tc.expErr)
			}
		})
	}
}

func TestMonoDecorator_SigVerificationCacheHit(t *testing.T) {
	env := setupMonoEnv(t)
	dec := env.dec
	msgs := []*evmsdktypes.MsgEthereumTx{signMsgEthereumTx(t, env.privKey, defaultEthTxArgs())}
	tx, err := utiltx.PrepareEthTx(env.cfg.TxConfig, nil, toMsgSlice(msgs)...)
	require.NoError(t, err)
	newCtx := func() sdk.Context { return newMonoCtx().WithIncarnationCache(map[string]any{}) }
	next := func(c sdk.Context, _ sdk.Tx, _ bool) (sdk.Context, error) { return c, nil }
	cachedErr := errors.New("cached sig verification failure")

	t.Run("cached error short-circuits", func(t *testing.T) {
		ctx := newCtx()
		ctx.SetIncarnationCache(evm.EthSigVerificationResultCacheKey, cachedErr)
		_, err := dec.AnteHandle(ctx, tx, true, next)
		require.ErrorIs(t, err, cachedErr)
	})

	t.Run("non-error cached value returns explicit error", func(t *testing.T) {
		ctx := newCtx()
		ctx.SetIncarnationCache(evm.EthSigVerificationResultCacheKey, "not-an-error")
		_, err := dec.AnteHandle(ctx, tx, true, next)
		require.ErrorContains(t, err, "unexpected type string")
	})
}
