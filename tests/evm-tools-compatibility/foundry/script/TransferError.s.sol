// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import "forge-std/Script.sol";
import "forge-std/console.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";

/**
 * @title TransferErrorScript
 * @notice Attempts a transfer exceeding balance and logs revert details.
 */
contract TransferError is Script {
    function run() external {
        // Load environment variables
        uint256 pk          = vm.envUint("PRIVATE_KEY");
        address tokenAddr   = vm.envAddress("CONTRACT");
        address sender      = vm.addr(pk);
        address recipient   = vm.envAddress("ACCOUNT_2");
        uint256 chainId     = vm.envUint("CHAIN_ID");

        // Amount to transfer (exceeds typical balance)
        uint256 amount = 2000 * 10 ** 18; // 2000 tokens with 18 decimals

        console.log("Chain ID:", chainId);
        console.log("Sender address:", sender);
        console.log("Token contract address:", tokenAddr);
        console.log("Recipient address:", recipient);
        console.log("Attempting transfer of", amount, "tokens");

        vm.startBroadcast(pk);
        // Try-catch to capture revert reason or low-level data
        try IERC20(tokenAddr).transfer(recipient, amount) returns (bool success) {
            console.log("Unexpected success: transfer returned", success);
        } catch Error(string memory reason) {
            console.log("Revert reason:", reason);
        } catch (bytes memory lowLevelData) {
            console.log("Revert low-level data:");
            console.logBytes(lowLevelData);
        }
        vm.stopBroadcast();
    }
}
