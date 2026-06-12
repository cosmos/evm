// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

contract WATOM {
    string public name = "Wrapped Atom";
    string public symbol = "WATOM";
    uint8 public decimals = 18;

    mapping(address => uint256) public balanceOf;

    event Deposit(address indexed dst, uint256 amount);
    event Withdrawal(address indexed src, uint256 amount);
    event Transfer(address indexed src, address indexed dst, uint256 amount);

    // Receive ether and wrap it into ATOM
    receive() external payable {
        deposit();
    }

    function deposit() public payable {
        balanceOf[msg.sender] += msg.value;
        emit Deposit(msg.sender, msg.value);
    }

    function withdraw(uint256 amount) public {
        require(balanceOf[msg.sender] >= amount, "insufficient balance");
        balanceOf[msg.sender] -= amount;
        (bool success, ) = payable(msg.sender).call{value: amount}("");
        require(success, "failed to withdraw to sender");
        emit Withdrawal(msg.sender, amount);
    }

    function transfer(address to, uint256 amount) public returns (bool) {
        require(balanceOf[msg.sender] >= amount, "insufficient balance");
        balanceOf[msg.sender] -= amount;
        balanceOf[to] += amount;
        emit Transfer(msg.sender, to, amount);
        return true;
    }
}
