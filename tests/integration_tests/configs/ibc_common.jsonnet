{
  'coin-type': 60,
  'app-config'+: {
    'index-events': super['index-events'] + ['message.action'],
  },
  genesis+: {
    app_state+: {
      feemarket+: {
        params+: {
          no_base_fee: true,
          base_fee: '0',
          min_gas_price: '0',
        },
      },
      staking+: {
        params+: {
          unbonding_time: '1814400s',
        },
      },
    },
  },
}
