// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

contract DelegationManager {

    /// The delegation mapping is used to associate the EOA address that 
    /// actually made the delegate request with its corresponding delegation information. 
    mapping(address => mapping(string => uint256)) public delegations;
     
    /// The unbonding queue is used to store the unbonding operations that are in progress.
    mapping(address => UnbondingDelegation[]) public unbondingDelegations;

    /// The unbonding entry struct represents an unbonding operation that is in progress. 
    /// It contains information about the validator and the amount of tokens that are being unbonded.
    struct UnbondingDelegation {
        /// @dev The validator address is the address of the validator that is being unbonded.
        string validator;
        /// @dev The amount of tokens that are being unbonded.
        uint256 amount;
        /// @dev The creation height is the height at which the unbonding operation was created.
        uint256 creationHeight;
        /// @dev The completion time is the time at which the unbonding operation will complete.
        int64 completionTime;
    }

    function _increaseAmount(address _delegator, string memory _validator, uint256 _amount) internal {
        delegations[_delegator][_validator] += _amount;
    }

    function _decreaseAmount(address _delegator, string memory _validator, uint256 _amount) internal {
        require(delegations[_delegator][_validator] >= _amount, "Insufficient delegation amount");
        delegations[_delegator][_validator] -= _amount;
    }

    function _undelegate(string memory _validatorAddr, uint256 _amount, int64 completionTime) internal {
        unbondingDelegations[msg.sender].push(UnbondingDelegation({
            validator: _validatorAddr,
            amount: _amount,
            creationHeight: block.number,
            completionTime: completionTime
        }));
    }

    /// @dev This function is used to dequeue unbonding entries that have expired.
    ///
    /// @notice StakingCaller acts as the delegator and manages delegation/unbonding state per EoA.
    /// Reflecting x/staking unbondingDelegations changes in real-time would require event listening.
    /// To simplify unbonding entry processing, this function is called during delegate/undelegate calls.
    /// Although updating unbondingDelegations state isn't tested in the staking precompile integration tests,
    /// it is included for the completeness of the contract.
    function _dequeueUnbondingDelegation() internal {
        for (uint256 i = 0; i < unbondingDelegations[msg.sender].length; i++) {
            UnbondingDelegation storage entry = unbondingDelegations[msg.sender][i];
            if (uint256(int256(entry.completionTime)) <= block.timestamp) {
                delete unbondingDelegations[msg.sender][i];
                delegations[msg.sender][entry.validator] -= entry.amount;
            }
        }
    }

    /// @dev This function is used to cancel unbonding entries that have been cancelled.
    /// @param _creationHeight The creation height of the unbonding entry to cancel.
    /// @param _amount The amount to cancel.
    function _cancelUnbonding(uint256 _creationHeight, uint256 _amount) internal {
        UnbondingDelegation[] storage entries = unbondingDelegations[msg.sender];

        for (uint256 i = 0; i < entries.length; i++) {
            UnbondingDelegation storage entry = entries[i];

            if (entry.creationHeight != _creationHeight) { continue; }

            require(entry.amount >= _amount, "amount exceeds unbonding entry amount");
            entry.amount -= _amount;

            // If the amount is now 0, remove the entry
            if (entry.amount == 0) { delete entries[i]; }

            // Only cancel one entry per call
            break; 
        }
    }

    function _checkDelegation(string memory _validatorAddr, uint256 _delegateAmount) internal view {
        require(
            delegations[msg.sender][_validatorAddr] >= _delegateAmount, 
            "Delegation does not exist or insufficient delegation amount"
        );
    }

    function _checkUnbondingDelegation(address _delegatorAddr, string memory _validatorAddr) internal view {
        bool found;
        for (uint256 i = 0; i < unbondingDelegations[_delegatorAddr].length; i++) {
            UnbondingDelegation storage entry = unbondingDelegations[_delegatorAddr][i];
            if (
                _equalStrings(entry.validator, _validatorAddr) && 
                uint256(int256(entry.completionTime)) > block.timestamp
            ) {
                found = true;
                break;
            }
        }
        require(found == true, "Unbonding delegation does not exist");
    }

    function _equalStrings(string memory a, string memory b) internal pure returns (bool) {
        return keccak256(bytes(a)) == keccak256(bytes(b));
    }
}