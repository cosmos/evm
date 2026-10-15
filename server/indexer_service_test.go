package server

import (
	"context"
	"errors"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"

	abci "github.com/cometbft/cometbft/abci/types"
	coretypes "github.com/cometbft/cometbft/rpc/core/types"
	cmttypes "github.com/cometbft/cometbft/types"

	servertypes "github.com/cosmos/evm/server/types"
)

// fakeBlockSource serves every height; fetching fails for heights in failFetch.
type fakeBlockSource struct {
	failFetch map[int64]bool
}

func (f *fakeBlockSource) Block(_ context.Context, h *int64) (*coretypes.ResultBlock, error) {
	if f.failFetch[*h] {
		return nil, errors.New("block not found")
	}
	return &coretypes.ResultBlock{Block: &cmttypes.Block{Header: cmttypes.Header{Height: *h}}}, nil
}

func (f *fakeBlockSource) BlockResults(_ context.Context, h *int64) (*coretypes.ResultBlockResults, error) {
	return &coretypes.ResultBlockResults{Height: *h}, nil
}

// fakeIndexer records indexed heights; IndexBlock fails once for each height in failOnce.
type fakeIndexer struct {
	indexed  []int64
	failOnce map[int64]bool
}

func (f *fakeIndexer) LastIndexedBlock() (int64, error) { return -1, nil }

func (f *fakeIndexer) IndexBlock(b *cmttypes.Block, _ []*abci.ExecTxResult) error {
	if f.failOnce[b.Height] {
		delete(f.failOnce, b.Height)
		return errors.New("write batch: no space left on device")
	}
	f.indexed = append(f.indexed, b.Height)
	return nil
}

func (f *fakeIndexer) GetByTxHash(common.Hash) (*servertypes.TxResult, error) { return nil, nil }
func (f *fakeIndexer) GetByBlockAndIndex(int64, int32) (*servertypes.TxResult, error) {
	return nil, nil
}

// A height whose IndexBlock fails must be retried, not skipped: before the
// fix the failed height was passed over and its transactions never indexed.
func TestIndexBlocksRetriesFailedIndex(t *testing.T) {
	idx := &fakeIndexer{failOnce: map[int64]bool{3: true}}
	eis := NewEVMIndexerService(idx, nil)
	src := &fakeBlockSource{}

	last, err := eis.indexBlocks(context.Background(), src, 0, 5)
	require.Error(t, err)
	require.Equal(t, int64(2), last, "stops before the height that failed to index")
	require.Equal(t, []int64{1, 2}, idx.indexed)

	last, err = eis.indexBlocks(context.Background(), src, last, 5)
	require.NoError(t, err)
	require.Equal(t, int64(5), last)
	require.Equal(t, []int64{1, 2, 3, 4, 5}, idx.indexed, "the failed height is indexed on retry, none skipped")
}

// A height that cannot be fetched stops the pass at the previous height.
func TestIndexBlocksStopsAtFetchError(t *testing.T) {
	idx := &fakeIndexer{}
	eis := NewEVMIndexerService(idx, nil)
	src := &fakeBlockSource{failFetch: map[int64]bool{4: true}}

	last, err := eis.indexBlocks(context.Background(), src, 0, 6)
	require.Error(t, err)
	require.Equal(t, int64(3), last)

	delete(src.failFetch, 4)
	last, err = eis.indexBlocks(context.Background(), src, last, 6)
	require.NoError(t, err)
	require.Equal(t, int64(6), last)
	require.Equal(t, []int64{1, 2, 3, 4, 5, 6}, idx.indexed)
}
