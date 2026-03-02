// SPDX-License-Identifier: MIT

pragma solidity ^0.8.0;

import "./precompiles/ics20/ICS20I.sol";
import "./precompiles/erc20/IERC20.sol";
import "./precompiles/common/Types.sol";

/**
 * @dev Contract that receives ERC20 tokens and performs sequential max ICS20 sends.
 * Used to test that two sequential max balance sends revert properly.
 */
contract SequentialICS20Sender {
    event ICS20SendAttempt(uint256 indexed attempt, uint256 amount, bool success);
    event SequentialSendReverted(uint256 indexed attempt, string reason);
    event BalanceQueried(uint256 balance);
    event TokensReceived(address token, uint256 amount);
    event TokensReturned(address token, uint256 amount);

    /// @dev Receive tokens, perform two sequential ICS20 sends, return remaining tokens.
    /// Mirrors production flow: transfer in -> ICS20 sends -> transfer out.
    /// @param token ERC20 token address
    /// @param sourcePort IBC source port
    /// @param sourceChannel IBC source channel
    /// @param denom Token denomination (e.g., "erc20:0x...")
    /// @param receiver Bech32 receiver address on destination chain
    /// @param timeoutHeight IBC timeout height
    /// @param amount The amount to transfer in and send in each ICS20 transfer
    function receiveAndSendTwice(
        address token,
        string memory sourcePort,
        string memory sourceChannel,
        string memory denom,
        string memory receiver,
        uint64 timeoutHeight,
        uint256 amount
    ) external {
        // 1. Transfer tokens from sender to this contract
        require(
            IERC20(token).transferFrom(msg.sender, address(this), amount),
            "Transfer in failed"
        );
        emit TokensReceived(token, amount);

        // 2. First ICS20 send
        try ICS20_CONTRACT.transfer(
            sourcePort,
            sourceChannel,
            denom,
            amount,
            address(this),
            receiver,
            Height({revisionNumber: 1, revisionHeight: timeoutHeight}),
            0,
            ""
        ) returns (uint64) {
        } catch {
        }

        // 3. Second ICS20 send
        try ICS20_CONTRACT.transfer(
            sourcePort,
            sourceChannel,
            denom,
            amount,
            address(this),
            receiver,
            Height({revisionNumber: 1, revisionHeight: timeoutHeight}),
            0,
            ""
        ) returns (uint64) {
        } catch {
            revert("Second Transfer Failed");
        }
    }


    /// @dev Query balance of an ERC20 token for this contract
    function getBalance(address token) external view returns (uint256) {
        return IERC20(token).balanceOf(address(this));
    }
}
