// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./precompiles/staking/StakingI.sol";
import "./precompiles/common/Types.sol";

/**
 * @dev Simple contract that can be created and receive value
 */
contract SimpleReceiver {
    uint256 public value;
    address public creator;

    event ReceiverCreated(address indexed creator, uint256 initialValue);

    constructor(bool shouldRevert) payable {
        if (shouldRevert) {
            revert("Intentional constructor revert");
        }
        creator = msg.sender;
        value = msg.value;
        emit ReceiverCreated(msg.sender, msg.value);
    }

    receive() external payable {
        value += msg.value;
    }

    function getValue() external view returns (uint256) {
        return value;
    }
}

/**
 * @dev Contract to test contract creation with precompile calls
 * Tests scenarios 4-7 with various combinations of CREATE and precompile operations
 */
contract ContractCreationTester {
    event OperationCompleted(string operation, bool success);
    event ContractCreated(address indexed contractAddr, uint256 value);
    event ValueSent(address indexed recipient, uint256 amount);

    address[] public createdContracts;

    /// @dev Helper function that creates a contract and then reverts
    /// @dev Used for testing reverted contract creation scenarios
    function createAndRevert(uint256 creationValue) public payable returns (SimpleReceiver) {
        SimpleReceiver newContract = new SimpleReceiver{value: creationValue}(false);
        revert("Intentional revert after creation");
    }

    /// @dev Scenario 4: Precompile -> Create Contract -> Precompile
    function scenario4_delegateCreateDelegate(
        string memory validatorAddr,
        uint256 delegateAmount1,
        uint256 creationValue,
        uint256 delegateAmount2
    ) external payable {
        // 1. Precompile call (delegate) - convert from wei to base denom
        uint256 delegateAmount1BaseDenom = delegateAmount1;
        STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmount1BaseDenom
        );
        emit OperationCompleted("delegate1", true);

        // 2. Create another contract with value (shouldRevert = false)
        SimpleReceiver newContract = new SimpleReceiver{value: creationValue}(false);
        createdContracts.push(address(newContract));
        emit ContractCreated(address(newContract), creationValue);
        emit OperationCompleted("create", true);

        // 3. Precompile call (delegate again) - convert from wei to base denom
        uint256 delegateAmount2BaseDenom = delegateAmount2;
        STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmount2BaseDenom
        );
        emit OperationCompleted("delegate2", true);
    }

    /// @dev Scenario 5: Precompile -> Create Contract (reverted & caught) -> Precompile
    function scenario5_delegateCreateRevertDelegate(
        string memory validatorAddr,
        uint256 delegateAmount1,
        uint256 creationValue,
        uint256 delegateAmount2
    ) external payable {
        // 1. Precompile call (delegate) - convert from wei to base denom
        uint256 delegateAmount1BaseDenom = delegateAmount1;
        STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmount1BaseDenom
        );
        emit OperationCompleted("delegate1", true);

        // 2. Try to create contract (will revert if insufficient value, catch it)
        try new SimpleReceiver{value: creationValue}(false) returns (SimpleReceiver newContract) {
            createdContracts.push(address(newContract));
            emit ContractCreated(address(newContract), creationValue);
            emit OperationCompleted("create", true);
        } catch {
            emit OperationCompleted("create", false);
        }

        // 3. Precompile call (delegate again) - convert from wei to base denom
        uint256 delegateAmount2BaseDenom = delegateAmount2;
        STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmount2BaseDenom
        );
        emit OperationCompleted("delegate2", true);
    }

    /// @dev Scenario 6: Create+Revert (caught) -> Precompile -> Create+Revert (caught)
    /// @dev Creates fail because the helper function reverts after creation, testing auto-flush with reverted creations
    function scenario6_createRevertDelegateCreateRevert(
        uint256 creationValue1,
        string memory validatorAddr,
        uint256 delegateAmount,
        uint256 creationValue2
    ) external payable {
        // 1. Try to create contract (will revert after creation, catch it)
        try this.createAndRevert(creationValue1) returns (SimpleReceiver newContract1) {
            // This won't execute because createAndRevert reverts
            createdContracts.push(address(newContract1));
            emit ContractCreated(address(newContract1), creationValue1);
            emit OperationCompleted("create1", true);
        } catch {
            emit OperationCompleted("create1_reverted", false);
        }

        // 2. Precompile call - convert from wei to base denom
        uint256 delegateAmountBaseDenom = delegateAmount;
        STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmountBaseDenom
        );
        emit OperationCompleted("delegate", true);

        // 3. Try to create contract again (will revert after creation, catch it)
        try this.createAndRevert(creationValue2) returns (SimpleReceiver newContract2) {
            // This won't execute because createAndRevert reverts
            createdContracts.push(address(newContract2));
            emit ContractCreated(address(newContract2), creationValue2);
            emit OperationCompleted("create2", true);
        } catch {
            emit OperationCompleted("create2_reverted", false);
        }
    }

    /// @dev Scenario 7: Create+Send -> Precompile (reverted & caught) -> Send more
    function scenario7_createDelegateRevertSend(
        uint256 creationValue,
        string memory validatorAddr,
        uint256 delegateAmount,
        uint256 sendAmount
    ) external payable {
        // 1. Create contract and send it value (shouldRevert = false)
        SimpleReceiver newContract = new SimpleReceiver{value: creationValue}(false);
        createdContracts.push(address(newContract));
        emit ContractCreated(address(newContract), creationValue);
        emit OperationCompleted("create", true);

        // 2. Precompile call (delegate) - convert from wei to base denom, reverted and caught
        uint256 delegateAmountBaseDenom = delegateAmount;
        try STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmountBaseDenom
        ) {
            emit OperationCompleted("delegate", true);
        } catch {
            emit OperationCompleted("delegate", false);
        }

        // 3. Send more value to the created contract
        (bool success, ) = address(newContract).call{value: sendAmount}("");
        require(success, "Value send failed");
        emit ValueSent(address(newContract), sendAmount);
        emit OperationCompleted("send", true);
    }

    /// @dev Scenario 8: Create+Revert (caught) -> Delegate -> Create+Success
    /// @dev Tests that reverted creation doesn't prevent successful creation after delegation
    function scenario8_createRevertDelegateCreateSuccess(
        uint256 revertCreationValue,
        string memory validatorAddr,
        uint256 delegateAmount,
        uint256 successCreationValue
    ) external payable {
        // 1. Try to create contract (will revert after creation, catch it)
        try this.createAndRevert{value: revertCreationValue}
        (revertCreationValue) returns (SimpleReceiver newContract1) {
            // This won't execute because createAndRevert reverts
            createdContracts.push(address(newContract1));
            emit ContractCreated(address(newContract1), revertCreationValue);
            emit OperationCompleted("create1", true);
        } catch {
            emit OperationCompleted("create1_reverted", false);
        }

        // 2. Precompile call - convert from wei to base denom
        uint256 delegateAmountBaseDenom = delegateAmount;
        STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmountBaseDenom
        );
        emit OperationCompleted("delegate", true);

        // 3. Create contract successfully (shouldRevert = false)
        SimpleReceiver newContract2 = new SimpleReceiver{value: successCreationValue}(false);
        createdContracts.push(address(newContract2));
        emit ContractCreated(address(newContract2), successCreationValue);
        emit OperationCompleted("create2", true);
    }

    /// @dev Get count of created contracts
    function getCreatedContractsCount() external view returns (uint256) {
        return createdContracts.length;
    }

    /// @dev Get created contract at index
    function getCreatedContract(uint256 index) external view returns (address) {
        require(index < createdContracts.length, "Index out of bounds");
        return createdContracts[index];
    }

    receive() external payable {}
}
