local config = import 'min_gas_price.jsonnet';

config {
  'evm-canary-net-1'+: {
    genesis+: {
      app_state+: {
        feemarket+: {
          params+: {
            base_fee_change_denominator: '300',
            elasticity_multiplier: '4000',
          },
        },
      },
    },
  },
}
