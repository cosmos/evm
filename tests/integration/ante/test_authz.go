package ante

import (
	"fmt"
	"math/big"
	"time"

	ethtypes "github.com/ethereum/go-ethereum/core/types"

	abci "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/testutil/integration/base/factory"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	sdkvesting "github.com/cosmos/cosmos-sdk/x/auth/vesting/types"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

func (suite *AnteTestSuite) TestRejectMsgsInAuthz() {
	_, testAddresses, err := testutil.GeneratePrivKeyAddressPairs(10)
	suite.Require().NoError(err)

	var gasLimit uint64 = 1000000
	distantFuture := time.Date(9000, 1, 1, 0, 0, 0, 0, time.UTC)

	nw := suite.GetNetwork()
	evmDenom := evmtypes.GetEVMCoinDenom()

	baseFeeRes, err := nw.GetEvmClient().BaseFee(nw.GetContext(), &evmtypes.QueryBaseFeeRequest{})
	suite.Require().NoError(err, "failed to get base fee")

	// create a dummy MsgEthereumTx for the test
	// otherwise throws error that cannot unpack tx data
	msgEthereumTx := evmtypes.NewTx(&evmtypes.EvmTxArgs{
		ChainID:   nw.GetEIP155ChainID(),
		Nonce:     0,
		GasLimit:  gasLimit,
		GasFeeCap: baseFeeRes.BaseFee.BigInt(),
		GasTipCap: big.NewInt(1),
		Input:     nil,
		Accesses:  &ethtypes.AccessList{},
	})

	newMsgGrant := func(msgTypeUrl string) *authz.MsgGrant {
		msg, err := authz.NewMsgGrant(
			testAddresses[0],
			testAddresses[1],
			authz.NewGenericAuthorization(msgTypeUrl),
			&distantFuture,
		)
		if err != nil {
			panic(err)
		}
		return msg
	}

	testcases := []struct {
		name         string
		msgs         []sdk.Msg
		expectedCode uint32
		isEIP712     bool
	}{
		{
			name:         "a MsgGrant with MsgEthereumTx typeURL on the authorization field is blocked",
			msgs:         []sdk.Msg{newMsgGrant(sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}))},
			expectedCode: sdkerrors.ErrUnauthorized.ABCICode(),
		},
		{
			name:         "a MsgGrant with MsgCreateVestingAccount typeURL on the authorization field is blocked",
			msgs:         []sdk.Msg{newMsgGrant(sdk.MsgTypeURL(&sdkvesting.MsgCreateVestingAccount{}))},
			expectedCode: sdkerrors.ErrUnauthorized.ABCICode(),
		},
		{
			name:         "a MsgGrant with MsgEthereumTx typeURL on the authorization field included on EIP712 tx is blocked",
			msgs:         []sdk.Msg{newMsgGrant(sdk.MsgTypeURL(&evmtypes.MsgEthereumTx{}))},
			expectedCode: sdkerrors.ErrUnauthorized.ABCICode(),
			isEIP712:     true,
		},
		{
			name: "a MsgExec with nested messages (valid: MsgSend and invalid: MsgEthereumTx) is blocked",
			msgs: []sdk.Msg{
				testutil.NewMsgExec(
					testAddresses[1],
					[]sdk.Msg{
						banktypes.NewMsgSend(
							testAddresses[0],
							testAddresses[3],
							sdk.NewCoins(sdk.NewInt64Coin(evmDenom, 100e6)),
						),
						msgEthereumTx,
					},
				),
			},
			expectedCode: sdkerrors.ErrUnauthorized.ABCICode(),
		},
		{
			name: "a MsgExec with nested MsgExec messages that has invalid messages is blocked",
			msgs: []sdk.Msg{
				testutil.CreateNestedMsgExec(
					testAddresses[1],
					2,
					[]sdk.Msg{
						msgEthereumTx,
					},
				),
			},
			expectedCode: sdkerrors.ErrUnauthorized.ABCICode(),
		},
		{
			name: "a MsgExec with more nested MsgExec messages than allowed and with valid messages is blocked",
			msgs: []sdk.Msg{
				testutil.CreateNestedMsgExec(
					testAddresses[1],
					6,
					[]sdk.Msg{
						banktypes.NewMsgSend(
							testAddresses[0],
							testAddresses[3],
							sdk.NewCoins(sdk.NewInt64Coin(evmDenom, 100e6)),
						),
					},
				),
			},
			expectedCode: sdkerrors.ErrUnauthorized.ABCICode(),
		},
		{
			name: "two MsgExec messages NOT containing a blocked msg but between the two have more nesting than the allowed. Then, is blocked",
			msgs: []sdk.Msg{
				testutil.CreateNestedMsgExec(
					testAddresses[1],
					5,
					[]sdk.Msg{
						banktypes.NewMsgSend(
							testAddresses[0],
							testAddresses[3],
							sdk.NewCoins(sdk.NewInt64Coin(evmDenom, 100e6)),
						),
					},
				),
				testutil.CreateNestedMsgExec(
					testAddresses[1],
					5,
					[]sdk.Msg{
						banktypes.NewMsgSend(
							testAddresses[0],
							testAddresses[3],
							sdk.NewCoins(sdk.NewInt64Coin(evmDenom, 100e6)),
						),
					},
				),
			},
			expectedCode: sdkerrors.ErrUnauthorized.ABCICode(),
		},
	}

	for _, tc := range testcases {
		suite.Run(fmt.Sprintf("Case %s", tc.name), func() {
			suite.SetupTest()
			nw = suite.GetNetwork()
			var (
				tx  sdk.Tx
				err error
			)
			ctx := nw.GetContext()
			priv := suite.GetKeyring().GetPrivKey(0)

			if tc.isEIP712 {
				coinAmount := sdk.NewCoin(evmDenom, math.NewInt(20))
				fees := sdk.NewCoins(coinAmount)
				cosmosTxArgs := utiltx.CosmosTxArgs{
					TxCfg:   suite.GetClientCtx().TxConfig,
					Priv:    priv,
					ChainID: ctx.ChainID(),
					Gas:     200000,
					Fees:    fees,
					Msgs:    tc.msgs,
				}

				tx, err = utiltx.CreateEIP712CosmosTx(
					ctx,
					nw.App,
					utiltx.EIP712TxArgs{
						CosmosTxArgs:       cosmosTxArgs,
						UseLegacyTypedData: true,
					},
				)
			} else {
				tx, err = suite.GetTxFactory().BuildCosmosTx(
					priv,
					factory.CosmosTxArgs{
						Gas:  &gasLimit,
						Msgs: tc.msgs,
					},
				)
			}
			suite.Require().NoError(err)

			txEncoder := suite.GetClientCtx().TxConfig.TxEncoder()
			bz, err := txEncoder(tx)
			suite.Require().NoError(err)

			resCheckTx, err := nw.App.CheckTx(
				&abci.RequestCheckTx{
					Tx:   bz,
					Type: abci.CheckTxType_New,
				},
			)
			suite.Require().NoError(err)
			suite.Require().Equal(resCheckTx.Code, tc.expectedCode, resCheckTx.Log)

			header := ctx.BlockHeader()
			blockRes, err := nw.App.FinalizeBlock(
				&abci.RequestFinalizeBlock{
					Height:             ctx.BlockHeight() + 1,
					Txs:                [][]byte{bz},
					Hash:               header.AppHash,
					NextValidatorsHash: header.NextValidatorsHash,
					ProposerAddress:    header.ProposerAddress,
					Time:               header.Time.Add(time.Second),
				},
			)
			suite.Require().NoError(err)
			suite.Require().Len(blockRes.TxResults, 1)
			txRes := blockRes.TxResults[0]
			suite.Require().Equal(txRes.Code, tc.expectedCode, txRes.Log)
		})
	}
}
