// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

import "../StakingI.sol" as staking;
import "./DelegationManager.sol";

/// @title StakingCaller
/// @author Evmos Core Team
/// @dev This contract is used to test external contract calls to the staking precompile.
contract StakingCaller is DelegationManager{
    /// counter is used to test the state persistence bug, when EVM and Cosmos state were both
    /// changed in the same function.
    uint256 public counter;
    string[] private delegateMethod = [staking.MSG_DELEGATE];

    /// @dev This function calls the staking precompile's create validator method
    /// using the msg.sender as the validator's operator address.
    /// @param _descr The initial description
    /// @param _commRates The initial commissionRates
    /// @param _minSelfDel The validator's self declared minimum self delegation
    /// @param _valAddr The validator's operator address
    /// @param _pubkey The consensus public key of the validator
    /// @param _value The amount of the coin to be self delegated to the validator
    /// @return success Whether or not the create validator was successful
    function testCreateValidator(
        staking.Description calldata _descr,
        staking.CommissionRates calldata _commRates,
        uint256 _minSelfDel,
        address _valAddr,
        string memory _pubkey,
        uint256 _value
    ) public returns (bool) {
        return
            staking.STAKING_CONTRACT.createValidator(
            _descr,
            _commRates,
            _minSelfDel,
            _valAddr,
            _pubkey,
            _value
        );
    }

    /// @dev This function calls the staking precompile's edit validator
    /// method using the msg.sender as the validator's operator address.
    /// @param _descr Description parameter to be updated. Use the string "[do-not-modify]"
    /// as the value of fields that should not be updated.
    /// @param _valAddr The validator's operator address
    /// @param _commRate CommissionRate parameter to be updated.
    /// Use commissionRate = -1 to keep the current value and not update it.
    /// @param _minSelfDel MinSelfDelegation parameter to be updated.
    /// Use minSelfDelegation = -1 to keep the current value and not update it.
    /// @return success Whether or not edit validator was successful.
    function testEditValidator(
        staking.Description calldata _descr,
        address _valAddr,
        int256 _commRate,
        int256 _minSelfDel
    ) public returns (bool) {
        return
            staking.STAKING_CONTRACT.editValidator(
            _descr,
            _valAddr,
            _commRate,
            _minSelfDel
        );
    }

    /// @dev This function calls the staking precompile's delegate method.
    /// delegator must call this function with the native coin value he wants to delegate.
    /// @param _validatorAddr The validator address to delegate to.
    function testDelegate(
        string memory _validatorAddr
    ) public payable {
        _dequeueUnbondingDelegation();
        bool success = staking.STAKING_CONTRACT.delegate(
            address(this),
            _validatorAddr,
            msg.value
        );
        require(success, "delegate failed");
        _increaseAmount(msg.sender, _validatorAddr, msg.value);
    }

    /// @dev This function calls the staking precompile's undelegate method.
    /// @param _validatorAddr The validator address to delegate to.
    /// @param _amount The amount to delegate.
    function testUndelegate(
        string memory _validatorAddr,
        uint256 _amount
    ) public {
        _checkDelegation(_validatorAddr, _amount);
        _dequeueUnbondingDelegation();
        int64 completionTime = staking.STAKING_CONTRACT.undelegate(address(this), _validatorAddr, _amount);
        require(completionTime > 0, "Failed to undelegate");
        _undelegate(_validatorAddr, _amount, completionTime);
    }

    /// @dev This function calls the staking precompile's redelegate method.
    /// @param _validatorSrcAddr The validator address to delegate from.
    /// @param _validatorDstAddr The validator address to delegate to.
    /// @param _amount The amount to delegate.
    function testRedelegate(
        string memory _validatorSrcAddr,
        string memory _validatorDstAddr,
        uint256 _amount
    ) public {
        _checkDelegation(_validatorSrcAddr, _amount);
        int64 completionTime = staking.STAKING_CONTRACT.redelegate(
            address(this),
            _validatorSrcAddr,
            _validatorDstAddr,
            _amount
        );
        require(completionTime > 0, "Failed to redelegate");
        _decreaseAmount(msg.sender, _validatorSrcAddr, _amount);
        _increaseAmount(msg.sender, _validatorDstAddr, _amount);
    }

    /// @dev This function calls the staking precompile's cancel unbonding delegation method.
    /// @param _validatorAddr The validator address to delegate from.
    /// @param _amount The amount to delegate.
    /// @param _creationHeight The creation height of the unbonding delegation.
    function testCancelUnbonding(
        string memory _validatorAddr,
        uint256 _amount,
        uint256 _creationHeight
    ) public {
        _dequeueUnbondingDelegation();
        _checkUnbondingDelegation(msg.sender, _validatorAddr);        
        bool success = staking.STAKING_CONTRACT.cancelUnbondingDelegation(
            address(this),
            _validatorAddr,
            _amount,
            _creationHeight
        );
        require(success, "Failed to cancel unbonding");
        _cancelUnbonding(_creationHeight, _amount);
    }

    /// @dev This function calls the staking precompile's validator query method.
    /// @param _validatorAddr The validator address to query.
    /// @return validator The validator.
    function getValidator(
        address _validatorAddr
    ) public view returns (staking.Validator memory validator) {
        return staking.STAKING_CONTRACT.validator(_validatorAddr);
    }

    /// @dev This function calls the staking precompile's validators query method.
    /// @param _status The status of the validators to query.
    /// @param _pageRequest The page request to query.
    /// @return validators The validators.
    /// @return pageResponse The page response.
    function getValidators(
        string memory _status,
        staking.PageRequest calldata _pageRequest
    )
    public
    view
    returns (
        staking.Validator[] memory validators,
        staking.PageResponse memory pageResponse
    )
    {
        return staking.STAKING_CONTRACT.validators(_status, _pageRequest);
    }

    /// @dev This function calls the staking precompile's delegation query method.
    /// @param _delegatorAddr The delegator address to query.
    /// @param _validatorAddr The validator address to delegate from.
    /// @return shares The shares of the delegation.
    /// @return balance The balance of the delegation.
    function getDelegation(
        address _delegatorAddr,
        string memory _validatorAddr
    ) public view returns (uint256 shares, staking.Coin memory balance) {
        return staking.STAKING_CONTRACT.delegation(_delegatorAddr, _validatorAddr);
    }

    /// @dev This function calls the staking precompile's redelegations query method.
    /// @param _delegatorAddr The delegator address to query.
    /// @param _validatorSrcAddr The validator address to delegate from.
    /// @param _validatorDstAddr The validator address to delegate to.
    /// @return redelegation The redelegation output.
    function getRedelegation(
        address _delegatorAddr,
        string memory _validatorSrcAddr,
        string memory _validatorDstAddr
    ) public view returns (staking.RedelegationOutput memory redelegation) {
        return
            staking.STAKING_CONTRACT.redelegation(
            _delegatorAddr,
            _validatorSrcAddr,
            _validatorDstAddr
        );
    }

    /// @dev This function calls the staking precompile's redelegations query method.
    /// @param _delegatorAddr The delegator address.
    /// @param _validatorSrcAddr The validator address to delegate from.
    /// @param _validatorDstAddr The validator address to delegate to.
    /// @param _pageRequest The page request to query.
    /// @return response The redelegation response.
    function getRedelegations(
        address _delegatorAddr,
        string memory _validatorSrcAddr,
        string memory _validatorDstAddr,
        staking.PageRequest memory _pageRequest
    )
    public
    view
    returns (
        staking.RedelegationResponse[] memory response,
        staking.PageResponse memory pageResponse
    )
    {
        return
            staking.STAKING_CONTRACT.redelegations(
            _delegatorAddr,
            _validatorSrcAddr,
            _validatorDstAddr,
            _pageRequest
        );
    }

    /// @dev This function calls the staking precompile's unbonding delegation query method.
    /// @param _delegatorAddr The delegator address.
    /// @param _validatorAddr The validator address to delegate from.
    /// @return unbondingDelegation The unbonding delegation output.
    function getUnbondingDelegation(
        address _delegatorAddr,
        string memory _validatorAddr
    )
    public
    view
    returns (staking.UnbondingDelegationOutput memory unbondingDelegation)
    {
        return
            staking.STAKING_CONTRACT.unbondingDelegation(_delegatorAddr, _validatorAddr);
    }

    /// @dev This function is used to test the behaviour when executing transactions using special
    /// function calling opcodes,
    /// like call, delegatecall, staticcall, and callcode.
    /// @param _validatorAddr The validator address to delegate from.
    /// @param _amount The amount to undelegate.
    /// @param _calltype The opcode to use.
    function testCallUndelegate(
        string memory _validatorAddr,
        uint256 _amount,
        string memory _calltype
    ) public {
        _dequeueUnbondingDelegation();
        address calledContractAddress = staking.STAKING_PRECOMPILE_ADDRESS;
        bytes memory payload = abi.encodeWithSignature(
            "undelegate(address,string,uint256)",
            address(this),
            _validatorAddr,
            _amount
        );
        bytes32 calltypeHash = keccak256(abi.encodePacked(_calltype));

        int64 completionTime = int64(int256(block.timestamp + 21 days));
        if (calltypeHash == keccak256(abi.encodePacked("delegatecall"))) {
            (bool success, bytes memory returnData) = calledContractAddress.delegatecall(payload);
            require(success, "failed delegatecall to precompile");
            completionTime = abi.decode(returnData, (int64));
        } else if (calltypeHash == keccak256(abi.encodePacked("staticcall"))) {
            (bool success, bytes memory returnData) = calledContractAddress.staticcall(payload);
            require(success, "failed staticcall to precompile");
            completionTime = abi.decode(returnData, (int64));
        } else if (calltypeHash == keccak256(abi.encodePacked("call"))) {
            (bool success, bytes memory returnData) = calledContractAddress.call(payload);
            require(success, "failed call to precompile");
            completionTime = abi.decode(returnData, (int64));
        } else if (calltypeHash == keccak256(abi.encodePacked("callcode"))) {
            // NOTE: callcode is deprecated and now only available via inline assembly
            assembly {
            // Load the function signature and argument data onto the stack
                let ptr := add(payload, 0x20)
                let len := mload(payload)

            // Invoke the contract at calledContractAddress in the context of the current contract
            // using CALLCODE opcode and the loaded function signature and argument data
                let success := callcode(
                    gas(),
                    calledContractAddress,
                    0,
                    ptr,
                    len,
                    0,
                    0
                )

            // Check if the call was successful and revert the transaction if it failed
                if iszero(success) {
                    revert(0, 0)
                }
            }
        } else {
            revert("invalid calltype");
        }
        _undelegate(_validatorAddr, _amount, completionTime);
    }

    /// @dev This function is used to test the behaviour when executing queries using special function calling opcodes,
    /// like call, delegatecall, staticcall, and callcode.
    /// @param _delegatorAddr The address of the delegator.
    /// @param _validatorAddr The validator address to query for.
    /// @param _calltype The opcode to use.
    function testCallDelegation(
        address _delegatorAddr,
        string memory _validatorAddr,
        string memory _calltype
    ) public returns (uint256 shares, staking.Coin memory coin) {
        address calledContractAddress = staking.STAKING_PRECOMPILE_ADDRESS;
        bytes memory payload = abi.encodeWithSignature(
            "delegation(address,string)",
            _delegatorAddr,
            _validatorAddr
        );
        bytes32 calltypeHash = keccak256(abi.encodePacked(_calltype));

        if (calltypeHash == keccak256(abi.encodePacked("delegatecall"))) {
            (bool success, bytes memory data) = calledContractAddress
                .delegatecall(payload);
            require(success, "failed delegatecall to precompile");
            (shares, coin) = abi.decode(data, (uint256, staking.Coin));
        } else if (calltypeHash == keccak256(abi.encodePacked("staticcall"))) {
            (bool success, bytes memory data) = calledContractAddress
                .staticcall(payload);
            require(success, "failed staticcall to precompile");
            (shares, coin) = abi.decode(data, (uint256, staking.Coin));
        } else if (calltypeHash == keccak256(abi.encodePacked("call"))) {
            (bool success, bytes memory data) = calledContractAddress.call(
                payload
            );
            require(success, "failed call to precompile");
            (shares, coin) = abi.decode(data, (uint256, staking.Coin));
        } else if (calltypeHash == keccak256(abi.encodePacked("callcode"))) {
            //Function signature
            bytes4 sig = bytes4(keccak256(bytes("delegation(address,string)")));
            // Length of the input data is 164 bytes on 32bytes chunks:
            //                          Memory location
            // 0 - 4 byte signature     x
            // 1 - 0x0000..address		x + 0x04
            // 2 - 0x0000..00			x + 0x24
            // 3 - 0x40..0000			x + 0x44
            // 4 - val_addr_chunk1		x + 0x64
            // 5 - val_addr_chunk2..000	x + 0x84
            uint256 len = 164;
            // Coin type includes denom & amount
            // need to get these separately from the bytes response
            string memory denom;
            uint256 amt;

            // NOTE: callcode is deprecated and now only available via inline assembly
            assembly {
                let chunk1 := mload(add(_validatorAddr, 32)) // first 32 bytes of validator address string
                let chunk2 := mload(add(add(_validatorAddr, 32), 32)) // remaining 19 bytes of val address string

            // Load the function signature and argument data onto the stack
                let x := mload(0x40) // Find empty storage location using "free memory pointer"
                mstore(x, sig) // Place function signature at beginning of empty storage
                mstore(add(x, 0x04), _delegatorAddr) // Place the address (input param) after the function sig
                mstore(add(x, 0x24), 0x40) // These are needed for
                mstore(add(x, 0x44), 0x33) // bytes unpacking
                mstore(add(x, 0x64), chunk1) // Place the validator address in 2 chunks (input param)
                mstore(add(x, 0x84), chunk2) // because mstore stores 32bytes

            // Invoke the contract at calledContractAddress in the context of the current contract
            // using CALLCODE opcode and the loaded function signature and argument data
                let success := callcode(
                    gas(),
                    calledContractAddress, // to addr
                    0, // no value
                    x, // inputs are stored at location x
                    len, // inputs length
                    x, //store output over input (saves space)
                    0xC0 // output length for this call
                )

            // output length for this call is 192 bytes splitted on these 32 bytes chunks:
            // 1 - 0x00..amt   -> @ 0x40
            // 2 - 0x000..00   -> @ 0x60
            // 3 - 0x40..000   -> @ 0x80
            // 4 - 0x00..amt    -> @ 0xC0
            // 5 - 0x00..denom  -> @ 0xE0   TODO: cannot get the return value

                shares := mload(x) // Assign shares output value - 32 bytes long
                amt := mload(add(x, 0x60)) // Assign output value to c - 64 bytes long (string & uint256)

                mstore(0x40, add(x, 0x100)) // Set storage pointer to empty space

            // Check if the call was successful and revert the transaction if it failed
                if iszero(success) {
                    revert(0, 0)
                }
            }
            // NOTE: this is returning a blank denom because unpacking the denom is not
            // straightforward and hasn't been solved, which is okay for this generic test case.
            coin = staking.Coin(denom, amt);
        } else {
            revert("invalid calltype");
        }

        return (shares, coin);
    }

    /// @dev This function showcased, that there was a bug in the EVM implementation, that occurred when
    /// Cosmos state is modified in the same transaction as state information inside
    /// the EVM.
    /// @param _validatorAddr Address of the validator to delegate to
    function testDelegateIncrementCounter(
        string memory _validatorAddr
    ) public payable {
        _dequeueUnbondingDelegation();
        bool success = staking.STAKING_CONTRACT.delegate(
            address(this),
            _validatorAddr,
            msg.value
        );
        require(success, "delegate failed");
        _increaseAmount(msg.sender, _validatorAddr, msg.value);
        counter += 1;
    }

    /// @dev This function is suppose to fail because the amount to delegate is
    /// higher than the amount transfered.
    function testDelegateAndFailCustomLogic(
        string memory _validatorAddr
    ) public payable {
        _dequeueUnbondingDelegation();
        bool success = staking.STAKING_CONTRACT.delegate(
            address(this),
            _validatorAddr,
            msg.value
        );
        require(success, "delegate failed");
        _increaseAmount(msg.sender, _validatorAddr, msg.value);

        // This should fail since the balance is already spent in the previous call
        payable(msg.sender).transfer(msg.value);
    }

    /// @dev This function is used to check that both the cosmos and evm state are correctly
    /// updated for a successful transaction or reverted for a failed transaction.
    /// To test this, deploy an ERC20 token contract to chain and mint some tokens to this
    /// contract's address.
    /// This contract will then transfer some tokens to the msg.sender address as well as
    /// delegate some tokens to a validator using the staking EVM extension.
    /// @param _contract Address of the ERC20 to call
    /// @param _validatorAddr Address of the validator to delegate to
    function callERC20AndDelegate(
        address _contract,
        string memory _validatorAddr,
        uint256 _amount
    ) public payable {
        _dequeueUnbondingDelegation();
        (bool success, ) = _contract.call(
            abi.encodeWithSignature(
                "transfer(address,uint256)",
                msg.sender,
                _amount
            )
        );
        require(success, "transfer failed");
        success = staking.STAKING_CONTRACT.delegate(address(this), _validatorAddr, msg.value);
        require(success, "delegate failed");
        _increaseAmount(msg.sender, _validatorAddr, msg.value);
    }
}
