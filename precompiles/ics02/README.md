# ICS02 Precompile

The ICS02 enavles native Cosmos SDK light clients to be accessed and managed through the standard [`ILightClient`](https://github.com/cosmos/solidity-ibc-eureka/blob/906ee5954e5817c408c0d86bee99197a655b1d95/contracts/interfaces/ILightClient.sol) interface in [`solidity-ibc-eureka`](https://github.com/cosmos/solidity-ibc-eureka).

## Interface

The precompile implements the standard `ILightClient` interface:

### ILightClient Methods

```solidity
/// @title Light Client Interface
/// @notice Interface for all IBC Eureka light clients to implement.
interface ILightClient {
    /// @notice Updating the client and consensus state
    /// @param updateMsg The encoded update message e.g., an SP1 proof.
    /// @return The result of the update operation
    function updateClient(bytes calldata updateMsg) external returns (ILightClientMsgs.UpdateResult);

    /// @notice Querying the membership of a key-value pair
    /// @dev Notice that this message is not view, as it may update the client state for caching purposes.
    /// @param msg_ The membership message
    /// @return The unix timestamp of the verification height in the counterparty chain in seconds.
    function verifyMembership(ILightClientMsgs.MsgVerifyMembership calldata msg_) external returns (uint256);

    /// @notice Querying the non-membership of a key
    /// @dev Notice that this message is not view, as it may update the client state for caching purposes.
    /// @param msg_ The membership message
    /// @return The unix timestamp of the verification height in the counterparty chain in seconds.
    function verifyNonMembership(ILightClientMsgs.MsgVerifyNonMembership calldata msg_) external returns (uint256);

    /// @notice Misbehaviour handling, moves the light client to the frozen state if misbehaviour is detected
    /// @param misbehaviourMsg The misbehaviour message
    function misbehaviour(bytes calldata misbehaviourMsg) external;

    /// @notice Upgrading the client
    /// @param upgradeMsg The upgrade message
    function upgradeClient(bytes calldata upgradeMsg) external;

    /// @notice Returns the client state.
    /// @return The client state.
    function getClientState() external view returns (bytes memory);
}
```

### ILightClientMsgs Structures

```solidity
/// @title LightClient Messages
/// @notice Interface defining light client messages
interface ILightClientMsgs {
    /// @notice Message for querying the membership of a key-value pair in the Merkle root at a given height.
    /// @param proof The proof
    /// @param proofHeight The height of the proof
    /// @param path The path of the value in the Merkle tree
    /// @param value The value in the Merkle tree
    struct MsgVerifyMembership {
        bytes proof;
        IICS02ClientMsgs.Height proofHeight;
        bytes[] path;
        bytes value;
    }

    /// @notice Message for querying the non-membership of a key in the Merkle root at a given height.
    /// @param proof The proof
    /// @param proofHeight The height of the proof
    /// @param path The path of the value in the Merkle tree
    struct MsgVerifyNonMembership {
        bytes proof;
        IICS02ClientMsgs.Height proofHeight;
        bytes[] path;
    }

    /// @notice The result of an update operation
    enum UpdateResult {
        /// The update was successful
        Update,
        /// A misbehaviour was detected
        Misbehaviour,
        /// Client is already up to date
        NoOp
    }
}
```

## Gas Costs

The following gas costs are charged for each method:


| Method | Gas Cost |
|--------|----------|
| `updateClient` | 40,000 |
| `verifyMembership` | 15,000 |
| `verifyNonMembership` | 15,000 |
| `misbehaviour` | 40,000 |
| `upgradeClient` | 40,000 |
| `getClientState` | 4,000 |
