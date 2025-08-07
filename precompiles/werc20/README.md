# WERC20 Precompile

The WERC20 (Wrapped ERC20) precompile provides a wrapped ERC20 interface for the native EVM token,
similar to WETH on Ethereum. This allows the native token to be used in smart contracts that expect
ERC20 tokens, maintaining compatibility with DeFi protocols and other ERC20-based applications.

## Interface

The WERC20 precompile extends the standard ERC20 interface with additional deposit and withdraw functionality:

### Inherited ERC20 Methods

All standard ERC20 and ERC20Metadata methods are available:

```solidity
// ERC20 Standard Methods
function totalSupply() external view returns (uint256);
function balanceOf(address account) external view returns (uint256);
function transfer(address to, uint256 amount) external returns (bool);
function allowance(address owner, address spender) external view returns (uint256);
function approve(address spender, uint256 amount) external returns (bool);
function transferFrom(address from, address to, uint256 amount) external returns (bool);

// ERC20 Metadata
function name() external view returns (string memory);
function symbol() external view returns (string memory);
function decimals() external view returns (uint8);
```

### WERC20 Specific Methods

```solidity
// Deposit native tokens to receive wrapped tokens
function deposit() external payable;

// Withdraw wrapped tokens to receive native tokens (no-op for compatibility)
function withdraw(uint256 wad) external;

// Fallback function - calls deposit()
fallback() external payable;

// Receive function - calls deposit()
receive() external payable;
```

## Gas Costs

| Method | Gas Cost |
|--------|----------|
| `deposit` | 23,878 |
| `withdraw` | 9,207 |
| ERC20 methods | Same as ERC20 precompile |

## Implementation Details

### Deposit Mechanism

The deposit function has a unique implementation:

1. Accepts native tokens via `msg.value`
2. Sends the native tokens back to the caller using the bank module
3. Adjusts EVM state balances to reflect the "wrapping"
4. Emits a `Deposit` event

This approach ensures that the native tokens remain in the user's account while providing the wrapped token interface.

### Withdraw Mechanism

The withdraw function is implemented as a no-op that:

1. Validates the user has sufficient balance
2. Emits a `Withdrawal` event
3. Does not actually perform any token transfers

This maintains interface compatibility with WETH-style contracts while preserving the native token functionality.

### Balance Representation

- Native token balances are automatically reflected as wrapped token balances
- No actual wrapping occurs - the precompile provides a view over native balances
- All ERC20 operations work on the underlying native token

### Extended Precision

The precompile handles the extended precision of the native token
(18 decimals in EVM vs 6 decimals in Cosmos SDK) through internal conversion.

## Events

```solidity
// Emitted when native tokens are "deposited"
event Deposit(address indexed dst, uint256 wad);

// Emitted when tokens are "withdrawn"
event Withdrawal(address indexed src, uint256 wad);

// Standard ERC20 events
event Transfer(address indexed from, address indexed to, uint256 value);
event Approval(address indexed owner, address indexed spender, uint256 value);
```

## Security Considerations

1. **No Lock-up**: Native tokens are never locked in the precompile
2. **Direct Integration**: Operations directly interact with the bank module
3. **Balance Consistency**: EVM and Cosmos SDK balances remain synchronized
4. **Fallback Protection**: Sending native tokens to the contract automatically triggers deposit

## Usage Examples

### Basic Deposit and Transfer

```solidity
// Assume WERC20 is deployed at a specific address
IWERC20 wrappedToken = IWERC20(0x...); // WERC20 precompile address

// Deposit native tokens (sent value is returned to sender as wrapped tokens)
wrappedToken.deposit{value: 1 ether}();

// Now you can use it as an ERC20 token
wrappedToken.transfer(recipient, 0.5 ether);

// Check balance
uint256 balance = wrappedToken.balanceOf(msg.sender);
```

### Integration with DeFi Protocols

```solidity
contract DeFiProtocol {
    IWERC20 public wrappedNative;
    
    constructor(address _wrappedNative) {
        wrappedNative = IWERC20(_wrappedNative);
    }
    
    function depositCollateral() external payable {
        // Automatically wraps native tokens
        wrappedNative.deposit{value: msg.value}();
        
        // Now treat it as ERC20 collateral
        // The protocol can use standard ERC20 operations
    }
    
    function swapTokens(IERC20 tokenIn, uint256 amountIn) external {
        // Works seamlessly with wrapped native tokens
        tokenIn.transferFrom(msg.sender, address(this), amountIn);
        
        // Perform swap logic...
    }
}
```

### Fallback Handling

```solidity
// Sending native tokens directly to the WERC20 address triggers deposit
address payable wrappedAddress = payable(0x...); // WERC20 address
wrappedAddress.transfer(1 ether); // Automatically deposits
```

## Differences from Traditional WETH

1. **No Actual Wrapping**: Tokens remain as native tokens, only the interface changes
2. **No Lock-up**: Native tokens are not locked in the contract
3. **Automatic Reflection**: Native balance changes are immediately reflected
4. **Withdraw No-op**: Withdraw function exists for compatibility but doesn't transfer

## Integration Notes

- The WERC20 precompile address is determined by the token pair configuration
- Compatible with any protocol expecting ERC20 tokens
- Maintains full native token functionality
- Gas costs are comparable to standard ERC20 operations
- Ideal for DeFi protocols, DEXs, and other ERC20-based applications
