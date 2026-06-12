// SPDX-License-Identifier: MIT
pragma solidity ^0.8.15;

import "forge-std/Test.sol";
import "../src/SimpleERC20.sol";

contract SimpleERC20Test is Test {
    SimpleERC20 token;
    address alice = address(0x1);
    address bob   = address(0x2);

    function setUp() public {
        // Make Alice the deployer of the token contract
        vm.prank(alice);
        token = new SimpleERC20(1000 ether);
    }

    function testTotalSupply() public {
        // From Alice’s perspective, verify total supply and her balance
        vm.prank(alice);
        assertEq(token.totalSupply(), 1000 ether);
        assertEq(token.balanceOf(alice), 1000 ether);
    }

    function testTransfer() public {
        // Have Alice transfer 100 tokens to Bob
        vm.prank(alice);
        token.transfer(bob, 100 ether);

        // Verify Bob’s and Alice’s balances after transfer
        assertEq(token.balanceOf(bob), 100 ether);
        assertEq(token.balanceOf(alice), 900 ether);
    }
}