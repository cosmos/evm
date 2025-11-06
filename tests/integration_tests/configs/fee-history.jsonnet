local config = import 'default.jsonnet';

config {
  'evm-canary-net-1'+: {
    genesis+: {
      app_state+: {
        feemarket+: {
          params+: {
            elasticity_multiplier: 3,
            base_fee_change_denominator: 100000000,
            // min_gas_price: '0.000000000000000000', # TODO: remove after basefee fix
          },
        },
      },
    },
  },
}
