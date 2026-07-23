# cosmos-evm-contracts

A collection of smart contracts for the Cosmos EVM blockchain.
The published package includes precompile interface sources (`.sol`) and ABIs as typed ESM/CJS modules.

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
| `cosmos-evm-contracts/precompiles/{module}/{Interface}.sol` | Solidity sources (`.sol`) |
| `cosmos-evm-contracts/precompiles/{module}/{Interface}` | ABI as typed ESM/CJS modules |

Included precompiles: `bank`, `bech32`, `callbacks`, `common`, `distribution`, `erc20`, `gov`, `ics02`, `ics20`, `slashing`, `staking`, `werc20` (testdata and testutil excluded).

## Usage

### Loading ABI with TypeScript / viem (typed)

Import the named ABI constant so that `functionName`, `args`, and return types are inferred:

```typescript
import { createPublicClient, http } from "viem";
import { iBankAbi } from "cosmos-evm-contracts/precompiles/bank/IBank";

const client = createPublicClient({ transport: http() });

// functionName and args are type-checked and autocompleted
const balances = await client.readContract({
  address: "0x0000000000000000000000000000000000000804",
  abi: iBankAbi,
  functionName: "balances",
  args: ["0x..."],
});
```

Use the same pattern for other precompiles, e.g. `distributionIAbi` from `precompiles/distribution/DistributionI`, `stakingIAbi` from `precompiles/staking/StakingI`, etc.

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

- Interface ABI: `cosmos-evm-contracts/precompiles/{module}/{Interface}` (typed ESM/CJS module)
  e.g. `precompiles/staking/StakingI`
- Common types: `cosmos-evm-contracts/precompiles/common/Types.sol` (structs only, no ABI)
