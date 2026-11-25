# Distribution precompile behavior upgrade

The distribution precompile accepts validator addresses in every format
configured in the chain's staking keeper, including 0x-hex addresses where
supported, for `withdrawDelegatorRewards`, `withdrawValidatorCommission`, and
`depositValidatorRewardsPool`. Shared `DecCoin` outputs report precision `0`
because their amounts are truncated integers. The amount and Solidity
signatures stay the same.

Previously rejected calls can now succeed and change chain state. Activate
the new application binary at the same planned upgrade height across all
validators.

1. Include the staking precompile change and complete its
   [keeper and application upgrade steps](staking-precompile.md). Custom Go
   callers of `distribution.NewMsgWithdrawValidatorCommission` must pass the
   chain's validator address codec.
2. Use the chain's normal software upgrade handler and rehearse activation
   at the planned height on a testnet. This change needs no additional store
   migration or reward state rewrite. Run any module migrations required
   by the release as usual.
3. At the planned height, switch validators and other full nodes to the new
   binary. Confirm the network finalizes blocks after activation.
4. Update callers that assumed precision `18` for `DecCoin` outputs to read
   the reported precision. Fractional amounts below one base unit are still
   truncated. The separate `Dec` output type continues to report precision
   `18`.
