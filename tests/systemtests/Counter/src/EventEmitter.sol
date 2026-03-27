// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.13;

contract EventEmitter {
    uint256 public count;

    event Incremented(address indexed sender, uint256 newCount);
    event ValueSet(address indexed sender, uint256 value);

    function increment() public {
        count++;
        emit Incremented(msg.sender, count);
        emit ValueSet(msg.sender, count);
    }
}
