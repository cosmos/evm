// SPDX-License-Identifier: MIT
pragma solidity ^0.8.19;

import "forge-std/Script.sol";

contract SimpleSendsScript is Script {
    function run() external {
        // Get deployer private key
        uint256 deployerPrivateKey = vm.envUint("PRIVATE_KEY");
        address deployer = vm.addr(deployerPrivateKey);

        // Create some recipient addresses
        address[8] memory recipients = [
                0x1111111111111111111111111111111111111111,
                0x2222222222222222222222222222222222222222,
                0x3333333333333333333333333333333333333333,
                0x4444444444444444444444444444444444444444,
                0x5555555555555555555555555555555555555555,
                0x6666666666666666666666666666666666666666,
                0x7777777777777777777777777777777777777777,
                0x8888888888888888888888888888888888888888
        ];

        uint256 sendAmount = 0.01 ether; // Small amount to send

        vm.startBroadcast(deployerPrivateKey);

        // Send ETH to multiple recipients (very small count to avoid gas issues)
        for (uint i = 0; i < 10; i++) {
                payable(recipients[i%8]).transfer(1);
        }

        vm.stopBroadcast();
    }
}
