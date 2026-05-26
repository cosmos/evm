package mempool

import sdk "github.com/cosmos/cosmos-sdk/types"

// LookupGasWantedForTest exposes lookupGasWanted to external _test packages.
func (m *Mempool) LookupGasWantedForTest(tx sdk.Tx) (uint64, bool) {
	return m.lookupGasWanted(tx)
}
