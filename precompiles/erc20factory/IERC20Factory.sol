// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

/**
 * @dev The ERC20 Factory contract's address.
 */
address constant ERC20_FACTORY_PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000000900;

/**
 * @dev The ERC20 Factory contract's instance.
 */
IERC20Factory constant ERC20_FACTORY_CONTRACT = IERC20Factory(ERC20_FACTORY_PRECOMPILE_ADDRESS);

interface IERC20Factory {
    /**
     * @dev Defines a method for creating an ERC20 token.
     * @param tokenPairType Token Pair type
     * @param salt Salt used for deployment
     * @param name The name of the token.
     * @param symbol The symbol of the token.
     * @param decimals the decimals of the token.
     * @return tokenAddress The ERC20 token address.
     */
    function create(
        uint8 tokenPairType,
        bytes32 salt,
        string memory name,
        string memory symbol,
        uint8 decimals,
        address minter,
        uint256 premintedSupply
    ) external returns (address tokenAddress);

    /**
     * @dev Calculates the deterministic address for a new token.
     * @param tokenPairType Token Pair type
     * @param salt Salt used for deployment
     * @return tokenAddress The calculated ERC20 token address.
     */
    function calculateAddress(uint8 tokenPairType, bytes32 salt) external view returns (address tokenAddress);
}
