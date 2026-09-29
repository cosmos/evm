package backend

import (
	"github.com/ethereum/go-ethereum/common/hexutil"
	"google.golang.org/grpc/metadata"

	rpcbackend "github.com/cosmos/evm/rpc/backend"
	"github.com/cosmos/evm/rpc/backend/mocks"
	utiltx "github.com/cosmos/evm/testutil/tx"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// without the app-side EVM mempool the txpool namespace reports an empty pool
func (s *TestSuite) TestTxPoolNoOpMempool() {
	s.backend.Mempool = rpcbackend.NoOpMempool{}

	// Content and ContentFrom read the current header before consulting the pool
	var header metadata.MD
	queryClient := s.backend.QueryClient.QueryClient.(*mocks.EVMQueryClient)
	client := s.backend.ClientCtx.Client.(*mocks.Client)
	height := int64(1)
	RegisterParams(queryClient, &header, height)
	RegisterBlock(client, height, nil)
	RegisterBlockResults(client, height)
	RegisterBaseFee(queryClient, math.NewInt(1))
	RegisterValidatorAccount(queryClient, sdk.AccAddress(utiltx.GenerateAddress().Bytes()))
	RegisterConsensusParams(client, height)

	content, err := s.backend.Content(s.Ctx())
	s.Require().NoError(err)
	s.Require().Empty(content[rpcbackend.StatusPending])
	s.Require().Empty(content[rpcbackend.StatusQueued])

	contentFrom, err := s.backend.ContentFrom(s.Ctx(), utiltx.GenerateAddress())
	s.Require().NoError(err)
	s.Require().Empty(contentFrom[rpcbackend.StatusPending])
	s.Require().Empty(contentFrom[rpcbackend.StatusQueued])

	inspect, err := s.backend.Inspect(s.Ctx())
	s.Require().NoError(err)
	s.Require().Empty(inspect[rpcbackend.StatusPending])
	s.Require().Empty(inspect[rpcbackend.StatusQueued])

	status, err := s.backend.Status(s.Ctx())
	s.Require().NoError(err)
	s.Require().Equal(hexutil.Uint(0), status[rpcbackend.StatusPending])
	s.Require().Equal(hexutil.Uint(0), status[rpcbackend.StatusQueued])
}
