// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import "forge-std/Script.sol";
import "forge-std/console.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/**
 * @title Transfer
 * @notice Script to perform an ERC-20 token transfer and log balances before and after.
 */
contract Transfer is Script {
    /**
     * @notice Runs the transfer script.
     */
    function run() external {
        // Load private key and derive sender address
        uint256 pk = vm.envUint("PRIVATE_KEY");
        address tokenAddr = vm.envAddress("CONTRACT");
        address sender = vm.addr(pk);
        address receiver = vm.envAddress("ACCOUNT_2");
        uint256 amount = 1 ether;

        IERC20 token = IERC20(tokenAddr);

        // Log balances before transfer
        console.log("Sender balance before:", token.balanceOf(sender));
        console.log("Receiver balance before:", token.balanceOf(receiver));

        // Broadcast the transfer transaction
        vm.startBroadcast(pk);
        bool success = token.transfer(receiver, amount);
        require(success, "Transfer failed");
        vm.stopBroadcast();

        // Log balances after transfer
        console.log("Sender balance after:", token.balanceOf(sender));
        console.log("Receiver balance after:", token.balanceOf(receiver));
    }
}
