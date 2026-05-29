# Foundry Compatibility Test

This project showcases how Foundry works seamlessly with a Cosmos-SDK app chain that integrates Cosmos-EVM.
It uses a simple ERC-20 token to validate deployment, minting, and transfer operations.
You can run these tests with both `forge` and `cast` and compare the results side by side.

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
   cd evm-tools-compatibility/foundry
   ```

2. **Install dependencies**

   ```bash
   forge install
   forge install OpenZeppelin/openzeppelin-contracts@5.3.0
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

### Test ERC20 contract in virtual environment

```bash
source .env
forge test \
  --fork-url $CUSTOM_RPC \
  --chain-id $CHAIN_ID \
  --gas-report
```

### Query Network Info

`case call`

```bash
./shellscripts/get-network-info.sh
```

`forge script`

```bash
source .env                                
forge script script/NetworkInfo.s.sol \
  --rpc-url $CUSTOM_RPC \
  --chain-id $CHAIN_ID \
  --broadcast
```

### Deploy Contract

`forge script`

```bash
source .env
forge script script/DeployERC20.s.sol \
  --rpc-url $CUSTOM_RPC \
  --broadcast \
  --chain-id $CHAIN_ID
```

### Read State

`cast call`

```bash
./shellscripts/read_state.sh $CONTRACT
```

`forge script`

```bash
source .env
forge script script/ReadState.s.sol:ReadState \
  --rpc-url $CUSTOM_RPC \
  --chain-id $CHAIN_ID \
  --broadcast
```

### ERC20 Transfer

`cast send`

```bash
./shellscripts/transfer.sh $CONTRACT $ACCOUNT_2 1000000000000000000
```

`forge script`

```bash
source .env
forge script script/Transfer.s.sol:Transfer \
  --rpc-url $CUSTOM_RPC \
  --chain-id $CHAIN_ID \
  --broadcast
```

### ERC20 Transfer Revert

`cast send`

```bash
source .env
shellscripts/transfer_error.sh
```

`forge script`

```bash
source .env
forge script script/TransferError.s.sol:TransferError \
  --rpc-url $CUSTOM_RPC \
  --chain-id $CHAIN_ID \
  --broadcast
```

## Common Issues & Notes

- **Import errors for forge-std or ds-test**:  
  Ensure `remappings.txt` exists and contains at least:

  ```text
  forge-std/=lib/forge-std/
  ds-test/=lib/ds-test/src/
  @openzeppelin/contracts/=lib/openzeppelin-contracts/contracts/
  ```

  Then restart your editor’s language server.
