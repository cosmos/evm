// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.4;

import "./MyToken.sol";

contract MyToken2 is MyToken {
    function newFeature() public pure returns (string memory) {
        return "Upgraded!";
    }
}