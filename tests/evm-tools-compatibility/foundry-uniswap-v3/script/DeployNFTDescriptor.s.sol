// SPDX-License-Identifier: MIT
pragma solidity ^0.7.6;

import "forge-std/Script.sol";
import "@uniswap/v3-periphery/contracts/libraries/NFTDescriptor.sol";

contract DeployNFTDescriptor is Script {
    function run() external {
        uint256 pk = vm.envUint("PRIVATE_KEY");
        vm.startBroadcast(pk);

        address nftDescLib = deployCode(
            "lib/v3-periphery/contracts/libraries/NFTDescriptor.sol:NFTDescriptor"
        );
        console.log("NFTDescriptor lib:", nftDescLib);

        vm.stopBroadcast();
    }
}