package evmd

import (
	"testing"

	"github.com/stretchr/testify/require"
	protov2 "google.golang.org/protobuf/proto"

	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/log/v2"

	"github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

// stubTx is a minimal sdk.Tx whose identity is its pointer.
type stubTx struct {
	msgs []sdk.Msg
}

func (s *stubTx) GetMsgs() []sdk.Msg                    { return s.msgs }
func (s *stubTx) GetMsgsV2() ([]protov2.Message, error) { return nil, nil }

// recordingVerifier records which txs reached full verification and which
// were only encoded.
type recordingVerifier struct {
	baseapp.ProposalTxVerifier // unused methods
	verified                   []sdk.Tx
	encoded                    []sdk.Tx
}

func (v *recordingVerifier) PrepareProposalVerifyTx(tx sdk.Tx) ([]byte, error) {
	v.verified = append(v.verified, tx)
	return []byte("verified"), nil
}

func (v *recordingVerifier) TxEncode(tx sdk.Tx) ([]byte, error) {
	v.encoded = append(v.encoded, tx)
	return []byte("encoded"), nil
}

// stampedSnapshot maps txs to the height they were validated at.
type stampedSnapshot map[sdk.Tx]uint64

func (s stampedSnapshot) ProposalTxValidatedAt(tx sdk.Tx, base uint64) bool {
	height, ok := s[tx]
	return ok && height == base
}

func TestSnapshotVerifiedTxVerifier(t *testing.T) {
	const base = int64(10)

	var (
		fresh1 = &stubTx{}
		fresh2 = &stubTx{}
		stale1 = &stubTx{}
		stale2 = &stubTx{}
		evm    = &stubTx{msgs: []sdk.Msg{&evmtypes.MsgEthereumTx{}}}
	)
	snapshot := stampedSnapshot{
		fresh1: uint64(base),
		fresh2: uint64(base),
		stale1: uint64(base) - 1,
		stale2: uint64(base) - 1,
		evm:    uint64(base),
	}

	testCases := []struct {
		name         string
		base         int64
		txs          []sdk.Tx
		wantVerified []sdk.Tx
		wantEncoded  []sdk.Tx
	}{
		{
			name:        "txs validated at base are encoded, not verified",
			base:        base,
			txs:         []sdk.Tx{fresh1, fresh2},
			wantEncoded: []sdk.Tx{fresh1, fresh2},
		},
		{
			name:         "encoded txs are verified ahead of the stale tx that follows",
			base:         base,
			txs:          []sdk.Tx{fresh1, fresh2, stale1},
			wantVerified: []sdk.Tx{fresh1, fresh2, stale1},
			wantEncoded:  []sdk.Tx{fresh1, fresh2},
		},
		{
			name:         "each encoded tx is replayed once",
			base:         base,
			txs:          []sdk.Tx{fresh1, stale1, fresh2, stale2},
			wantVerified: []sdk.Tx{fresh1, stale1, fresh2, stale2},
			wantEncoded:  []sdk.Tx{fresh1, fresh2},
		},
		{
			name:         "an unknown base verifies everything",
			base:         0,
			txs:          []sdk.Tx{fresh1, stale1},
			wantVerified: []sdk.Tx{fresh1, stale1},
		},
		{
			name:         "evm txs are encoded but never replayed",
			base:         base,
			txs:          []sdk.Tx{evm, stale1},
			wantVerified: []sdk.Tx{stale1},
			wantEncoded:  []sdk.Tx{evm},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			recorder := &recordingVerifier{}
			verifier := NewSnapshotVerifiedTxVerifier(recorder, snapshot, log.NewNopLogger())
			verifier.SetProposalBase(tc.base)

			for _, tx := range tc.txs {
				_, err := verifier.PrepareProposalVerifyTx(tx)
				require.NoError(t, err)
			}

			require.Equal(t, tc.wantVerified, recorder.verified)
			require.Equal(t, tc.wantEncoded, recorder.encoded)
		})
	}
}

// A new proposal rebuilds the prepare-proposal state, so txs encoded for the
// previous proposal must not be replayed into it.
func TestSnapshotVerifiedTxVerifier_NewProposalDropsQueue(t *testing.T) {
	const base = int64(10)
	fresh := &stubTx{}
	stale := &stubTx{}
	snapshot := stampedSnapshot{fresh: uint64(base), stale: uint64(base) - 1}

	recorder := &recordingVerifier{}
	verifier := NewSnapshotVerifiedTxVerifier(recorder, snapshot, log.NewNopLogger())

	verifier.SetProposalBase(base)
	_, err := verifier.PrepareProposalVerifyTx(fresh)
	require.NoError(t, err)
	require.Empty(t, recorder.verified)

	verifier.SetProposalBase(base)
	_, err = verifier.PrepareProposalVerifyTx(stale)
	require.NoError(t, err)
	require.Equal(t, []sdk.Tx{stale}, recorder.verified, "previous proposal's queue must not be replayed")
}
