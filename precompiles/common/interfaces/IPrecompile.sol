// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

/// @dev Shared protocol, transport, and Cosmos SDK errors inherited by precompile interfaces.
interface IPrecompile {
    error RequesterIsNotMsgSender(address msgSender, address requester);
    error InvalidAddress(string bad);
    error InvalidAmount(string amount);
    error InvalidHeight(string height);
    error InvalidPubkey(string pubkey);
    error InvalidPubkeySize(uint256 got, uint256 expected);
    error ABISetupFailed(string reason);
    error InvalidNumberOfArgs(uint256 expected, uint256 got);
    error UnknownMethod(string methodName);
    error QueryFailed(string queryMethod, string reason);
    error MsgServerFailed(string msgMethod, string reason);
    error EventEmitFailed(string eventKind, string reason);

    error SDKUnauthorized();
    error SDKInsufficientFunds();
    error SDKInvalidAddress();
    error SDKInvalidCoins();
    error SDKInvalidRequest();
    error SDKInvalidType();
    error SDKNotFound();
    error UnmappedCosmosError(string codespace, uint32 code);
}
