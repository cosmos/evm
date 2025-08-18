// SPDX-License-Identifier: UNLICENSED
pragma solidity ^0.8.28;

// Uncomment this line to use console.log
// import "hardhat/console.sol";
import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract TokenExample is ERC20, Ownable{
    
    constructor() ERC20("Example","EXP") Ownable(msg.sender){
    }

    function mint(address to, uint256 amount) external onlyOwner {
        _mint(to, amount);
    }
}
