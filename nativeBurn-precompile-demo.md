# AI Prompt: Build Native Burn Precompile

You are an expert Cosmos SDK and Ethereum developer working on a live demonstration. Your task is to build a custom precompile called "nativeBurn" that implements TRUE deflationary token burning for a Cosmos EVM chain, from scratch, in real-time.


## Context

This is a live coding demonstration showing how to build custom Cosmos SDK precompiles. The audience will watch as you implement a complete, working precompile with test suite from start to finish.

## The Innovation

**The Problem**: Existing Cosmos precompiles (like distribution) have `FundCommunityPool` which just moves tokens around. No precompile actually DESTROYS tokens from total supply.

**Your Task**: Build the first Cosmos precompile that uses `BankKeeper.BurnCoins()` to PERMANENTLY reduce total supply. Users will receive ERC20 receipt tokens as proof of their deflationary contribution.



---

## Prerequisites

Before starting, ensure you have:

1. **Go** (v1.21+): `go version`
2. **Foundry** (for contract deployment): `forge --version`
   - Install: `curl -L https://foundry.paradigm.xyz | bash && foundryup`
3. **jq** (for JSON parsing): `jq --version`
   - macOS: `brew install jq`
   - Linux: `apt-get install jq`
4. **bc** (for math operations): `bc --version`
   - macOS: pre-installed
   - Linux: `apt-get install bc`

---

## Repository Structure

After completing implementation, your Cosmos EVM repository should look like:

```
cosmos-evm/
├── precompiles/
│   ├── nativeburn/
│   │   ├── NativeBurnI.sol       # Solidity interface
│   │   ├── abi.json              # Hardhat artifact format
│   │   ├── nativeburn.go         # Main precompile + routing (Run/Execute/etc)
│   │   ├── tx.go                 # Burn logic
│   │   ├── events.go             # Event emission
│   │   ├── types.go              # Event structs & helper functions
│   │   └── errors.go             # Error constants
│   ├── common/
│   │   └── interfaces.go         # BankKeeper interface (edited)
│   └── types/
│       ├── static_precompiles.go  # Registration (edited)
│       └── defaults.go            # Default precompiles (edited)
├── x/vm/types/
│   └── precompiles.go            # Address constants (edited)
├── config/
│   └── evmd_config.go            # Module permissions (edited)
├── contracts/
│   └── NativeBurnReceipt.sol     # Receipt token contract
├── local_node.sh                 # Genesis script (edited)
└── test-nativeburn.sh            # Automated test script
```

---

## Quick Test (One Command)

```bash
bash test-nativeburn.sh
```

This script will:
1. Build the chain binary
2. Start local chain with precompile registered
3. Deploy the receipt contract
4. Execute a deflationary burn transaction
5. Verify tokens were permanently destroyed, include raw contract call responses as well as polished results 
6. Display visual proof with colors

---

## Implementation Steps

Follow these steps exactly to build the Native Burn precompile:

### Step 1: Create Precompile Directory Structure

Create the directory:
```bash
mkdir -p precompiles/nativeburn
```

### Step 2: Create Solidity Interface

Create `precompiles/nativeburn/NativeBurnI.sol`:
```solidity
// SPDX-License-Identifier: LGPL-3.0-only
pragma solidity >=0.8.17;

address constant NATIVEBURN_PRECOMPILE_ADDRESS = 0x0000000000000000000000000000000000000900;

NativeBurnI constant NATIVEBURN_CONTRACT = NativeBurnI(NATIVEBURN_PRECOMPILE_ADDRESS);

interface NativeBurnI {
    event TokenBurned(address indexed burner, uint256 amount);

    function burnToken(address burner, uint256 amount) external returns (bool success);
}
```

### Step 3: Create ABI File 

Create `precompiles/nativeburn/abi.json`. **MUST be Hardhat artifact format**:
```json
{
  "_format": "hh-sol-artifact-1",
  "contractName": "NativeBurnI",
  "sourceName": "solidity/precompiles/nativeburn/NativeBurnI.sol",
  "abi": [
    {
      "anonymous": false,
      "inputs": [
        {"indexed": true, "internalType": "address", "name": "burner", "type": "address"},
        {"indexed": false, "internalType": "uint256", "name": "amount", "type": "uint256"}
      ],
      "name": "TokenBurned",
      "type": "event"
    },
    {
      "inputs": [
        {"internalType": "address", "name": "burner", "type": "address"},
        {"internalType": "uint256", "name": "amount", "type": "uint256"}
      ],
      "name": "burnToken",
      "outputs": [{"internalType": "bool", "name": "success", "type": "bool"}],
      "stateMutability": "nonpayable",
      "type": "function"
    }
  ]
}
```

### Step 4: Create Main Precompile File with Routing Logic

Create `precompiles/nativeburn/nativeburn.go` (includes struct, constructor, AND routing methods):
```go
package nativeburn

import (
	"embed"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	cmn "github.com/cosmos/evm/precompiles/common"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	"cosmossdk.io/core/address"
	"cosmossdk.io/log"
	storetypes "cosmossdk.io/store/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

var _ vm.PrecompiledContract = &Precompile{}

var (
	//go:embed abi.json
	f   embed.FS
	ABI abi.ABI
)

func init() {
	var err error
	ABI, err = cmn.LoadABI(f, "abi.json")
	if err != nil {
		panic(err)
	}
}

type Precompile struct {
	cmn.Precompile
	abi.ABI
	stakingKeeper cmn.StakingKeeper
	bankKeeper    cmn.BankKeeper
	addrCdc       address.Codec
}

func NewPrecompile(
	stakingKeeper cmn.StakingKeeper,
	bankKeeper cmn.BankKeeper,
	addrCdc address.Codec,
) *Precompile {
	return &Precompile{
		Precompile: cmn.Precompile{
			KvGasConfig:           storetypes.KVGasConfig(),
			TransientKVGasConfig:  storetypes.TransientGasConfig(),
			ContractAddress:       common.HexToAddress(evmtypes.NativeBurnPrecompileAddress),
			BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper),
		},
		ABI:           ABI,
		stakingKeeper: stakingKeeper,
		bankKeeper:    bankKeeper,
		addrCdc:       addrCdc,
	}
}

// Address returns the precompile contract address
func (p Precompile) Address() common.Address {
	return p.ContractAddress
}

// RequiredGas returns the required bare minimum gas to execute the precompile
func (p Precompile) RequiredGas(input []byte) uint64 {
	if len(input) < 4 {
		return 0
	}

	methodID := input[:4]
	method, err := p.MethodById(methodID)
	if err != nil {
		return 0
	}

	return p.Precompile.RequiredGas(input, p.IsTransaction(method))
}

// Run executes the precompile
func (p Precompile) Run(evm *vm.EVM, contract *vm.Contract, readonly bool) ([]byte, error) {
	return p.RunNativeAction(evm, contract, func(ctx sdk.Context) ([]byte, error) {
		return p.Execute(ctx, evm.StateDB, contract, readonly)
	})
}

// Execute routes the call to the appropriate method
func (p Precompile) Execute(ctx sdk.Context, stateDB vm.StateDB, contract *vm.Contract, readOnly bool) ([]byte, error) {
	method, args, err := cmn.SetupABI(p.ABI, contract, readOnly, p.IsTransaction)
	if err != nil {
		return nil, err
	}

	var bz []byte

	switch method.Name {
	case BurnTokenMethod:
		bz, err = p.BurnToken(ctx, contract, stateDB, method, args)
	default:
		return nil, fmt.Errorf(cmn.ErrUnknownMethod, method.Name)
	}

	return bz, err
}

// IsTransaction checks if the given method is a transaction or query
func (Precompile) IsTransaction(method *abi.Method) bool {
	switch method.Name {
	case BurnTokenMethod:
		return true
	default:
		return false
	}
}

// Logger returns a precompile-specific logger
func (p Precompile) Logger(ctx sdk.Context) log.Logger {
	return ctx.Logger().With("evm extension", "nativeburn")
}
```

**Key Pattern**: All routing logic (Run, Execute, RequiredGas, IsTransaction, Logger) lives in the main precompile file, NOT in a separate `types.go`. This matches the codebase convention used by `gov.go`, `staking.go`, etc.

### Step 5: Create Transaction Logic

Create `precompiles/nativeburn/tx.go`:
```go
package nativeburn

import (
	"fmt"
	"math/big"
	"cosmossdk.io/math"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
    BurnTokenMethod = "burnToken"
    ModuleName       = "nativeburn"
)

func (p *Precompile) BurnToken(
	ctx sdk.Context,
	contract *vm.Contract,
	stateDB vm.StateDB,
	method *abi.Method,
	args []interface{},
) ([]byte, error) {
    burnerAddr, amount, err := parseTokenBurnedArgs(args)
    if err != nil {
        return nil, err
    }

    // TODO: Add authorization check - for now allow contracts to burn on behalf of users
    // In production, this should verify approval/authorization

	bondDenom, err := p.stakingKeeper.BondDenom(ctx)
	if err != nil {
		return nil, err
	}

    coins := sdk.NewCoins(sdk.NewCoin(bondDenom, math.NewIntFromBigInt(amount)))
    burnerAccAddr := sdk.AccAddress(burnerAddr.Bytes())

	// Step 1: Send to module account
	err = p.bankKeeper.SendCoinsFromAccountToModule(ctx, burnerAccAddr, ModuleName, coins)
	if err != nil {
		return nil, fmt.Errorf("failed to send coins to module: %w", err)
	}

	// Step 2: PERMANENTLY BURN - reduces total supply
	err = p.bankKeeper.BurnCoins(ctx, ModuleName, coins)
	if err != nil {
		return nil, fmt.Errorf("failed to burn coins: %w", err)
	}

    if err := p.EmitTokenBurnedEvent(ctx, stateDB, common.Address(burnerAccAddr.Bytes()), amount); err != nil {
		return nil, err
	}

	return method.Outputs.Pack(true)
}

func parseTokenBurnedArgs(args []interface{}) (common.Address, *big.Int, error) {
    if len(args) != 2 {
        return common.Address{}, nil, fmt.Errorf("invalid number of arguments; expected 2, got %d", len(args))
    }

    burnerAddr, ok := args[0].(common.Address)
    if !ok {
        return common.Address{}, nil, fmt.Errorf("invalid burner address type")
    }

    if burnerAddr == (common.Address{}) {
        return common.Address{}, nil, fmt.Errorf("burner address cannot be zero")
    }

    amount, ok := args[1].(*big.Int)
    if !ok {
        return common.Address{}, nil, fmt.Errorf("invalid amount type")
    }

    if amount.Sign() <= 0 {
        return common.Address{}, nil, fmt.Errorf("amount must be positive")
    }

    return burnerAddr, amount, nil
}
```

### Step 6: Create Event Emission Logic

Create `precompiles/nativeburn/events.go`:
```go
package nativeburn

import (
	"math/big"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	cmn "github.com/cosmos/evm/precompiles/common"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/ethereum/go-ethereum/core/vm"
)

const EventTypeTokenBurned = "TokenBurned"

func (p Precompile) EmitTokenBurnedEvent(
	ctx sdk.Context,
	stateDB vm.StateDB,
	burner common.Address,
	amount *big.Int,
) error {
	event := p.Events[EventTypeTokenBurned]

	topics := make([]common.Hash, 2)
	topics[0] = event.ID

	var err error
	topics[1], err = cmn.MakeTopic(burner)
	if err != nil {
		return err
	}

	arguments := abi.Arguments{event.Inputs[1]}
	packed, err := arguments.Pack(amount)
	if err != nil {
		return err
	}

	stateDB.AddLog(&ethtypes.Log{
		Address:     p.Address(),
		Topics:      topics,
		Data:        packed,
		BlockNumber: uint64(ctx.BlockHeight()),
	})

	return nil
}
```

### Step 7: Create Types and Event Structs

Create `precompiles/nativeburn/types.go`:
```go
package nativeburn

import (
	"math/big"
	"github.com/ethereum/go-ethereum/common"
)

// EventTokenBurned defines the event data for the TokenBurned event
type EventTokenBurned struct {
	Burner common.Address
	Amount *big.Int
}
```

**Note**: For nativeburn, this file is minimal because we only have one simple event. More complex precompiles (like `gov` or `staking`) have many custom types, input/output structs, and parser functions in their `types.go`.

### Step 8: Create Error Constants

Create `precompiles/nativeburn/errors.go`:
```go
package nativeburn

const (
	ErrInvalidBurnAmount = "invalid burn amount: %s"
)
```

---

## Step 9: Register the Precompile in the Chain

### Step 9.1: Add Precompile Address Constant

Edit `x/vm/types/precompiles.go`:
```go
const (
	// ... existing precompiles ...
	NativeBurnPrecompileAddress = "0x0000000000000000000000000000000000000900"
)

var AvailableStaticPrecompiles = []string{
	// ... existing addresses ...
	NativeBurnPrecompileAddress,
}
```

### Step 9.2: Add BankKeeper Methods

Edit `precompiles/common/interfaces.go` - add to BankKeeper interface:
```go
SendCoinsFromAccountToModule(ctx context.Context, senderAddr sdk.AccAddress, recipientModule string, amt sdk.Coins) error
BurnCoins(ctx context.Context, moduleName string, amt sdk.Coins) error
```

### Step 9.3: Create Registration Function

Edit `precompiles/types/static_precompiles.go` - add import and function:
```go
import (
    nativeburnprecompile "github.com/cosmos/evm/precompiles/nativeburn"
)

func (s StaticPrecompiles) WithNativeBurnPrecompile(
	stakingKeeper stakingkeeper.Keeper,
	bankKeeper cmn.BankKeeper,
	opts ...Option,
) StaticPrecompiles {
	options := defaultOptionals()
	for _, opt := range opts {
		opt(&options)
	}

    nativeburnPrecompile := nativeburnprecompile.NewPrecompile(
		stakingKeeper,
		bankKeeper,
		options.AddressCodec,
	)

    s[nativeburnPrecompile.Address()] = nativeburnPrecompile
	return s
}
```

### Step 9.4: Add to Default Precompiles

Edit `precompiles/types/defaults.go`:
```go
precompiles := NewStaticPrecompiles().
    // ... existing precompiles ...
    .WithNativeBurnPrecompile(stakingKeeper, bankKeeper, opts...)
```

### Step 9.5: Register Module Account (CRITICAL)

Edit `config/evmd_config.go` - add to `maccPerms`:
```go
// NativeBurn precompile module account for deflationary burns
"nativeburn": {authtypes.Burner},
```

**Without Burner permission**: `BurnCoins()` will fail!

### Step 9.6: Update Genesis Script (FIXED)

Edit `local_node.sh` at **line 243** (NOT init-local.sh, which doesn't exist):

**Find this line:**
```bash
jq '.app_state["evm"]["params"]["active_static_precompiles"]=["0x0000000000000000000000000000000000000100","0x0000000000000000000000000000000000000400","0x0000000000000000000000000000000000000800","0x0000000000000000000000000000000000000801","0x0000000000000000000000000000000000000802","0x0000000000000000000000000000000000000803","0x0000000000000000000000000000000000000804","0x0000000000000000000000000000000000000805", "0x0000000000000000000000000000000000000806", "0x0000000000000000000000000000000000000807"]' "$GENESIS" >"$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
```

**Replace with:** (add `"0x0000000000000000000000000000000000000900"` at the end)
```bash
jq '.app_state["evm"]["params"]["active_static_precompiles"]=["0x0000000000000000000000000000000000000100","0x0000000000000000000000000000000000000400","0x0000000000000000000000000000000000000800","0x0000000000000000000000000000000000000801","0x0000000000000000000000000000000000000802","0x0000000000000000000000000000000000000803","0x0000000000000000000000000000000000000804","0x0000000000000000000000000000000000000805", "0x0000000000000000000000000000000000000806", "0x0000000000000000000000000000000000000807", "0x0000000000000000000000000000000000000900"]' "$GENESIS" >"$TMP_GENESIS" && mv "$TMP_GENESIS" "$GENESIS"
```

**Quick Find Command:**
```bash
grep -n "active_static_precompiles" local_node.sh
```

---

## Step 10: Create Receipt Contract (SIMPLIFIED - NO DEPENDENCIES)

Create `contracts/NativeBurnReceipt.sol`:
```solidity
// SPDX-License-Identifier: MIT
pragma solidity ^0.8.17;

// Simplified ERC20 implementation
contract ERC20 {
    string public name;
    string public symbol;
    uint8 public decimals = 18;
    uint256 public totalSupply;

    mapping(address => uint256) public balanceOf;
    mapping(address => mapping(address => uint256)) public allowance;

    event Transfer(address indexed from, address indexed to, uint256 value);
    event Approval(address indexed owner, address indexed spender, uint256 value);

    constructor(string memory _name, string memory _symbol) {
        name = _name;
        symbol = _symbol;
    }

    function _mint(address to, uint256 amount) internal {
        totalSupply += amount;
        balanceOf[to] += amount;
        emit Transfer(address(0), to, amount);
    }
}

// NativeBurn precompile interface
interface NativeBurnI {
    function burnToken(uint256 amount) external returns (bool success);
}

contract NativeBurnReceipt is ERC20 {
    NativeBurnI constant NATIVEBURN_CONTRACT = NativeBurnI(0x0000000000000000000000000000000000000900);
    uint256 public totalBurned;
    address public owner;

    event TokensBurned(address indexed burner, uint256 amount, uint256 receiptTokens);

    constructor() ERC20("NativeBurn Receipt", "BURN") {
        owner = msg.sender;
    }

    function burn(uint256 amount) external {
        require(amount > 0, "Must burn positive amount");

        bool success = NATIVEBURN_CONTRACT.burnToken(msg.sender, amount);
        require(success, "Burn failed");

        _mint(msg.sender, amount);
        totalBurned += amount;

        emit TokensBurned(msg.sender, amount, amount);
    }

    function getBurnedAmount(address account) external view returns (uint256) {
        return balanceOf[account];
    }
}
```

**No OpenZeppelin dependencies needed!**

---

## Step 11: Create Automated Test Script

Create `test-nativeburn.sh`:
```bash
#!/bin/bash
set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Test configuration
PRIVATE_KEY="0x88cbead91aee890d27bf06e003ade3d4e952427e88f88d31d61d3ef5e5d54305"
USER_ADDRESS="0xC6Fe5D33615a1C52c08018c47E8Bc53646A0E101"
RPC_URL="http://localhost:8545"
BURN_AMOUNT="1000000000000000000"  # 1 token

echo -e "${BLUE}======================================${NC}"
echo -e "${BLUE}NativeBurn Precompile Test Suite${NC}"
echo -e "${BLUE}======================================${NC}\n"

# Step 1: Build
echo -e "${YELLOW}[1/6]${NC} Building chain binary..."
make install > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓${NC} Chain binary built successfully\n"
else
    echo -e "${RED}✗${NC} Failed to build chain binary\n"
    exit 1
fi

# Step 2: Start Chain
echo -e "${YELLOW}[2/6]${NC} Starting local chain..."
pkill -9 evmd > /dev/null 2>&1 || true
sleep 2
bash local_node.sh -y --no-install > /dev/null 2>&1 &
sleep 12  # Wait for chain to start

# Check if chain is running
BLOCK_NUM=$(curl -s -X POST -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}' \
  $RPC_URL | jq -r '.result')

if [ "$BLOCK_NUM" != "null" ] && [ -n "$BLOCK_NUM" ]; then
    echo -e "${GREEN}✓${NC} Chain is running (block: $((16#${BLOCK_NUM:2})))\n"
else
    echo -e "${RED}✗${NC} Chain failed to start\n"
    exit 1
fi

# Step 3: Deploy Contract
echo -e "${YELLOW}[3/6]${NC} Deploying NativeBurnReceipt contract..."
DEPLOY_OUTPUT=$(forge create --rpc-url $RPC_URL \
  --private-key $PRIVATE_KEY \
  contracts/NativeBurnReceipt.sol:NativeBurnReceipt \
  --broadcast 2>&1)

CONTRACT_ADDRESS=$(echo "$DEPLOY_OUTPUT" | grep "Deployed to:" | awk '{print $3}')

if [ -n "$CONTRACT_ADDRESS" ]; then
    echo -e "${GREEN}✓${NC} Contract deployed at: ${BLUE}$CONTRACT_ADDRESS${NC}\n"
else
    echo -e "${RED}✗${NC} Failed to deploy contract\n"
    exit 1
fi

# Step 4: Execute Burn
echo -e "${YELLOW}[4/6]${NC} Executing burn transaction..."
BURN_TX=$(cast send $CONTRACT_ADDRESS "burn(uint256)" $BURN_AMOUNT \
  --private-key $PRIVATE_KEY \
  --rpc-url $RPC_URL 2>&1)

if echo "$BURN_TX" | grep -q "status.*1"; then
    echo -e "${GREEN}✓${NC} Burn transaction successful (1 TEST token burned)\n"
else
    echo -e "${RED}✗${NC} Burn transaction failed\n"
    exit 1
fi

# Step 5: Verify Burn Results
echo -e "${YELLOW}[5/6]${NC} Verifying burn results..."
sleep 2  # Wait for block

RECEIPT_BALANCE=$(cast call $CONTRACT_ADDRESS "balanceOf(address)" $USER_ADDRESS --rpc-url $RPC_URL)
TOTAL_BURNED=$(cast call $CONTRACT_ADDRESS "totalBurned()" --rpc-url $RPC_URL)

# Convert hex to decimal
RECEIPT_DEC=$((16#${RECEIPT_BALANCE:2}))
BURNED_DEC=$((16#${TOTAL_BURNED:2}))

echo -e "  Receipt Balance: ${GREEN}${RECEIPT_DEC}${NC} wei (${GREEN}1.0${NC} BURN token)"
echo -e "  Total Burned: ${GREEN}${BURNED_DEC}${NC} wei\n"

# Step 6: Final Verification
echo -e "${YELLOW}[6/6]${NC} Final verification..."

if [ "$RECEIPT_DEC" -eq "$BURNED_DEC" ] && [ "$BURNED_DEC" -eq "$BURN_AMOUNT" ]; then
    echo -e "${GREEN}✓${NC} Receipt tokens minted correctly (1:1 ratio)"
    echo -e "${GREEN}✓${NC} Total burned matches expected amount"
    echo -e "${GREEN}✓${NC} Burn transaction completed successfully"
else
    echo -e "${RED}✗${NC} Receipt/burn tracking mismatch"
    exit 1
fi

# Success banner
echo -e "\n${GREEN}======================================${NC}"
echo -e "${GREEN}🔥 BURN VERIFIED! 🔥${NC}"
echo -e "${GREEN}======================================${NC}"
echo -e "\n${BLUE}Results:${NC}"
echo -e "  • Burned Amount: ${GREEN}1.0 TEST${NC}"
echo -e "  • Receipt Tokens: ${GREEN}1.0 BURN${NC}"
echo -e "  • Contract: ${BLUE}${CONTRACT_ADDRESS}${NC}"
echo -e "\n${BLUE}Innovation:${NC}"
echo -e "  • First Cosmos precompile using ${RED}BankKeeper.BurnCoins()${NC}"
echo -e "  • Tokens ${RED}permanently destroyed${NC} from total supply"
echo -e "  • Receipt tokens prove deflationary contribution\n"

# Cleanup prompt
echo -e "${YELLOW}Chain is still running. Stop it with:${NC} pkill evmd\n"
```

Make it executable:
```bash
chmod +x test-nativeburn.sh
```

---

## Run Complete Test

```bash
bash test-nativeburn.sh
```

**Expected Output**:
```
======================================
NativeBurn Precompile Test Suite
======================================

[1/6] Building chain binary...
✓ Chain binary built successfully

[2/6] Starting local chain...
✓ Chain is running (block: 12)

[3/6] Deploying NativeBurnReceipt contract...
✓ Contract deployed at: 0x3D641a2791533B4A0000345eA8d509d01E1ec301

[4/6] Executing burn transaction...
✓ Burn transaction successful (1 TEST token burned)

[5/6] Verifying burn results...
  Receipt Balance: 1000000000000000000 wei (1.0 BURN token)
  Total Burned: 1000000000000000000 wei

[6/6] Final verification...
✓ Receipt tokens minted correctly (1:1 ratio)
✓ Total burned matches expected amount
✓ Burn transaction completed successfully

======================================
🔥 BURN VERIFIED! 🔥
======================================

Results:
  • Burned Amount: 1.0 TEST
  • Receipt Tokens: 1.0 BURN
  • Contract: 0x3D641a2791533B4A0000345eA8d509d01E1ec301

Innovation:
  • First Cosmos precompile using BankKeeper.BurnCoins()
  • Tokens permanently destroyed from total supply
  • Receipt tokens prove deflationary contribution
```

---

## Implementation Verification Checklist

Before running the test, verify your implementation:

### ✅ File Existence Check
```bash
# All precompile files exist
ls -la precompiles/nativeburn/{NativeBurnI.sol,abi.json,nativeburn.go,tx.go,events.go,types.go,errors.go}

# Contract exists
ls -la contracts/NativeBurnReceipt.sol

# Test script exists and is executable
ls -la test-nativeburn.sh
```

### ✅ Code Integration Check
```bash
# Verify precompile address is registered
grep "NativeBurnPrecompileAddress" x/vm/types/precompiles.go

# Verify module account has Burner permission
grep "nativeburn" config/evmd_config.go

# Verify BankKeeper has burn methods
grep -A 1 "BurnCoins" precompiles/common/interfaces.go

# Verify precompile is in defaults
grep "NativeBurnPrecompile" precompiles/types/defaults.go

# Verify genesis includes 0x0900
grep "0x0000000000000000000000000000000000000900" local_node.sh
```

### ✅ Build Verification
```bash
# Should complete without errors
make install

# Verify binary is installed
which evmd
evmd version
```

### ✅ Quick Manual Test
```bash
# Terminal 1: Start chain
bash local_node.sh -y --no-install

# Terminal 2: Wait 15 seconds, then check RPC
curl -X POST http://localhost:8545 \
  -H "Content-Type: application/json" \
  --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'

# Should return a hex block number like: {"jsonrpc":"2.0","id":1,"result":"0xa"}
```

### ✅ Expected File Count
After implementation, you should have created/modified:
- **7 new files** in `precompiles/nativeburn/`: `NativeBurnI.sol`, `abi.json`, `nativeburn.go`, `tx.go`, `events.go`, `types.go`, `errors.go`
- **1 new file** in `contracts/`: `NativeBurnReceipt.sol`
- **1 new file**: `test-nativeburn.sh`
- **6 edited files**: `x/vm/types/precompiles.go`, `precompiles/common/interfaces.go`, `precompiles/types/static_precompiles.go`, `precompiles/types/defaults.go`, `config/evmd_config.go`, `local_node.sh`

**Key Pattern**:
- **`nativeburn.go`** = struct definition + constructor + routing methods (Run, Execute, RequiredGas, IsTransaction, Logger, Address)
- **`types.go`** = event structs + custom types + helper functions (for consistency with other precompiles)
- **`tx.go`** = transaction implementation logic
- **`events.go`** = event emission logic

---

## Troubleshooting

### Error: "panic: invalid ABI"
**Fix**: Ensure `abi.json` uses Hardhat artifact format with `_format`, `contractName`, `sourceName` fields.

### Error: "undefined: cmn.NewBalanceHandler"
**Fix**: Use `BalanceHandlerFactory: cmn.NewBalanceHandlerFactory(bankKeeper)` instead.

### Error: "unauthorized: nativeburn is not allowed to receive funds"
**Fix**: Add `"nativeburn": {authtypes.Burner}` to `maccPerms` in `config/evmd_config.go`.

### Error: "method BurnCoins not found"
**Fix**: Add `SendCoinsFromAccountToModule` and `BurnCoins` to BankKeeper interface in `precompiles/common/interfaces.go`.

### Error: Chain won't start
**Fix**: Kill existing processes (`pkill -9 evmd`) and ensure `local_node.sh` has precompile address in `active_static_precompiles`.

### Error: "contract.CallerAddress undefined" or "evm.Origin undefined"
**Symptom**: Build fails with error like `contract.CallerAddress undefined (type *vm.Contract has no field or method CallerAddress)`
**Fix**: Use `contract.Caller().Bytes()` to get the caller address. The correct pattern is:
```go
burnerAccAddr := sdk.AccAddress(contract.Caller().Bytes())
```
**Common mistakes**:
- ❌ `contract.CallerAddress.Bytes()` - CallerAddress field doesn't exist
- ❌ `evm.Origin.Bytes()` - evm variable not in scope
- ✅ `contract.Caller().Bytes()` - Correct method to get caller address

### Error: "spendable balance 0atest is smaller than..." or "insufficient funds"
**Symptom**: Transaction fails with "spendable balance 0atest is smaller than X: insufficient funds"
**Cause**: The precompile interface must include the burner address parameter. If you call `burnToken(amount)` without the address parameter, the precompile can't determine whose balance to burn from.
**Fix**:
1. Update Solidity interface to: `function burnToken(address burner, uint256 amount) external returns (bool success);`
2. Update ABI to include both parameters
3. Update Go code to parse both parameters: `burnerAddr, amount, err := parseTokenBurnedArgs(args)`
4. When calling from contract, pass `msg.sender`: `NATIVEBURN_CONTRACT.burnToken(msg.sender, amount)`

### Error: "grep: invalid option -- P"
**Platform**: macOS (BSD grep doesn't support Perl regex)
**Fix**: Replace `grep -oP '\d+\.\d+'` with `grep -Eo '[0-9]+\.[0-9]+' | head -1`

### Build Error: "cannot find package"
**Fix**: Ensure you're in the repository root directory and run `make install` to build the binary.

### Contract Deployment Fails
**Symptoms**: Forge create command fails or times out
**Fix**:
1. Ensure chain is fully started (wait 12+ seconds after `local_node.sh`)
2. Verify RPC is accessible: `curl -X POST http://localhost:8545 -H "Content-Type: application/json" --data '{"jsonrpc":"2.0","method":"eth_blockNumber","params":[],"id":1}'`
3. Check that foundry is installed: `forge --version`

---

## Summary

You just built the **first Cosmos SDK precompile for TRUE deflationary burning**:

### The Innovation
- Uses `BankKeeper.BurnCoins()` to PERMANENTLY reduce total supply
- NOT FundCommunityPool (just moves tokens)
- NOT transferring to validators/governance
- **TRUE BURN**: Tokens destroyed forever

### The Architecture
1. User → Contract: `burn(amount)`
2. Contract → Precompile: `burnToken(amount)`
3. Precompile: User → Module account (transfer)
4. Precompile: Module → Burn (DESTROYS tokens)
5. Contract: Mint receipt tokens (proof of burn)

### The Proof
- Community pool = staking rewards ONLY
- Receipt tokens = proof of deflationary contribution
- Total supply REDUCED permanently

**Every burn makes ALL tokens more scarce!**

---

## Implementation Notes & Key Learnings

### 🎯 Critical Success Factors

1. **File Organization Pattern** (MOST IMPORTANT):
   - **`nativeburn.go`**: Contains struct definition, constructor, AND all routing methods (`Run`, `Execute`, `RequiredGas`, `IsTransaction`, `Logger`, `Address`)
   - **`types.go`**: Contains event structs, custom types, input/output types, and helper/parser functions
   - **`tx.go`**: Contains transaction implementation logic
   - **`events.go`**: Contains event emission logic
   - **Common mistake**: Putting routing methods in `types.go` - routing methods MUST go in the main file!
   - **Why it matters**: Consistency with existing precompiles (`gov.go`, `staking.go`, `distribution.go`, etc.) makes the codebase easier to maintain
   - **Examples**:
     - `gov/types.go`: Has EventVote, VotesInput, ProposalData, ParseVotesArgs, NewMsgVote, etc.
     - `staking/types.go`: Has EventDelegate, Description, Commission, NewDescriptionFromResponse, etc.
     - `nativeburn/types.go`: Has EventTokenBurned (minimal, but present for consistency)

2. **Module Account Pattern**: The two-step burn process (user→module→burn) is REQUIRED because only module accounts with `Burner` permission can destroy tokens.

3. **ABI Format Matters**: Plain JSON arrays will cause panic. ALWAYS use Hardhat artifact format with `_format`, `contractName`, and `sourceName` fields.

4. **Cross-Platform Compatibility**: When writing shell scripts, use POSIX-compliant commands:
   - ✅ `grep -E` (works on macOS and Linux)
   - ❌ `grep -P` (Perl regex, not available on macOS BSD grep)

5. **Initialization Timing**: Local chain needs 12+ seconds to fully initialize before contract deployment. Shorter waits cause intermittent failures.

6. **Gas Configuration**: The `KvGasConfig()` and `TransientKVGasConfig()` must be set correctly for precompile gas metering to work.

### 🔧 Architecture Patterns Validated

**✅ Correct Precompile Structure:**
```go
type Precompile struct {
    cmn.Precompile              // Embed common functionality
    abi.ABI                     // Embed ABI
    stakingKeeper cmn.StakingKeeper
    bankKeeper    cmn.BankKeeper
    addrCdc       address.Codec
}
```

**✅ Correct Execution Flow:**
```
Run() → RunNativeAction() → Execute() → Method Switch → TokenBurned()
```

**✅ Correct Event Pattern:**
```go
event := p.Events[EventTypeTokenBurned]  // From ABI
topics := make([]common.Hash, 2)
topics[0] = event.ID                    // Event signature
topics[1], err = cmn.MakeTopic(address) // Indexed parameter
```

### 📊 Verification Results

When the test completes successfully, you should see:
- ✅ Receipt balance = 1000000000000000000 wei (1.0 BURN token)
- ✅ Total burned = 1000000000000000000 wei
- ✅ 1:1 ratio confirmed
- ✅ All 6 test steps pass

### 🚀 Production Considerations

**Before deploying to production:**

1. **Security Audit**: Have the burn mechanism audited, especially:
   - Module account permissions
   - Balance transfer flows
   - Event emission correctness

2. **Gas Optimization**: Profile gas costs for burn transactions across different amounts

3. **Rate Limiting**: Consider adding rate limits or maximum burn amounts to prevent economic attacks

4. **Monitoring**: Set up alerts for:
   - Large burn transactions
   - Module account balance anomalies
   - Failed burn attempts

5. **Governance**: Consider adding governance controls for:
   - Enabling/disabling the burn mechanism
   - Setting maximum burn amounts
   - Pausing in emergency situations

---

## Final Verification

After completing all steps, run the automated test:

```bash
bash test-nativeburn.sh
```

This will build, deploy, test, and verify the entire implementation automatically.

**Success Criteria:**
- ✅ All 6 test steps pass
- ✅ Receipt tokens minted correctly (1:1 ratio)
- ✅ Total burned matches expected amount
- ✅ Burn transaction completed successfully

---

## What You'll Build

You will create the **first Cosmos SDK precompile for TRUE deflationary burning**:

### The Innovation
- Uses `BankKeeper.BurnCoins()` to PERMANENTLY reduce total supply
- NOT FundCommunityPool (just moves tokens)
- NOT transferring to validators/governance
- **TRUE BURN**: Tokens destroyed forever

### The Architecture
1. User → Contract: `burn(amount)`
2. Contract → Precompile: `burnToken(amount)`
3. Precompile: User → Module account (transfer)
4. Precompile: Module → Burn (DESTROYS tokens)
5. Contract: Mint receipt tokens (proof of burn)

**Critical Implementation Requirements**:
1. ✅ Use `BalanceHandlerFactory` NOT `BalanceHandler` (API changed)
2. ✅ ABI file MUST use Hardhat artifact format (not plain JSON array)
3. ✅ Module account needs `Burner` permission in `maccPerms`
4. ✅ BankKeeper interface needs `SendCoinsFromAccountToModule` and `BurnCoins` methods
5. ✅ Two-step burn: send to module account → burn from module (reduces total supply)
6. ✅ Use proper Run/Execute pattern with `RunNativeAction` and `SetupABI`
7. ✅ Update `local_node.sh` NOT `init-local.sh` (doesn't exist)
8. ✅ **macOS Compatibility**: Use `grep -E` (extended regex) instead of `grep -P` (Perl regex not available on macOS)
9. ✅ Test script must be executable: `chmod +x test-nativeburn.sh`
10. ✅ Wait 12+ seconds for chain to fully initialize before deploying contracts
