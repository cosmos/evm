# Integration Test Suite

## Test File Naming Convention

To support external integration testing, all test-related helper files in this module are named
using the `test_*.go` prefix instead of the conventional `*_test.go`.

This change is intentional. In Go, files ending with `*_test.go` are excluded from regular build contexts
and only compiled during `go test` execution.
As a result, such files are not included when the package is imported by other modules.
For example, downstream chains cannot reuse our EVM test harness.

To work around this, we rename those files to `test_*.go`, which:

- Keeps them recognizable as test helpers

- Ensures they are compiled as part of the package during regular builds

- Allows external chains to import and invoke testing utilities like `NewKeeperTestSuite()` or `NewUnitTestNetwork(...)`

> **Note:** These files are still test-focused and should not be used in production builds.

## External Client Usage

All tests defined here can be used by any client application that implements the `EvmApp` interface.
You can find usage examples under `evmd/tests/integration`.

For instance, if you want to test your own application with the Bank Precompile Integration Test Suite,
implement your own `CreateApp` function and pass it in as shown below:

```go
package integration

import (
    "testing"

    "github.com/stretchr/testify/suite"
    "github.com/cosmos/evm/tests/integration/precompiles/bank"
)

func TestBankPrecompileTestSuite(t *testing.T) {
    s := bank.NewPrecompileTestSuite(CreateEvmd)
    suite.Run(t, s)
}

func TestBankPrecompileIntegrationTestSuite(t *testing.T) {
    bank.TestIntegrationSuite(t, CreateEvmd)
}
```
