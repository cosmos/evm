package filters

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"
)

func TestFilterLogs(t *testing.T) {
	addr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")

	topic1 := common.HexToHash("0x1111111111111111111111111111111111111111111111111111111111111111")
	topic2 := common.HexToHash("0x2222222222222222222222222222222222222222222222222222222222222222")
	topic3 := common.HexToHash("0x3333333333333333333333333333333333333333333333333333333333333333")

	testCases := []struct {
		name      string
		logs      []*ethtypes.Log
		fromBlock *big.Int
		toBlock   *big.Int
		addresses []common.Address
		topics    [][]common.Hash
		expected  int
	}{
		{
			name: "no filters - return all logs",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1}, BlockNumber: 1},
				{Address: addr2, Topics: []common.Hash{topic2}, BlockNumber: 2},
			},
			fromBlock: nil,
			toBlock:   nil,
			addresses: nil,
			topics:    nil,
			expected:  2,
		},
		{
			name: "filter by block range",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1}, BlockNumber: 1},
				{Address: addr1, Topics: []common.Hash{topic1}, BlockNumber: 5},
				{Address: addr1, Topics: []common.Hash{topic1}, BlockNumber: 10},
			},
			fromBlock: big.NewInt(2),
			toBlock:   big.NewInt(8),
			addresses: nil,
			topics:    nil,
			expected:  1, // only block 5
		},
		{
			name: "filter by address",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1}, BlockNumber: 1},
				{Address: addr2, Topics: []common.Hash{topic1}, BlockNumber: 1},
			},
			fromBlock: nil,
			toBlock:   nil,
			addresses: []common.Address{addr1},
			topics:    nil,
			expected:  1,
		},
		{
			name: "filter by single topic - match",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1, topic2}, BlockNumber: 1},
				{Address: addr1, Topics: []common.Hash{topic2, topic1}, BlockNumber: 1},
			},
			fromBlock: nil,
			toBlock:   nil,
			addresses: nil,
			topics:    [][]common.Hash{{topic1}},
			expected:  1, // only first log matches topic1 in position 0
		},
		{
			name: "filter by single topic - no match (tests !slices.Contains branch)",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1, topic2}, BlockNumber: 1},
				{Address: addr1, Topics: []common.Hash{topic2, topic3}, BlockNumber: 1},
			},
			fromBlock: nil,
			toBlock:   nil,
			addresses: nil,
			topics:    [][]common.Hash{{topic3}}, // topic3 not in position 0 of any log
			expected:  0,
		},
		{
			name: "filter by multiple topics - all match",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1, topic2}, BlockNumber: 1},
				{Address: addr1, Topics: []common.Hash{topic1, topic3}, BlockNumber: 1},
			},
			fromBlock: nil,
			toBlock:   nil,
			addresses: nil,
			topics:    [][]common.Hash{{topic1}, {topic2}},
			expected:  1, // only first log has topic1 AND topic2
		},
		{
			name: "filter by multiple topics - second position no match (tests !slices.Contains branch)",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1, topic2}, BlockNumber: 1},
				{Address: addr1, Topics: []common.Hash{topic1, topic3}, BlockNumber: 1},
			},
			fromBlock: nil,
			toBlock:   nil,
			addresses: nil,
			topics:    [][]common.Hash{{topic1}, {topic1}}, // second position requires topic1, but logs have topic2/topic3
			expected:  0,
		},
		{
			name: "filter by OR topics - match any",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1, topic2}, BlockNumber: 1},
				{Address: addr1, Topics: []common.Hash{topic2, topic3}, BlockNumber: 1},
				{Address: addr1, Topics: []common.Hash{topic3, topic1}, BlockNumber: 1},
			},
			fromBlock: nil,
			toBlock:   nil,
			addresses: nil,
			topics:    [][]common.Hash{{topic1, topic2}}, // topic1 OR topic2 in position 0
			expected:  2,                                  // first two logs match
		},
		{
			name: "filter by OR topics in multiple positions",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1, topic2}, BlockNumber: 1},
				{Address: addr1, Topics: []common.Hash{topic2, topic1}, BlockNumber: 1},
				{Address: addr1, Topics: []common.Hash{topic1, topic3}, BlockNumber: 1},
			},
			fromBlock: nil,
			toBlock:   nil,
			addresses: nil,
			topics:    [][]common.Hash{{topic1, topic2}, {topic1, topic2}}, // (topic1 OR topic2) AND (topic1 OR topic2)
			expected:  2,                                                     // first two logs match
		},
		{
			name: "topics filter longer than log topics - skip",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1}, BlockNumber: 1},
			},
			fromBlock: nil,
			toBlock:   nil,
			addresses: nil,
			topics:    [][]common.Hash{{topic1}, {topic2}}, // requires 2 topics, but log only has 1
			expected:  0,
		},
		{
			name: "empty topic sub-array (wildcard) - matches anything in that position",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1, topic2}, BlockNumber: 1},
				{Address: addr1, Topics: []common.Hash{topic2, topic3}, BlockNumber: 1},
			},
			fromBlock: nil,
			toBlock:   nil,
			addresses: nil,
			topics:    [][]common.Hash{}, // empty topics array = no topic filtering
			expected:  2,
		},
		{
			name: "combined filters - address, block range, and topics",
			logs: []*ethtypes.Log{
				{Address: addr1, Topics: []common.Hash{topic1, topic2}, BlockNumber: 5},
				{Address: addr2, Topics: []common.Hash{topic1, topic2}, BlockNumber: 5},
				{Address: addr1, Topics: []common.Hash{topic2, topic2}, BlockNumber: 5},
				{Address: addr1, Topics: []common.Hash{topic1, topic2}, BlockNumber: 10},
			},
			fromBlock: big.NewInt(1),
			toBlock:   big.NewInt(8),
			addresses: []common.Address{addr1},
			topics:    [][]common.Hash{{topic1}},
			expected:  1, // only first log matches all criteria
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := FilterLogs(tc.logs, tc.fromBlock, tc.toBlock, tc.addresses, tc.topics)
			require.Len(t, result, tc.expected, "expected %d logs, got %d", tc.expected, len(result))
		})
	}
}
