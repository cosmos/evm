// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import "forge-std/Script.sol";
import "forge-std/console.sol";

contract NetworkInfo is Script {
    function run() external view {
        // Print the EVM chain ID (via block.chainid)
        console.log("Chain ID:", block.chainid);
        // Print the current block number
        console.log("Block Number:", block.number);
    }
}