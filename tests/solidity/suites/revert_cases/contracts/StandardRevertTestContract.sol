// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

/**
 * @title StandardRevertTestContract
 * @dev Contract for testing standard Ethereum revert scenarios and error message handling
 * Compatible with any Ethereum node (Geth, etc.) - no custom precompiles required
 */
contract StandardRevertTestContract {
    uint256 public counter = 0;
    
    // Events to track what operations are performed
    event StandardRevert(string reason);
    event OutOfGasSimulated(uint256 gasLeft);
    event CounterIncremented(uint256 newValue);
    
    constructor() payable {}
    
    // ============ STANDARD CONTRACT REVERT CASES ============
    
    /**
     * @dev Simple revert with custom message
     */
    function standardRevert(string calldata reason) external {
        counter++;
        emit StandardRevert(reason);
        revert(reason);
    }
    
    /**
     * @dev Revert with require statement
     */
    function requireRevert(uint256 value, uint256 threshold) external {
        counter++;
        require(value < threshold, "Value exceeds threshold");
        emit CounterIncremented(counter);
    }
    
    /**
     * @dev Assert revert (should generate Panic error)
     */
    function assertRevert() external {
        counter++;
        assert(false);
    }
    
    /**
     * @dev Division by zero (should generate Panic error)
     */
    function divisionByZero() external pure returns (uint256) {
        uint256 zero = 0;
        return 1 / zero;
    }
    
    /**
     * @dev Array out of bounds (should generate Panic error)
     */
    function arrayOutOfBounds() external pure returns (uint256) {
        uint256[] memory arr = new uint256[](2);
        return arr[5]; // This will cause an out of bounds error
    }
    
    /**
     * @dev Division by zero in transaction context (should generate Panic error)
     */
    function divisionByZeroTx() external returns (uint256) {
        counter++; // State change to make it a transaction
        uint256 zero = 0;
        return 1 / zero;
    }
    
    /**
     * @dev Array out of bounds in transaction context (should generate Panic error)
     */
    function arrayOutOfBoundsTx() external returns (uint256) {
        counter++; // State change to make it a transaction
        uint256[] memory arr = new uint256[](2);
        return arr[5]; // This will cause an out of bounds error
    }
    
    /**
     * @dev Overflow error (should generate Panic error in older Solidity)
     */
    function overflowError() external pure returns (uint256) {
        unchecked {
            uint256 max = type(uint256).max;
            return max + 1; // This might overflow depending on Solidity version
        }
    }
    
    // ============ OUT OF GAS ERROR CASES ============
    
    /**
     * @dev Standard contract call that runs out of gas
     */
    function standardOutOfGas() external {
        counter++;
        emit OutOfGasSimulated(gasleft());
        
        // Consume all remaining gas
        while (gasleft() > 1000) {
            // Consume gas in a loop
            counter++;
        }
    }
    
    /**
     * @dev Expensive computation that can run out of gas
     */
    function expensiveComputation(uint256 iterations) external {
        counter++;
        emit OutOfGasSimulated(gasleft());
        
        // Perform expensive operations
        for (uint256 i = 0; i < iterations; i++) {
            // Hash operations are gas-expensive
            keccak256(abi.encode(i, block.timestamp, msg.sender));
            counter++;
        }
    }
    
    /**
     * @dev Storage operations that can run out of gas
     */
    function expensiveStorage(uint256 iterations) external {
        counter++;
        emit OutOfGasSimulated(gasleft());
        
        // Storage operations are very gas-expensive
        for (uint256 i = 0; i < iterations; i++) {
            assembly {
                sstore(add(counter.slot, i), i)
            }
        }
    }
    
    // ============ COMPLEX REVERT SCENARIOS ============
    
    /**
     * @dev Multiple function calls with revert
     */
    function multipleCallsWithRevert() external {
        counter++;
        
        // First, do some successful operations
        this.incrementCounter();
        
        // Then revert
        revert("Multiple calls revert");
    }
    
    /**
     * @dev Try-catch with revert
     */
    function tryCatchRevert(bool shouldRevert) external {
        counter++;
        
        if (shouldRevert) {
            try this.internalRevert() {
                // Should not reach here
            } catch (bytes memory reason) {
                // Re-throw the error to maintain the revert
                assembly {
                    revert(add(reason, 0x20), mload(reason))
                }
            }
        } else {
            // Successful path
            emit CounterIncremented(counter);
        }
    }
    
    /**
     * @dev Internal function that always reverts
     */
    function internalRevert() external pure {
        revert("Internal function revert");
    }
    
    // ============ UTILITY FUNCTIONS ============
    
    /**
     * @dev Simple function that increments counter
     */
    function incrementCounter() external {
        counter++;
        emit CounterIncremented(counter);
    }
    
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
    
    /**
     * @dev Get contract balance
     */
    function getBalance() external view returns (uint256) {
        return address(this).balance;
    }
}

/**
 * @title SimpleWrapper
 * @dev Helper contract for testing reverts via external contracts
 */
contract SimpleWrapper {
    event WrapperCall(string operation, bool success);
    
    constructor() payable {}
    
    /**
     * @dev Wrapper function that calls standard revert
     */
    function wrappedStandardCall(StandardRevertTestContract target, string calldata reason) external {
        emit WrapperCall("standard_revert", false);
        target.standardRevert(reason);
    }
    
    /**
     * @dev Wrapper function that runs out of gas
     */
    function wrappedOutOfGasCall(StandardRevertTestContract target) external {
        emit WrapperCall("out_of_gas", false);
        
        // Consume most gas first
        for (uint256 i = 0; i < 100000; i++) {
            // Gas consuming operation
            keccak256(abi.encode(i));
        }
        
        // Then try the expensive call
        target.expensiveComputation(10000);
    }
    
    /**
     * @dev Fund wrapper contract
     */
    receive() external payable {}
}