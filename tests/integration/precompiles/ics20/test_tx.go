package ics20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/holiman/uint256"

	"github.com/cosmos/evm"
	cmn "github.com/cosmos/evm/precompiles/common"
	"github.com/cosmos/evm/precompiles/ics20"
	precompileTestutil "github.com/cosmos/evm/precompiles/testutil"
	"github.com/cosmos/evm/testutil"
	evmibctesting "github.com/cosmos/evm/testutil/ibc"
	"github.com/cosmos/evm/testutil/tx"
	transfertypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	clienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

type testCase struct {
	name              string
	port              string
	channelID         string
	useDynamicChannel bool
	overrideSender    bool
	receiver          string
	wantErr           error
}

func (s *PrecompileTestSuite) TestTransferErrors() {
	evmAppA := s.chainA.App.(evm.EvmApp)
	denom, err := evmAppA.GetStakingKeeper().BondDenom(s.chainA.GetContext())
	s.Require().NoError(err)

	timeoutHeight := clienttypes.NewHeight(1, 110)
	amount := sdkmath.NewInt(1)
	defaultSender := common.BytesToAddress(s.chainA.SenderAccount.GetAddress().Bytes())
	defaultReceiver := s.chainB.SenderAccount.GetAddress().String()
	badSender := tx.GenerateAddress()

	tests := []testCase{
		{
			name:      "invalid source channel",
			port:      transfertypes.PortID,
			channelID: "invalid/channel",
			receiver:  defaultReceiver,
			wantErr: cmn.NewRevertWithSolidityError(
				ics20.ABI,
				cmn.SolidityErrMsgServerFailed,
				ics20.TransferMethod,
				"invalid source channel ID invalid/channel: identifier invalid/channel cannot contain separator '/': invalid identifier",
			),
		},
		{
			name:      "channel not found",
			port:      transfertypes.PortID,
			channelID: "channel-9",
			receiver:  defaultReceiver,
			// Err comes from validateV1TransferChannel -> Wrapf(ErrChannelNotFound, "port ID (...) channel ID (...)")
			wantErr: cmn.NewRevertWithSolidityError(
				ics20.ABI,
				cmn.SolidityErrMsgServerFailed,
				ics20.TransferMethod,
				"port ID (transfer) channel ID (channel-9): channel not found",
			),
		},
		{
			name:              "invalid receiver",
			port:              transfertypes.PortID,
			useDynamicChannel: true,
			receiver:          "",
			// CreateAndValidateMsgTransfer -> MsgTransfer.ValidateBasic error
			wantErr: cmn.NewRevertWithSolidityError(
				ics20.ABI,
				cmn.SolidityErrMsgServerFailed,
				ics20.TransferMethod,
				"missing recipient address: invalid address",
			),
		},
		{
			name:              "msg sender is not a contract caller",
			port:              transfertypes.PortID,
			useDynamicChannel: true,
			overrideSender:    true,
			receiver:          defaultReceiver,
			wantErr:           cmn.NewRevertWithSolidityError(ics20.ABI, cmn.SolidityErrRequesterIsNotMsgSender, defaultSender, badSender),
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()

			path := evmibctesting.NewTransferPath(s.chainA, s.chainB)
			path.Setup()

			channel := tc.channelID
			if tc.useDynamicChannel {
				channel = path.EndpointA.ChannelID
			}

			sender := defaultSender
			if tc.overrideSender {
				sender = badSender
			}

			args := []interface{}{
				tc.port,
				channel,
				denom,
				amount.BigInt(),
				sender,
				tc.receiver,
				timeoutHeight,
				uint64(0),
				"",
			}

			ctx := s.chainA.GetContext()
			stateDB := testutil.NewStateDB(ctx, evmAppA.GetEVMKeeper())
			method := s.chainAPrecompile.Methods[ics20.TransferMethod]

			contract := vm.NewContract(defaultSender, s.chainAPrecompile.Address(), uint256.NewInt(0), uint64(1_000_000), nil)
			_, err = s.chainAPrecompile.Transfer(ctx, contract, stateDB, &method, args)
			precompileTestutil.RequireExactError(s.T(), err, tc.wantErr)
		})
	}
}

func (s *PrecompileTestSuite) TestTransfer() {
	path := evmibctesting.NewTransferPath(s.chainA, s.chainB)
	path.Setup()

	evmAppA := s.chainA.App.(evm.EvmApp)
	denom, err := evmAppA.GetStakingKeeper().BondDenom(s.chainA.GetContext())
	s.Require().NoError(err)

	amount := sdkmath.NewInt(5)
	sourceAddr := common.BytesToAddress(s.chainA.SenderAccount.GetAddress().Bytes())
	receiver := s.chainB.SenderAccount.GetAddress().String()
	timeoutHeight := clienttypes.NewHeight(1, 110)

	sourcePort := path.EndpointA.ChannelConfig.PortID
	sourceChannel := path.EndpointA.ChannelID
	data, err := s.chainAPrecompile.ABI.Pack(
		"transfer",
		sourcePort,
		sourceChannel,
		denom,
		amount.BigInt(),
		sourceAddr,
		receiver,
		timeoutHeight,
		uint64(0),
		"",
	)
	s.Require().NoError(err)

	res, _, _, err := s.chainA.SendEvmTx(
		s.chainA.SenderAccounts[0],
		0,
		s.chainAPrecompile.Address(),
		big.NewInt(0),
		data,
		0,
	)
	s.Require().NoError(err)

	packet, err := evmibctesting.ParsePacketFromEvents(res.Events)
	s.Require().NoError(err)

	err = path.RelayPacket(packet)
	s.Require().NoError(err)

	trace := transfertypes.NewHop(path.EndpointB.ChannelConfig.PortID, path.EndpointB.ChannelID)
	chainBDenom := transfertypes.NewDenom(denom, trace)
	evmAppB := s.chainB.App.(evm.EvmApp)
	balance := evmAppB.GetBankKeeper().GetBalance(
		s.chainB.GetContext(),
		s.chainB.SenderAccount.GetAddress(),
		chainBDenom.IBCDenom(),
	)
	s.Require().Equal(sdk.NewCoin(chainBDenom.IBCDenom(), amount), balance)
}
