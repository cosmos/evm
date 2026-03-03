// SPDX-License-Identifier: MIT
pragma solidity ^0.8.0;

interface IWERC20 {
    function transfer(address to, uint256 amount) external returns (bool);
    function transferFrom(address from, address to, uint256 amount) external returns (bool);
    function approve(address spender, uint256 amount) external returns (bool);
    function balanceOf(address account) external view returns (uint256);
    function deposit() external payable;
}

/// @notice Interface with a view function that will be called via STATICCALL.
interface ICallback {
    function callback() external view returns (string memory);
}

/// @notice Contract that attempts state changes when called via a view interface.
/// When ViewCaller calls ICallback(target).callback(), Solidity uses STATICCALL because
/// the ICallback interface declares callback() as view. Inside that static context, this
/// contract attempts to call werc20.transfer().
///
/// KEY: The interface declares view, but this implementation does NOT.
/// This compiles fine, but when called via the interface, STATICCALL is used.
contract StaticCallbackTarget {
    IWERC20 public werc20;
    address public transferTarget;
    uint256 public transferAmount;

    constructor(address _werc20) {
        werc20 = IWERC20(_werc20);
    }

    function setTransferParams(address target, uint256 amount) external {
        transferTarget = target;
        transferAmount = amount;
    }

    /// @notice NOT marked as view here, but ICallback interface declares it as view.
    /// When called via ICallback(this).callback(), the caller uses STATICCALL.
    /// Inside that static context, we attempt a state-changing precompile call.
    function callback() external returns (string memory) {
        // Attempt transfer inside static context - use low-level call to capture success.
        // Note: We use a regular CALL here (not staticcall) to test if the EVM
        // correctly propagates the static flag to precompile calls.
        (bool success, ) = address(werc20).call(
            abi.encodeWithSelector(IWERC20.transfer.selector, transferTarget, transferAmount)
        );

        if (success) {
            return "TRANSFER_SUCCEEDED";
        }
        return "TRANSFER_FAILED";
    }
}

/// @notice Contract that invokes a view function on another contract.
contract ViewCaller {
    /// @notice Calls callback() on the target using STATICCALL (implicit because it's view).
    /// Returns the result string from the view call.
    function callViewFunction(address target) external view returns (string memory) {
        return ICallback(target).callback();
    }

    /// @notice Non-view version that executes the staticcall and returns result.
    function callViewAndReturn(address target) external returns (string memory) {
        return ICallback(target).callback();
    }
}

/// @notice Simple test contract for basic STATICCALL + SSTORE protection.
contract StaticCallTester {
    IWERC20 public werc20;
    uint256 public counter;

    constructor(address _werc20) {
        werc20 = IWERC20(_werc20);
    }

    /// @notice Direct WERC20 transfer (non-static context).
    function nonStaticPrecompileTransfer(address to, uint256 amount) external returns (bool) {
        return werc20.transfer(to, amount);
    }

    /// @notice Helper called via staticcall by staticPrecompileTransfer.
    function doPrecompileTransfer(address to, uint256 amount) external returns (bool) {
        return werc20.transfer(to, amount);
    }

    /// @notice Attempts WERC20 transfer inside a STATICCALL context.
    function staticPrecompileTransfer(address to, uint256 amount) external returns (bool) {
        (bool success, ) = address(this).staticcall(
            abi.encodeWithSelector(this.doPrecompileTransfer.selector, to, amount)
        );
        return success;
    }

    /// @notice Direct counter increment (non-static context).
    function nonStaticIncrement() external returns (uint256) {
        counter++;
        return counter;
    }

    /// @notice Helper called via staticcall by staticIncrement.
    function doIncrement() external returns (uint256) {
        counter++;
        return counter;
    }

    /// @notice Attempts counter increment inside a STATICCALL context.
    function staticIncrement() external returns (bool) {
        (bool success, ) = address(this).staticcall(
            abi.encodeWithSelector(this.doIncrement.selector)
        );
        return success;
    }

    function getCounter() external view returns (uint256) {
        return counter;
    }
}
