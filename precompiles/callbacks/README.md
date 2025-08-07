# Callbacks Interface

## Description

The Callbacks interface defines a standard for smart contracts to receive notifications about IBC packet lifecycle events.
This is not a precompile with a fixed address, but rather an interface specification
that contracts must implement to receive callbacks from the IBC module.

## Interface

### Methods

#### onPacketAcknowledgement

```solidity
function onPacketAcknowledgement(
    string memory channelId,
    string memory portId,
    uint64 sequence,
    bytes memory data,
    bytes memory acknowledgement
) external
```

Called when an IBC packet sent by the implementing contract receives an acknowledgement from the destination chain.

**Parameters:**

- `channelId`: The IBC channel identifier
- `portId`: The IBC port identifier
- `sequence`: The packet sequence number
- `data`: The original packet data
- `acknowledgement`: The acknowledgement data from the destination chain

**Invocation:**

- Only called by the IBC module
- Only invoked for packets sent by the implementing contract
- Called after successful packet delivery and acknowledgement

#### onPacketTimeout

```solidity
function onPacketTimeout(
    string memory channelId,
    string memory portId,
    uint64 sequence,
    bytes memory data
) external
```

Called when an IBC packet sent by the implementing contract times out without being processed by the destination chain.

**Parameters:**

- `channelId`: The IBC channel identifier
- `portId`: The IBC port identifier
- `sequence`: The packet sequence number
- `data`: The original packet data

**Invocation:**

- Only called by the IBC module
- Only invoked for packets sent by the implementing contract
- Called when packet timeout conditions are met

## Implementation Requirements

### Access Control

Implementing contracts must ensure that only the IBC module can invoke these callback methods.
This prevents unauthorized contracts from triggering callback logic.

### State Management

Contracts should maintain appropriate state to correlate callbacks with their original packet sends,
typically using the sequence number as a unique identifier.

### Error Handling

Callback implementations should handle errors gracefully as failures in callback execution may affect the IBC packet lifecycle.

### Gas Considerations

Callback execution consumes gas on the source chain.
Implementations should be gas-efficient to avoid transaction failures due to out-of-gas errors.
