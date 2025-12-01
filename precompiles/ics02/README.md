# ICS02 Precompile

The ICS02 precompile provides an EVM interface to the IBC-Go's `02-client` module,
enabling smart contracts to interact with IBC light clients using the ICS-02 standard.

## Address

The precompile is available at the fixed address: `0x0000000000000000000000000000000000000807`

## Interface

### Data Structures

```solidity
/// @notice The result of an update operation
enum UpdateResult {
    /// The update was successful
    Update,
    /// A misbehaviour was detected
    Misbehaviour
}
```

### Transaction Methods

```solidity
/// @notice Updates the client with the given client identifier.
/// @param clientId The client identifier
/// @param updateMsg The encoded update message e.g., a protobuf any.
/// @return The result of the update operation
function updateClient(string calldata clientId, bytes calldata updateMsg) external returns (UpdateResult);

/// @notice Querying the membership of a key-value pair
/// @dev Notice that this message is not view, as it may update the client state for caching purposes.
/// @param proof The proof of membership
/// @param proofHeight The height of the proof
/// @param path The path of the value in the Merkle tree
/// @param value The value in the Merkle tree
/// @return The unix timestamp of the verification height in the counterparty chain in seconds.
function verifyMembership(
    string calldata clientId,
    bytes calldata proof,
    Height calldata proofHeight,
    bytes[] calldata path,
    bytes calldata value
) external returns (uint256);

/// @notice Querying the non-membership of a key
/// @dev Notice that this message is not view, as it may update the client state for caching purposes.
/// @param proof The proof of membership
/// @param proofHeight The height of the proof
/// @param path The path of the value in the Merkle tree
/// @return The unix timestamp of the verification height in the counterparty chain in seconds.
function verifyNonMembership(
    string calldata clientId,
    bytes calldata proof,
    Height calldata proofHeight,
    bytes[] calldata path
) external returns (uint256);
```

### Query Methods

```solidity
/// @notice Returns the client state.
/// @param clientId The client identifier
/// @return The client state.
function getClientState(string calldata clientId) external view returns (bytes memory);
```

## Gas Costs

Gas costs are calculated dynamically based on:

- Base gas for the method
- Additional gas for IBC operations
- Key-value storage operations

The precompile uses standard gas configuration for storage operations.

## Implementation Details

### `GetClientState`

This method is meant to be called by `solidity-ibc` relayers. It validates the client identifier and retrieves the protobuf encoded client state from the IBC-Go store. The relayer must decode the client state to correct type to use it.

### `UpdateClient`

This method allows updating the IBC light client state on-chain. It accepts a client identifier and an encoded client message (a protobuf Any). The method processes the update message, verifies it, and updates the client state accordingly. It returns an `UpdateResult` enum indicating whether the update was successful or if misbehaviour was detected.

### `VerifyMembership` and `VerifyNonMembership`

These methods allow smart contracts to verify the membership or non-membership of key-value pairs in the IBC light client's state. They accept a client identifier, a proof, proof height, and the path (and value for membership). The methods validate the proofs against the client state and return the unix timestamp (in seconds) of the verification height in the counterparty chain.

## Events

The ICS-02 precompile does not emit any events. Since it is meant to be called by other [`solidity-ibc`](https://github.com/cosmos/solidity-ibc-eureka) contracts, events should be emitted by the calling contract if needed.

## Usage Example

A usage example is to be included in `solidity-ibc` repository. For now, we link the tracking issue: [cosmos/solidity-ibc-eureka#794](https://github.com/cosmos/solidity-ibc-eureka/issues/794).
