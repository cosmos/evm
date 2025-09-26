// SPDX-License-Identifier: MIT

pragma solidity ^0.8.0;

import "@account-abstraction/contracts/interfaces/IAccount.sol";
import "@account-abstraction/contracts/interfaces/UserOperation.sol";

contract SimpleEntryPoint {
    event UserOperationEvent(bytes32 indexed userOpHash, address indexed sender, bool success);

    function handleOps(UserOperation[] calldata ops) external {
        for (uint i = 0; i < ops.length; i++) {
            UserOperation calldata op = ops[i];

            bytes32 userOpHash = _getUserOpHash(op);
            
            try IAccount(op.sender).validateUserOp(op, userOpHash, 0) {
                (bool success, ) = address(op.sender).call(op.callData);
                emit UserOperationEvent(userOpHash, op.sender, success);
            } catch {
                emit UserOperationEvent(userOpHash, op.sender, false);
            }
        }
    }

    function _getUserOpHash(UserOperation calldata op) internal view returns (bytes32) {
        UserOperation memory mOp = op;

        bytes32 initCodeHash = keccak256(mOp.initCode);
        bytes32 callDataHash = keccak256(mOp.callData);
        bytes32 paymasterAndDataHash = keccak256(mOp.paymasterAndData);
        address entryPoint = address(this);
        uint256 chainId = block.chainid;

        return keccak256(
            abi.encode(
                mOp.sender,
                mOp.nonce,
                initCodeHash,
                callDataHash,
                mOp.callGasLimit,
                mOp.verificationGasLimit,
                mOp.preVerificationGas,
                mOp.maxFeePerGas,
                mOp.maxPriorityFeePerGas,
                paymasterAndDataHash,
                entryPoint,
                chainId
            )
        );
    }
}
