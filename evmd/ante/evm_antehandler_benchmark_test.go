package ante_test

import (
	"fmt"
	"math/big"
	"testing"

	"github.com/cosmos/evm/ante"
	ethante "github.com/cosmos/evm/ante/evm"
	evmdante "github.com/cosmos/evm/evmd/ante"
	"github.com/cosmos/evm/evmd/tests/integration"
	basefactory "github.com/cosmos/evm/testutil/integration/base/factory"
	"github.com/cosmos/evm/testutil/integration/evm/factory"
	"github.com/cosmos/evm/testutil/integration/evm/grpc"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	testkeyring "github.com/cosmos/evm/testutil/keyring"
	cosmosevmtypes "github.com/cosmos/evm/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/errors"
	"cosmossdk.io/math"

	sdktypes "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type benchmarkSuite struct {
	network     *network.UnitTestNetwork
	grpcHandler grpc.Handler
	txFactory   factory.TxFactory
	keyring     testkeyring.Keyring
}

// Setup
var table = []struct {
	name     string
	txType   string
	simulate bool
}{
	{
		"evm_transfer_sim",
		"evm_transfer",
		true,
	},
	{
		"evm_transfer",
		"evm_transfer",
		false,
	},
	{
		"bank_msg_send_sim",
		"bank_msg_send",
		true,
	},
	{
		"bank_msg_send",
		"bank_msg_send",
		false,
	},
}

//nolint:thelper // RunBenchmarkAnteHandler is not a helper function; it's an externally called benchmark entry point
func RunBenchmarkAnteHandler(b *testing.B, create network.CreateEvmApp, options ...network.ConfigOption) {
	keyring := testkeyring.New(2)

	for _, v := range table {
		// Reset chain on every tx type to have a clean state
		// and a fair benchmark
		b.StopTimer()
		opts := []network.ConfigOption{
			network.WithPreFundedAccounts(keyring.GetAllAccAddrs()...),
		}
		opts = append(opts, options...)
		unitNetwork := network.NewUnitTestNetwork(create, opts...)
		grpcHandler := grpc.NewIntegrationHandler(unitNetwork)
		txFactory := factory.New(unitNetwork, grpcHandler)
		suite := benchmarkSuite{
			network:     unitNetwork,
			grpcHandler: grpcHandler,
			txFactory:   txFactory,
			keyring:     keyring,
		}

		handlerOptions := suite.generateHandlerOptions()
		ante := evmdante.NewAnteHandler(handlerOptions)
		b.StartTimer()

		b.Run(fmt.Sprintf("tx_type_%v", v.name), func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				// Stop timer while building the tx setup
				b.StopTimer()
				// Start with a clean block
				if err := unitNetwork.NextBlock(); err != nil {
					b.Fatal(errors.Wrap(err, "failed to create block"))
				}
				ctx := unitNetwork.GetContext()

				// Generate fresh tx type
				tx, err := suite.generateTxType(v.txType)
				if err != nil {
					b.Fatal(errors.Wrap(err, "failed to generate tx type"))
				}
				b.StartTimer()

				// Run benchmark
				_, err = ante(ctx, tx, v.simulate)
				if err != nil {
					b.Fatal(errors.Wrap(err, "failed to run ante handler"))
				}
			}
		})
	}
}

func (s *benchmarkSuite) generateTxType(txType string) (sdktypes.Tx, error) {
	switch txType {
	case "evm_transfer":
		senderPriv := s.keyring.GetPrivKey(0)
		receiver := s.keyring.GetKey(1)
		txArgs := evmtypes.EvmTxArgs{
			To:     &receiver.Addr,
			Amount: big.NewInt(1000),
		}
		return s.txFactory.GenerateSignedEthTx(senderPriv, txArgs)
	case "bank_msg_send":
		sender := s.keyring.GetKey(1)
		receiver := s.keyring.GetAccAddr(0)
		bankmsg := banktypes.NewMsgSend(
			sender.AccAddr,
			receiver,
			sdktypes.NewCoins(
				sdktypes.NewCoin(
					s.network.GetBaseDenom(),
					math.NewInt(1000),
				),
			),
		)
		txArgs := basefactory.CosmosTxArgs{Msgs: []sdktypes.Msg{bankmsg}}
		return s.txFactory.BuildCosmosTx(sender.Priv, txArgs)
	default:
		return nil, fmt.Errorf("invalid tx type")
	}
}

func (s *benchmarkSuite) generateHandlerOptions() ante.HandlerOptions {
	encCfg := s.network.GetEncodingConfig()
	return ante.HandlerOptions{
		Cdc:                    s.network.App.AppCodec(),
		AccountKeeper:          s.network.App.GetAccountKeeper(),
		BankKeeper:             s.network.App.GetBankKeeper(),
		ExtensionOptionChecker: cosmosevmtypes.HasDynamicFeeExtensionOption,
		EvmKeeper:              s.network.App.GetEVMKeeper(),
		FeegrantKeeper:         s.network.App.GetFeeGrantKeeper(),
		IBCKeeper:              s.network.App.GetIBCKeeper(),
		FeeMarketKeeper:        s.network.App.GetFeeMarketKeeper(),
		SignModeHandler:        encCfg.TxConfig.SignModeHandler(),
		SigGasConsumer:         ante.SigVerificationGasConsumer,
		MaxTxGasWanted:         1_000_000_000,
		TxFeeChecker:           ethante.NewDynamicFeeChecker(s.network.App.GetFeeMarketKeeper()),
	}
}

func BenchmarkAnteHandler(b *testing.B) {
	// Run the benchmark with a mock EVM app
	RunBenchmarkAnteHandler(b, integration.CreateEvmd)
}
