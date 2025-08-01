pragma solidity ^0.8.20;

import "../../../../precompiles/callbacks/ICallbacks.sol";
import "../../../../precompiles/erc20/IERC20.sol";

contract CounterWithCallbacks is ICallbacks {
    // State variables
    int public counter;

    // Mapping: user address => token address => balance
    mapping(address => mapping(address => uint256)) public userTokenBalances;

    // Events
    event CounterIncremented(int newValue, address indexed user);
    event TokensDeposited(
        address indexed user,
        address indexed token,
        uint256 amount,
        uint256 newBalance
    );
    event PacketAcknowledged(
        string indexed channelId,
        string indexed portId,
        uint64 sequence,
        bytes data,
        bytes acknowledgement
    );
    event PacketTimedOut(
        string indexed channelId,
        string indexed portId,
        uint64 sequence,
        bytes data
    );

    /**
     * @dev Increment the counter and deposit ERC20 tokens
     * @param token The address of the ERC20 token
     * @param amount The amount of tokens to deposit
     */
    function add(address token, uint256 amount)
    external
    {
        // Transfer tokens from user to this contract
        IERC20(token).transferFrom(msg.sender, address(this), amount);

        // Increment counter
        counter += 1;

        // Add to user's token balance
        userTokenBalances[msg.sender][token] += amount;

        // Emit events
        emit CounterIncremented(counter, msg.sender);
        emit TokensDeposited(msg.sender, token, amount, userTokenBalances[msg.sender][token]);
    }

    /**
     * @dev Get the current counter value
     * @return The current counter value
     */
    function getCounter() external view returns (int) {
        return counter;
    }

    /**
     * @dev Get a user's balance for a specific token
     * @param user The address of the user
     * @param token The address of the token
     * @return The user's token balance
     */
    function getTokenBalance(address user, address token) external view returns (uint256) {
        return userTokenBalances[user][token];
    }

    /**
     * @dev Implementation of ICallbacks interface
     * Called when a packet acknowledgement is received
     */
    function onPacketAcknowledgement(
        string memory channelId,
        string memory portId,
        uint64 sequence,
        bytes memory data,
        bytes memory acknowledgement
    ) external override {
        // Emit event when packet is acknowledged
        emit PacketAcknowledged(channelId, portId, sequence, data, acknowledgement);

        counter += 1; // Increment counter on acknowledgement
    }

    /**
     * @dev Implementation of ICallbacks interface
     * Called when a packet times out
     */
    function onPacketTimeout(
        string memory channelId,
        string memory portId,
        uint64 sequence,
        bytes memory data
    ) external override {
        // Emit event when packet times out
        emit PacketTimedOut(channelId, portId, sequence, data);
        counter -= 1; // Decrement counter on timeout
    }

    /**
     * @dev Reset the counter
     */
    function resetCounter() external {
        counter = 0;
    }
}