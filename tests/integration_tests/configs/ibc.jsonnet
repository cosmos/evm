local config = import 'default.jsonnet';
local rly_chain = import 'rly_chain.jsonnet';
local rly_common = import 'rly_common.jsonnet';
local chain = (import 'chains.jsonnet')[std.extVar('CHAIN_CONFIG')];
local basic = config['evm-canary-net-1'];
local ibc_common = import 'ibc_common.jsonnet';

config {
  'evm-canary-net-1'+: ibc_common {
    key_name: 'signer2',
    'account-prefix': chain['account-prefix'],
  },
  'evm-canary-net-2'+: basic + ibc_common {
    key_name: 'signer1',
    'account-prefix': chain['account-prefix'],
    validators: [validator {
      base_port: 26800 + i * 10,
    } for i in std.range(0, std.length(super.validators) - 1) for validator in [super.validators[i]]],
  },
  relayer: rly_common {
    chains: [
      rly_chain {
        id: 'evm-canary-net-1',
        gas_price+: {
          denom: chain.evm_denom,
        },
      },
      rly_chain {
        id: 'evm-canary-net-2',
        gas_price+: {
          denom: chain.evm_denom,
        },
      },
    ],
  },
}
