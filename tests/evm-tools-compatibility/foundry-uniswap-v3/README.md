# Foundry Uniswap V3 Deployment Test

This project showcases how Foundry works seamlessly with a Cosmos-SDK app chain that integrates Cosmos-EVM.
It uses a Uniswap V3 contracts to validate deployment of contracts with complex interdependencies.

## Prerequisites

- **Foundry**: Ensure Foundry (`forge` and `cast`) is installed:

  ```bash
  curl -L https://foundry.paradigm.xyz | bash
  foundryup
  ```

- **Local node**: A running Cosmos-SDK / CometBFT chain exposing an Ethereum-compatible RPC endpoint at `$CUSTOM_RPC`.
- **GNU Make** (optional) for convenience.

## Initial Setup

1. **Clone the repository**

   ```bash
   git clone https://github.com/b-harvest/evm-tools-compatibility.git
   cd evm-tools-compatibility/foundry-uniswap-v3
   ```

2. **Install dependencies**

   ```bash
   forge install
   ```

3. **Create environment file**
   Create a `.env` file in this directory with:

   ```bash
   cp .env.example .env
   # modify .env
   ```

   > **Note:** Do not commit `.env` to version control.

## Usage

### Compile

```bash
forge build
```

### Deploy UniswapV3 Contract

Deploy NFTDescriptor library first.

```bash
source .env
forge script script/DeployNFTDescriptor.s.sol:DeployNFTDescriptor \
  --rpc-url $CUSTOM_RPC \
  --broadcast \
  --chain-id $CHAIN_ID
```

Then, deploy other contracts with NFTDescriptor library address.

```bash
source .env
forge script script/DeployUniswapV3.s.sol:DeployUniswapV3 \
  --rpc-url $CUSTOM_RPC \
  --chain-id $CHAIN_ID \
  --broadcast \
  --slow \
  --libraries lib/v3-periphery/contracts/libraries/NFTDescriptor.sol:NFTDescriptor:$LIBRARY_CONTRACT
```
