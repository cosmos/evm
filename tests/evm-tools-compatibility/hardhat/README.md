# Hardhat Compatibility Test

This project demonstrates the compatibility of Hardhat for cross-chain (interchain) development.\
It uses a basic ERC-20 token contract to test deployment, minting, and transfers.\
You can compare the execution results across different local or forked Ethereum networks.

______________________________________________________________________

## Basic Test Environment (Ethereum Fork)

```shell
npx hardhat test
```

## Interchain Test (Local Node)

```shell
npx hardhat test --network localhost
```

### Test for Uniswap deployment

```shell
npx hardhat test test/uniswap.test.js --network localhost --show-stack-traces
```

### Test Compile for Uniswap v3-core

```shell
cd external/v3-core
git submodule init
git submodule update
npm install
npx hardhat compile
```

### Test Compile for Uniswap v3-periphery

```shell
cd external/v3-periphery
git submodule init
git submodule update
npm install
npx hardhat compile
```
