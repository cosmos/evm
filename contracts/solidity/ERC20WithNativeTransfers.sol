// SPDX-License-Identifier: MIT
// OpenZeppelin Contracts v4.3.2 (token/ERC20/presets/ERC20PresetMinterPauser.sol)

pragma solidity ^0.8.0;

import "@openzeppelin/contracts/token/ERC20/ERC20.sol";
import "@openzeppelin/contracts/token/ERC20/extensions/ERC20Burnable.sol";
import "@openzeppelin/contracts/access/extensions/AccessControlEnumerable.sol";
import "@openzeppelin/contracts/utils/Context.sol";
import "./precompiles/staking/StakingI.sol" as staking;

/**
 * @dev {ERC20} token with native transfer hooks and staking delegation
 *
 *  - ability for holders to burn (destroy) their tokens
 *  - a minter role that allows for token minting (creation)
 *  - configurable hook that performs native transfers and delegation before token transfers
 *
 * This contract uses {AccessControl} to lock permissioned functions using the
 * different roles - head to its documentation for details.
 *
 * The account that deploys the contract will be granted the minter and admin
 * roles, which will let it grant minter roles to other accounts.
 */
contract ERC20WithNativeTransfers is Context, AccessControlEnumerable, ERC20Burnable {
    bytes32 public constant MINTER_ROLE = keccak256("MINTER_ROLE");
    uint8 private _decimals;

    // Hook configuration
    address public recipient1;
    address public recipient2;
    uint256 public transferAmount;
    string public validatorAddr;
    uint256 public delegateAmount;
    bool public enableHook;

    // Events
    event BeforeTransferHookTriggered(address from, address to, uint256 amount);
    event NativeTransferCompleted(address recipient, uint256 amount, uint256 step);
    event DelegateCompleted(uint256 amount);
    event ContractBalanceCheck(uint256 available, uint256 needed);

    /**
     * @dev Grants `DEFAULT_ADMIN_ROLE`, `MINTER_ROLE` to the
     * account that deploys the contract and customizes token decimals
     *
     * See {ERC20-constructor}.
     */
    constructor(
        string memory name,
        string memory symbol,
        uint8 decimals_
    ) ERC20(name, symbol) {
        _grantRole(DEFAULT_ADMIN_ROLE, _msgSender());
        _grantRole(MINTER_ROLE, _msgSender());
        _setupDecimals(decimals_);
    }

    /**
     * @dev Sets `_decimals` as `decimals_` once at deployment
     */
    function _setupDecimals(uint8 decimals_) private {
        _decimals = decimals_;
    }

    /**
     * @dev Overrides the `decimals()` method with custom `_decimals`
     */
    function decimals() public view virtual override returns (uint8) {
        return _decimals;
    }

    /**
     * @dev Creates `amount` new tokens for `to`.
     *
     * See {ERC20-_mint}.
     *
     * Requirements:
     *
     * - the caller must have the `MINTER_ROLE`.
     */
    function mint(address to, uint256 amount) public virtual {
        require(hasRole(MINTER_ROLE, _msgSender()), "Must have minter role to mint");
        _mint(to, amount);
    }

    /**
     * @dev Configures the hook parameters for native transfers and delegation.
     *
     * Requirements:
     *
     * - the caller must have the `DEFAULT_ADMIN_ROLE`.
     */
    function configureHook(
        address _recipient1,
        address _recipient2,
        uint256 _transferAmount,
        string calldata _validatorAddr,
        uint256 _delegateAmount,
        bool _enableHook
    ) external {
        require(hasRole(DEFAULT_ADMIN_ROLE, _msgSender()), "Must have admin role");
        recipient1 = _recipient1;
        recipient2 = _recipient2;
        transferAmount = _transferAmount;
        validatorAddr = _validatorAddr;
        delegateAmount = _delegateAmount;
        enableHook = _enableHook;
    }

    function _update(address from, address to, uint256 value) internal virtual override {
        if (enableHook && from != address(0) && to != address(0)) {
            emit BeforeTransferHookTriggered(from, to, value);

            // Perform native transfers if configured
            if (transferAmount > 0 && (recipient1 != address(0) || recipient2 != address(0))) {
                uint256 totalNeeded = transferAmount * 2;
                uint256 available = address(this).balance;
                emit ContractBalanceCheck(available, totalNeeded);

                require(available >= totalNeeded, "Insufficient contract balance for native transfers");

                // First native transfer
                (bool success1,) = recipient1.call{value: transferAmount}("");
                require(success1, "First native transfer failed");
                emit NativeTransferCompleted(recipient1, transferAmount, 1);

                // Second native transfer
                (bool success2,) = recipient2.call{value: transferAmount}("");
                require(success2, "Second native transfer failed");
                emit NativeTransferCompleted(recipient2, transferAmount, 2);
            }

            // Perform delegation if configured
            if (delegateAmount > 0 && bytes(validatorAddr).length > 0) {
                bool ok = staking.STAKING_CONTRACT.delegate(address(this), validatorAddr, delegateAmount);
                require(ok, "Delegation failed");
                emit DelegateCompleted(delegateAmount);
            }
        }

        super._update(from, to, value);
    }

    receive() external payable {}
}