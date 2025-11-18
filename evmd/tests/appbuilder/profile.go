package appbuilder

// Profile enumerates the preconfigured app variants used by integration tests.
// Each profile corresponds to a full, internally consistent app recipe
// (store keys, module accounts, module set, precompiles, etc.).
type Profile string

const (
	// Base uses the standard bank module and no extra precompiles/keepers beyond
	// what the main app wiring provides.
	Base Profile = "base"
	// BasePreciseBank swaps the bank module with precisebank, without adding the
	// full precompile suite.
	BasePreciseBank Profile = "base-precisebank"
	// FullPrecompiles enables the full precompile set (including bank) using the
	// standard bank module.
	FullPrecompiles Profile = "full-precompiles"
	// FullPrecompilesPreciseBank enables the full precompile set while using
	// precisebank in place of the standard bank module.
	FullPrecompilesPreciseBank Profile = "full-precompiles-precisebank"
)
