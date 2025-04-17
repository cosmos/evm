package ibctesting

import (
	"encoding/json"

	dbm "github.com/cosmos/cosmos-db"
	ibctesting "github.com/cosmos/ibc-go/v10/testing"

	"github.com/cosmos/evm/evmd"
	"github.com/cosmos/evm/evmd/testutil"
	feemarkettypes "github.com/cosmos/evm/x/feemarket/types"

	"cosmossdk.io/log"

	simtestutil "github.com/cosmos/cosmos-sdk/testutil/sims"
)

func SetupExampleApp() (ibctesting.TestingApp, map[string]json.RawMessage) {
	app := evmd.NewExampleApp(
		log.NewNopLogger(),
		dbm.NewMemDB(),
		nil,
		true,
		simtestutil.EmptyAppOptions{},
		testutil.NoOpEvmAppOptions,
	)
	// disable base fee for testing
	genesisState := app.DefaultGenesis()
	fmGen := feemarkettypes.DefaultGenesisState()
	fmGen.Params.NoBaseFee = true
	genesisState[feemarkettypes.ModuleName] = app.AppCodec().MustMarshalJSON(fmGen)

	return app, genesisState
}
