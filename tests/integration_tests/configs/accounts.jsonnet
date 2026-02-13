local config = import 'default.jsonnet';
local chain = (import 'chains.jsonnet')[std.extVar('CHAIN_CONFIG')];

config {
  'evm-canary-net-1'+: {
    accounts: super.accounts[:std.length(super.accounts) - 1] + [
      {
        'coin-type': 60,
        name: 'user' + i,
        coins: '100000000000000000' + chain.evm_denom,
      }
      for i in std.range(0, 5)
    ],
  },
}
