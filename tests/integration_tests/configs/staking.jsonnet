local config = import 'default.jsonnet';
local chain = (import 'chains.jsonnet')[std.extVar('CHAIN_CONFIG')];
local gas_price = 1000000000;

config {
  'evm-canary-net-1'+: {
    validators: super.validators + [{
      'coin-type': 60,
      coins: '100000000000000000000' + chain.evm_denom,
      staked: '10000000000000000000' + chain.evm_denom,
      gas_prices: gas_price + chain.evm_denom,
      min_self_delegation: 1000000000000000000,
      mnemonic: '${VALIDATOR4_MNEMONIC}',
    }],
  },
}
