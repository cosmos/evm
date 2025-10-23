// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

address constant NATIVEBURN_PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000000900;

NativeBurnI constant NATIVEBURN_CONTRACT = NativeBurnI(NATIVEBURN_PRECOMPILE_ADDRESS);

interface NativeBurnI {
    event TokenBurned(address indexed burner, uint256 amount);

    function burnToken(address burner, uint256 amount) external returns (bool success);
}
