package bank2

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/vm"

	"github.com/cosmos/evm/x/vm/statedb"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	sdkerrors "github.com/cosmos/cosmos-sdk/types/errors"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

type MockBankKeeper struct {
	// use int64 for simplicity
	balances  map[string]map[string]int64
	supplies  map[string]int64
	metadatas map[string]banktypes.Metadata
}

type MockBankMsgServer struct {
	keeper MockBankKeeper
}

var (
	_ BankKeeper    = MockBankKeeper{}
	_ BankMsgServer = MockBankMsgServer{}
)

func NewMockBankKeeper() MockBankKeeper {
	return MockBankKeeper{
		balances:  make(map[string]map[string]int64),
		supplies:  make(map[string]int64),
		metadatas: make(map[string]banktypes.Metadata),
	}
}

func NewBankMsgServer(keeper MockBankKeeper) MockBankMsgServer {
	return MockBankMsgServer{keeper}
}

func (k MockBankKeeper) registerDenom(denom string, metadata banktypes.Metadata) {
	k.metadatas[denom] = metadata
}

func (k MockBankKeeper) mint(to sdk.AccAddress, amt sdk.Coins) {
	for _, coin := range amt {
		m := k.balances[string(to)]
		if m == nil {
			m = make(map[string]int64)
			k.balances[string(to)] = m
		}
		amount := coin.Amount.Int64()
		m[coin.Denom] += amount
		k.supplies[coin.Denom] += amount
	}
}

func (k MockBankKeeper) burn(from sdk.AccAddress, amt sdk.Coins) error {
	for _, coin := range amt {
		amount := coin.Amount.Int64()
		m, ok := k.balances[string(from)]
		if !ok {
			return errorsmod.Wrapf(sdkerrors.ErrInsufficientFunds, "address: 0x%x, denom: %s, expect: %d, got: %d", from.Bytes(), coin.Denom, amount, 0)
		}
		available := m[coin.Denom]
		if available < amount {
			return errorsmod.Wrapf(sdkerrors.ErrInsufficientFunds, "address: 0x%x, denom: %s, expect: %d, got: %d", from.Bytes(), coin.Denom, amount, available)
		}
		m[coin.Denom] = available - amount
		k.supplies[coin.Denom] -= amount
	}
	return nil
}

func (k MockBankKeeper) send(from sdk.AccAddress, to sdk.AccAddress, amt sdk.Coins) error {
	if err := k.burn(from, amt); err != nil {
		return err
	}
	k.mint(to, amt)
	return nil
}

func (k MockBankKeeper) GetSupply(ctx context.Context, denom string) sdk.Coin {
	return sdk.NewCoin(denom, sdkmath.NewInt(k.supplies[denom]))
}

func (k MockBankKeeper) GetDenomMetaData(ctx context.Context, denom string) (banktypes.Metadata, bool) {
	md, ok := k.metadatas[denom]
	return md, ok
}

func (k MockBankKeeper) GetBalance(ctx context.Context, addr sdk.AccAddress, denom string) sdk.Coin {
	amount := int64(0)
	if m, ok := k.balances[string(addr)]; ok {
		amount = m[denom]
	}

	return sdk.NewCoin(denom, sdkmath.NewInt(amount))
}

func (ms MockBankMsgServer) Send(goCtx context.Context, msg *banktypes.MsgSend) (*banktypes.MsgSendResponse, error) {
	from, err := sdk.AccAddressFromBech32(msg.FromAddress)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid from address: %s", err)
	}
	to, err := sdk.AccAddressFromBech32(msg.ToAddress)
	if err != nil {
		return nil, sdkerrors.ErrInvalidAddress.Wrapf("invalid to address: %s", err)
	}
	if err := ms.keeper.send(from, to, msg.Amount); err != nil {
		return nil, err
	}
	return &banktypes.MsgSendResponse{}, nil
}

func NewMockEVM(ctx sdk.Context) *vm.EVM {
	evmKeeper := statedb.NewMockKeeper()
	db := statedb.New(ctx, evmKeeper, statedb.NewEmptyTxConfig())
	blockCtx := vm.BlockContext{
		CanTransfer: core.CanTransfer,
		Transfer:    core.Transfer,
		GetHash:     nil,
		GasLimit:    10000000,
		BlockNumber: big.NewInt(1),
		Time:        1,
		Difficulty:  big.NewInt(0), // unused. Only required in PoW context
		BaseFee:     big.NewInt(1000),
		Random:      &common.MaxHash, // need to be different than nil to signal it is after the merge and pick up the right opcodes
	}
	vmConfig := vm.Config{}
	return vm.NewEVM(blockCtx, db, evmtypes.GetEthChainConfig(), vmConfig)
}
