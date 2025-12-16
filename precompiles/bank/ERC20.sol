// SPDX-License-Identifier: MIT

pragma solidity ^0.8.0;

interface IBankPrecompile {
    // queries
    function name(string memory denom) external view returns (string memory);
    function symbol(string memory denom) external view returns (string memory);
    function decimals(string memory denom) external view returns (uint8);
    function totalSupply(string memory denom) external view returns (uint256);
    function balanceOf(address account, string memory denom) external view returns (uint256);

    function transferFrom(address from, address to, uint256 value, string memory denom) external returns (bool);
}

contract ERC20 {
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    error ERC20InvalidSender(address sender);
    error ERC20InvalidReceiver(address receiver);
    error ERC20InsufficientAllowance(address spender, uint256 allowance, uint256 needed);
    error ERC20InvalidApprover(address approver);
    error ERC20InvalidSpender(address spender);


    string public denom;
    mapping(address account => mapping(address spender => uint256)) public allowance;

    IBankPrecompile public immutable bank;

    constructor(string memory denom_, IBankPrecompile bank_) {
        denom = denom_;
        bank = bank_;
    }

    function name() public view returns (string memory) {
        return bank.name(denom);
    }

    function symbol() public view returns (string memory) {
        return bank.symbol(denom);
    }

    function decimals() public view returns (uint8) {
        return bank.decimals(denom);
    }

    function totalSupply() public view returns (uint256) {
        return bank.totalSupply(denom);
    }

    function balanceOf(address account) public view returns (uint256) {
        return bank.balanceOf(account, denom);
    }

    function transfer(address to, uint256 value) public returns (bool) {
        _transfer(msg.sender, to, value);
        return true;
    }

    function approve(address spender, uint256 value) public returns (bool) {
        _approve(msg.sender, spender, value);
        return true;
    }

    function transferFrom(address from, address to, uint256 value) public returns (bool) {
        address spender = msg.sender;
        _spendAllowance(from, spender, value);
        _transfer(from, to, value);
        return true;
    }

    function _transfer(address from, address to, uint256 value) internal {
        if (from == address(0)) {
            revert ERC20InvalidSender(address(0));
        }
        if (to == address(0)) {
            revert ERC20InvalidReceiver(address(0));
        }

        bank.transferFrom(from, to, value, denom);
        emit Transfer(from, to, value);
    }

    function _approve(address owner, address spender, uint256 value) internal {
        _approve(owner, spender, value, true);
    }

    function _approve(address owner, address spender, uint256 value, bool emitEvent) internal virtual {
        if (owner == address(0)) {
            revert ERC20InvalidApprover(address(0));
        }
        if (spender == address(0)) {
            revert ERC20InvalidSpender(address(0));
        }
        allowance[owner][spender] = value;
        if (emitEvent) {
            emit Approval(owner, spender, value);
        }
    }

    function _spendAllowance(address owner, address spender, uint256 value) internal virtual {
        uint256 currentAllowance = allowance[owner][spender];
        if (currentAllowance < type(uint256).max) {
            if (currentAllowance < value) {
                revert ERC20InsufficientAllowance(spender, currentAllowance, value);
            }
            unchecked {
                _approve(owner, spender, currentAllowance - value, false);
            }
        }
    }
}
