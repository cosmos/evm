# cosmos-evm-contracts

A collection of smart contracts for the Cosmos EVM blockchain.  
The published package includes precompile interface sources (`.sol`) and ABIs as typed TypeScript (`.ts`).

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
| `cosmos-evm-contracts/abi/precompiles/` | ABI as typed ESM (`.ts`) |

Included precompiles: `bank`, `bech32`, `callbacks`, `common`, `distribution`, `erc20`, `gov`, `ics02`, `ics20`, `slashing`, `staking`, `werc20` (testdata and testutil excluded).

## Usage

### Loading ABI with TypeScript / viem (typed)

Import the named ABI constant so that `functionName`, `args`, and return types are inferred:

```typescript
import { createPublicClient, http } from "viem";
import { IBank_ABI } from "cosmos-evm-contracts/abi/precompiles/bank/IBank";

const client = createPublicClient({ transport: http() });

// functionName and args are type-checked and autocompleted
const balances = await client.readContract({
  address: "0x0000000000000000000000000000000000000804",
  abi: IBank_ABI,
  functionName: "balances",
  args: ["0x..."],
});
```

Use the same pattern for other precompiles, e.g. `DistributionI_ABI` from `abi/precompiles/distribution/DistributionI`, `StakingI_ABI` from `abi/precompiles/staking/StakingI`, etc.

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

- Interface ABI: `cosmos-evm-contracts/abi/precompiles/{module}/{Interface}` (`.ts`)  
  e.g. `abi/precompiles/staking/StakingI`
- Common types: `cosmos-evm-contracts/precompiles/common/Types.sol` (structs only, no ABI)
