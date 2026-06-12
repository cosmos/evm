# ERC20 Precompile

The ERC20 precompile enables native Cosmos SDK coins to be accessed and managed through the standard ERC20
token interface within the EVM. This allows smart contracts to interact with native tokens using familiar ERC20 methods.

## Interface

The precompile implements the standard ERC20 interface with additional metadata support:

### IERC20 Methods

```solidity
// Query Methods
function totalSupply() external view returns (uint256);
function balanceOf(address account) external view returns (uint256);
function allowance(address owner, address spender) external view returns (uint256);

// Transaction Methods
function transfer(address to, uint256 amount) external returns (bool);
function approve(address spender, uint256 amount) external returns (bool);
function transferFrom(address from, address to, uint256 amount) external returns (bool);
```

### IERC20Metadata Methods

```solidity
function name() external view returns (string memory);
function symbol() external view returns (string memory);
function decimals() external view returns (uint8);
```

## Gas Costs

The following gas costs are charged for each method:

| Method | Gas Cost |
|--------|----------|
| `transfer` | 9,000 |
| `transferFrom` | 30,500 |
| `approve` | 8,100 |
| `name` | 3,421 |
| `symbol` | 3,464 |
| `decimals` | 427 |
| `totalSupply` | 2,480 |
| `balanceOf` | 2,870 |
| `allowance` | 3,225 |

## Implementation Details

### Token Pair Mapping

Each ERC20 precompile instance is associated with a `TokenPair` that links:

- A Cosmos SDK denomination (e.g., `uatom`)
- An ERC20 contract address

The precompile address is determined by the token pair configuration.

### Transfer Mechanism

- **Direct transfers** (`transfer`): Execute a bank send message from the caller to the recipient
- **Delegated transfers** (`transferFrom`):
    - Check and update the spender's allowance
    - Execute a bank send message from the token owner to the recipient
    - Emit both Transfer and Approval events

### Metadata Handling

Token metadata is resolved in the following priority:

1. Bank module metadata (if registered)
2. IBC voucher base denomination (for IBC tokens)
3. Inferred from denomination (e.g., `uatom` → name: "Atom", symbol: "ATOM", decimals: 6)

### Balance Integration

The precompile integrates with the Cosmos SDK bank module:

- Balances are read directly from the bank keeper
- Transfers use bank send messages for state changes
- Special handling for the EVM native token (18 decimal conversion)

### Error Handling

- Prevents receiving funds directly to the precompile address
- Validates transfer amounts and allowances
- Converts bank module errors to ERC20-compatible errors

## Events

The precompile emits standard ERC20 events:

```solidity
event Transfer(address indexed from, address indexed to, uint256 value);
event Approval(address indexed owner, address indexed spender, uint256 value);
```

## Security Considerations

1. **No Direct Funding**: The precompile cannot receive funds through `msg.value` to prevent loss of funds
2. **Allowance Management**: Follows the standard ERC20 allowance pattern with proper checks
3. **Balance Consistency**: All balance changes go through the bank module ensuring consistency

## Usage Example

```solidity
// Assuming the precompile is deployed at a specific address for a native token
IERC20 token = IERC20(0x...); // Precompile address for the native token

// Check balance
uint256 balance = token.balanceOf(msg.sender);

// Transfer tokens
token.transfer(recipient, 1000000); // Transfer 1 token (assuming 6 decimals)

// Approve and transferFrom
token.approve(spender, 5000000);
// Spender can now call:
token.transferFrom(owner, recipient, 3000000);
```

## Integration Notes

- The precompile is automatically available for registered token pairs
- Smart contracts can interact with native tokens without wrapping
- Full compatibility with existing ERC20 tooling and libraries
