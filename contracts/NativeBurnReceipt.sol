// SPDX-License-Identifier: MIT
pragma solidity ^0.8.17;

// Simplified ERC20 implementation
contract ERC20 {
    string public name;
    string public symbol;
    uint8 public decimals = 18;
    uint256 public totalSupply;

    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    constructor(string memory _name, string memory _symbol) {
        name = _name;
        symbol = _symbol;
    }

    function _mint(address to, uint256 amount) internal {
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
    }
}

// NativeBurn precompile interface
interface NativeBurnI {
    function burnToken(address burner, uint256 amount) external returns (bool success);
}

contract NativeBurnReceipt is ERC20 {
    NativeBurnI constant NATIVEBURN_CONTRACT = NativeBurnI(0x0000000000000000000000000000000000000900);
    uint256 public totalBurned;
    address public owner;

    event TokensBurned(address indexed burner, uint256 amount, uint256 receiptTokens);

    constructor() ERC20("NativeBurn Receipt", "BURN") {
        owner = msg.sender;
    }

    function burn(uint256 amount) external {
        require(amount > 0, "Must burn positive amount");

        bool success = NATIVEBURN_CONTRACT.burnToken(msg.sender, amount);
        require(success, "Burn failed");

        _mint(msg.sender, amount);
        totalBurned += amount;

        emit TokensBurned(msg.sender, amount, amount);
    }

    function getBurnedAmount(address account) external view returns (uint256) {
        return balanceOf[account];
    }
}
