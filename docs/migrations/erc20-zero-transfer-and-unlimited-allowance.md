# ERC-20 and WERC-20 precompile transfers

[PR #1337](https://github.com/cosmos/evm/pull/1337) changes the execution of
`transfer` and `transferFrom` in the ERC-20 and WERC-20 precompiles:

- Zero-value transfers emit `Transfer` without moving coins when sending is
  enabled for the denomination and the recipient can receive funds.
- `transferFrom` keeps an allowance of `type(uint256).max` at that value and
  omits the corresponding `Approval` event. Finite allowances still decrease.

The change affects transaction success, logs, and allowance writes. Validators
must activate the upgraded binary at the same planned chain upgrade height.
Follow the [coordinated upgrade procedure](./v0.6.x_to_v0.7.0.md#step-8-coordinate-the-upgrade)
and stage the new binary for that height.

Existing allowance entries keep their encoding and value. The new binary
interprets entries already set to the maximum as unlimited, so the upgrade
handler needs no allowance or balance rewrite.

Before scheduling the upgrade, verify on a staging chain that ERC-20 and WERC-20
zero-value transfers emit `Transfer`, zero-value transfers still respect bank
send restrictions, and a maximum allowance remains at its stored value after a
successful `transferFrom`.
