// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./precompiles/staking/StakingI.sol";
import "./precompiles/erc20/IERC20.sol";
import "./precompiles/bech32/Bech32I.sol";
import "./precompiles/common/Types.sol";

/**
 * @dev Contract to test sequential operations with auto-flush behavior.
 * Tests combinations of ERC20 transfers, staking operations, and state changes.
 */
contract SequentialOperationsTester {
    event OperationCompleted(string operation, bool success);
    event BalanceChecked(string label, uint256 balance);
    event EventCountChecked(string label, uint256 count);

    /// @dev Scenario 1: Transfer ERC20 -> Delegate -> Transfer ERC20
    /// @param token ERC20 token address
    /// @param recipient Recipient for ERC20 transfers
    /// @param amount Amount to transfer
    /// @param validatorAddr Validator address (EVM format)
    /// @param delegateAmount Amount to delegate
    function scenario1_transferDelegateTransfer(
        address token,
        address recipient,
        uint256 amount,
        string memory validatorAddr,
        uint256 delegateAmount
    ) external {
        // 1. Transfer ERC20
        require(
            IERC20(token).transfer(recipient, amount),
            "First transfer failed"
        );
        emit OperationCompleted("transfer1", true);

        // 2. Staking delegate - convert from wei (18 decimals) to base denom (6 decimals)
        uint256 delegateAmountBaseDenom = delegateAmount / 1e12;
        STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmountBaseDenom
        );
        emit OperationCompleted("delegate", true);

        // 3. Transfer ERC20 again
        require(
            IERC20(token).transfer(recipient, amount),
            "Second transfer failed"
        );
        emit OperationCompleted("transfer2", true);
    }

    /// @dev Scenario 2: Transfer ERC20 -> Delegate (reverted & caught) -> Transfer ERC20
    /// @param token ERC20 token address
    /// @param recipient Recipient for ERC20 transfers
    /// @param amount Amount to transfer
    /// @param validatorAddr Validator address (EVM format)
    /// @param delegateAmount Amount to delegate (will revert)
    function scenario2_transferDelegateRevertTransfer(
        address token,
        address recipient,
        uint256 amount,
        string memory validatorAddr,
        uint256 delegateAmount
    ) external {
        // 1. Transfer ERC20
        require(
            IERC20(token).transfer(recipient, amount),
            "First transfer failed"
        );
        emit OperationCompleted("transfer1", true);

        // 2. Try to delegate (will revert, catch it)
        uint256 delegateAmountBaseDenom = delegateAmount / 1e12;
        try STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmountBaseDenom
        ) {
            emit OperationCompleted("delegate", true);
        } catch {
            emit OperationCompleted("delegate", false);
        }

        // 3. Transfer ERC20 again
        require(
            IERC20(token).transfer(recipient, amount),
            "Second transfer failed"
        );
        emit OperationCompleted("transfer2", true);
    }

    /// @dev Scenario 3: Native transfer -> Delegate -> Native transfer
    /// @param recipient Recipient for native transfers
    /// @param amount Amount of native tokens to transfer (in wei)
    /// @param validatorAddr Validator address
    /// @param delegateAmount Amount to delegate (in wei, will be converted to base denom)
    function scenario3_nativeTransferDelegateNativeTransfer(
        address payable recipient,
        uint256 amount,
        string memory validatorAddr,
        uint256 delegateAmount
    ) external payable {
        // 1. Transfer native tokens
        (bool success1, ) = recipient.call{value: amount}("");
        require(success1, "First native transfer failed");
        emit OperationCompleted("native_transfer1", true);

        // 2. Staking delegate - convert from wei (18 decimals) to base denom (6 decimals)
        uint256 delegateAmountBaseDenom = delegateAmount / 1e12;
        STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmountBaseDenom
        );
        emit OperationCompleted("delegate", true);

        // 3. Transfer native tokens again
        (bool success2, ) = recipient.call{value: amount}("");
        require(success2, "Second native transfer failed");
        emit OperationCompleted("native_transfer2", true);
    }

    /// @dev Scenario 4: Native transfer -> Delegate (reverted & caught) -> Native transfer
    /// @param recipient Recipient for native transfers
    /// @param amount Amount of native tokens to transfer (in wei)
    /// @param validatorAddr Validator address
    /// @param delegateAmount Amount to delegate (in wei, will be converted to base denom, will revert)
    function scenario4_nativeTransferDelegateRevertNativeTransfer(
        address payable recipient,
        uint256 amount,
        string memory validatorAddr,
        uint256 delegateAmount
    ) external payable {
        // 1. Transfer native tokens
        (bool success1, ) = recipient.call{value: amount}("");
        require(success1, "First native transfer failed");
        emit OperationCompleted("native_transfer1", true);

        // 2. Try to delegate - convert from wei (18 decimals) to base denom (6 decimals)
        uint256 delegateAmountBaseDenom = delegateAmount / 1e12;
        try STAKING_CONTRACT.delegate(
            address(this),
            validatorAddr,
            delegateAmountBaseDenom
        ) {
            emit OperationCompleted("delegate", true);
        } catch {
            emit OperationCompleted("delegate", false);
        }

        // 3. Transfer native tokens again
        (bool success2, ) = recipient.call{value: amount}("");
        require(success2, "Second native transfer failed");
        emit OperationCompleted("native_transfer2", true);
    }

    /// @dev Get balance of ERC20 token
    function getTokenBalance(address token, address account) external view returns (uint256) {
        return IERC20(token).balanceOf(account);
    }

    /// @dev Get native balance
    function getNativeBalance(address account) external view returns (uint256) {
        return account.balance;
    }

    /// @dev Test function to check contract balance
    function getContractBalance() external view returns (uint256) {
        return address(this).balance;
    }

    /// @dev Simple native transfer test
    function testNativeTransfer(address payable recipient, uint256 amount) external payable returns (bool) {
        (bool success, ) = recipient.call{value: amount}("");
        return success;
    }

    /// @dev Receive function to accept native tokens
    receive() external payable {}
}
