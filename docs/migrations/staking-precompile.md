# Staking precompile behavior upgrade

The staking precompile accepts validator addresses in every format configured
in the chain's staking keeper, including 0x-hex addresses where supported.
`Unbond` reports the amount actually undelegated. Immediate redelegations
report a completion time of `0`, and cancellation rejects creation heights
outside the nonnegative `int64` range. The Solidity function and event
signatures remain the same.

These changes can alter transaction success and EVM logs. Activate the new
application binary at the same planned upgrade height across all validators.
A rolling binary upgrade across multiple heights can produce different
execution results on different validators.

1. If your app supplies a custom `precompiles/common.StakingKeeper`, implement
   `ValidatorAddressCodec()` using the same codec as the staking message server.
   Update custom callers of `EmitUnbondEvent` to pass the undelegated amount
   from the SDK response.
2. Register the chain's normal software upgrade handler and rehearse the
   upgrade height on a testnet. This change needs no additional store
   migration or staking state rewrite. Run any module migrations required
   by the release as usual.
3. At the planned height, switch validators and other full nodes to the new
   binary. Confirm the network finalizes blocks after activation.
4. Update consumers of staking logs to handle the actual undelegated amount
   and the immediate-completion sentinel. Historical logs retain their
   original values.
