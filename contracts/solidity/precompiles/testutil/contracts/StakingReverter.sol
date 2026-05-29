// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "../../staking/StakingI.sol";

contract StakingReverter {
    uint counter = 0;

    constructor() payable {}

    function run(uint numTimes, string calldata validatorAddress) external {
        counter++;

        for (uint i = 0; i < numTimes; i++) {
            try
            StakingReverter(address(this)).performDelegation(
                validatorAddress
            )
            {} catch {}
        }
    }

    function multipleDelegations(
        uint numTimes,
        string calldata validatorAddress
    ) external {
        counter++;

        for (uint i = 0; i < numTimes; i++) {
            StakingReverter(address(this)).performDelegation(validatorAddress);
        }
    }

    /// @dev callPrecompileBeforeAndAfterRevert tests whether precompile calls that occur 
    /// before and after an intentionally ignored revert correctly modify the state.
    /// This method assumes that the StakingReverter.sol contract holds a native balance. 
    /// Therefore, in order to call this method, the contract must be funded with a balance in advance.
    function callPrecompileBeforeAndAfterRevert(uint numTimes, string calldata validatorAddress) external {
        STAKING_CONTRACT.delegate(address(this), validatorAddress, 10);

        for (uint i = 0; i < numTimes; i++) {
            try
            StakingReverter(address(this)).performDelegation(
                validatorAddress
            )
            {} catch {}
        }

        STAKING_CONTRACT.delegate(address(this), validatorAddress, 10);
    }

    /// @dev nestedTryCatchDelegations performs nested try/catch calls to precompile
    /// where inner calls revert intentionally. Only the successful delegations
    /// outside the reverting scope should persist.
    ///
    /// Expected successful delegations: 1 (before loop) + outerTimes (after each catch) + 1 (after loop)
    function nestedTryCatchDelegations(uint outerTimes, uint innerTimes, string calldata validatorAddress) external {
        // Initial successful delegate before any nested reverts
        STAKING_CONTRACT.delegate(address(this), validatorAddress, 10);

        for (uint i = 0; i < outerTimes; i++) {
            // Outer call that will revert and be caught
            try StakingReverter(address(this)).performDelegation(validatorAddress) {
                // no-op
            } catch {
                // After catching the revert, perform a successful delegate
                STAKING_CONTRACT.delegate(address(this), validatorAddress, 10);

                // Inner nested loop of reverting calls
                for (uint j = 0; j < innerTimes; j++) {
                    try StakingReverter(address(this)).performDelegation(validatorAddress) {
                        // no-op
                    } catch {}
                }
            }
        }

        // Final successful delegate after the loops
        STAKING_CONTRACT.delegate(address(this), validatorAddress, 10);
    }

    function performDelegation(string calldata validatorAddress) external {
        STAKING_CONTRACT.delegate(address(this), validatorAddress, 10);
        revert();
    }

    function getCurrentStake(
        string calldata validatorAddress
    ) external view returns (uint256 shares, Coin memory balance) {
        return STAKING_CONTRACT.delegation(address(this), validatorAddress);
    }

    function multipleQueries(
        uint numTimes,
        address validatorAddress
    ) external view returns (Validator memory validator) {
        for (uint i = 0; i < numTimes; i++) {
            validator = STAKING_CONTRACT.validator(validatorAddress);
        }
        return validator;
    }
}
