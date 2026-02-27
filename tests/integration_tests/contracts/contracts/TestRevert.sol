// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.4;

contract TestRevert {
    uint256 state;
    constructor() {
        state = 0;
    }

    function transfer(uint256 value) public payable {
        uint256 minimal = 5 * 10 ** 18;
        state = value;
        if (state < minimal) {
            revert("Not enough tokens to transfer");
        }
    }

    function query() public view returns (uint256) {
        return state;
    }

    // 0x9ffb86a5
    function revertWithMsg() public pure {
        revert("Function has been reverted");
    }

    // 0x3246485d
    function revertWithoutMsg() public pure {
        revert();
    }
}
