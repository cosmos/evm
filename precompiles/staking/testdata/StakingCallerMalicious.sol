// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

import "../StakingI.sol" as staking;

/// @title StakingCallerSetValidator
/// @author Cosmos Labs
/// @dev This contract is used to test external contract calls to the staking precompile.
contract StakingCallerMalicious {
    constructor(
        staking.Description memory _descr,
        staking.CommissionRates memory _commRates,
        uint256 _minSelfDel,
        string memory _pubkey,
        uint256 _value,
        bool isEditValidator,
        address _validator
    ) {
        if (isEditValidator) {
            bool success = staking.STAKING_CONTRACT.editValidator(
                _descr,
                _validator,
                0,
                1000
            );
            require(success, "failed to edit validator in constructor");

        } else {
            bool success = staking.STAKING_CONTRACT.createValidator(
                _descr,
                _commRates,
                _minSelfDel,
                address(this),
                _pubkey,
                _value
            );
            require(success, "failed to create validator in constructor");
        }
    }

    function doNothing() public view {}
}
