# Upgrade for StateDB precompile balance event accounting

[PR #1330](https://github.com/cosmos/evm/pull/1330) changes how StateDB applies
bank balance events from precompile actions that make nested EVM calls. A
transaction that previously committed stale balances can commit different
balances with the corrected execution. The change takes effect when the new
binary starts processing blocks.

1. Include the change in the chain's normal coordinated software upgrade.
   Follow the [v0.7 operator upgrade steps](v0.6.x_to_v0.7.0.md#step-8-coordinate-the-upgrade)
   and use your chain's release plan name and height.
2. Rehearse the upgrade against a recent snapshot before scheduling it.
   Verify that nodes running the target binary agree on the app hash after
   activation and that precompile integration tests check the resulting
   bank and EVM balances.
3. Switch validators to the same release binary at the scheduled upgrade
   height. Old and new binaries can produce different app hashes for the
   same block, so a rolling binary replacement can fork the chain.

Processed event ranges exist only during transaction execution. This change
requires no new module store migration. The upgrade inherits the existing
upgrade handler and does not rewrite balances committed by earlier blocks.
