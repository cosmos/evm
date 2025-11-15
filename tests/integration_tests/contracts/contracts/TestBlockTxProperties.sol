// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.4;

contract TestBlockTxProperties {
    function getBlockHash(uint256 blockNumber) public view returns (bytes32) {
        return blockhash(blockNumber);
    }
}
