// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.18;

import "./IERC20MetadataAllowance.sol";

/**
 * @author Evmos Team
 * @title ERC20 Precompile Interface
 * @dev Interface for the ERC20 precompile contract with additional query methods.
 */
interface IERC20Precompile is IERC20MetadataAllowance {
    /** @dev Queries the ERC20 module parameters.
      * @return enableErc20 Whether ERC20 conversion is enabled
      * @return nativePrecompiles Array of native precompile addresses
      * @return dynamicPrecompiles Array of dynamic precompile addresses
      * @return permissionlessRegistration Whether permissionless registration is allowed
    */
    function getParams()
        external
        view
        returns (
            bool enableErc20,
            address[] memory nativePrecompiles,
            address[] memory dynamicPrecompiles,
            bool permissionlessRegistration
        );
}