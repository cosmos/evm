local config = import 'default.jsonnet';
local chain = (import 'chains.jsonnet')[std.extVar('CHAIN_CONFIG')];
local gas_price = 10000000000000;

config {
  'evm-canary-net-1'+: {
    validators: [validator {
      gas_prices: gas_price + chain.evm_denom,
    } for validator in super.validators],
    genesis+: {
      consensus+: {
        params+: {
          block+: {
            max_gas+: '81500000',
          },
        },
      },
      app_state+: {
        feemarket+: {
          params+: {
            base_fee_change_denominator: '3',
            elasticity_multiplier: '4',
            base_fee: gas_price,
            min_gas_price: gas_price,
          },
        },
      },
    },
  },
}
