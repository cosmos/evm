// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

import "../ISlashing.sol" as slashing;

contract SlashingCaller {
    event TestResult(string message, bool success);

    function testUnjail(address validatorAddr) public returns (bool success) {
        return slashing.SLASHING_CONTRACT.unjail(validatorAddr);
    }
}