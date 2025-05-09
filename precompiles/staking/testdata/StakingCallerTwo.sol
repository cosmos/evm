// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

import "../StakingI.sol" as staking;

/// @title StakingCaller
/// @author Evmos Core Team
/// @dev This contract is used to test external contract calls to the staking precompile.
contract StakingCallerTwo {
    /// counter is used to test the state persistence bug, when EVM and Cosmos state were both
    /// changed in the same function.
    uint256 public counter;

    /// The delegation mapping is used to associate the EOA address that 
    /// actually made the delegate request with its corresponding delegation information. 
    mapping(address => mapping(string => uint256)) public delegation;

    /// @dev This function showcased, that there was a bug in the EVM implementation, that occurred when
    /// Cosmos state is modified in the same transaction as state information inside
    /// the EVM.
    /// @param _validatorAddr Address of the validator to delegate to
    /// @param _before Boolean to specify if funds should be transferred to delegator before the precompile call
    /// @param _after Boolean to specify if funds should be transferred to delegator after the precompile call
    function testDelegateWithCounterAndTransfer(
        string memory _validatorAddr,
        bool _before,
        bool _after
    ) public payable {
        if (_before) {
            counter++;
            (bool sent, ) = msg.sender.call{value: 15}("");
            require(sent, "Failed to send Ether to delegator");
        }
        bool success = staking.STAKING_CONTRACT.delegate(
            address(this), 
            _validatorAddr, 
            msg.value
        );
        require(success, "Failed to delegate");
        delegation[msg.sender][_validatorAddr] += msg.value;
        if (_after) {
            counter++;
            (bool sent, ) = msg.sender.call{value: 15}("");
            require(sent, "Failed to send Ether to delegator");
        }
    }

    /// @dev This function showcased, that there was a bug in the EVM implementation, that occurred when
    /// Cosmos state is modified in the same transaction as state information inside
    /// the EVM.
    /// @param _dest Address to send some funds from the contract
    /// @param _delegator Address of the delegator
    /// @param _validatorAddr Address of the validator to delegate to
    /// @param _before Boolean to specify if funds should be transferred to delegator before the precompile call
    /// @param _after Boolean to specify if funds should be transferred to delegator after the precompile call
    function testDelegateWithTransfer(
        address payable _dest,
        address payable _delegator,
        string memory _validatorAddr,
        bool _before,
        bool _after
    ) public payable{
        if (_before) {
            counter++;
            (bool sent, ) = _dest.call{value: 15}("");
            require(sent, "Failed to send Ether to delegator");
        }
        bool success = staking.STAKING_CONTRACT.delegate(address(this), _validatorAddr, msg.value);
        require(success, "Failed to delegate");
        delegation[_delegator][_validatorAddr] += msg.value;
        if (_after) {
            counter++;
            (bool sent, ) = _dest.call{value: 15}("");
            require(sent, "Failed to send Ether to delegator");
        }
    }    

    /// @dev This function calls the staking precompile's create validator method
    /// and transfers of funds to the validator address (if specified).
    /// @param _descr The initial description
    /// @param _commRates The initial commissionRates
    /// @param _minSelfDel The validator's self declared minimum self delegation
    /// @param _validator The validator's operator address
    /// @param _pubkey The consensus public key of the validator
    /// @param _before Boolean to specify if funds should be transferred to delegator before the precompile call
    /// @param _after Boolean to specify if funds should be transferred to delegator after the precompile call
    function testCreateValidatorWithTransfer(
        staking.Description calldata _descr,
        staking.CommissionRates calldata _commRates,
        uint256 _minSelfDel,
        address _validator,
        string memory _pubkey,
        bool _before,
        bool _after
    ) public payable {
        if (_before) {
            counter++;
            (bool sent, ) = _validator.call{value: 15}("");
            require(sent, "Failed to send Ether to delegator");
        }
        bool success = staking.STAKING_CONTRACT.createValidator(
            _descr,
            _commRates,
            _minSelfDel,
            _validator,
            _pubkey,
            msg.value
        );
        require(success, "Failed to create the validator");
        if (_after) {
            counter++;
            (bool sent, ) = _validator.call{value: 15}("");
            require(sent, "Failed to send Ether to delegator");
        }
    }
}
