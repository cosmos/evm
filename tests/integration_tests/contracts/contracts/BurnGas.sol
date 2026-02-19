// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.4;

contract BurnGas {
    int[] expensive;

    function burnGas(uint256 count) public {
        for (uint i = 0; i < count; i++) {
            unchecked {
                expensive.push(10);
            }
        }
    }
}
