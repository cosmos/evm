// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import "forge-std/Script.sol";
import "forge-std/console.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";

contract ReadState is Script {
    /// @notice Run entrypoint accepts the token contract address as an argument.
    function run() external {
        address tokenAddr = vm.envAddress("CONTRACT");
        address acc1 = vm.envAddress("ACCOUNT_1");

        console.log("Chain ID:", block.chainid);

        IERC20Metadata token = IERC20Metadata(tokenAddr);

        console.log("totalSupply():", token.totalSupply());
        console.log("balanceOf(acc1):", token.balanceOf(acc1));
        console.log("name():", token.name());
        console.log("symbol():", token.symbol());
        console.log("decimals():", token.decimals());
    }
}