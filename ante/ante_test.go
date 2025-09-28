package ante_test

import (
	"context"
	"math/big"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	antepkg "github.com/cosmos/evm/ante"
	anteinterfaces "github.com/cosmos/evm/ante/interfaces"
	"github.com/cosmos/evm/encoding"
	"github.com/cosmos/evm/testutil/constants"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibckeeper "github.com/cosmos/ibc-go/v10/modules/core/keeper"

	signingv1beta1 "cosmossdk.io/api/cosmos/tx/signing/v1beta1"
	addresscodec "cosmossdk.io/core/address"
	"cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	txsigning "cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkSigning "github.com/cosmos/cosmos-sdk/types/tx/signing"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

type handlerArgs struct {
	cdc                    codec.BinaryCodec
	accountKeeper          anteinterfaces.AccountKeeper
	bankKeeper             anteinterfaces.BankKeeper
	feeMarketKeeper        anteinterfaces.FeeMarketKeeper
	evmKeeper              anteinterfaces.EVMKeeper
	extensionOptionChecker sdkante.ExtensionOptionChecker
	signModeHandler        *txsigning.HandlerMap
	sigGasConsumer         func(storetypes.GasMeter, sdkSigning.SignatureV2, authtypes.Params) error
	maxTxGasWanted         uint64
	txFeeChecker           sdkante.TxFeeChecker
	pendingTxListener      antepkg.PendingTxListener
	opts                   []antepkg.HandlerOption
	expectedIBCKeeper      *ibckeeper.Keeper
	expectedFeegrant       sdkante.FeegrantKeeper
}

type dummySignModeHandler struct{}

func (dummySignModeHandler) Mode() signingv1beta1.SignMode {
	return signingv1beta1.SignMode_SIGN_MODE_DIRECT
}

func (dummySignModeHandler) GetSignBytes(context.Context, txsigning.SignerData, txsigning.TxData) ([]byte, error) {
	return nil, nil
}

type stubAccountKeeper struct{}

func (*stubAccountKeeper) NewAccountWithAddress(context.Context, sdk.AccAddress) sdk.AccountI {
	return nil
}
func (*stubAccountKeeper) GetModuleAddress(string) sdk.AccAddress                  { return sdk.AccAddress{} }
func (*stubAccountKeeper) GetAccount(context.Context, sdk.AccAddress) sdk.AccountI { return nil }
func (*stubAccountKeeper) SetAccount(context.Context, sdk.AccountI)                {}
func (*stubAccountKeeper) RemoveAccount(context.Context, sdk.AccountI)             {}
func (*stubAccountKeeper) GetParams(context.Context) authtypes.Params {
	return authtypes.DefaultParams()
}
func (*stubAccountKeeper) GetSequence(context.Context, sdk.AccAddress) (uint64, error) { return 0, nil }
func (*stubAccountKeeper) AddressCodec() addresscodec.Codec                            { return nil }
func (*stubAccountKeeper) UnorderedTransactionsEnabled() bool                          { return false }
func (*stubAccountKeeper) RemoveExpiredUnorderedNonces(sdk.Context) error              { return nil }
func (*stubAccountKeeper) TryAddUnorderedNonce(sdk.Context, []byte, time.Time) error   { return nil }

type stubBankKeeper struct{}

func (*stubBankKeeper) GetBalance(context.Context, sdk.AccAddress, string) sdk.Coin {
	return sdk.Coin{}
}
func (*stubBankKeeper) IsSendEnabledCoins(context.Context, ...sdk.Coin) error { return nil }
func (*stubBankKeeper) SendCoins(context.Context, sdk.AccAddress, sdk.AccAddress, sdk.Coins) error {
	return nil
}

func (*stubBankKeeper) SendCoinsFromAccountToModule(context.Context, sdk.AccAddress, string, sdk.Coins) error {
	return nil
}

type stubFeeMarketKeeper struct{}

func (*stubFeeMarketKeeper) GetParams(sdk.Context) feemarkettypes.Params {
	return feemarkettypes.DefaultParams()
}
func (*stubFeeMarketKeeper) AddTransientGasWanted(sdk.Context, uint64) (uint64, error) { return 0, nil }
func (*stubFeeMarketKeeper) GetBaseFeeEnabled(sdk.Context) bool                        { return true }
func (*stubFeeMarketKeeper) GetBaseFee(sdk.Context) math.LegacyDec                     { return math.LegacyZeroDec() }

type stubEVMKeeper struct{}

func (*stubEVMKeeper) GetAccount(sdk.Context, common.Address) *statedb.Account { return nil }
func (*stubEVMKeeper) GetState(sdk.Context, common.Address, common.Hash) common.Hash {
	return common.Hash{}
}
func (*stubEVMKeeper) GetCode(sdk.Context, common.Hash) []byte { return nil }
func (*stubEVMKeeper) GetCodeHash(sdk.Context, common.Address) common.Hash {
	return common.Hash{}
}

func (*stubEVMKeeper) ForEachStorage(sdk.Context, common.Address, func(common.Hash, common.Hash) bool) {
}
func (*stubEVMKeeper) SetAccount(sdk.Context, common.Address, statedb.Account) error { return nil }
func (*stubEVMKeeper) DeleteState(sdk.Context, common.Address, common.Hash)          {}
func (*stubEVMKeeper) SetState(sdk.Context, common.Address, common.Hash, []byte)     {}
func (*stubEVMKeeper) DeleteCode(sdk.Context, []byte)                                {}
func (*stubEVMKeeper) SetCode(sdk.Context, []byte, []byte)                           {}
func (*stubEVMKeeper) DeleteAccount(sdk.Context, common.Address) error               { return nil }
func (*stubEVMKeeper) KVStoreKeys() map[string]*storetypes.KVStoreKey {
	return map[string]*storetypes.KVStoreKey{}
}

func (*stubEVMKeeper) NewEVM(sdk.Context, core.Message, *statedb.EVMConfig, *tracing.Hooks, vm.StateDB) *vm.EVM {
	return nil
}

func (*stubEVMKeeper) DeductTxCostsFromUserBalance(sdk.Context, sdk.Coins, common.Address) error {
	return nil
}

func (*stubEVMKeeper) SpendableCoin(sdk.Context, common.Address) *uint256.Int {
	return uint256.NewInt(0)
}
func (*stubEVMKeeper) ResetTransientGasUsed(sdk.Context)         {}
func (*stubEVMKeeper) GetTxIndexTransient(sdk.Context) uint64    { return 0 }
func (*stubEVMKeeper) GetParams(sdk.Context) evmtypes.Params     { return evmtypes.DefaultParams() }
func (*stubEVMKeeper) GetBaseFee(sdk.Context) *big.Int           { return big.NewInt(0) }
func (*stubEVMKeeper) GetMinGasPrice(sdk.Context) math.LegacyDec { return math.LegacyZeroDec() }

type stubFeegrantKeeper struct{}

func (stubFeegrantKeeper) UseGrantedFees(context.Context, sdk.AccAddress, sdk.AccAddress, sdk.Coins, []sdk.Msg) error {
	return nil
}

func TestCreateHandlerOptions(t *testing.T) {
	encodingCfg := encoding.MakeConfig(constants.ExampleChainID.EVMChainID)
	handlerMap := txsigning.NewHandlerMap(dummySignModeHandler{})

	baseArgs := handlerArgs{
		cdc:             encodingCfg.Codec,
		accountKeeper:   &stubAccountKeeper{},
		bankKeeper:      &stubBankKeeper{},
		feeMarketKeeper: &stubFeeMarketKeeper{},
		evmKeeper:       &stubEVMKeeper{},
		extensionOptionChecker: func(*codectypes.Any) bool {
			return true
		},
		signModeHandler: handlerMap,
		sigGasConsumer: func(storetypes.GasMeter, sdkSigning.SignatureV2, authtypes.Params) error {
			return nil
		},
		maxTxGasWanted: 100,
		txFeeChecker: func(sdk.Context, sdk.Tx) (sdk.Coins, int64, error) {
			return sdk.Coins{}, 0, nil
		},
		pendingTxListener: func(common.Hash) {},
	}

	testCases := []struct {
		name    string
		mutate  func(*handlerArgs)
		wantErr string
		assert  func(*testing.T, antepkg.HandlerOptions, handlerArgs)
	}{
		{
			name: "success without options",
			assert: func(t *testing.T, got antepkg.HandlerOptions, args handlerArgs) {
				t.Helper()
				require.Equal(t, args.cdc, got.Cdc)
				require.Equal(t, args.accountKeeper, got.AccountKeeper)
				require.Equal(t, args.bankKeeper, got.BankKeeper)
				require.Equal(t, args.feeMarketKeeper, got.FeeMarketKeeper)
				require.Equal(t, args.evmKeeper, got.EvmKeeper)
				require.Equal(t, args.signModeHandler, got.SignModeHandler)
				require.Equal(t, args.maxTxGasWanted, got.MaxTxGasWanted)
				require.Nil(t, got.IBCKeeper, "expected redundant relay decorator to remain disabled when WithIBCKeeper is not provided")
				require.Nil(t, got.FeegrantKeeper)
				require.NotNil(t, got.ExtensionOptionChecker)
				require.NotNil(t, got.SigGasConsumer)
				require.NotNil(t, got.TxFeeChecker)
				require.NotNil(t, got.PendingTxListener)
			},
		},
		{
			name: "success with optional keepers",
			mutate: func(args *handlerArgs) {
				args.expectedIBCKeeper = new(ibckeeper.Keeper)
				args.expectedFeegrant = stubFeegrantKeeper{}
				args.opts = []antepkg.HandlerOption{
					antepkg.WithIBCKeeper(args.expectedIBCKeeper),
					antepkg.WithFeegrantKeeper(args.expectedFeegrant),
				}
			},
			assert: func(t *testing.T, got antepkg.HandlerOptions, args handlerArgs) {
				t.Helper()
				require.Equal(t, args.expectedIBCKeeper, got.IBCKeeper)
				require.NotNil(t, got.IBCKeeper, "expected redundant relay decorator to be enabled when WithIBCKeeper is provided")
				require.Equal(t, args.expectedFeegrant, got.FeegrantKeeper)
			},
		},
		{
			name: "missing codec",
			mutate: func(args *handlerArgs) {
				args.cdc = nil
			},
			wantErr: "codec is required",
		},
		{
			name: "missing account keeper",
			mutate: func(args *handlerArgs) {
				args.accountKeeper = nil
			},
			wantErr: "account keeper is required",
		},
		{
			name: "missing bank keeper",
			mutate: func(args *handlerArgs) {
				args.bankKeeper = nil
			},
			wantErr: "bank keeper is required",
		},
		{
			name: "missing fee market keeper",
			mutate: func(args *handlerArgs) {
				args.feeMarketKeeper = nil
			},
			wantErr: "fee market keeper is required",
		},
		{
			name: "missing evm keeper",
			mutate: func(args *handlerArgs) {
				args.evmKeeper = nil
			},
			wantErr: "evm keeper is required",
		},
		{
			name: "missing signature gas consumer",
			mutate: func(args *handlerArgs) {
				args.sigGasConsumer = nil
			},
			wantErr: "signature gas consumer is required",
		},
		{
			name: "missing sign mode handler",
			mutate: func(args *handlerArgs) {
				args.signModeHandler = nil
			},
			wantErr: "sign mode handler is required",
		},
		{
			name: "missing tx fee checker",
			mutate: func(args *handlerArgs) {
				args.txFeeChecker = nil
			},
			wantErr: "tx fee checker is required",
		},
		{
			name: "missing pending tx listener",
			mutate: func(args *handlerArgs) {
				args.pendingTxListener = nil
			},
			wantErr: "pending tx listener is required",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			args := baseArgs
			args.opts = nil
			args.expectedIBCKeeper = nil
			args.expectedFeegrant = nil
			if tc.mutate != nil {
				tc.mutate(&args)
			}

			got, err := antepkg.CreateHandlerOptions(
				args.cdc,
				args.accountKeeper,
				args.bankKeeper,
				args.feeMarketKeeper,
				args.evmKeeper,
				args.extensionOptionChecker,
				args.signModeHandler,
				args.sigGasConsumer,
				args.maxTxGasWanted,
				args.txFeeChecker,
				args.pendingTxListener,
				args.opts...,
			)

			if tc.wantErr != "" {
				require.Error(t, err)
				require.Contains(t, err.Error(), tc.wantErr)
				return
			}

			require.NoError(t, err)
			if tc.assert != nil {
				tc.assert(t, got, args)
			}
		})
	}
}
