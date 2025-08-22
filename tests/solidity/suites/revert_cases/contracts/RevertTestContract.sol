// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./cosmos/staking/StakingI.sol";
import "./cosmos/distribution/DistributionI.sol";
import "./cosmos/bank/IBank.sol";
import "./cosmos/common/Types.sol";

/**
 * @title RevertTestContract
 * @dev Contract for testing Cosmos precompile revert scenarios and error message handling
 * Focuses specifically on precompile calls and interactions with Cosmos SDK modules
 */
contract RevertTestContract {
    uint256 public counter = 0;
    
    // Events to track what operations are performed
    event PrecompileCallMade(string precompileName, bool success);
    event OutOfGasSimulated(uint256 gasLeft);
    
    constructor() payable {}
    
    // ============ DIRECT PRECOMPILE CALL REVERTS ============
    
    /**
     * @dev Direct staking precompile call that will revert
     */
    function directStakingRevert(string calldata invalidValidator) external {
        counter++;
        emit PrecompileCallMade("staking", false);
        // This should revert with invalid validator address
        STAKING_CONTRACT.delegate(address(this), invalidValidator, 1);
    }
    
    /**
     * @dev Direct distribution precompile call that will revert
     */
    function directDistributionRevert(string calldata invalidValidator) external {
        counter++;
        emit PrecompileCallMade("distribution", false);
        // This should revert with invalid validator address
        DISTRIBUTION_CONTRACT.withdrawDelegatorRewards(address(this), invalidValidator);
    }
    
    /**
     * @dev Direct bank precompile call that will revert
     */
    function directBankRevert() external pure {
        revert("intended revert");
    }
    
    // ============ PRECOMPILE CALL VIA CONTRACT REVERTS ============
    
    /**
     * @dev Precompile call via contract that reverts
     */
    function precompileViaContractRevert(string calldata invalidValidator) external {
        counter++;
        try this.internalStakingCall(invalidValidator) {
            // Should not reach here
        } catch (bytes memory reason) {
            // Re-throw the error to maintain the revert
            assembly {
                revert(add(reason, 0x20), mload(reason))
            }
        }
    }
    
    /**
     * @dev Internal function for precompile call via contract
     */
    function internalStakingCall(string calldata validatorAddress) external {
        require(msg.sender == address(this), "Only self can call");
        emit PrecompileCallMade("staking_internal", false);
        STAKING_CONTRACT.delegate(address(this), validatorAddress, 1);
    }
    
    /**
     * @dev Complex scenario: multiple precompile calls with revert
     */
    function multiplePrecompileCallsWithRevert(string calldata validatorAddress) external {
        counter++;
        
        // First, make a successful call
        try IBANK_CONTRACT.balances(address(this)) returns (Balance[] memory) {
            emit PrecompileCallMade("bank", true);
        } catch {
            emit PrecompileCallMade("bank", false);
        }
        
        // Then make a call that will revert
        emit PrecompileCallMade("staking_multi", false);
        STAKING_CONTRACT.delegate(address(this), validatorAddress, 1);
    }
    
    // ============ PRECOMPILE OUT OF GAS ERROR CASES ============
    
    /**
     * @dev Direct precompile call that runs out of gas
     */
    function directStakingOutOfGas(string calldata validatorAddress) external {
        counter++;
        emit OutOfGasSimulated(gasleft());
        
        // First consume most gas
        for (uint256 i = 0; i < 1000000; i++) {
            counter++;
        }
        
        // Then try precompile call with remaining gas
        STAKING_CONTRACT.delegate(address(this), validatorAddress, 1);
    }
    
    /**
     * @dev Precompile call via contract that runs out of gas
     */
    function precompileViaContractOutOfGas(string calldata validatorAddress) external {
        counter++;
        emit OutOfGasSimulated(gasleft());
        
        // Consume most gas first
        for (uint256 i = 0; i < 1000000; i++) {
            counter++;
        }
        
        // Then try internal precompile call
        this.internalStakingCall(validatorAddress);
    }
    
    /**
     * @dev Wrapper precompile call that runs out of gas
     */
    function wrappedPrecompileOutOfGas(string calldata validatorAddress) external {
        counter++;
        emit OutOfGasSimulated(gasleft());
        
        // Consume most gas in expensive operations
        for (uint256 i = 0; i < 500000; i++) {
            keccak256(abi.encode(i, block.timestamp, msg.sender));
            counter++;
        }
        
        // Then try multiple precompile calls
        STAKING_CONTRACT.delegate(address(this), validatorAddress, 1);
        DISTRIBUTION_CONTRACT.withdrawDelegatorRewards(address(this), validatorAddress);
    }
    
    // ============ UTILITY FUNCTIONS ============
    
    /**
     * @dev Get current counter value
     */
    function getCounter() external view returns (uint256) {
        return counter;
    }
    
    /**
     * @dev Reset counter (for testing)
     */
    function resetCounter() external {
        counter = 0;
    }
    
    /**
     * @dev Fund contract with native tokens
     */
    receive() external payable {}
    
    /**
     * @dev Withdraw funds (for testing)
     */
    function withdraw() external {
        payable(msg.sender).transfer(address(this).balance);
    }
}

/**
 * @title PrecompileWrapper
 * @dev Helper contract for testing precompile calls via external contracts
 */
contract PrecompileWrapper {
    event WrapperCall(string operation, bool success);
    
    constructor() payable {}
    
    /**
     * @dev Wrapper function that calls staking precompile and reverts
     */
    function wrappedStakingCall(string calldata validatorAddress, uint256 amount) external {
        emit WrapperCall("staking", false);
        STAKING_CONTRACT.delegate(address(this), validatorAddress, amount);
        revert("Wrapper intentional revert");
    }
    
    /**
     * @dev Wrapper function that calls distribution precompile and reverts
     */
    function wrappedDistributionCall(string calldata validatorAddress) external {
        emit WrapperCall("distribution", false);
        DISTRIBUTION_CONTRACT.withdrawDelegatorRewards(address(this), validatorAddress);
        revert("Wrapper intentional revert");
    }
    
    /**
     * @dev Wrapper function that runs out of gas
     */
    function wrappedOutOfGasCall(string calldata validatorAddress) external {
        // Consume all gas
        for (uint256 i = 0; i < 1000000; i++) {
            // Gas consuming operation
            keccak256(abi.encode(i));
        }
        
        STAKING_CONTRACT.delegate(address(this), validatorAddress, 1);
    }
    
    /**
     * @dev Fund wrapper contract
     */
    receive() external payable {}
}
