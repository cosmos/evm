// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.18;

import "../common/Types.sol";

/// @dev The ICS02I contract's address.
address constant ICS02_PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000000807;

/// @dev The ICS02 contract's instance.
ICS02I constant ICS02_CONTRACT = ICS02I(ICS02_PRECOMPILE_ADDRESS);

/// @author CosmosLabs
/// @title ICS02 Client Router Precompile Interface
/// @dev The interface through which solidity contracts will interact with IBC Light Clients (ICS02)
/// @custom:address 0x0000000000000000000000000000000000000807
interface ICS02I {
    /// @notice The result of an update operation
    enum UpdateResult {
        /// The update was successful
        Update,
        /// A misbehaviour was detected
        Misbehaviour
    }

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

    /// @notice Returns the client state.
    /// @param clientId The client identifier
    /// @return The client state.
    function getClientState(string calldata clientId) external view returns (bytes memory);
}
