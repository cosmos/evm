// SPDX-License-Identifier: MIT
pragma solidity ^0.7.6;

import "forge-std/Script.sol";
import "forge-std/console.sol";

import "@uniswap/v3-core/contracts/UniswapV3Factory.sol";
import { SwapRouter } from "@uniswap/v3-periphery/contracts/SwapRouter.sol";
import { NonfungiblePositionManager } from "@uniswap/v3-periphery/contracts/NonfungiblePositionManager.sol";
import "@uniswap/v3-periphery/contracts/NonfungibleTokenPositionDescriptor.sol";
import "@uniswap/v3-periphery/contracts/libraries/NFTDescriptor.sol";

import "src/TestToken.sol";
import "src/WETH9Mock.sol";

contract DeployUniswapV3 is Script {
  function run() external {
    uint256 pk = vm.envUint("PRIVATE_KEY");
    vm.startBroadcast(pk);

    UniswapV3Factory factory = new UniswapV3Factory();
    console.log("Factory:", address(factory));

    TestToken token0 = new TestToken("MockUSDC", "mUSDC");
    TestToken token1 = new TestToken("MockUSDT", "mUSDT");
    console.log("Token0:", address(token0));
    console.log("Token1:", address(token1));

    WETH9Mock weth = new WETH9Mock();
    console.log("WETH9Mock:", address(weth));

    // artifactPath is specified as the relative path according to remappings.txt, ending with the contract name.
    address descriptor = deployCode(
      "lib/v3-periphery/contracts/NonfungibleTokenPositionDescriptor.sol:NonfungibleTokenPositionDescriptor",
      abi.encode(address(weth), "ETH")
    );
    console.log("Descriptor:", descriptor);

    NonfungiblePositionManager manager = new NonfungiblePositionManager(
      address(factory), address(weth), descriptor
    );
    console.log("Manager:", address(manager));

    SwapRouter router = new SwapRouter(address(factory), address(weth));
    console.log("Router:", address(router));

    vm.stopBroadcast();
  }
}

