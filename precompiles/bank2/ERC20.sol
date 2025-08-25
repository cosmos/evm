// SPDX-License-Identifier: MIT

pragma solidity ^0.8.0;

library BankPrecompile {
    error BankError(bytes);

    enum BankMethod {
        NAME,
        SYMBOL,
        DECIMALS,
        TOTAL_SUPPLY,
        BALANCE_OF,
        TRANSFER_FROM
    }

    function name(address bank, string memory denom) internal view returns (string memory) {
        bytes memory result = _staticcall_bank(bank, abi.encodePacked(uint8(BankMethod.NAME), denom));
        return string(result);
    }

    function symbol(address bank, string memory denom) internal view returns (string memory) {
        bytes memory result = _staticcall_bank(bank, abi.encodePacked(uint8(BankMethod.SYMBOL), denom));
        return string(result);
    }

    function decimals(address bank, string memory denom) internal view returns (uint8) {
        bytes memory data = _staticcall_bank(bank, abi.encodePacked(uint8(BankMethod.DECIMALS), denom));

        uint8 result;
        assembly {
            result := byte(0, mload(add(data, 0x20)))
        }
        return result;
    }

    function totalSupply(address bank, string memory denom) internal view returns (uint256) {
        bytes memory data = _staticcall_bank(bank, abi.encodePacked(uint8(BankMethod.TOTAL_SUPPLY), denom));

        uint256 result;
        assembly {
            result := mload(add(data, 0x20))
        }
        return result;
    }

    function balanceOf(address bank, address account, string memory denom) internal view returns (uint256) {
        bytes memory data = _staticcall_bank(bank, abi.encodePacked(uint8(BankMethod.BALANCE_OF), account, denom));

        uint256 result;
        assembly {
            result := mload(add(data, 0x20))
        }
        return result;
    }

    function transferFrom(address bank, address from, address to, uint256 amount, string memory denom) internal returns (bool) {
        _call_bank(bank, abi.encodePacked(uint8(BankMethod.TRANSFER_FROM), from, to, amount, denom));
        return true;
    }

    function _staticcall_bank(address bank, bytes memory _calldata) internal view returns (bytes memory) {
        (bool success, bytes memory data) = bank.staticcall(_calldata);
        if (!success) {
            revert BankError(data);
        }

        return data;
    }

    function _call_bank(address bank, bytes memory _calldata) internal returns (bytes memory) {
         (bool success, bytes memory data) = bank.call(_calldata);
         if (!success) {
             revert BankError(data);
         }

         return data;
     }
}

interface IERC20 {
    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);
    function totalSupply() external view returns (uint256);
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 value) external returns (bool);
    function allowance(address owner, address spender) external view returns (uint256);
    function approve(address spender, uint256 value) external returns (bool);
    function transferFrom(address from, address to, uint256 value) external returns (bool);
}

interface IERC20Metadata is IERC20 {
    function name() external view returns (string memory);
    function symbol() external view returns (string memory);
    function decimals() external view returns (uint8);
}

interface IERC20Errors {
    error ERC20InvalidSender(address sender);
    error ERC20InvalidReceiver(address receiver);
    error ERC20InsufficientAllowance(address spender, uint256 allowance, uint256 needed);
    error ERC20InvalidApprover(address approver);
    error ERC20InvalidSpender(address spender);
}

contract ERC20 is IERC20, IERC20Metadata, IERC20Errors {
    using BankPrecompile for address;

    string public denom;
    mapping(address account => mapping(address spender => uint256)) public allowance;

    address public immutable bank;

    constructor(string memory denom_, address bank_) {
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
