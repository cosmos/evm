// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/**
 * @title DelegationTarget
 * @notice Minimal contract for EIP-7702 delegation that allows deploying other contracts
 * @dev This contract can be set as a delegation target for an EOA via EIP-7702
 */
contract DelegationTarget {
    // Storage slot where the counter (last offset) is stored
    uint256 constant COUNTER_SLOT = 0xffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff;

    event ContractDeployed(address indexed deployed, bytes32 salt);
    event ContractDestroyed(address indexed destroyed, uint256 index, bool success);
    event CodeHash(address indexed deployed, bytes32 codeHash);

    /**
     * @notice Deploy a contract using CREATE
     * @param bytecode The bytecode of the contract to deploy
     * @return deployed The address of the deployed contract
     * @dev Stores deployed addresses at slots 0, 1, 2, ... and counter at COUNTER_SLOT
     */
    function deploy(bytes memory bytecode) external returns (address deployed) {
        assembly {
            deployed := create(0, add(bytecode, 0x20), mload(bytecode))
            // Load current counter from COUNTER_SLOT
            let currentSlot := sload(COUNTER_SLOT)
            // Store deployed address at slot currentSlot (0, 1, 2, ...)
            sstore(currentSlot, deployed)
            // Increment counter and store back
            sstore(COUNTER_SLOT, add(currentSlot, 1))
        }
        require(deployed != address(0), "Deployment failed");
        emit ContractDeployed(deployed, bytes32(0));
    }

    /**
     * @notice Get the deployed address at a given index
     * @param index The index (slot) to read from
     * @return addr The deployed address
     */
    function getDeployedAt(uint256 index) external view returns (address addr) {
        assembly {
            addr := sload(index)
        }
    }

    /**
     * @notice Get the current counter (number of deployments)
     * @return count The number of deployed contracts
     */
    function getDeploymentCount() external view returns (uint256 count) {
        assembly {
            count := sload(COUNTER_SLOT)
        }
    }

    /**
     * @notice Selfdestruct all deployed contracts
     * @dev Calls destroy() on each deployed contract. Contracts must implement destroy() with selfdestruct
     */
    function selfdestructAll() external {
        uint256 count;
        assembly {
            count := sload(COUNTER_SLOT)
        }

        for (uint256 i = 0; i < count; i++) {
            address target;
            assembly {
                target := sload(i)
            }
            // Call destroy() selector: 0x83197ef0
            (bool success,) = target.call(abi.encodeWithSelector(MinimalContract.selfDestruct.selector));
            emit ContractDestroyed(target, i, success);
        }
    }

    function emitAllCodeHashes() external {
        uint256 count;
        assembly {
            count := sload(COUNTER_SLOT)
        }

        for (uint256 i = 0; i < count; i++) {
            address target;
            assembly {
                target := sload(i)
            }
            bytes32 codeHash;
            assembly {
                codeHash := extcodehash(target)
            }
            emit CodeHash(target, codeHash);
        }
    }

    /**
     * @notice Execute arbitrary call (useful for delegated EOA)
     * @param target The target address
     * @param data The calldata
     * @return success Whether the call succeeded
     * @return result The return data
     */
    function execute(address target, bytes calldata data) external payable returns (bool success, bytes memory result) {
        (success, result) = target.call{value: msg.value}(data);
    }

    receive() external payable {}
}




/**
 * @title MinimalContract
 * @notice A minimal contract that gets deployed by MaliciousDeployer
 */
contract MinimalContract {
    address public immutable creator;
    uint256 public value;

    constructor() {
        creator = msg.sender;
    }

    function setValue(uint256 _value) external {
        value = _value;
    }

    function selfDestruct() external {
        selfdestruct(payable(creator));
    }
}

/**
 * @title FinalContract
 * @notice The final contract that gets deployed by MaliciousDeployer
 */
contract FinalContract {
    address public immutable creator;
    uint256 public value;

    constructor() {
        creator = msg.sender;
    }

    function singletonFunction() external pure returns (string memory) {
        return "I am the final contract!";
    }
}