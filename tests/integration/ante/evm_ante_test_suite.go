package ante

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm"
	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/ethereum/eip712"
	"github.com/cosmos/evm/testutil"
	"github.com/cosmos/evm/testutil/constants"
	"github.com/cosmos/evm/testutil/integration/evm/network"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"
	ibctypes "github.com/cosmos/ibc-go/v10/modules/apps/transfer/types"
	ibcclienttypes "github.com/cosmos/ibc-go/v10/modules/core/02-client/types"

	sdkmath "cosmossdk.io/math"
	storetypes "cosmossdk.io/store/types"
	evtypes "cosmossdk.io/x/evidence/types"
	"cosmossdk.io/x/feegrant"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/crypto/types/multisig"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	sdkante "github.com/cosmos/cosmos-sdk/x/auth/ante"
	"github.com/cosmos/cosmos-sdk/x/auth/migrations/legacytx"
	authsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypesv1 "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
	teststaking "github.com/cosmos/cosmos-sdk/x/staking/testutil"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

type EvmAnteTestSuite struct {
	*AnteTestSuite
	UseLegacyEIP712TypedData bool
}

func NewEvmAnteTestSuite(create network.CreateEvmApp, options ...network.ConfigOption) *EvmAnteTestSuite {
	return &EvmAnteTestSuite{
		AnteTestSuite: NewAnteTestSuite(create),
	}
}

func (s *EvmAnteTestSuite) CreateTxBuilder(privKey cryptotypes.PrivKey, txArgs evmtypes.EvmTxArgs, unsetExtensionOptions ...bool) client.TxBuilder {
	var option *codectypes.Any
	var err error
	if len(unsetExtensionOptions) == 0 {
		option, err = codectypes.NewAnyWithValue(&evmtypes.ExtensionOptionsEthereumTx{})
		s.Require().NoError(err)
	}
	msgEthTx, err := s.GetTxFactory().GenerateMsgEthereumTx(privKey, txArgs)
	s.Require().NoError(err)

	signedMsg, err := s.GetTxFactory().SignMsgEthereumTx(privKey, msgEthTx)
	s.Require().NoError(err)
	s.Require().NoError(signedMsg.ValidateBasic())

	tb := s.GetClientCtx().TxConfig.NewTxBuilder()
	builder, ok := tb.(authtx.ExtensionOptionsTxBuilder)
	s.Require().True(ok)

	if len(unsetExtensionOptions) == 0 {
		builder.SetExtensionOptions(option)
	}

	err = builder.SetMsgs(&signedMsg)
	s.Require().NoError(err)

	ethTx := signedMsg.AsTransaction()
	s.Require().NotNil(ethTx)

	fees := sdk.NewCoins(sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewIntFromBigInt(signedMsg.GetFee())))
	builder.SetFeeAmount(fees)
	builder.SetGasLimit(signedMsg.GetGas())
	return builder
}

func (s *EvmAnteTestSuite) RequireErrorForLegacyTypedData(err error) {
	if s.UseLegacyEIP712TypedData {
		s.Require().Error(err)
	} else {
		s.Require().NoError(err)
	}
}

func (s *EvmAnteTestSuite) TxForLegacyTypedData(txBuilder client.TxBuilder) sdk.Tx {
	if s.UseLegacyEIP712TypedData {
		// Since the TxBuilder will be nil on failure,
		// we return an empty Tx to avoid panics.
		emptyTxBuilder := s.GetClientCtx().TxConfig.NewTxBuilder()
		return emptyTxBuilder.GetTx()
	}

	return txBuilder.GetTx()
}

func (s *EvmAnteTestSuite) CreateTestCosmosTxBuilder(gasPrice sdkmath.Int, denom string, msgs ...sdk.Msg) client.TxBuilder {
	txBuilder := s.GetClientCtx().TxConfig.NewTxBuilder()

	txBuilder.SetGasLimit(TestGasLimit)
	fees := &sdk.Coins{{Denom: denom, Amount: gasPrice.MulRaw(int64(TestGasLimit))}}
	txBuilder.SetFeeAmount(*fees)
	err := txBuilder.SetMsgs(msgs...)
	s.Require().NoError(err)
	return txBuilder
}

func (s *EvmAnteTestSuite) CreateTestEIP712TxBuilderMsgSend(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	// Build MsgSend
	recipient := sdk.AccAddress(common.Address{}.Bytes())
	msgSend := banktypes.NewMsgSend(from, recipient, sdk.NewCoins(sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(1))))
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgSend)
}

func (s *EvmAnteTestSuite) CreateTestEIP712TxBuilderMsgDelegate(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	// Build MsgDelegate
	val := s.GetNetwork().GetValidators()[0]
	msgDelegate := stakingtypes.NewMsgDelegate(from.String(), val.OperatorAddress, sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(20)))
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgDelegate)
}

func (s *EvmAnteTestSuite) CreateTestEIP712MsgCreateValidator(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	// Build MsgCreateValidator
	valAddr := sdk.ValAddress(from.Bytes())
	privEd := ed25519.GenPrivKey()
	evmDenom := evmtypes.GetEVMCoinDenom()
	msgCreate, err := stakingtypes.NewMsgCreateValidator(
		valAddr.String(),
		privEd.PubKey(),
		sdk.NewCoin(evmDenom, sdkmath.NewInt(20)),
		stakingtypes.NewDescription("moniker", "identity", "website", "security_contract", "details"),
		stakingtypes.NewCommissionRates(sdkmath.LegacyOneDec(), sdkmath.LegacyOneDec(), sdkmath.LegacyOneDec()),
		sdkmath.OneInt(),
	)
	s.Require().NoError(err)
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgCreate)
}

func (s *EvmAnteTestSuite) CreateTestEIP712MsgCreateValidator2(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	// Build MsgCreateValidator
	valAddr := sdk.ValAddress(from.Bytes())
	privEd := ed25519.GenPrivKey()
	msgCreate, err := stakingtypes.NewMsgCreateValidator(
		valAddr.String(),
		privEd.PubKey(),
		sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(20)),
		// Ensure optional fields can be left blank
		stakingtypes.NewDescription("moniker", "identity", "", "", ""),
		stakingtypes.NewCommissionRates(sdkmath.LegacyOneDec(), sdkmath.LegacyOneDec(), sdkmath.LegacyOneDec()),
		sdkmath.OneInt(),
	)
	s.Require().NoError(err)
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgCreate)
}

func (s *EvmAnteTestSuite) CreateTestEIP712SubmitProposal(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins, deposit sdk.Coins) (client.TxBuilder, error) {
	proposal, ok := govtypes.ContentFromProposalType("My proposal", "My description", govtypes.ProposalTypeText)
	s.Require().True(ok)
	msgSubmit, err := govtypes.NewMsgSubmitProposal(proposal, deposit, from)
	s.Require().NoError(err)
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgSubmit)
}

func (s *EvmAnteTestSuite) CreateTestEIP712GrantAllowance(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	spendLimit := sdk.NewCoins(sdk.NewInt64Coin(s.GetNetwork().GetBaseDenom(), 10))
	threeHours := time.Now().Add(3 * time.Hour)
	basic := &feegrant.BasicAllowance{
		SpendLimit: spendLimit,
		Expiration: &threeHours,
	}
	granted := utiltx.GenerateAddress()
	grantedAddr := s.GetNetwork().App.GetAccountKeeper().NewAccountWithAddress(s.GetNetwork().GetContext(), granted.Bytes())
	msgGrant, err := feegrant.NewMsgGrantAllowance(basic, from, grantedAddr.GetAddress())
	s.Require().NoError(err)
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgGrant)
}

func (s *EvmAnteTestSuite) CreateTestEIP712MsgEditValidator(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	valAddr := sdk.ValAddress(from.Bytes())
	msgEdit := stakingtypes.NewMsgEditValidator(
		valAddr.String(),
		stakingtypes.NewDescription("moniker", "identity", "website", "security_contract", "details"),
		nil,
		nil,
	)
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgEdit)
}

func (s *EvmAnteTestSuite) CreateTestEIP712MsgSubmitEvidence(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	pk := ed25519.GenPrivKey()
	msgEvidence, err := evtypes.NewMsgSubmitEvidence(from, &evtypes.Equivocation{
		Height:           11,
		Time:             time.Now().UTC(),
		Power:            100,
		ConsensusAddress: pk.PubKey().Address().String(),
	})
	s.Require().NoError(err)

	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgEvidence)
}

func (s *EvmAnteTestSuite) CreateTestEIP712MsgVoteV1(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	msgVote := govtypesv1.NewMsgVote(from, 1, govtypesv1.VoteOption_VOTE_OPTION_YES, "")
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgVote)
}

func (s *EvmAnteTestSuite) CreateTestEIP712SubmitProposalV1(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	// Build V1 proposal messages. Must all be same-type, since EIP-712
	// does not support arrays of variable type.
	authAcc := s.GetNetwork().App.GetGovKeeper().GetGovernanceAccount(s.GetNetwork().GetContext())

	proposal1, ok := govtypes.ContentFromProposalType("My proposal 1", "My description 1", govtypes.ProposalTypeText)
	s.Require().True(ok)
	content1, err := govtypesv1.NewLegacyContent(
		proposal1,
		sdk.MustBech32ifyAddressBytes(sdk.GetConfig().GetBech32AccountAddrPrefix(), authAcc.GetAddress().Bytes()),
	)
	s.Require().NoError(err)

	proposal2, ok := govtypes.ContentFromProposalType("My proposal 2", "My description 2", govtypes.ProposalTypeText)
	s.Require().True(ok)
	content2, err := govtypesv1.NewLegacyContent(
		proposal2,
		sdk.MustBech32ifyAddressBytes(sdk.GetConfig().GetBech32AccountAddrPrefix(), authAcc.GetAddress().Bytes()),
	)
	s.Require().NoError(err)

	proposalMsgs := []sdk.Msg{
		content1,
		content2,
	}

	// Build V1 proposal
	msgProposal, err := govtypesv1.NewMsgSubmitProposal(
		proposalMsgs,
		sdk.NewCoins(sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(100))),
		sdk.MustBech32ifyAddressBytes(sdk.GetConfig().GetBech32AccountAddrPrefix(), from.Bytes()),
		"Metadata", "title", "summary",
		false,
	)

	s.Require().NoError(err)

	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgProposal)
}

func (s *EvmAnteTestSuite) CreateTestEIP712MsgExec(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	recipient := sdk.AccAddress(common.Address{}.Bytes())
	msgSend := banktypes.NewMsgSend(from, recipient, sdk.NewCoins(sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(1))))
	msgExec := authz.NewMsgExec(from, []sdk.Msg{msgSend})
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, &msgExec)
}

func (s *EvmAnteTestSuite) CreateTestEIP712MultipleMsgSend(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	recipient := sdk.AccAddress(common.Address{}.Bytes())
	msgSend := banktypes.NewMsgSend(from, recipient, sdk.NewCoins(sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(1))))
	return s.CreateTestEIP712CosmosTxBuilder(priv, chainID, evmChainID, gas, gasAmount, []sdk.Msg{msgSend, msgSend, msgSend})
}

func (s *EvmAnteTestSuite) CreateTestEIP712MultipleDifferentMsgs(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	recipient := sdk.AccAddress(common.Address{}.Bytes())
	msgSend := banktypes.NewMsgSend(from, recipient, sdk.NewCoins(sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(1))))

	msgVote := govtypesv1.NewMsgVote(from, 1, govtypesv1.VoteOption_VOTE_OPTION_YES, "")

	valEthAddr := utiltx.GenerateAddress()
	valAddr := sdk.ValAddress(valEthAddr.Bytes())
	msgDelegate := stakingtypes.NewMsgDelegate(from.String(), valAddr.String(), sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(20)))

	return s.CreateTestEIP712CosmosTxBuilder(priv, chainID, evmChainID, gas, gasAmount, []sdk.Msg{msgSend, msgVote, msgDelegate})
}

func (s *EvmAnteTestSuite) CreateTestEIP712SameMsgDifferentSchemas(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	msgVote1 := govtypesv1.NewMsgVote(from, 1, govtypesv1.VoteOption_VOTE_OPTION_YES, "")
	msgVote2 := govtypesv1.NewMsgVote(from, 5, govtypesv1.VoteOption_VOTE_OPTION_ABSTAIN, "With Metadata")

	return s.CreateTestEIP712CosmosTxBuilder(priv, chainID, evmChainID, gas, gasAmount, []sdk.Msg{msgVote1, msgVote2})
}

func (s *EvmAnteTestSuite) CreateTestEIP712ZeroValueArray(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	recipient := sdk.AccAddress(common.Address{}.Bytes())
	msgSend := banktypes.NewMsgSend(from, recipient, sdk.NewCoins())
	return s.CreateTestEIP712CosmosTxBuilder(priv, chainID, evmChainID, gas, gasAmount, []sdk.Msg{msgSend})
}

func (s *EvmAnteTestSuite) CreateTestEIP712ZeroValueNumber(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	msgVote := govtypesv1.NewMsgVote(from, 0, govtypesv1.VoteOption_VOTE_OPTION_NO, "")

	return s.CreateTestEIP712CosmosTxBuilder(priv, chainID, evmChainID, gas, gasAmount, []sdk.Msg{msgVote})
}

func (s *EvmAnteTestSuite) CreateTestEIP712MsgTransfer(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	msgTransfer := s.createMsgTransfer(from, "With Memo")
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgTransfer)
}

func (s *EvmAnteTestSuite) CreateTestEIP712MsgTransferWithoutMemo(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	msgTransfer := s.createMsgTransfer(from, "")
	return s.CreateTestEIP712SingleMessageTxBuilder(priv, chainID, evmChainID, gas, gasAmount, msgTransfer)
}

func (s *EvmAnteTestSuite) createMsgTransfer(from sdk.AccAddress, memo string) *ibctypes.MsgTransfer {
	recipient := sdk.AccAddress(common.Address{}.Bytes())
	msgTransfer := ibctypes.NewMsgTransfer("transfer", "channel-25", sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(100000)), from.String(), recipient.String(), ibcclienttypes.NewHeight(1000, 1000), 1000, memo)
	return msgTransfer
}

func (s *EvmAnteTestSuite) CreateTestEIP712MultipleSignerMsgs(from sdk.AccAddress, priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins) (client.TxBuilder, error) {
	recipient := sdk.AccAddress(common.Address{}.Bytes())
	msgSend1 := banktypes.NewMsgSend(from, recipient, sdk.NewCoins(sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(1))))
	msgSend2 := banktypes.NewMsgSend(recipient, from, sdk.NewCoins(sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(1))))
	return s.CreateTestEIP712CosmosTxBuilder(priv, chainID, evmChainID, gas, gasAmount, []sdk.Msg{msgSend1, msgSend2})
}

// StdSignBytes returns the bytes to sign for a transaction.
func StdSignBytes(cdc *codec.LegacyAmino, chainID string, accnum uint64, sequence uint64, timeout uint64, fee legacytx.StdFee, msgs []sdk.Msg, memo string) []byte {
	msgsBytes := make([]json.RawMessage, 0, len(msgs))
	for _, msg := range msgs {
		legacyMsg, ok := msg.(legacytx.LegacyMsg)
		if !ok {
			panic(fmt.Errorf("expected %T when using amino JSON", (*legacytx.LegacyMsg)(nil)))
		}

		msgsBytes = append(msgsBytes, json.RawMessage(legacyMsg.GetSignBytes()))
	}

	bz, err := cdc.MarshalJSON(legacytx.StdSignDoc{
		AccountNumber: accnum,
		ChainID:       chainID,
		Fee:           json.RawMessage(fee.Bytes()),
		Memo:          memo,
		Msgs:          msgsBytes,
		Sequence:      sequence,
		TimeoutHeight: timeout,
	})
	if err != nil {
		panic(err)
	}

	return sdk.MustSortJSON(bz)
}

func (s *EvmAnteTestSuite) CreateTestEIP712SingleMessageTxBuilder(
	priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins, msg sdk.Msg,
) (client.TxBuilder, error) {
	msgs := []sdk.Msg{msg}
	return s.CreateTestEIP712CosmosTxBuilder(
		priv,
		chainID,
		evmChainID,
		gas,
		gasAmount,
		msgs,
	)
}

func (s *EvmAnteTestSuite) CreateTestEIP712CosmosTxBuilder(
	priv cryptotypes.PrivKey, chainID string, evmChainID, gas uint64, gasAmount sdk.Coins, msgs []sdk.Msg,
) (client.TxBuilder, error) {
	txConf := s.GetClientCtx().TxConfig
	cosmosTxArgs := utiltx.CosmosTxArgs{
		TxCfg:   txConf,
		Priv:    priv,
		ChainID: chainID,
		Gas:     gas,
		Fees:    gasAmount,
		Msgs:    msgs,
	}

	return utiltx.PrepareEIP712CosmosTx(
		s.GetNetwork().GetContext(),
		s.GetNetwork().App,
		utiltx.EIP712TxArgs{
			CosmosTxArgs:       cosmosTxArgs,
			UseLegacyTypedData: s.UseLegacyEIP712TypedData,
			EVMChainID:         evmChainID,
		},
	)
}

// Generate a set of pub/priv keys to be used in creating multi-keys
func (s *EvmAnteTestSuite) GenerateMultipleKeys(n int) ([]cryptotypes.PrivKey, []cryptotypes.PubKey) {
	privKeys := make([]cryptotypes.PrivKey, n)
	pubKeys := make([]cryptotypes.PubKey, n)
	for i := 0; i < n; i++ {
		privKey, err := ethsecp256k1.GenerateKey()
		s.Require().NoError(err)
		privKeys[i] = privKey
		pubKeys[i] = privKey.PubKey()
	}
	return privKeys, pubKeys
}

// generateSingleSignature signs the given sign doc bytes using the given signType (EIP-712 or Standard)
func (s *EvmAnteTestSuite) generateSingleSignature(signMode signing.SignMode, privKey cryptotypes.PrivKey, signDocBytes []byte, signType string) (signature signing.SignatureV2) {
	var (
		msg []byte
		err error
	)

	msg = signDocBytes

	if signType == "EIP-712" {
		msg, err = eip712.GetEIP712BytesForMsg(signDocBytes)
		s.Require().NoError(err)
	}

	sigBytes, _ := privKey.Sign(msg)
	sigData := &signing.SingleSignatureData{
		SignMode:  signMode,
		Signature: sigBytes,
	}

	return signing.SignatureV2{
		PubKey: privKey.PubKey(),
		Data:   sigData,
	}
}

// generateMultikeySignatures signs a set of messages using each private key within a given multi-key
func (s *EvmAnteTestSuite) generateMultikeySignatures(signMode signing.SignMode, privKeys []cryptotypes.PrivKey, signDocBytes []byte, signType string) (signatures []signing.SignatureV2) {
	n := len(privKeys)
	signatures = make([]signing.SignatureV2, n)

	for i := 0; i < n; i++ {
		privKey := privKeys[i]
		currentType := signType

		// If mixed type, alternate signing type on each iteration
		if signType == "mixed" {
			if i%2 == 0 {
				currentType = "EIP-712"
			} else {
				currentType = "Standard"
			}
		}

		signatures[i] = s.generateSingleSignature(
			signMode,
			privKey,
			signDocBytes,
			currentType,
		)
	}

	return signatures
}

// RegisterAccount creates an account with the keeper and populates the initial balance
func (s *EvmAnteTestSuite) RegisterAccount(pubKey cryptotypes.PubKey, balance *uint256.Int) {
	ctx := s.GetNetwork().GetContext()

	acc := s.GetNetwork().App.GetAccountKeeper().NewAccountWithAddress(ctx, sdk.AccAddress(pubKey.Address()))
	s.GetNetwork().App.GetAccountKeeper().SetAccount(ctx, acc)

	err := s.GetNetwork().App.GetEVMKeeper().SetBalance(ctx, common.BytesToAddress(pubKey.Address()), balance)
	s.Require().NoError(err)
}

// createSignerBytes generates sign doc bytes using the given parameters
func (s *EvmAnteTestSuite) createSignerBytes(chainID string, signMode signing.SignMode, pubKey cryptotypes.PubKey, txBuilder client.TxBuilder) []byte {
	ctx := s.GetNetwork().GetContext()
	acc, err := sdkante.GetSignerAcc(ctx, s.GetNetwork().App.GetAccountKeeper(), sdk.AccAddress(pubKey.Address()))
	s.Require().NoError(err)
	signerInfo := authsigning.SignerData{
		Address:       sdk.MustBech32ifyAddressBytes(sdk.GetConfig().GetBech32AccountAddrPrefix(), acc.GetAddress().Bytes()),
		ChainID:       chainID,
		AccountNumber: acc.GetAccountNumber(),
		Sequence:      acc.GetSequence(),
		PubKey:        pubKey,
	}

	signerBytes, err := authsigning.GetSignBytesAdapter(
		ctx,
		s.GetClientCtx().TxConfig.SignModeHandler(),
		signMode,
		signerInfo,
		txBuilder.GetTx(),
	)

	s.Require().NoError(err)

	return signerBytes
}

// createBaseTxBuilder creates a TxBuilder to be used for Single- or Multi-signing
func (s *EvmAnteTestSuite) createBaseTxBuilder(msg sdk.Msg, gas uint64) client.TxBuilder {
	txBuilder := s.GetClientCtx().TxConfig.NewTxBuilder()

	txBuilder.SetGasLimit(gas)
	txBuilder.SetFeeAmount(sdk.NewCoins(
		sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(10000)),
	))

	err := txBuilder.SetMsgs(msg)
	s.Require().NoError(err)

	txBuilder.SetMemo("")

	return txBuilder
}

// CreateTestSignedMultisigTx creates and sign a multi-signed tx for the given message. `signType` indicates whether to use standard signing ("Standard"),
// EIP-712 signing ("EIP-712"), or a mix of the two ("mixed").
func (s *EvmAnteTestSuite) CreateTestSignedMultisigTx(privKeys []cryptotypes.PrivKey, signMode signing.SignMode, msg sdk.Msg, chainID string, gas uint64, signType string) client.TxBuilder {
	pubKeys := make([]cryptotypes.PubKey, len(privKeys))
	for i, privKey := range privKeys {
		pubKeys[i] = privKey.PubKey()
	}

	// Re-derive multikey
	numKeys := len(privKeys)
	multiKey := kmultisig.NewLegacyAminoPubKey(numKeys, pubKeys)

	s.RegisterAccount(multiKey, uint256.NewInt(10000000000))

	txBuilder := s.createBaseTxBuilder(msg, gas)

	// Prepare signature field
	sig := multisig.NewMultisig(len(pubKeys))
	err := txBuilder.SetSignatures(signing.SignatureV2{
		PubKey: multiKey,
		Data:   sig,
	})
	s.Require().NoError(err)

	signerBytes := s.createSignerBytes(chainID, signMode, multiKey, txBuilder)

	// Sign for each key and update signature field
	sigs := s.generateMultikeySignatures(signMode, privKeys, signerBytes, signType)
	for _, pkSig := range sigs {
		err := multisig.AddSignatureV2(sig, pkSig, pubKeys)
		s.Require().NoError(err)
	}

	err = txBuilder.SetSignatures(signing.SignatureV2{
		PubKey: multiKey,
		Data:   sig,
	})
	s.Require().NoError(err)

	return txBuilder
}

func (s *EvmAnteTestSuite) CreateTestSingleSignedTx(privKey cryptotypes.PrivKey, signMode signing.SignMode, msg sdk.Msg, chainID string, gas uint64, signType string) client.TxBuilder {
	pubKey := privKey.PubKey()

	s.RegisterAccount(pubKey, uint256.NewInt(10_000_000_000))

	txBuilder := s.createBaseTxBuilder(msg, gas)

	// Prepare signature field
	sig := signing.SingleSignatureData{}
	err := txBuilder.SetSignatures(signing.SignatureV2{
		PubKey: pubKey,
		Data:   &sig,
	})
	s.Require().NoError(err)

	signerBytes := s.createSignerBytes(chainID, signMode, pubKey, txBuilder)

	sigData := s.generateSingleSignature(signMode, privKey, signerBytes, signType)
	err = txBuilder.SetSignatures(sigData)
	s.Require().NoError(err)

	return txBuilder
}

// prepareAccount is a helper function that assigns the corresponding
// balance and rewards to the provided account
func (s *EvmAnteTestSuite) prepareAccount(ctx sdk.Context, addr sdk.AccAddress, balance, rewards sdkmath.Int) sdk.Context {
	ctx, err := PrepareAccountsForDelegationRewards(
		s.T(), ctx, s.GetNetwork().App, addr, balance, rewards,
	)
	s.Require().NoError(err, "error while preparing accounts for delegation rewards")
	return ctx.
		WithBlockGasMeter(storetypes.NewGasMeter(1e19)).
		WithBlockHeight(ctx.BlockHeight() + 1)
}

// PrepareAccountsForDelegationRewards prepares the test suite for testing to withdraw delegation rewards.
//
// Balance is the amount of tokens that will be left in the account after the setup is done.
// For each defined reward, a validator is created and tokens are allocated to it using the distribution keeper,
// such that the given amount of tokens is outstanding as a staking reward for the account.
//
// The setup is done in the following way:
//   - Fund the account with the given address with the given balance.
//   - If the given balance is zero, the account will be created with zero balance.
//
// For every reward defined in the rewards argument, the following steps are executed:
//   - Set up a validator with zero commission and delegate to it -> the account delegation will be 50% of the total delegation.
//   - Allocate rewards to the validator.
//
// The function returns the updated context along with a potential error.
func PrepareAccountsForDelegationRewards(t *testing.T, ctx sdk.Context, app evm.EvmApp, addr sdk.AccAddress, balance sdkmath.Int, rewards ...sdkmath.Int) (sdk.Context, error) {
	t.Helper()
	// Calculate the necessary amount of tokens to fund the account in order for the desired residual balance to
	// be left after creating validators and delegating to them.
	totalRewards := sdkmath.ZeroInt()
	for _, reward := range rewards {
		totalRewards = totalRewards.Add(reward)
	}
	totalNeededBalance := balance.Add(totalRewards)

	accountKeeper := app.GetAccountKeeper()
	bankKeeper := app.GetBankKeeper()
	distrKeeper := app.GetDistrKeeper()
	stakingKeeper := app.GetStakingKeeper()
	if totalNeededBalance.IsZero() {
		accountKeeper.SetAccount(ctx, accountKeeper.NewAccountWithAddress(ctx, addr))
	} else {
		// Fund account with enough tokens to stake them
		err := testutil.FundAccountWithBaseDenom(ctx, bankKeeper, addr, totalNeededBalance.Int64())
		if err != nil {
			return sdk.Context{}, fmt.Errorf("failed to fund account: %s", err.Error())
		}
	}

	if totalRewards.IsZero() {
		return ctx, nil
	}

	// reset historical count in distribution keeper which is necessary
	// for the delegation rewards to be calculated correctly
	distrKeeper.DeleteAllValidatorHistoricalRewards(ctx)

	// set distribution module account balance which pays out the rewards
	distrAcc := distrKeeper.GetDistributionAccount(ctx)
	err := testutil.FundModuleAccount(ctx, bankKeeper, distrAcc.GetName(), sdk.NewCoins(sdk.NewCoin(constants.ExampleAttoDenom, totalRewards)))
	if err != nil {
		return sdk.Context{}, fmt.Errorf("failed to fund distribution module account: %s", err.Error())
	}
	accountKeeper.SetModuleAccount(ctx, distrAcc)

	for _, reward := range rewards {
		if reward.IsZero() {
			continue
		}

		// Set up validator and delegate to it
		privKey := ed25519.GenPrivKey()
		addr2, _ := utiltx.NewAccAddressAndKey()
		err := testutil.FundAccountWithBaseDenom(ctx, bankKeeper, addr2, reward.Int64())
		if err != nil {
			return sdk.Context{}, fmt.Errorf("failed to fund validator account: %s", err.Error())
		}

		zeroDec := sdkmath.LegacyZeroDec()
		stakingParams, err := stakingKeeper.GetParams(ctx)
		if err != nil {
			return sdk.Context{}, fmt.Errorf("failed to get staking params: %s", err.Error())
		}
		stakingParams.BondDenom = constants.ExampleAttoDenom
		stakingParams.MinCommissionRate = zeroDec
		err = stakingKeeper.SetParams(ctx, stakingParams)
		require.NoError(t, err)

		stakingHelper := teststaking.NewHelper(t, ctx, stakingKeeper)
		stakingHelper.Commission = stakingtypes.NewCommissionRates(zeroDec, zeroDec, zeroDec)
		stakingHelper.Denom = constants.ExampleAttoDenom

		valAddr := sdk.ValAddress(addr2.Bytes())
		// self-delegate the same amount of tokens as the delegate address also stakes
		// this ensures, that the delegation rewards are 50% of the total rewards
		stakingHelper.CreateValidator(valAddr, privKey.PubKey(), reward, true)
		stakingHelper.Delegate(addr, valAddr, reward)

		// end block to bond validator and increase block height
		// Not using Commit() here because code panics due to invalid block height
		_, err = stakingKeeper.EndBlocker(ctx)
		require.NoError(t, err)

		// allocate rewards to validator (of these 50% will be paid out to the delegator)
		validator, err := stakingKeeper.Validator(ctx, valAddr)
		if err != nil {
			return sdk.Context{}, fmt.Errorf("failed to get validator: %s", err.Error())
		}
		allocatedRewards := sdk.NewDecCoins(sdk.NewDecCoin(constants.ExampleAttoDenom, reward.Mul(sdkmath.NewInt(2))))
		if err = distrKeeper.AllocateTokensToValidator(ctx, validator, allocatedRewards); err != nil {
			return sdk.Context{}, fmt.Errorf("failed to allocate tokens to validator: %s", err.Error())
		}
	}

	// Increase block height in ctx for the rewards calculation
	// NOTE: this will only work for unit tests that use the context
	// returned by this function
	currentHeight := ctx.BlockHeight()
	return ctx.WithBlockHeight(currentHeight + 1), nil
}
