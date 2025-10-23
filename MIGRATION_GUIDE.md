# Precompile Test Migration Guide to go-abi

## Overview

This guide explains how to migrate precompile integration tests from the old go-ethereum ABI API to the new custom `go-abi` library (github.com/yihuang/go-abi).

## Background

The precompiles have been refactored to use a custom `go-abi` library instead of the standard `go-ethereum/accounts/abi` package. This change requires updates to the test code to work with the new API.

## Migration Status

| Precompile | Status | Notes |
|------------|--------|-------|
| **Bank** | ✅ Complete | Fully migrated, all tests working |
| **Bech32** | ✅ Complete | Fully migrated, all tests working |
| **Slashing** | 🟡 Partial | Query tests migrated, integration/tx tests need work |
| **P256** | ✅ Complete | Already working, no changes needed |
| **Distribution** | ❌ Pending | Complex test suite, needs migration |
| **ERC20** | ❌ Pending | Many test files, needs migration |
| **Gov** | ❌ Pending | Test files exist, needs migration |
| **ICS20** | ❌ Pending | Complex IBC tests, needs migration |
| **Staking** | ❌ Pending | Large test suite, needs migration |
| **WERC20** | ❌ Pending | Event handling issues, needs migration |

## Key Differences

### Old API (go-ethereum/accounts/abi)
- Used `abi.ABI.Pack()` to encode method calls
- Used `UnpackIntoInterface()` and `Unpack()` to decode results
- Relied on reflection for encoding/decoding
- Tests accessed `precompile.ABI` field

### New API (go-abi)
- Uses generated types with methods like `EncodeWithSelector()`
- Uses generated `Decode()` methods on result types
- No reflection, fully type-safe
- No `ABI` field on precompile struct

## Migration Steps

### 1. Update ContractData Struct

**Before:**
```go
type ContractData struct {
    ownerPriv cryptotypes.PrivKey
    contractAddr   common.Address
    contractABI    abi.ABI
    precompileAddr common.Address
    precompileABI  abi.ABI  // ← Remove this
}
```

**After:**
```go
type ContractData struct {
    ownerPriv cryptotypes.PrivKey
    contractAddr   common.Address
    contractABI    abi.ABI
    precompileAddr common.Address
    // precompileABI removed
}
```

### 2. Remove precompileABI from initialization

**Before:**
```go
contractData = ContractData{
    ownerPriv:      sender.Priv,
    precompileAddr: is.precompile.Address(),
    precompileABI:  is.precompile.ABI,  // ← Remove this
    contractAddr:   bankCallerContractAddr,
    contractABI:    bankCallerContract.ABI,
}
```

**After:**
```go
contractData = ContractData{
    ownerPriv:      sender.Priv,
    precompileAddr: is.precompile.Address(),
    contractAddr:   bankCallerContractAddr,
    contractABI:    bankCallerContract.ABI,
}
```

### 3. Update getTxAndCallArgs Function

This function handles encoding for direct precompile calls. Replace manual encoding with `EncodeWithSelector()`:

```go
func getTxAndCallArgs(
    callType int,
    contractData ContractData,
    methodName string,
    args ...interface{},
) (evmtypes.EvmTxArgs, testutiltypes.CallArgs) {
    txArgs := evmtypes.EvmTxArgs{}
    callArgs := testutiltypes.CallArgs{}

    switch callType {
    case directCall:
        var input []byte
        switch methodName {
        case bank.BalancesMethod:
            addr := args[0].(common.Address)
            call := bank.BalancesCall{Account: addr}
            input, _ = call.EncodeWithSelector()  // Use built-in method
        case bank.TotalSupplyMethod:
            var call bank.TotalSupplyCall
            input, _ = call.EncodeWithSelector()
        case bank.SupplyOfMethod:
            addr := args[0].(common.Address)
            call := bank.SupplyOfCall{Erc20Address: addr}
            input, _ = call.EncodeWithSelector()
        default:
            panic(fmt.Sprintf("unknown method: %s", methodName))
        }
        txArgs.To = &contractData.precompileAddr
        txArgs.Input = input
        callArgs.ContractABI = abi.ABI{}
    case contractCall:
        txArgs.To = &contractData.contractAddr
        callArgs.ContractABI = contractData.contractABI
    }

    callArgs.MethodName = methodName
    callArgs.Args = args
    return txArgs, callArgs
}
```

### 5. Replace UnpackIntoInterface Calls

**Before:**
```go
var balances []bank.Balance
err = is.precompile.UnpackIntoInterface(&balances, bank2.BalancesMethod, ethRes.Ret)
Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")
```

**After (Recommended):**
```go
var ret bank.BalancesReturn
_, err = ret.Decode(ethRes.Ret)
Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")
Expect(ret.Balances).To(Equal(expectedBalances))
```

**After (With Helper):**
```go
balances, err := decodeBalancesResult(ethRes.Ret)
Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")
```

### 6. Replace Unpack Calls

**Before:**
```go
out, err := is.precompile.Unpack(bank2.SupplyOfMethod, ethRes.Ret)
Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")
Expect(out[0].(*big.Int).String()).To(Equal(expectedValue.String()))
```

**After (Recommended):**
```go
var ret bank.SupplyOfReturn
_, err = ret.Decode(ethRes.Ret)
Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")
Expect(ret.TotalSupply.String()).To(Equal(expectedValue.String()))
```

**After (With Helper):**
```go
supply, err := decodeSupplyOfResult(ethRes.Ret)
Expect(err).ToNot(HaveOccurred(), "failed to unpack balances")
Expect(supply.String()).To(Equal(expectedValue.String()))
```

### 7. Update Unit Tests (test_query.go)

**Before:**
```go
func (s *PrecompileTestSuite) TestBalances() {
    s.SetupTest()
    method := s.precompile.Methods[bank.BalancesMethod]  // ← Methods field doesn't exist

    // Test cases using method variable...
    bz, err := s.precompile.Balances(ctx, &method, args)
    var balances []bank.Balance
    err = s.precompile.UnpackIntoInterface(&balances, method.Name, bz)
}
```

**After:**
```go
func (s *PrecompileTestSuite) TestBalances() {
    s.SetupTest()

    // Test cases use typed call structs directly
    call := &bank.BalancesCall{Account: addr}
    result, err := s.precompile.Balances(ctx, call)

    balances := result.Balances  // Direct access to result fields
}
```

## Generated Types

The `go-abi` tool generates these types for each method:

- `{MethodName}Call` - Input parameters struct
- `{MethodName}Return` - Output results struct
- `{MethodName}Selector` - Method selector constant
- `{MethodName}ID` - Method ID constant
- `{MethodName}Method` - Method name constant

## Example for Other Precompiles

For each precompile (bech32, distribution, erc20, gov, etc.), you need to:

1. Check the generated types in `{precompile}.abi.go`
2. Create appropriate decode helper functions
3. Update `getTxAndCallArgs` to handle the precompile's methods
4. Replace all `UnpackIntoInterface` and `Unpack` calls
5. Update unit tests to use typed call structs

## Verification

After migration, verify the tests build successfully:

```bash
go build -tags=tests ./tests/integration/precompiles/{precompile_name}/...
```

## Common Issues and Solutions

### Issue 1: `s.precompile.Methods` undefined
**Error:** `s.precompile.Methods undefined (type *"github.com/cosmos/evm/precompiles/xxx".Precompile has no field or method Methods)`

**Solution:** The `Methods` field no longer exists. Use typed call structs directly:
```go
// Old:
method := s.precompile.Methods[xxx.SomeMethod]
result, err := s.precompile.SomeMethod(ctx, &method, args)

// New:
var call xxx.SomeCall
result, err := s.precompile.SomeMethod(ctx, &call)
```

### Issue 2: `s.precompile.ABI` undefined
**Error:** `s.precompile.ABI undefined (type *"github.com/cosmos/evm/precompiles/xxx".Precompile has no field or method ABI)`

**Solution:** The `ABI` field no longer exists. For direct precompile calls, encode with `EncodeWithSelector()`:
```go
// Old:
input, err := s.precompile.Pack(xxx.SomeMethod, args...)
s.precompile.UnpackIntoInterface(&out, xxx.SomeMethod, data)

// New:
var call xxx.SomeCall{CreateArgs: args}
input, _ := call.EncodeWithSelector()
var ret xxx.SomeReturn
_, err := ret.Decode(data)
```

### Issue 3: Too many arguments in call
**Error:** `too many arguments in call to s.precompile.SomeMethod`

**Solution:** The new API uses typed structs instead of variadic `[]interface{}`:
```go
// Old:
result, err := s.precompile.SomeMethod(ctx, contract, stateDB, []interface{}{arg1, arg2})

// New:
var call xxx.SomeCall{Arg1: value1, Arg2: value2}
result, err := s.precompile.SomeMethod(ctx, call, stateDB, contract)
```

### Issue 4: Factory calls with CallArgs
**Error:** `cannot use callArgs as "github.com/yihuang/go-abi".Method value`

**Solution:** When calling through factory functions, set the `Method` field in `CallArgs`:
```go
// Old:
callArgs := testutiltypes.CallArgs{
    ContractABI: contract.ABI,
    MethodName:  "someMethod",
    Args:        []interface{}{arg1, arg2},
}

// New:
callArgs := testutiltypes.CallArgs{
    ContractABI: contract.ABI,
    MethodName:  "someMethod",
    Args:        []interface{}{arg1, arg2},
    Method:      &SomeCall{Arg1: arg1, Arg2: arg2}, // Add this
}
```

## References

- Bank precompile migration: `tests/integration/precompiles/bank/`
- Bech32 precompile migration: `tests/integration/precompiles/bech32/`
- Slashing precompile migration: `tests/integration/precompiles/slashing/test_query.go`
- go-abi library: github.com/yihuang/go-abi
- Generated types example: `precompiles/bank/bank.abi.go`
