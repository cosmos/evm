// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract Reverter {
    uint256 public x;

    function revertWithReason(string memory reason) external pure {
        revert(reason);
    }

    // Writes state then burns gas to ensure that, under insufficient gas, the
    // entire transaction reverts with no state change persisted.
    function setThenBurnGas(uint256 v, uint256 iters) external {
        x = v;
        uint256 sum;
        for (uint256 i = 0; i < iters; i++) {
            unchecked {
                sum += i;
            }
        }
        assembly {
            pop(sum)
        }
    }
}
