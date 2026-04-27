// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.18;

import "../IBank.sol";

contract BankCaller {

    function callBalances(address account) external view returns (Balance[] memory balances) {
        return IBANK_CONTRACT.balances(account);
    }

    function callTotalSupply() external view returns (Balance[] memory totalSupply) {
        return IBANK_CONTRACT.totalSupply();
    }

    function callSupplyOf(address erc20Address) external view returns (uint256) {
        return IBANK_CONTRACT.supplyOf(erc20Address);
    }

    // Calls totalSupply with explicit gas forwarding and measures the gas consumed
    // by the inner call. Returns whether the call succeeded and the actual gas used.
    function callTotalSupplyWithGas(uint256 gasForward) external view returns (bool success, uint256 innerGasUsed) {
        uint256 gasBefore = gasleft();
        (success, ) = IBANK_PRECOMPILE_ADDRESS.staticcall{gas: gasForward}(
            abi.encodeWithSelector(IBank.totalSupply.selector)
        );
        innerGasUsed = gasBefore - gasleft();
    }
}