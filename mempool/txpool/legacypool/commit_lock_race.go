//go:build race

package legacypool

// commitAwareChain is implemented by chains that expose optional commit RW locks.
// These are used to coordinate background readers with Commit in tests/race builds.
type commitAwareChain interface {
	BeginRead()
	EndRead()
}

// beginCommitRead acquires a shared read lock when available and returns a
// function that releases it. In race/test builds this coordinates with Commit
// to avoid read/write races in storage. In non-race builds this is a no-op.
func beginCommitRead(chain any) func() {
	if ca, ok := chain.(commitAwareChain); ok {
		ca.BeginRead()
		return func() { ca.EndRead() }
	}
	return func() {}
}
