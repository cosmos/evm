package indexer

import (
	"bytes"
	"encoding/binary"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"

	abci "github.com/cometbft/cometbft/abci/types"
)

// BlockPhase represents the execution phase of a block
type BlockPhase string

const (
	BlockPhasePreBlock   BlockPhase = "preblock"
	BlockPhaseBeginBlock BlockPhase = "beginblock"
	BlockPhaseEndBlock   BlockPhase = "endblock"
)

// CosmosEventTransformer transforms cosmos events into EVM receipt/tx data
type CosmosEventTransformer interface {
	// CanHandle returns true if this transformer can handle the given event type
	CanHandle(eventType string) bool

	// Transform converts a cosmos event into EVM receipt/tx data.
	// ethTxHash is provided by the caller to ensure consistency.
	Transform(event abci.Event, height int64, ethTxHash common.Hash) (*TransformedTxData, error)
}

// GenerateTransformedEthTxHash generates a deterministic eth tx hash by length-prefixing and hashing inputs.
// Used for cosmos txs (cosmosTxHash) and block phases (phase + blockHash).
// Length-prefixing prevents hash collisions from different input groupings.
func GenerateTransformedEthTxHash(data ...[]byte) common.Hash {
	var buf bytes.Buffer
	for _, d := range data {
		_ = binary.Write(&buf, binary.BigEndian, uint32(len(d)))
		buf.Write(d)
	}
	return crypto.Keccak256Hash(buf.Bytes())
}
