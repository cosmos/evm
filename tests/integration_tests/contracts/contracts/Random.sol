// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.4;

contract Random {
    function randomTokenId() public view returns (uint256) {
        return uint256(keccak256(abi.encodePacked(block.prevrandao)));
    }
}
