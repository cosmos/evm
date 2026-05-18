# EVM Callbacks

The EVM Callbacks module implements the EVM contractKeeper interface that will interact
with ibc-go's [callbacks middleware](http://github.com/cosmos/ibc-go/blob/main/modules/apps/callbacks/README.md).
EVM Callbacks are implemented specifically for the ICS-20 transfer application.

The `onRecvPacket` callback is implemented in order to provide a destination-side EVM contract with custom calldata
provided by the packet sender. This allows external contracts to be called atomically along with transfer and for
the contract to use the funds received in the packet. An example use case might be to transfer tokens to a destination
chains and then swap them to a different denomination using a DEX contract.

The `onAcknowledgePacket` and `onTimeoutPacket` are implemented in order to provide contracts with information on
the status of the packet lifecycle completion. Thus, the `onAcknowledgePacket` and `onTimeoutPacket` callbacks are
designed to call a specific entrypoint on the contract that is designed to provide the packet information and the acknowledgement.

## How do EVM callbacks work?

EVM Callbacks are made possible through the `memo` field included in every ICS-20 transfer packet,
as introduced in
[IBC v3.4.0](https://medium.com/the-interchain-foundation/moving-beyond-simple-token-transfers-d42b2b1dc29b).

The EVM Callbacks keeper parses an ICS20 transfer, and if the `memo` field has the form expected by IBC Callbacks,
it will execute an EVM contract call.

The following sections detail the `memo` format for EVM contract calls and the execution guarantees provided.

### EVM Contract Execution Format

Before diving into the IBC metadata format, it's important to understand how we will execute EVM contract calls from
outside the state machine. Provided below is the EVM keeper's `CallData` function.

```go
func (k Keeper) CallEVMWithData(
	ctx sdk.Context,
	from common.Address,
	contract *common.Address,
	data []byte,
	commit bool,
    gasCap *big.Int,
) (*types.MsgEthereumTxResponse, error)
```

For use with EVM `recvPacket callbacks, the message fields above can be derived from the following:

- `Sender`: IBC packet senders cannot be explicitly trusted, as they can be deceitful. Chains cannot
risk the sender being confused with a particular local user or module address. To prevent this, the
`sender` is replaced with an account that represents the sender prefixed by the channel and a VM module
prefix. This is done by setting the sender to `address.Module(ModuleName, channelId, sender)`, where the
`channelId` is the channel id on the destination chain.
- `Contract`: This field should be directly obtained from the ICS-20 packet metadata
- `Data`: This field should be directly obtained from the ICS-20 packet metadata.
- `commit`: true
- `gasCap`: IBC callbacks gas limit

<!-- markdown-link-check-disable-next-line -->
> ***WARNING:***  Due to a [bug](https://twitter.com/SCVSecurity/status/1682329758020022272) in the
> packet forward middleware, we cannot trust the sender from chains that use PFM. Until that is fixed,
> we recommend chains to not trust the sender on contracts executed via IBC callbacks.

### ICS20 packet structure

Given the details above, you can propagate the implied ICS-20 packet data structure.
ICS20 is JSON native, so you can use JSON for the memo format.

```json
{
    //... other ibc fields that we don't care about
    "data":{
    	"denom": "denom on counterparty chain (e.g. uatom)",  // will be transformed to the local denom (ibc/...)
        "amount": "1000",
        "sender": "addr on counterparty chain", // will be transformed
        "receiver": "isolated receiver address for sender",
    	"memo": {
           "dest_callback": {
              "address": "evmContractAddress",
              "gas_limit": "1000000",
              "calldata": "{abipacked_contract_calldata}",
            }
        }
    }
}
```

An ICS-20 packet is formatted correctly for EVM callbacks on destination chain if all of the following are true:

- The `memo` is not blank.
- The`memo` is valid JSON.
- The `memo` has at least one key, with the value `"dest_callback"`.
- The `memo["dest_callback"]` has these three entries, `"address"`, `"gas_limit"` and `"calldata"`.
- The `receiver == "isolated_receiver_address"` as defined:
`sdkaddress.Module("ibc-callbacks", packet.destChannelId, packet.sender)`

An ICS-20 packet is directed toward EVM callbacks if all of the following are true:

- The `memo` is not blank.
- The `memo` is valid JSON.
- The `memo` has at least one key, with the name `"dest_callback"`.

If an ICS-20 packet is not directed towards EVM callbacks, EVM callbacks doesn't do anything.
If an ICS-20 packet is directed towards EVM callbacks, and is formatted incorrectly, then EVM
callbacks returns an error and the recv packet application state changes are reverted and an
error acknowledgement is returned.

### Execution flow

1. Pre-EVM Callbacks:

- Core IBC TAO checks on RecvPacket are executed (e.g. timeout, replay checks)

2. In EVM callbacks, pre-packet execution:

- Ensure the packet is correctly formatted (as defined above).
- Ensure the receiver is correctly set to isolated address.

3. In EVM callbacks, post packet execution:

- Execute the EVM call on requested EVM contract
- If the EVM call returns an error, return `ErrAck`.
- Otherwise, continue through middleware.

## Ack and Timeout callbacks

A contract that sends an IBC transfer may need to listen for the outcome of the packet lifecyle.
`Ack`and `Timeout` callbacks allow
contracts to execute custom logic on the basis of how the packet lifecyle completes.

### Design

The sender of an IBC transfer packet may specify a contract to be called when the packet lifecycle completes.
This contract **must** implement the expected entrypoints for `onAcknowledgePacket` and `onTimeoutPacket`.

Crucially, **only the IBC packet sender can set the callback**.

### Use case

The cross-chain swaps implementation sends an IBC transfer. If the transfer were to fail, the sender should
be able to retrieve their funds which would otherwise be stuck in the contract. A contract may also wish to
retry sending the packet. In order to do either, the contract must receive the acknowledgement and timeout
callback to understand what occured in the packet lifecyle.

### Implementation

#### Callback information in memo

For the callback to be processed, the transfer packet's `memo` should contain the following in its JSON:

```json
"memo": {
    "src_callback": {
        "address": "evm_contract_addr",
        "gas_limit": "1000000",
    }
}
```

NOTE: For the source callbacks, the calldata **must** be empty since we do not support custom calldata and
instead expect to call a specific entrypoint with the packet information and acknowledgement.

#### Interface for receiving the Acks and Timeouts

The contract that awaits the callback should implement the following interface defined in the
[precompile directory](../../../precompiles/callbacks/ICallbacks.sol):

```solidity
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
```

## Limitations

The receiver side callback **must** receive funds to an ephemeral address generated from the channelId and packet
sender address. Note that since this is a generated address, no user has the ability to sign messages on behalf of
this account even though it is a cross-chain representation of the packet sender.

Thus, a contract that receives the funds and calldata from the isolated receiver address **must** send the tokens
onwards to a desired address that is specified in the calldata. If tokens are deposited back into the isolated address,
they are unreachabe. If you wish to interact with a contract that does not implement functionality for sending the
tokens to a different address then you must interact with that contract through some wrapper contract interface that
can receive the funds, call the contract which deposits funds back to `msg.sender` and then the wrapper contract
can move the funds to a final desired address.

## Acknowledgements

This README is heavily inspired from the ibc-hooks
[README](https://github.com/cosmos/ibc-apps/tree/main/modules/ibc-hooks).
