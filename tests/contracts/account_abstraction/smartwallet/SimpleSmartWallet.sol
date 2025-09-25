// SPDX-License-Identifier: MIT

pragma solidity ^0.8.0;

import "@account-abstraction/contracts/interfaces/IAccount.sol";
import "@account-abstraction/contracts/core/EntryPoint.sol";

contract SimpleSmartWallet is IAccount {
    address public owner;
    EntryPoint public entryPoint;

    function initialize(address _owner, EntryPoint _entryPoint) external {
        require(owner == address(0), "already initialized");
        owner = _owner;
        entryPoint = _entryPoint;
    }

    function validateUserOp(
        UserOperation calldata userOp,
        bytes32 userOpHash,
        uint256 /* missingAccountFunds */
    ) external view override returns (uint256 validationData) {
        require(msg.sender == address(entryPoint), "only EntryPoint");

        (uint8 v, bytes32 r, bytes32 s) = _split(userOp.signature);
        address recovered = ecrecover(userOpHash, v, r, s);
        require(recovered == owner, "Invalid signature");

        return 0;
    }

    function execute(address target, uint256 value, bytes calldata data) external {
        require(msg.sender == address(entryPoint), "only EntryPoint");
        (bool success, ) = target.call{value: value}(data);
        require(success, "Execution failed");
    }

    function _split(bytes memory sig) internal pure returns (uint8 v, bytes32 r, bytes32 s) {
        require(sig.length == 65, "invalid signature length");
        assembly {
            r := mload(add(sig, 32))
            s := mload(add(sig, 64))
            v := byte(0, mload(add(sig, 96)))
        }
    }

    receive() external payable {}
}
