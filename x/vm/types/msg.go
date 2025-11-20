package types

import (
	"bytes"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	protov2 "google.golang.org/protobuf/proto"

	evmapi "github.com/cosmos/evm/api/cosmos/evm/vm/v1"

	errorsmod "cosmossdk.io/errors"
	sdkmath "cosmossdk.io/math"
	txsigning "cosmossdk.io/x/tx/signing"

	"github.com/cosmos/cosmos-sdk/client"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	sdk "github.com/cosmos/cosmos-sdk/types"
	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
	signingtypes "github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
)

var (
	_ sdk.Msg    = &MsgEthereumTx{}
	_ sdk.Tx     = &MsgEthereumTx{}
	_ ante.GasTx = &MsgEthereumTx{}
	_ sdk.Msg    = &MsgUpdateParams{}
)

// message type and route constants
const (
	// TypeMsgEthereumTx defines the type string of an Ethereum transaction
	TypeMsgEthereumTx = "ethereum_tx"
)

var MsgEthereumTxCustomGetSigner = txsigning.CustomGetSigner{
	MsgType: protov2.MessageName(&evmapi.MsgEthereumTx{}),
	Fn:      evmapi.GetSigners,
}

// NewTx returns a reference to a new Ethereum transaction message.
func NewTx(tx *EvmTxArgs) *MsgEthereumTx {
	return NewTxFromArgs(tx.ToTxData())
}

func NewTxFromArgs(args *TransactionArgs) *MsgEthereumTx {
	var msg MsgEthereumTx
	msg.FromEthereumTx(args.ToTransaction(ethtypes.LegacyTxType))
	if args.From != nil {
		msg.From = args.From.Bytes()
	}
	return &msg
}

// FromEthereumTx populates the message fields from the given ethereum transaction
func (msg *MsgEthereumTx) FromEthereumTx(tx *ethtypes.Transaction) {
	msg.Raw = EthereumTx{tx}
}

// FromSignedEthereumTx populates the message fields from the given signed ethereum transaction, and set From field.
func (msg *MsgEthereumTx) FromSignedEthereumTx(tx *ethtypes.Transaction, signer ethtypes.Signer) error {
	msg.Raw.Transaction = tx

	from, err := ethtypes.Sender(signer, tx)
	if err != nil {
		return err
	}

	msg.From = from.Bytes()
	return nil
}

// Route returns the route value of an MsgEthereumTx.
func (msg MsgEthereumTx) Route() string { return RouterKey }

// Type returns the type value of an MsgEthereumTx.
func (msg MsgEthereumTx) Type() string { return TypeMsgEthereumTx }

// ValidateBasic implements the sdk.Msg interface. It performs basic validation
// checks of a Transaction. If returns an error if validation fails.
func (msg MsgEthereumTx) ValidateBasic() error {
	if msg.Raw.Transaction == nil {
		return errorsmod.Wrapf(errortypes.ErrInvalidRequest, "raw transaction is required")
	}

	if len(msg.From) == 0 {
		return errorsmod.Wrapf(errortypes.ErrInvalidRequest, "sender address is missing")
	}

	tx := msg.Raw.Transaction

	// validate the transaction
	// Transactions can't be negative. This may never happen using RLP decoded
	// transactions but may occur for transactions created using the RPC.
	if tx.Value().Sign() < 0 {
		return txpool.ErrNegativeValue
	}
	// Sanity check for extremely large numbers (supported by RLP or RPC)
	if tx.GasFeeCap().BitLen() > 256 {
		return core.ErrFeeCapVeryHigh
	}
	if tx.GasTipCap().BitLen() > 256 {
		return core.ErrTipVeryHigh
	}
	if tx.GasTipCap().Sign() < 0 {
		return fmt.Errorf("%w: gas tip cap %v, minimum needed 0", txpool.ErrTxGasPriceTooLow, tx.GasTipCap())
	}
	// Ensure gasFeeCap is greater than or equal to gasTipCap
	if tx.GasFeeCapIntCmp(tx.GasTipCap()) < 0 {
		return core.ErrTipAboveFeeCap
	}

	intrGas, err := core.IntrinsicGas(tx.Data(), tx.AccessList(), tx.SetCodeAuthorizations(), tx.To() == nil, true, true, true)
	if err != nil {
		return err
	}
	if tx.Gas() < intrGas {
		return fmt.Errorf("%w: gas %v, minimum needed %v", core.ErrIntrinsicGas, tx.Gas(), intrGas)
	}

	if tx.Type() == ethtypes.SetCodeTxType {
		if len(tx.SetCodeAuthorizations()) == 0 {
			return fmt.Errorf("set code tx must have at least one authorization tuple")
		}
	}
	return nil
}

// GetMsgs returns a single MsgEthereumTx as an sdk.Msg.
func (msg *MsgEthereumTx) GetMsgs() []sdk.Msg {
	return []sdk.Msg{msg}
}

func (msg *MsgEthereumTx) GetMsgsV2() ([]protov2.Message, error) {
	return nil, errors.New("not implemented")
}

// GetSigners returns the expected signers for an Ethereum transaction message.
// For such a message, there should exist only a single 'signer'.
func (msg *MsgEthereumTx) GetSigners() []sdk.AccAddress {
	if len(msg.From) == 0 {
		return nil
	}
	return []sdk.AccAddress{msg.GetFrom()}
}

// GetSender convert the From field to common.Address
// From should always be set, which is validated in ValidateBasic
func (msg *MsgEthereumTx) GetSender() common.Address {
	return common.BytesToAddress(msg.From)
}

// GetSenderLegacy fallbacks to old behavior if From is empty, should be used by json-rpc
func (msg *MsgEthereumTx) GetSenderLegacy(signer ethtypes.Signer) (common.Address, error) {
	if len(msg.From) > 0 {
		return msg.GetSender(), nil
	}
	sender, err := msg.recoverSender(signer)
	if err != nil {
		return common.Address{}, err
	}
	msg.From = sender.Bytes()
	return sender, nil
}

// recoverSender recovers the sender address from the transaction signature.
func (msg *MsgEthereumTx) recoverSender(signer ethtypes.Signer) (common.Address, error) {
	return ethtypes.Sender(signer, msg.AsTransaction())
}

// GetSignBytes returns the Amino bytes of an Ethereum transaction message used
// for signing.
//
// NOTE: This method cannot be used as a chain ID is needed to create valid bytes
// to sign over. Use 'RLPSignBytes' instead.
func (msg MsgEthereumTx) GetSignBytes() []byte {
	panic("must use 'RLPSignBytes' with a chain ID to get the valid bytes to sign")
}

// Sign calculates a secp256k1 ECDSA signature and signs the transaction. It
// takes a keyring signer and the chainID to sign an Ethereum transaction according to
// EIP155 standard.
// This method mutates the transaction as it populates the V, R, S
// fields of the Transaction's Signature.
// The function will fail if the sender address is not defined for the msg or if
// the sender is not registered on the keyring
func (msg *MsgEthereumTx) Sign(ethSigner ethtypes.Signer, keyringSigner keyring.Signer) error {
	from := msg.GetFrom()
	if from.Empty() {
		return fmt.Errorf("sender address not defined for message")
	}

	tx := msg.AsTransaction()
	txHash := ethSigner.Hash(tx)

	sig, _, err := keyringSigner.SignByAddress(from, txHash.Bytes(), signingtypes.SignMode_SIGN_MODE_TEXTUAL)
	if err != nil {
		return err
	}

	tx, err = tx.WithSignature(ethSigner, sig)
	if err != nil {
		return err
	}

	return msg.FromSignedEthereumTx(tx, ethSigner)
}

// GetGas implements the GasTx interface. It returns the GasLimit of the transaction.
func (msg MsgEthereumTx) GetGas() uint64 {
	return msg.Raw.Gas()
}

// GetFee returns the fee for non dynamic fee tx
func (msg MsgEthereumTx) GetFee() *big.Int {
	i := new(big.Int).SetUint64(msg.Raw.Gas())
	return i.Mul(i, msg.Raw.GasPrice())
}

// GetEffectiveFee returns the fee for dynamic fee tx
func (msg MsgEthereumTx) GetEffectiveFee(baseFee *big.Int) *big.Int {
	i := new(big.Int).SetUint64(msg.Raw.Gas())
	gasTip, _ := msg.Raw.EffectiveGasTip(baseFee)
	effectiveGasPrice := new(big.Int).Add(gasTip, baseFee)
	return i.Mul(i, effectiveGasPrice)
}

// GetFrom loads the ethereum sender address from the sigcache and returns an
// sdk.AccAddress from its bytes
func (msg *MsgEthereumTx) GetFrom() sdk.AccAddress {
	return sdk.AccAddress(msg.From)
}

// AsTransaction creates an Ethereum Transaction type from the msg fields
func (msg MsgEthereumTx) AsTransaction() *ethtypes.Transaction {
	return msg.Raw.Transaction
}

// AsMessage vendors the core.TransactionToMessage function to avoid sender recovery,
// assume the From field is set correctly in the MsgEthereumTx.
func (msg MsgEthereumTx) AsMessage(baseFee *big.Int) *core.Message {
	tx := msg.AsTransaction()
	ethMsg := &core.Message{
		Nonce:                 tx.Nonce(),
		GasLimit:              tx.Gas(),
		GasPrice:              new(big.Int).Set(tx.GasPrice()),
		GasFeeCap:             new(big.Int).Set(tx.GasFeeCap()),
		GasTipCap:             new(big.Int).Set(tx.GasTipCap()),
		To:                    tx.To(),
		Value:                 tx.Value(),
		Data:                  tx.Data(),
		AccessList:            tx.AccessList(),
		SetCodeAuthorizations: tx.SetCodeAuthorizations(),
		SkipNonceChecks:       false,
		SkipFromEOACheck:      false,
		BlobHashes:            tx.BlobHashes(),
		BlobGasFeeCap:         tx.BlobGasFeeCap(),
	}
	// If baseFee provided, set gasPrice to effectiveGasPrice.
	if baseFee != nil {
		ethMsg.GasPrice = ethMsg.GasPrice.Add(ethMsg.GasTipCap, baseFee)
		if ethMsg.GasPrice.Cmp(ethMsg.GasFeeCap) > 0 {
			ethMsg.GasPrice = ethMsg.GasFeeCap
		}
	}
	ethMsg.From = msg.GetSender()
	return ethMsg
}

// VerifySender verify the sender address against the signature values using the latest signer for the given chainID.
func (msg *MsgEthereumTx) VerifySender(signer ethtypes.Signer) error {
	from, err := msg.recoverSender(signer)
	if err != nil {
		return err
	}

	if !bytes.Equal(msg.From, from.Bytes()) {
		return fmt.Errorf("sender verification failed. got %s, expected %s", from.String(), HexAddress(msg.From))
	}
	return nil
}

// UnmarshalBinary decodes the canonical encoding of transactions.
func (msg *MsgEthereumTx) UnmarshalBinary(b []byte, signer ethtypes.Signer) error {
	tx := &ethtypes.Transaction{}
	if err := tx.UnmarshalBinary(b); err != nil {
		return err
	}
	return msg.FromSignedEthereumTx(tx, signer)
}

func (msg *MsgEthereumTx) Hash() common.Hash {
	return msg.AsTransaction().Hash()
}

// BuildTx builds the canonical cosmos tx from ethereum msg
func (msg *MsgEthereumTx) BuildTx(b client.TxBuilder, evmDenom string) (signing.Tx, error) {
	return msg.BuildTxWithEvmParams(b, Params{
		EvmDenom: evmDenom,
		ExtendedDenomOptions: &ExtendedDenomOptions{
			ExtendedDenom: GetEVMCoinExtendedDenom(),
		},
	})
}

func (msg *MsgEthereumTx) BuildTxWithEvmParams(b client.TxBuilder, params Params) (signing.Tx, error) {
	builder, ok := b.(authtx.ExtensionOptionsTxBuilder)
	if !ok {
		return nil, errors.New("unsupported builder")
	}

	option, err := codectypes.NewAnyWithValue(&ExtensionOptionsEthereumTx{})
	if err != nil {
		return nil, err
	}

	fees := make(sdk.Coins, 0, 1)
	feeAmt := sdkmath.NewIntFromBigInt(msg.GetFee())
	if feeAmt.Sign() > 0 {
		fees = append(fees, sdk.NewCoin(params.EvmDenom, feeAmt))
		fees = ConvertCoinsDenomToExtendedDenomWithEvmParams(fees, params)
	}

	builder.SetExtensionOptions(option)

	// only keep the nessessary fields
	err = builder.SetMsgs(&MsgEthereumTx{
		From: msg.From,
		Raw:  msg.Raw,
	})
	if err != nil {
		return nil, err
	}
	builder.SetFeeAmount(fees)
	builder.SetGasLimit(msg.GetGas())
	tx := builder.GetTx()
	return tx, nil
}

// ValidateBasic does a sanity check of the provided data
func (m *MsgUpdateParams) ValidateBasic() error {
	if _, err := sdk.AccAddressFromBech32(m.Authority); err != nil {
		return errorsmod.Wrap(err, "invalid authority address")
	}

	return m.Params.Validate()
}

// GetSignBytes implements the LegacyMsg interface.
func (m MsgUpdateParams) GetSignBytes() []byte {
	return AminoCdc.MustMarshalJSON(&m)
}
