package stream

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	coretypes "github.com/cometbft/cometbft/rpc/core/types"

	"cosmossdk.io/log/v2"
)

// failingEventsClient is a mock rpcclient.EventsClient whose Subscribe always
// returns an error.
type failingEventsClient struct{}

func (f *failingEventsClient) Subscribe(_ context.Context, _, _ string, _ ...int) (<-chan coretypes.ResultEvent, error) {
	return nil, errors.New("connection refused")
}

func (f *failingEventsClient) Unsubscribe(_ context.Context, _, _ string) error {
	return nil
}

func (f *failingEventsClient) UnsubscribeAll(_ context.Context, _ string) error {
	return nil
}

// secondCallFailEventsClient succeeds on the first Subscribe (blockEvents)
// but fails on the second (evmEvents).
type secondCallFailEventsClient struct {
	callCount int
}

func (f *secondCallFailEventsClient) Subscribe(_ context.Context, _, _ string, _ ...int) (<-chan coretypes.ResultEvent, error) {
	f.callCount++
	if f.callCount == 1 {
		ch := make(chan coretypes.ResultEvent, 1)
		return ch, nil
	}
	return nil, errors.New("evm subscription failed")
}

func (f *secondCallFailEventsClient) Unsubscribe(_ context.Context, _, _ string) error {
	return nil
}

func (f *secondCallFailEventsClient) UnsubscribeAll(_ context.Context, _ string) error {
	return nil
}

func TestInitSubscriptions_BlockSubscribeFails(t *testing.T) {
	rpcStream := NewRPCStreams(&failingEventsClient{}, log.NewNopLogger(), nil)

	require.NotPanics(t, func() {
		stream := rpcStream.HeaderStream()
		require.NotNil(t, stream, "HeaderStream should return non-nil even on subscribe failure")
	})
}

func TestInitSubscriptions_LogSubscribeFails(t *testing.T) {
	rpcStream := NewRPCStreams(&secondCallFailEventsClient{}, log.NewNopLogger(), nil)

	require.NotPanics(t, func() {
		stream := rpcStream.LogStream()
		require.NotNil(t, stream, "LogStream should return non-nil even on second subscribe failure")
	})
}
