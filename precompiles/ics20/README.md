# ICS20 Precompile

The ICS20 precompile provides an EVM interface to the Inter-Blockchain Communication (IBC) transfer module,
enabling smart contracts to perform cross-chain token transfers using the ICS-20 standard.

## Address

The precompile is available at the fixed address: `0x0000000000000000000000000000000000000802`

## Interface

### Data Structures

```solidity
// Denomination information for IBC tokens
struct Denom {
    string base;      // Base denomination of the relayed fungible token
    Hop[] trace;      // List of hops for multi-hop transfers
}

// Port and channel pair for multi-hop transfers
struct Hop {
    string portId;
    string channelId;
}

// Height information for timeout
struct Height {
    uint64 revisionNumber;
    uint64 revisionHeight;
}
```

### Transaction Methods

```solidity
// Perform an IBC transfer
function transfer(
    string memory sourcePort,
    string memory sourceChannel,
    string memory denom,
    uint256 amount,
    address sender,
    string memory receiver,
    Height memory timeoutHeight,
    uint64 timeoutTimestamp,
    string memory memo
) external returns (uint64 nextSequence);
```

### Query Methods

```solidity
// Get all denominations with pagination
function denoms(
    PageRequest memory pageRequest
) external view returns (
    Denom[] memory denoms,
    PageResponse memory pageResponse
);

// Get a specific denomination by hash
function denom(
    string memory hash
) external view returns (Denom memory denom);

// Get the hash of a denomination trace
function denomHash(
    string memory trace
) external view returns (string memory hash);
```

## Gas Costs

Gas costs are calculated dynamically based on:

- Base gas for the method
- Additional gas for IBC operations
- Key-value storage operations

The precompile uses standard gas configuration for storage operations.

## Implementation Details

### Transfer Mechanism

1. **Channel Validation**:
   - For v1 packets: Validates that the channel exists and is in OPEN state
   - For v2 packets: Validates the client ID format
   - Checks that the underlying connection is OPEN

2. **Sender Verification**: The transaction sender must match the specified sender address

3. **Token Transfer**: Uses the IBC transfer keeper to execute the cross-chain transfer

4. **Sequence Tracking**: Returns the sequence number of the IBC packet sent

### Denomination Handling

- **Denom Traces**: Tracks the path of tokens through multiple IBC hops
- **Denom Hashes**: Provides unique identifiers for IBC denominations
- **Base Denominations**: Identifies the original token denomination

### Timeout Configuration

- **Height-based timeout**: Specify a block height for timeout
- **Timestamp-based timeout**: Specify an absolute timestamp in nanoseconds
- Setting either to 0 disables that timeout mechanism

## Events

```solidity
event IBCTransfer(
    address indexed sender,
    string indexed receiver,
    string sourcePort,
    string sourceChannel,
    string denom,
    uint256 amount,
    string memo
);
```

## Security Considerations

1. **Channel State Validation**: Ensures transfers only occur through active channels
2. **Sender Authentication**: Verifies the message sender matches the transfer sender
3. **Balance Handling**: Uses the balance handler for proper native token management
4. **Timeout Protection**: Prevents indefinite locking of tokens with timeout mechanisms

## Usage Example

```solidity
ICS20I ics20 = ICS20I(ICS20_PRECOMPILE_ADDRESS);

// Prepare transfer parameters
string memory sourcePort = "transfer";
string memory sourceChannel = "channel-0";
string memory denom = "uatom";
uint256 amount = 1000000; // 1 ATOM (6 decimals)
string memory receiver = "cosmos1..."; // Bech32 address on destination chain

// Set timeout (e.g., 1 hour from now)
Height memory timeoutHeight = Height({
    revisionNumber: 0,
    revisionHeight: 0  // Disabled
});
uint64 timeoutTimestamp = uint64(block.timestamp + 3600) * 1e9; // Convert to nanoseconds

// Execute IBC transfer
uint64 sequence = ics20.transfer(
    sourcePort,
    sourceChannel,
    denom,
    amount,
    msg.sender,
    receiver,
    timeoutHeight,
    timeoutTimestamp,
    "Transfer from EVM"
);

// Query denomination information
Denom memory denomInfo = ics20.denom("ibc/27394FB092D2ECCD56123C74F36E4C1F926001CEADA9CA97EA622B25F41E5EB2");
```

## Integration Notes

- The precompile integrates directly with the IBC transfer module
- Supports both IBC v1 (channel-based) and v2 (client-based) transfers
- Memo field can be used for additional transfer metadata or routing information
- Receiver addresses must be valid Bech32 addresses on the destination chain
- For v2 packets: Leave sourcePort empty and set sourceChannel to the client ID
