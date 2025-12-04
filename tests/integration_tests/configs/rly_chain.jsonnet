{
  max_gas: 2500000,
  gas_multiplier: 1.1,
  address_type: {
    derivation: 'ethermint',
    proto_type: {
      pk_type: '/cosmos.evm.crypto.v1.ethsecp256k1.PubKey',
    },
  },
  gas_price: {
    price: 0.1,
  },
  event_source: {
    batch_delay: '5000ms',
  },
  extension_options: [{
    type: 'cosmos_evm_dynamic_fee_v1',
    value: '10000000000000000',
  }],
}
