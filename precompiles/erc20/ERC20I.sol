// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/token/ERC20/extensions/IERC20Metadata.sol";
import "@openzeppelin/contracts/interfaces/draft-IERC6093.sol";

import "../common/Types.sol";

/// @dev Cosmos ERC-20 precompile: OpenZeppelin IERC20 (via IERC20Metadata) and metadata,
/// ERC-6093 IERC20Errors, shared IPrecompile errors, and native-value rejection.
interface ERC20I is IERC20Metadata, IERC20Errors, IPrecompile {
    error ERC20CannotReceiveFunds(uint256 value);
}
