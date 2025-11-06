local chain = (import 'chains.jsonnet')[std.extVar('CHAIN_CONFIG')];
local gas_price = 1000000000;

{
  dotenv: '../../scripts/.env',
  'evm-canary-net-1': {
    cmd: chain.cmd,
    'start-flags': '--trace',
    config: {
      mempool: {
        version: 'v1',
      },
    },
    'app-config': {
      evm: {
        'evm-chain-id': chain.evm_chain_id,
      },
      grpc: {
        'skip-check-header': true,
      },
      'minimum-gas-prices': '0' + chain.evm_denom,
      'index-events': ['ethereum_tx.ethereumTxHash'],
      'iavl-lazy-loading': true,
      'json-rpc': {
        enable: true,
        address: '127.0.0.1:{EVMRPC_PORT}',
        'ws-address': '127.0.0.1:{EVMRPC_PORT_WS}',
        api: 'eth,net,web3,debug,txpool',
        'feehistory-cap': 100,
        'block-range-cap': 10000,
        'logs-cap': 10000,
        'gas-cap': 30000000,
        'allow-unprotected-txs': true,
      },
      mempool: {
        'max-txs': 5000,
      },
    },
    validators: [{
      'coin-type': 60,
      coins: '100000000000000000000' + chain.evm_denom,
      staked: '10000000000000000000' + chain.evm_denom,
      gas_prices: gas_price + chain.evm_denom,
      mnemonic: '${VALIDATOR1_MNEMONIC}',
    }, {
      'coin-type': 60,
      coins: '100000000000000000000' + chain.evm_denom,
      staked: '10000000000000000000' + chain.evm_denom,
      gas_prices: gas_price + chain.evm_denom,
      mnemonic: '${VALIDATOR2_MNEMONIC}',
      config: {
        db_backend: 'pebbledb',
      },
      'app-config': {
        'app-db-backend': 'pebbledb',
      },
    }, {
      'coin-type': 60,
      coins: '100000000000000000000' + chain.evm_denom,
      staked: '10000000000000000000' + chain.evm_denom,
      gas_prices: gas_price + chain.evm_denom,
      mnemonic: '${VALIDATOR3_MNEMONIC}',
      config: {
        db_backend: 'goleveldb',
      },
      'app-config': {
        'app-db-backend': 'goleveldb',
      },
    }],
    accounts: [{
      'coin-type': 60,
      name: 'community',
      coins: '100000000000000000000' + chain.evm_denom + ',1000000000000atoken',
      mnemonic: '${COMMUNITY_MNEMONIC}',
    }, {
      'coin-type': 60,
      name: 'signer1',
      coins: '100000000000000000000' + chain.evm_denom,
      mnemonic: '${SIGNER1_MNEMONIC}',
    }, {
      'coin-type': 60,
      name: 'signer2',
      coins: '100000000000000000000' + chain.evm_denom,
      mnemonic: '${SIGNER2_MNEMONIC}',
    }, {
      'coin-type': 60,
      name: 'reserve',
      coins: '100000000000000000000' + chain.evm_denom,
      mnemonic: '${RESERVE_MNEMONIC}',
      vesting: '60s',
    }],
    genesis: {
      consensus: {
        params: {
          block: {
            max_bytes: '1048576',
            max_gas: '81500000',
          },
          abci: {
            vote_extensions_enable_height: '1',
          },
        },
      },
      app_state: {
        evm: chain.evm {
          params+: {
            evm_denom: chain.evm_denom,
            active_static_precompiles: [
              '0x0000000000000000000000000000000000000800',
              '0x0000000000000000000000000000000000000801',
              '0x0000000000000000000000000000000000000805',
              '0x0000000000000000000000000000000000000807',
            ],
          },
        },
        erc20: {
          native_precompiles: [
            '0x4200000000000000000000000000000000000006',
          ],
          token_pairs: [{
            erc20_address: '0x4200000000000000000000000000000000000006',
            denom: chain.evm_denom,
            enabled: true,
            contract_owner: 1,
          }],
        },
        feemarket: chain.feemarket {
          params+: {
            min_gas_multiplier: '0',
          },
        },
        gov: {
          params: {
            expedited_voting_period: '1s',
            voting_period: '10s',
            max_deposit_period: '10s',
            min_deposit: [
              {
                denom: chain.evm_denom,
                amount: '1',
              },
            ],
            expedited_min_deposit: [
              {
                denom: chain.evm_denom,
                amount: '2',
              },
            ],
          },
        },
        crisis: {
          constant_fee: {
            denom: chain.evm_denom,
          },
        },
        mint: {
          params: {
            mint_denom: chain.evm_denom,
          },
        },
        staking: {
          params: {
            bond_denom: chain.evm_denom,
            unbonding_time: '10s',
          },
        },
        bank: chain.bank {
          denom_metadata+: [{
            denom_units+: [
              {
                denom: 'atoken',
                exponent: 0,
              },
              {
                denom: 'token',
                exponent: 18,
              },
            ],
            base: 'atoken',
            display: 'token',
            name: 'Test Coin',
            symbol: 'ATOKEN',
          }],
        },
      },
    },
  },
}
