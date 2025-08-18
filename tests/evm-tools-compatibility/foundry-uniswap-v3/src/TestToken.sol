pragma solidity ^0.7.6;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/access/Ownable.sol";

contract TestToken is ERC20, Ownable {

    constructor(string memory name_, string memory symbol_) 
        ERC20(name_, symbol_) 
        Ownable() 
    {}

    function mint(address to, uint256 amount) external onlyOwner {
        _mint(to, amount);
    }
}