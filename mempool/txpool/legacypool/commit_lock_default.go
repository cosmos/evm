//go:build !race

package legacypool

// beginCommitRead is a no-op in non-race builds to avoid unnecessary overhead.
func beginCommitRead(_ any) func() { return func() {} }
