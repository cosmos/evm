local config = import 'default.jsonnet';

config {
  'evm-canary-net-1'+: {
    config+: {
      tx_index+: {
        indexer: 'null',
      },
    },
    'app-config'+: {
      'json-rpc'+: {
        'enable-indexer': true,
      },
    },
  },
}
