// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.18;

interface ICallbacks {
    /// @dev Callback function to be called on the source chain
    /// after the packet life cycle is completed and acknowledgement is processed
    /// by source chain. The contract address is passed the packet information and acknowledgmeent
    /// to execute the callback logic.
    /// @param channelId the channnel identifier of the packet
    /// @param portId the port identifier of the packet
    /// @param sequence the sequence number of the packet
    /// @param data the data of the packet
    /// @param acknowledgement the acknowledgement of the packet
    function onPacketAcknowledgement(
        string memory channelId,
        string memory portId,
        uint64 sequence,
        bytes memory data,
        bytes memory acknowledgement
    ) external;

    /// @dev Callback function to be called on the source chain
    /// after the packet life cycle is completed and the packet is timed out
    /// by source chain. The contract address is passed the packet information
    /// to execute the callback logic.
    /// @param channelId the channnel identifier of the packet
    /// @param portId the port identifier of the packet
    /// @param sequence the sequence number of the packet
    /// @param data the data of the packet
    function onPacketTimeout(
        string memory channelId,
        string memory portId,
        uint64 sequence,
        bytes memory data
    ) external;
}