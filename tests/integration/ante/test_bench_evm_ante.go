package ante

import (
	"fmt"
	"math/big"
	"testing"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	evmante "github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testutiltx "github.com/cosmos/evm/testutil/tx"
	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

//nolint:thelper // RunBenchmarkEthGasConsumeDecorator is not a helper function; it's an externally called benchmark entry point
func RunBenchmarkEthGasConsumeDecorator(b *testing.B, create network.CreateEvmApp, options ...network.ConfigOption) {
	s := NewEvmAnteTestSuite(create, options...)
	s.SetT(&testing.T{})
	s.SetupTest()
	ctx := s.network.GetContext()
	args := &evmtypes.EvmTxArgs{
		ChainID:  evmtypes.GetEthChainConfig().ChainID,
		Nonce:    1,
		Amount:   big.NewInt(10),
		GasLimit: uint64(1_000_000),
		GasPrice: big.NewInt(1_000_000),
	}

	var vmdb *statedb.StateDB

	testCases := []struct {
		name    string
		balance sdkmath.Int
		rewards sdkmath.Int
	}{
		{
			"legacy tx - enough funds to pay for fees",
			sdkmath.NewInt(1e16),
			sdkmath.ZeroInt(),
		},
	}
	b.ResetTimer()

	for _, tc := range testCases {
		b.Run(fmt.Sprintf("Case %s", tc.name), func(b *testing.B) {
			for n := 0; n < b.N; n++ {
				// Stop the timer to perform expensive test setup
				b.StopTimer()
				addr := testutiltx.GenerateAddress()
				args.Accesses = &ethtypes.AccessList{{Address: addr, StorageKeys: nil}}
				tx := evmtypes.NewTx(args)
				tx.From = addr.Bytes()

				cacheCtx, _ := ctx.CacheContext()
				// Create new stateDB for each test case from the cached context
				vmdb = testutil.NewStateDB(cacheCtx, s.GetNetwork().App.GetEVMKeeper())
				cacheCtx = s.prepareAccount(cacheCtx, addr.Bytes(), tc.balance, tc.rewards)
				s.Require().NoError(vmdb.Commit())

				baseFee := s.GetNetwork().App.GetFeeMarketKeeper().GetParams(ctx).BaseFee
				fee := tx.GetEffectiveFee(baseFee.BigInt())
				denom := evmtypes.GetEVMCoinDenom()
				fees := sdk.NewCoins(sdk.NewCoin(denom, sdkmath.NewIntFromBigInt(fee)))
				bechAddr := sdk.AccAddress(addr.Bytes())

				// Benchmark only the ante handler logic - start the timer
				b.StartTimer()

				err := evmante.ConsumeFeesAndEmitEvent(
					cacheCtx.WithIsCheckTx(true).WithGasMeter(storetypes.NewInfiniteGasMeter()),
					s.GetNetwork().App.GetEVMKeeper(),
					fees,
					bechAddr,
				)
				s.Require().NoError(err)
			}
		})
	}
}
