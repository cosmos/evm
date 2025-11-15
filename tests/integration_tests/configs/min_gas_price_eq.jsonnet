local config = import 'min_gas_price_lte.jsonnet';

config {
  'evm-canary-net-1'+: {
    genesis+: {
      consensus+: {
        params+: {
          block+: {
            max_gas: '84000000',
          },
        },
      },
    },
  },
}
