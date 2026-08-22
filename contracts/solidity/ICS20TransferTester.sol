// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

import "./precompiles/ics20/ICS20I.sol";
import "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import "./precompiles/staking/StakingI.sol";
import "./precompiles/common/Types.sol";

/**
 * @dev Contract to test ICS20 transfers with auto-flush behavior.
 * Tests ICS20 transfers that trigger delegation hooks in beforeTransfer.
 */
contract ICS20TransferTester {
    event OperationCompleted(string operation, bool success);
    event TransferInitiated(string receiver, uint256 amount);

    /// @dev Scenario 9: ERC20 Transfer -> ICS20 Transfer (with delegation in beforeTransfer)
    /// @param token ERC20 token address
    /// @param recipient Recipient for ERC20 transfer
    /// @param transferAmount Amount to transfer via ERC20
    /// @param sourcePort ICS20 source port
    /// @param sourceChannel ICS20 source channel
    /// @param denom Denomination for ICS20 transfer
    /// @param ics20Amount Amount for ICS20 transfer
    /// @param ics20Receiver Bech32 receiver address for ICS20
    /// @param timeoutHeight Timeout height for ICS20
    /// @param timeoutTimestamp Timeout timestamp for ICS20
    function scenario9_transferICS20Transfer(
        address token,
        address recipient,
        uint256 transferAmount,
        string memory sourcePort,
        string memory sourceChannel,
        string memory denom,
        uint256 ics20Amount,
        string memory ics20Receiver,
        Height memory timeoutHeight,
        uint64 timeoutTimestamp
    ) external {
        // 1. ERC20 transfer
        require(
            IERC20(token).transfer(recipient, transferAmount),
            "ERC20 transfer failed"
        );
        emit OperationCompleted("erc20_transfer", true);

        // 2. ICS20 transfer (this will trigger delegation in beforeTransfer hook)
        uint64 sequence = ICS20_CONTRACT.transfer(
            sourcePort,
            sourceChannel,
            denom,
            ics20Amount,
            address(this),
            ics20Receiver,
            timeoutHeight,
            timeoutTimestamp,
            "" // empty memo
        );
        emit TransferInitiated(ics20Receiver, ics20Amount);
        emit OperationCompleted("ics20_transfer", true);
    }

    /// @dev Scenario 10: ERC20 Transfer -> ICS20 Transfer (reverted & caught, with delegation in beforeTransfer)
    /// @param token ERC20 token address
    /// @param recipient Recipient for ERC20 transfer
    /// @param transferAmount Amount to transfer via ERC20
    /// @param sourcePort ICS20 source port
    /// @param sourceChannel ICS20 source channel
    /// @param denom Denomination for ICS20 transfer
    /// @param ics20Amount Amount for ICS20 transfer (will be excessive to cause revert)
    /// @param ics20Receiver Bech32 receiver address for ICS20
    /// @param timeoutHeight Timeout height for ICS20
    /// @param timeoutTimestamp Timeout timestamp for ICS20
    function scenario10_transferICS20TransferRevert(
        address token,
        address recipient,
        uint256 transferAmount,
        string memory sourcePort,
        string memory sourceChannel,
        string memory denom,
        uint256 ics20Amount,
        string memory ics20Receiver,
        Height memory timeoutHeight,
        uint64 timeoutTimestamp
    ) external {
        // 1. Try ERC20 transfer (will revert if insufficient balance, catch it)
        try IERC20(token).transfer(recipient, transferAmount) returns (bool success) {
            if (success) {
                emit OperationCompleted("erc20_transfer", true);
            } else {
                emit OperationCompleted("erc20_transfer", false);
            }
        } catch {
            emit OperationCompleted("erc20_transfer", false);
        }

        // 2. ICS20 transfer (should succeed with delegation in beforeTransfer hook)
        uint64 sequence = ICS20_CONTRACT.transfer(
            sourcePort,
            sourceChannel,
            denom,
            ics20Amount,
            address(this),
            ics20Receiver,
            timeoutHeight,
            timeoutTimestamp,
            "" // empty memo
        );
        emit TransferInitiated(ics20Receiver, ics20Amount);
        emit OperationCompleted("ics20_transfer", true);
    }

    /// @dev Get balance of ERC20 token
    function getTokenBalance(address token, address account) external view returns (uint256) {
        return IERC20(token).balanceOf(account);
    }

    /// @dev Receive function to accept native tokens
    receive() external payable {}
}
