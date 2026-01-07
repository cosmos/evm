local config = import 'default.jsonnet';
local chain = (import 'chains.jsonnet')[std.extVar('CHAIN_CONFIG')];

config {
  'evm-canary-net-1'+: {
    hw_account: {
      name: 'hw',
      coins: '8000000000000000000' + chain.evm_denom,
      'coin-type': 118,
    },
  },
}
