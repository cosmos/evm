package evmd

import (
	"encoding/json"
	"fmt"

	cmttypes "github.com/cometbft/cometbft/types"

	tmproto "github.com/cometbft/cometbft/proto/tendermint/types"

	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// ExportAppStateAndValidators exports the state of the application for a genesis
// file.
func (app *EVMD) ExportAppStateAndValidators(forZeroHeight bool, jailAllowedAddrs []string, modulesToExport []string) (servertypes.ExportedApp, error) {
	// as if they could withdraw from the start of the next block
	ctx := app.NewContextLegacy(true, tmproto.Header{Height: app.LastBlockHeight()})

	// We export at last height + 1, because that's the height at which
	// CometBFT will start InitChain.
	height := app.LastBlockHeight() + 1
	if forZeroHeight {
		height = 0
		if err := app.prepForZeroHeightGenesis(ctx, jailAllowedAddrs); err != nil {
			return servertypes.ExportedApp{}, err
		}
	}

	genState, err := app.ModuleManager.ExportGenesisForModules(ctx, app.appCodec, modulesToExport)
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	appState, err := json.MarshalIndent(genState, "", "  ")
	if err != nil {
		return servertypes.ExportedApp{}, err
	}

	// Export POA validators as CometBFT genesis validators.
	poaVals, err := app.POAKeeper.GetAllValidators(ctx)
	if err != nil {
		return servertypes.ExportedApp{}, fmt.Errorf("failed to get POA validators: %w", err)
	}

	validators := make([]cmttypes.GenesisValidator, 0, len(poaVals))
	for _, v := range poaVals {
		var pk cryptotypes.PubKey
		if err := app.appCodec.UnpackAny(v.PubKey, &pk); err != nil {
			return servertypes.ExportedApp{}, fmt.Errorf("failed to unpack validator pubkey: %w", err)
		}
		cmtPk, err := cryptocodec.ToCmtPubKeyInterface(pk)
		if err != nil {
			return servertypes.ExportedApp{}, fmt.Errorf("failed to convert pubkey: %w", err)
		}

		moniker := ""
		if v.Metadata != nil {
			moniker = v.Metadata.Moniker
		}

		validators = append(validators, cmttypes.GenesisValidator{
			Address: sdk.ConsAddress(cmtPk.Address()).Bytes(),
			PubKey:  cmtPk,
			Power:   v.Power,
			Name:    moniker,
		})
	}

	return servertypes.ExportedApp{
		AppState:        appState,
		Validators:      validators,
		Height:          height,
		ConsensusParams: app.GetConsensusParams(ctx),
	}, nil
}

// prepForZeroHeightGenesis prepares for a fresh start at zero height.
// In POA mode there is no staking/distribution/slashing state to reset,
// so this is essentially a no-op.
func (app *EVMD) prepForZeroHeightGenesis(_ sdk.Context, _ []string) error {
	return nil
}
