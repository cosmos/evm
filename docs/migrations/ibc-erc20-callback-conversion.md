# Upgrade for ERC20 conversion in IBC callbacks

[PR #1331](https://github.com/cosmos/evm/pull/1331) changes the state and gas
accounting for native ERC20 token conversions in IBC v1 and v2 packet
callbacks.

- When conversion on receive fails, IBC can write an error acknowledgement
  and discard the receive-side changes.
- When conversion on a timeout or error acknowledgement fails, the
  conversion's partial writes are discarded. The sender keeps the refunded
  bank coins if the enclosing transaction commits.
- A conversion has a single 25 million gas budget for the bank operations,
  EVM transfer, and balance checks together. It charges the enclosing
  transaction for work performed, even on failure. Internal EVM calls within
  that conversion charge gas before EVM refunds. If the transaction supplies
  less than the conversion budget and runs out during conversion, the
  transaction fails and the relayer can retry with more gas.

Use a scheduled software upgrade as described in the
[v0.7 operator upgrade steps](v0.6.x_to_v0.7.0.md#step-8-coordinate-the-upgrade).
Switch all validators to the same release binary at the planned height.
Running old and new binaries at the same height can produce different gas
charges and committed state for the same block.

Before activation, rehearse packet receive and refund with the native ERC20
tokens currently registered on your chain. A conversion that exceeds the
aggregate budget can fail even if each individual EVM call used to fit within
its per-call limit. Verify relayer simulation and account for callback gas
when setting relay transaction limits.

This change requires no new module store migration. It applies to callbacks
processed after activation and does not recover balances stranded by
historical transactions.
