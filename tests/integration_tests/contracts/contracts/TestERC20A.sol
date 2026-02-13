// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.4;
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";

contract TestERC20A is ERC20 {
    constructor() ERC20("Bitcoin MAX", "MAX") {
        _mint(msg.sender, 100000000000000000000000000);
    }

    function test_log0() public {
        bytes32 data = "hello world";
        assembly {
            let p := mload(0x20)
            mstore(p, data)
            log0(p, 0x20)
        }
    }
}
