package integration

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	evm "github.com/cosmos/evm"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	bankprecompile "github.com/cosmos/evm/precompiles/bank"
	testapp "github.com/cosmos/evm/testutil/app"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	"github.com/cosmos/evm/testutil/keyring"
	"github.com/cosmos/evm/x/vm/types"
)

// BenchmarkApplyTransaction measures keeper.ApplyTransaction for a plain value
// transfer and a bank precompile call (which branches the snapshot store).
func BenchmarkApplyTransaction(b *testing.B) {
	bankAddr := common.HexToAddress(types.BankPrecompileAddress)
	balancesInput, err := bankprecompile.ABI.Pack("balances", common.Address{0x1})
	require.NoError(b, err)

	cases := []struct {
		name  string
		to    common.Address
		data  []byte
		gas   uint64
		value int64
	}{
		{"transfer", common.Address{0x2}, nil, 21_000, 1},
		{"bank_precompile", bankAddr, balancesInput, 200_000, 0},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			vmAppCreator := testapp.ToEvmAppCreator[evm.VMIntegrationApp](CreateEvmd, "evm.VMIntegrationApp")
			keys := keyring.New(1)
			nw := network.NewUnitTestNetwork(vmAppCreator, network.WithPreFundedAccounts(keys.GetAllAccAddrs()...))
			ctx := nw.GetContext()
			k := nw.App.GetEVMKeeper()

			priv, err := keys.GetKey(0).Priv.(*ethsecp256k1.PrivKey).ToECDSA()
			require.NoError(b, err)
			signer := ethtypes.LatestSignerForChainID(types.GetEthChainConfig().ChainID)
			from := keys.GetAddr(0)

			b.ReportAllocs()
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				tx, err := ethtypes.SignNewTx(priv, signer, &ethtypes.LegacyTx{
					Nonce:    k.GetNonce(ctx, from),
					To:       &tc.to,
					Value:    big.NewInt(tc.value),
					Gas:      tc.gas,
					GasPrice: big.NewInt(0), // ApplyTransaction skips ante fee deduction, so refunds must be 0
					Data:     tc.data,
				})
				require.NoError(b, err)
				// fresh per-tx gas meter, matching what the EVM ante sets
				txCtx := ctx.WithGasMeter(types.NewInfiniteGasMeterWithLimit(tc.gas))
				b.StartTimer()

				res, err := k.ApplyTransaction(txCtx, tx)

				b.StopTimer()
				require.NoError(b, err)
				require.False(b, res.Failed(), res.VmError)
				b.StartTimer()
			}
		})
	}
}
