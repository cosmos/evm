{
  evmd: {
    'account-prefix': 'cosmos',
    evm_denom: 'atest',
    cmd: 'evmd',
    evm_chain_id: 262144,
    bank: {
      denom_metadata: [{
        description: 'Native 18-decimal denom metadata for Cosmos EVM chain',
        denom_units: [
          {
            denom: 'atest',
            exponent: 0,
          },
          {
            denom: 'test',
            exponent: 18,
          },
        ],
        base: 'atest',
        display: 'test',
        name: 'Cosmos EVM',
        symbol: 'ATOM',
      }],
    },
    evm: {},
    feemarket: {
      params: {
        base_fee: '1000000000',
        min_gas_price: '0',
      },
    },
  },
}
