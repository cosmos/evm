package vm

import (
	"encoding/json"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	ethparams "github.com/ethereum/go-ethereum/params"

	"github.com/cosmos/evm/server/config"
	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// historyContractGet calls get(height) on the EIP-2935 history storage contract.
func (s *KeeperTestSuite) historyContractGet(ctx sdk.Context, height uint64) common.Hash {
	to := ethparams.HistoryStorageAddress
	from := s.Keyring.GetAddr(0)
	data := hexutil.Bytes(common.LeftPadBytes(new(big.Int).SetUint64(height).Bytes(), 32))
	args, err := json.Marshal(&types.TransactionArgs{From: &from, To: &to, Data: &data})
	s.Require().NoError(err)

	res, err := s.Network.App.GetEVMKeeper().EthCall(ctx, &types.EthCallRequest{Args: args, GasCap: config.DefaultGasCap})
	s.Require().NoError(err)
	s.Require().Empty(res.VmError, "history storage contract reverted for height %d", height)
	return common.BytesToHash(res.Ret)
}

func blockHashAt(height uint64) common.Hash {
	return common.BytesToHash(crypto.Keccak256([]byte(fmt.Sprintf("block-%d", height))))
}

func (s *KeeperTestSuite) TestHistoryStorageContractMatchesBlockHash() {
	s.SetupTest()
	k := s.Network.App.GetEVMKeeper()
	ctx := s.Network.GetContext()
	s.Require().True(k.IsContract(ctx, ethparams.HistoryStorageAddress), "history storage contract is not preinstalled")

	// heights around the ring buffer boundary of the contract (8191)
	const head = uint64(8193)
	for height := uint64(8189); height <= head; height++ {
		k.SetHeaderHash(ctx.WithBlockHeight(int64(height)).WithHeaderHash(blockHashAt(height).Bytes()))
	}

	headCtx := ctx.WithBlockHeight(int64(head)).WithHeaderHash(blockHashAt(head).Bytes())
	for height := uint64(8189); height < head; height++ {
		s.Require().Equal(blockHashAt(height), k.GetHashFn(headCtx)(height), "BLOCKHASH(%d)", height)
		s.Require().Equal(blockHashAt(height), s.historyContractGet(headCtx, height), "history contract get(%d)", height)
	}
}

func (s *KeeperTestSuite) TestUpdateParamsReindexesHistoryStorage() {
	s.SetupTest()
	k := s.Network.App.GetEVMKeeper()
	ctx := s.Network.GetContext()

	// a chain that ran with a window that doesn't match the contract
	params := k.GetParams(ctx)
	params.HistoryServeWindow = 8192
	s.Require().NoError(k.SetParams(ctx, params))

	const head = uint64(9000)
	for height := head - 300; height <= head; height++ {
		k.SetHeaderHash(ctx.WithBlockHeight(int64(height)).WithHeaderHash(blockHashAt(height).Bytes()))
	}

	// switch to the contract's window in block `head`, after its hash was stored
	headCtx := ctx.WithBlockHeight(int64(head)).WithHeaderHash(blockHashAt(head).Bytes())
	params.HistoryServeWindow = ethparams.HistoryServeWindow
	_, err := k.UpdateParams(headCtx, &types.MsgUpdateParams{Authority: k.GetAuthority().String(), Params: params})
	s.Require().NoError(err)

	nextCtx := ctx.WithBlockHeight(int64(head + 1)).WithHeaderHash(blockHashAt(head + 1).Bytes())
	k.SetHeaderHash(nextCtx)
	for height := head - 300; height <= head; height++ {
		s.Require().Equal(blockHashAt(height), k.GetHashFn(nextCtx)(height), "BLOCKHASH(%d)", height)
		s.Require().Equal(blockHashAt(height), s.historyContractGet(nextCtx, height), "history contract get(%d)", height)
	}
}

func (s *KeeperTestSuite) TestUpdateParamsExpandingHistoryWindowClearsStaleHashes() {
	s.SetupTest()
	k := s.Network.App.GetEVMKeeper()
	ctx := s.Network.GetContext()

	params := k.GetParams(ctx)
	params.HistoryServeWindow = 100
	s.Require().NoError(k.SetParams(ctx, params))

	// with a window of 100, slot 0 holds the hash of 8300, which is slot 109 in a window of 8191
	const head = uint64(8300)
	for height := head - 199; height <= head; height++ {
		k.SetHeaderHash(ctx.WithBlockHeight(int64(height)).WithHeaderHash(blockHashAt(height).Bytes()))
	}

	headCtx := ctx.WithBlockHeight(int64(head)).WithHeaderHash(blockHashAt(head).Bytes())
	params.HistoryServeWindow = ethparams.HistoryServeWindow
	_, err := k.UpdateParams(headCtx, &types.MsgUpdateParams{Authority: k.GetAuthority().String(), Params: params})
	s.Require().NoError(err)

	nextCtx := ctx.WithBlockHeight(int64(head + 1)).WithHeaderHash(blockHashAt(head + 1).Bytes())
	k.SetHeaderHash(nextCtx)
	// the last 100 hashes are kept
	for height := head - 99; height <= head; height++ {
		s.Require().Equal(blockHashAt(height), k.GetHashFn(nextCtx)(height), "BLOCKHASH(%d)", height)
		s.Require().Equal(blockHashAt(height), s.historyContractGet(nextCtx, height), "history contract get(%d)", height)
	}
	// older heights are no longer known, and must not return a hash of another height
	for height := head - 199; height < head-99; height++ {
		s.Require().Equal(common.Hash{}, s.historyContractGet(nextCtx, height), "history contract get(%d)", height)
	}
	for height := uint64(8110); height < head-199; height++ {
		s.Require().Equal(common.Hash{}, s.historyContractGet(nextCtx, height), "history contract get(%d)", height)
	}
}

func (s *KeeperTestSuite) TestUpdateParamsRejectsHistoryWindowAboveMax() {
	s.SetupTest()
	k := s.Network.App.GetEVMKeeper()
	ctx := s.Network.GetContext()

	params := k.GetParams(ctx)
	params.HistoryServeWindow = types.MaxHistoryServeWindow + 1
	_, err := k.UpdateParams(ctx, &types.MsgUpdateParams{Authority: k.GetAuthority().String(), Params: params})
	s.Require().ErrorContains(err, "exceeds the maximum")
	s.Require().Equal(uint64(types.DefaultHistoryServeWindow), k.GetParams(ctx).HistoryServeWindow)
}

func (s *KeeperTestSuite) TestUnsetHistoryWindowKeepsPreviousLayout() {
	s.SetupTest()
	k := s.Network.App.GetEVMKeeper()
	ctx := s.Network.GetContext()

	params := k.GetParams(ctx)
	params.HistoryServeWindow = 0
	s.Require().NoError(k.SetParams(ctx, params))

	// an unset window keeps meaning 8192, so a binary upgrade doesn't move the stored hashes
	const height = uint64(types.UnsetHistoryServeWindow)
	heightCtx := ctx.WithBlockHeight(int64(height)).WithHeaderHash(blockHashAt(height).Bytes())
	k.SetHeaderHash(heightCtx)
	s.Require().Equal(blockHashAt(height), k.GetState(heightCtx, ethparams.HistoryStorageAddress, common.Hash{}))
	s.Require().Equal(blockHashAt(height), k.GetHashFn(heightCtx.WithBlockHeight(int64(height+1)))(height))
}
