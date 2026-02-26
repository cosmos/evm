# cosmos-evm-contracts

A collection of smart contracts for the Cosmos EVM blockchain.  
The published package includes precompile interface sources (`.sol`) and ABIs (`.json`).

## Installation

```bash
# pnpm
pnpm add cosmos-evm-contracts

# npm
npm install cosmos-evm-contracts

# yarn
yarn add cosmos-evm-contracts
```

## Package structure

After installation, use the following paths:

| Path | Description |
|------|-------------|
| `cosmos-evm-contracts/precompiles/` | Solidity sources (`.sol`) |
| `cosmos-evm-contracts/abi/precompiles/` | ABI-only JSON (`.json`) |

Included precompiles: `bank`, `bech32`, `callbacks`, `common`, `distribution`, `erc20`, `gov`, `ics02`, `ics20`, `slashing`, `staking`, `werc20` (testdata and testutil excluded).

## Usage

### Loading ABI (ethers / viem / web3, etc.)

```javascript
import IBankAbi from "cosmos-evm-contracts/abi/precompiles/bank/IBank.json" assert { type: "json" };

// or Node
const IBankAbi = require("cosmos-evm-contracts/abi/precompiles/bank/IBank.json");
```

### Using interfaces in Hardhat

Import by package path in your contract:

```solidity
import "cosmos-evm-contracts/precompiles/bank/IBank.sol";
```

### Using interfaces in Foundry

Add the following to `remappings.txt` for shorter import paths:

```
cosmos-evm-contracts/=node_modules/cosmos-evm-contracts/precompiles/
```

```solidity
import "cosmos-evm-contracts/bank/IBank.sol";
```

### Path reference

- Interface ABI: `cosmos-evm-contracts/abi/precompiles/{module}/{Interface}.json`  
  e.g. `abi/precompiles/staking/StakingI.json`
- Common types: `cosmos-evm-contracts/precompiles/common/Types.sol` (structs only, no ABI)
