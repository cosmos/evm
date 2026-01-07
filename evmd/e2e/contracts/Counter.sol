// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

contract Counter {
    event ValueChanged(uint256 newValue, address indexed caller);

    uint256 public value;

    function set(uint256 newValue) external {
        value = newValue;
        emit ValueChanged(newValue, msg.sender);
    }
}
