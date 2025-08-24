package ante

import (
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	ethparams "github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"

	ethante "github.com/cosmos/evm/ante/evm"
	"github.com/cosmos/evm/testutil"
	testconstants "github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	kmultisig "github.com/cosmos/cosmos-sdk/crypto/keys/multisig"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	"github.com/cosmos/cosmos-sdk/x/authz"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1beta1"
)

func (s *EvmAnteTestSuite) TestAnteHandler() {
	var (
		ctx     sdk.Context
		addr    common.Address
		privKey cryptotypes.PrivKey
	)
	to := utiltx.GenerateAddress()

	setup := func() {
		s.WithFeemarketEnabled(false)
		baseFee := sdkmath.LegacyNewDec(100)
		s.WithBaseFee(&baseFee)
		s.SetupTest() // reset

		fromKey := s.GetKeyring().GetKey(0)
		addr = fromKey.Addr
		privKey = fromKey.Priv
		ctx = s.GetNetwork().GetContext()
	}

	ethCfg := evmtypes.GetEthChainConfig()
	ethContractCreationTxParams := evmtypes.EvmTxArgs{
		ChainID:   ethCfg.ChainID,
		Nonce:     0,
		Amount:    big.NewInt(10),
		GasLimit:  100000,
		GasPrice:  big.NewInt(150),
		GasFeeCap: big.NewInt(200),
	}

	ethTxParams := evmtypes.EvmTxArgs{
		ChainID:   ethCfg.ChainID,
		To:        &to,
		Nonce:     0,
		Amount:    big.NewInt(10),
		GasLimit:  100000,
		GasPrice:  big.NewInt(150),
		GasFeeCap: big.NewInt(200),
	}

	testCases := []struct {
		name      string
		txFn      func() sdk.Tx
		checkTx   bool
		reCheckTx bool
		expPass   bool
	}{
		{
			"success - DeliverTx (contract)",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethContractCreationTxParams)
				s.Require().NoError(err)
				return tx
			},
			false, false, true,
		},
		{
			"success - CheckTx (contract)",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethContractCreationTxParams)
				s.Require().NoError(err)
				return tx
			},
			true, false, true,
		},
		{
			"success - ReCheckTx (contract)",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethContractCreationTxParams)
				s.Require().NoError(err)
				return tx
			},
			false, true, true,
		},
		{
			"success - DeliverTx",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethTxParams)
				s.Require().NoError(err)
				return tx
			},
			false, false, true,
		},
		{
			"success - CheckTx",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethTxParams)
				s.Require().NoError(err)
				return tx
			},
			true, false, true,
		},
		{
			"success - ReCheckTx",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethTxParams)
				s.Require().NoError(err)
				return tx
			}, false, true, true,
		},
		{
			"success - CheckTx (cosmos tx not signed)",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethTxParams)
				s.Require().NoError(err)
				return tx
			}, false, true, true,
		},
		{
			"fail - CheckTx (cosmos tx is not valid)",
			func() sdk.Tx {
				txBuilder := s.CreateTxBuilder(privKey, ethTxParams)

				// bigger than MaxGasWanted
				txBuilder.SetGasLimit(uint64(1 << 63))
				return txBuilder.GetTx()
			}, true, false, false,
		},
		{
			"fail - CheckTx (memo too long)",
			func() sdk.Tx {
				txBuilder := s.CreateTxBuilder(privKey, ethTxParams)

				txBuilder.SetMemo(strings.Repeat("*", 257))
				return txBuilder.GetTx()
			}, true, false, false,
		},
		{
			"fail - CheckTx (ExtensionOptionsEthereumTx not set)",
			func() sdk.Tx {
				txBuilder := s.CreateTxBuilder(privKey, ethTxParams, true)
				return txBuilder.GetTx()
			}, true, false, false,
		},
		{
			"fail - CheckTx (invalid EIP-155 chain ID)",
			func() sdk.Tx {
				chainID := big.NewInt(1)
				txParamsCopy := evmtypes.EvmTxArgs{
					Nonce:     ethTxParams.Nonce,
					GasLimit:  ethTxParams.GasLimit,
					Input:     ethTxParams.Input,
					GasFeeCap: ethTxParams.GasFeeCap,
					GasPrice:  ethTxParams.GasPrice,
					ChainID:   chainID,
					Amount:    ethTxParams.Amount,
					GasTipCap: ethTxParams.GasTipCap,
					To:        ethTxParams.To,
					Accesses:  ethTxParams.Accesses,
				}
				tx, err := s.GetTxFactory().GenerateSignedEthTxWithChainID(privKey, txParamsCopy, chainID)
				s.Require().NoError(err)
				return tx
			},
			true, false, false,
		},
		// Based on EVMBackend.SendTransaction, for cosmos tx, forcing null for some fields except ExtensionOptions, Fee, MsgEthereumTx
		// should be part of consensus
		{
			"fail - DeliverTx (cosmos tx signed)",
			func() sdk.Tx {
				nonce, err := s.GetNetwork().App.GetAccountKeeper().GetSequence(ctx, s.GetKeyring().GetAccAddr(0))
				s.Require().NoError(err)
				ethTxParams := evmtypes.EvmTxArgs{
					ChainID:  ethCfg.ChainID,
					To:       &to,
					Nonce:    nonce,
					Amount:   big.NewInt(10),
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
				}

				txBuilder := s.CreateTxBuilder(privKey, ethTxParams, true)
				s.Require().NoError(s.GetTxFactory().SignCosmosTx(privKey, txBuilder))
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"fail - DeliverTx (cosmos tx with memo)",
			func() sdk.Tx {
				nonce, err := s.GetNetwork().App.GetAccountKeeper().GetSequence(ctx, s.GetKeyring().GetAccAddr(0))
				s.Require().NoError(err)
				ethTxParams := evmtypes.EvmTxArgs{
					ChainID:  ethCfg.ChainID,
					To:       &to,
					Nonce:    nonce,
					Amount:   big.NewInt(10),
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
				}
				txBuilder := s.CreateTxBuilder(privKey, ethTxParams)
				txBuilder.SetMemo("memo for cosmos tx not allowed")
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"fail - DeliverTx (cosmos tx with timeoutheight)",
			func() sdk.Tx {
				nonce, err := s.GetNetwork().App.GetAccountKeeper().GetSequence(ctx, s.GetKeyring().GetAccAddr(0))
				s.Require().NoError(err)
				ethTxParams := evmtypes.EvmTxArgs{
					ChainID:  ethCfg.ChainID,
					To:       &to,
					Nonce:    nonce,
					Amount:   big.NewInt(10),
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
				}
				txBuilder := s.CreateTxBuilder(privKey, ethTxParams)
				txBuilder.SetTimeoutHeight(10)
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"fail - DeliverTx (invalid fee amount)",
			func() sdk.Tx {
				nonce, err := s.GetNetwork().App.GetAccountKeeper().GetSequence(ctx, s.GetKeyring().GetAccAddr(0))
				s.Require().NoError(err)
				ethTxParams := evmtypes.EvmTxArgs{
					ChainID:  ethCfg.ChainID,
					To:       &to,
					Nonce:    nonce,
					Amount:   big.NewInt(10),
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
				}
				txBuilder := s.CreateTxBuilder(privKey, ethTxParams)

				expFee := txBuilder.GetTx().GetFee()
				oneCoin := sdk.NewCoin(s.GetNetwork().GetBaseDenom(), sdkmath.NewInt(1))
				invalidFee := expFee.Add(oneCoin)
				txBuilder.SetFeeAmount(invalidFee)
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"fail - DeliverTx (invalid fee gaslimit)",
			func() sdk.Tx {
				nonce, err := s.GetNetwork().App.GetAccountKeeper().GetSequence(ctx, s.GetKeyring().GetAccAddr(0))
				s.Require().NoError(err)
				ethTxParams := evmtypes.EvmTxArgs{
					ChainID:  ethCfg.ChainID,
					To:       &to,
					Nonce:    nonce,
					Amount:   big.NewInt(10),
					GasLimit: 100000,
					GasPrice: big.NewInt(1),
				}
				txBuilder := s.CreateTxBuilder(privKey, ethTxParams)

				expGasLimit := txBuilder.GetTx().GetGas()
				invalidGasLimit := expGasLimit + 1
				txBuilder.SetGasLimit(invalidGasLimit)
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"success - DeliverTx EIP712 signed Cosmos Tx with MsgSend",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				gas := uint64(200000)
				amount := sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(100*int64(gas)))) //#nosec G115
				txBuilder, err := s.CreateTestEIP712TxBuilderMsgSend(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success - DeliverTx EIP712 signed Cosmos Tx with DelegateMsg",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				gas := uint64(200000)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(100*int64(gas))) //#nosec G115
				amount := sdk.NewCoins(coinAmount)
				txBuilder, err := s.CreateTestEIP712TxBuilderMsgDelegate(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 create validator",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MsgCreateValidator(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 create validator (with blank fields)",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MsgCreateValidator2(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 MsgSubmitProposal",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				gasAmount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				// reusing the gasAmount for deposit
				deposit := sdk.NewCoins(coinAmount)
				txBuilder, err := s.CreateTestEIP712SubmitProposal(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, gasAmount, deposit)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 MsgGrant",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				grantee := sdk.AccAddress("_______grantee______")
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				gasAmount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				blockTime := time.Date(1, 1, 1, 1, 1, 1, 1, time.UTC)
				expiresAt := blockTime.Add(time.Hour)
				msg, err := authz.NewMsgGrant(
					from, grantee, &banktypes.SendAuthorization{SpendLimit: gasAmount}, &expiresAt,
				)
				s.Require().NoError(err)
				builder, err := s.CreateTestEIP712SingleMessageTxBuilder(privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, gasAmount, msg)
				s.Require().NoError(err)

				return builder.GetTx()
			}, false, false, true,
		},

		{
			"success- DeliverTx EIP712 MsgGrantAllowance",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				gasAmount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712GrantAllowance(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, gasAmount)
				s.Require().NoError(err)

				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 edit validator",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MsgEditValidator(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 submit evidence",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MsgSubmitEvidence(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 submit proposal v1",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712SubmitProposalV1(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 MsgExec",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MsgExec(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 MsgVoteV1",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MsgVoteV1(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 Multiple MsgSend",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MultipleMsgSend(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 Multiple Different Msgs",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MultipleDifferentMsgs(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.RequireErrorForLegacyTypedData(err)
				return s.TxForLegacyTypedData(txBuilder)
			}, false, false, !s.UseLegacyEIP712TypedData,
		},
		{
			"success- DeliverTx EIP712 Same Msgs, Different Schemas",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712SameMsgDifferentSchemas(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.RequireErrorForLegacyTypedData(err)
				return s.TxForLegacyTypedData(txBuilder)
			}, false, false, !s.UseLegacyEIP712TypedData,
		},
		{
			"success- DeliverTx EIP712 Zero Value Array (Should Not Omit Field)",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712ZeroValueArray(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.RequireErrorForLegacyTypedData(err)
				return s.TxForLegacyTypedData(txBuilder)
			}, false, false, !s.UseLegacyEIP712TypedData,
		},
		{
			"success- DeliverTx EIP712 Zero Value Number (Should Not Omit Field)",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712ZeroValueNumber(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.RequireErrorForLegacyTypedData(err)
				return s.TxForLegacyTypedData(txBuilder)
			}, false, false, !s.UseLegacyEIP712TypedData,
		},
		{
			"success- DeliverTx EIP712 MsgTransfer",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MsgTransfer(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"success- DeliverTx EIP712 MsgTransfer Without Memo",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MsgTransferWithoutMemo(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"fails - DeliverTx EIP712 Multiple Signers",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				coinAmount := sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(20))
				amount := sdk.NewCoins(coinAmount)
				gas := uint64(200000)
				txBuilder, err := s.CreateTestEIP712MultipleSignerMsgs(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"fails - DeliverTx EIP712 signed Cosmos Tx with wrong Chain ID",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				gas := uint64(200000)
				amount := sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(100*int64(gas)))) //#nosec G115
				txBuilder, err := s.CreateTestEIP712TxBuilderMsgSend(from, privKey, "cosmos-1", 9002, gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"fails - DeliverTx EIP712 signed Cosmos Tx with different gas fees",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				gas := uint64(200000)
				amount := sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(100*int64(gas)))) //#nosec G115
				txBuilder, err := s.CreateTestEIP712TxBuilderMsgSend(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				txBuilder.SetGasLimit(uint64(300000))
				txBuilder.SetFeeAmount(sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(30))))
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"fails - DeliverTx EIP712 signed Cosmos Tx with invalid chain id",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				gas := uint64(200000)
				amount := sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(100*int64(gas)))) //#nosec G115
				txBuilder, err := s.CreateTestEIP712TxBuilderMsgSend(from, privKey, "cosmos-1", 9005, gas, amount)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"fails - DeliverTx EIP712 signed Cosmos Tx with invalid sequence",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				gas := uint64(200000)
				amount := sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(100*int64(gas)))) //#nosec G115
				txBuilder, err := s.CreateTestEIP712TxBuilderMsgSend(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				nonce, err := s.GetNetwork().App.GetAccountKeeper().GetSequence(ctx, s.GetKeyring().GetAccAddr(0))
				s.Require().NoError(err)
				sigsV2 := signing.SignatureV2{
					PubKey: privKey.PubKey(),
					Data: &signing.SingleSignatureData{
						SignMode: signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
					},
					Sequence: nonce - 1,
				}

				err = txBuilder.SetSignatures(sigsV2)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"fails - DeliverTx EIP712 signed Cosmos Tx with invalid signMode",
			func() sdk.Tx {
				from := s.GetKeyring().GetAccAddr(0)
				gas := uint64(200000)
				amount := sdk.NewCoins(sdk.NewCoin(testconstants.ExampleAttoDenom, sdkmath.NewInt(100*int64(gas)))) //#nosec G115
				txBuilder, err := s.CreateTestEIP712TxBuilderMsgSend(from, privKey, ctx.ChainID(), ethCfg.ChainID.Uint64(), gas, amount)
				s.Require().NoError(err)
				nonce, err := s.GetNetwork().App.GetAccountKeeper().GetSequence(ctx, s.GetKeyring().GetAccAddr(0))
				s.Require().NoError(err)
				sigsV2 := signing.SignatureV2{
					PubKey: privKey.PubKey(),
					Data: &signing.SingleSignatureData{
						SignMode: signing.SignMode_SIGN_MODE_UNSPECIFIED,
					},
					Sequence: nonce,
				}
				err = txBuilder.SetSignatures(sigsV2)
				s.Require().NoError(err)
				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"fails - invalid from",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethContractCreationTxParams)
				s.Require().NoError(err)
				msg := tx.GetMsgs()[0].(*evmtypes.MsgEthereumTx)
				msg.From = addr.Bytes()
				return tx
			}, true, false, false,
		},
		{
			"passes - Single-signer EIP-712",
			func() sdk.Tx {
				evmDenom := evmtypes.GetEVMCoinDenom()
				msg := banktypes.NewMsgSend(
					sdk.AccAddress(privKey.PubKey().Address()),
					addr[:],
					sdk.NewCoins(
						sdk.NewCoin(
							evmDenom,
							sdkmath.NewInt(1),
						),
					),
				)

				txBuilder := s.CreateTestSingleSignedTx(
					privKey,
					signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
					msg,
					ctx.ChainID(),
					2000000,
					"EIP-712",
				)

				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"passes - EIP-712 multi-key",
			func() sdk.Tx {
				numKeys := 5
				privKeys, pubKeys := s.GenerateMultipleKeys(numKeys)
				pk := kmultisig.NewLegacyAminoPubKey(numKeys, pubKeys)

				msg := banktypes.NewMsgSend(
					sdk.AccAddress(pk.Address()),
					addr[:],
					sdk.NewCoins(
						sdk.NewCoin(
							"uatomz",
							sdkmath.NewInt(1),
						),
					),
				)

				txBuilder := s.CreateTestSignedMultisigTx(
					privKeys,
					signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
					msg,
					ctx.ChainID(),
					2000000,
					"EIP-712",
				)

				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"passes - Mixed multi-key",
			func() sdk.Tx {
				numKeys := 5
				privKeys, pubKeys := s.GenerateMultipleKeys(numKeys)
				pk := kmultisig.NewLegacyAminoPubKey(numKeys, pubKeys)

				msg := banktypes.NewMsgSend(
					sdk.AccAddress(pk.Address()),
					addr[:],
					sdk.NewCoins(
						sdk.NewCoin(
							"uatomz",
							sdkmath.NewInt(1),
						),
					),
				)

				txBuilder := s.CreateTestSignedMultisigTx(
					privKeys,
					signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
					msg,
					ctx.ChainID(),
					2000000,
					"mixed", // Combine EIP-712 and standard signatures
				)

				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"passes - Mixed multi-key with MsgVote",
			func() sdk.Tx {
				numKeys := 5
				privKeys, pubKeys := s.GenerateMultipleKeys(numKeys)
				pk := kmultisig.NewLegacyAminoPubKey(numKeys, pubKeys)

				msg := govtypes.NewMsgVote(
					sdk.AccAddress(pk.Address()),
					1,
					govtypes.OptionYes,
				)

				txBuilder := s.CreateTestSignedMultisigTx(
					privKeys,
					signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
					msg,
					ctx.ChainID(),
					2000000,
					"mixed", // Combine EIP-712 and standard signatures
				)

				return txBuilder.GetTx()
			}, false, false, true,
		},
		{
			"Fails - Multi-Key with incorrect Chain ID",
			func() sdk.Tx {
				numKeys := 5
				privKeys, pubKeys := s.GenerateMultipleKeys(numKeys)
				pk := kmultisig.NewLegacyAminoPubKey(numKeys, pubKeys)

				msg := banktypes.NewMsgSend(
					sdk.AccAddress(pk.Address()),
					addr[:],
					sdk.NewCoins(
						sdk.NewCoin(
							"uatomz",
							sdkmath.NewInt(1),
						),
					),
				)

				txBuilder := s.CreateTestSignedMultisigTx(
					privKeys,
					signing.SignMode_SIGN_MODE_LEGACY_AMINO_JSON,
					msg,
					"cosmos_9005-1",
					2000000,
					"mixed",
				)

				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"Fails - Multi-Key with incorrect sign mode",
			func() sdk.Tx {
				numKeys := 5
				privKeys, pubKeys := s.GenerateMultipleKeys(numKeys)
				pk := kmultisig.NewLegacyAminoPubKey(numKeys, pubKeys)

				msg := banktypes.NewMsgSend(
					sdk.AccAddress(pk.Address()),
					addr[:],
					sdk.NewCoins(
						sdk.NewCoin(
							"uatomz",
							sdkmath.NewInt(1),
						),
					),
				)

				txBuilder := s.CreateTestSignedMultisigTx(
					privKeys,
					signing.SignMode_SIGN_MODE_DIRECT,
					msg,
					ctx.ChainID(),
					2000000,
					"mixed",
				)

				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"Fails - Multi-Key with too little gas",
			func() sdk.Tx {
				numKeys := 5
				privKeys, pubKeys := s.GenerateMultipleKeys(numKeys)
				pk := kmultisig.NewLegacyAminoPubKey(numKeys, pubKeys)

				msg := banktypes.NewMsgSend(
					sdk.AccAddress(pk.Address()),
					addr[:],
					sdk.NewCoins(
						sdk.NewCoin(
							"uatomz",
							sdkmath.NewInt(1),
						),
					),
				)

				txBuilder := s.CreateTestSignedMultisigTx(
					privKeys,
					signing.SignMode_SIGN_MODE_DIRECT,
					msg,
					ctx.ChainID(),
					2000,
					"mixed", // Combine EIP-712 and standard signatures
				)

				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"Fails - Multi-Key with different payload than one signed",
			func() sdk.Tx {
				numKeys := 1
				privKeys, pubKeys := s.GenerateMultipleKeys(numKeys)
				pk := kmultisig.NewLegacyAminoPubKey(numKeys, pubKeys)

				msg := banktypes.NewMsgSend(
					sdk.AccAddress(pk.Address()),
					addr[:],
					sdk.NewCoins(
						sdk.NewCoin(
							"uatomz",
							sdkmath.NewInt(1),
						),
					),
				)

				txBuilder := s.CreateTestSignedMultisigTx(
					privKeys,
					signing.SignMode_SIGN_MODE_DIRECT,
					msg,
					ctx.ChainID(),
					2000,
					"EIP-712",
				)

				msg.Amount[0].Amount = sdkmath.NewInt(5)
				err := txBuilder.SetMsgs(msg)
				s.Require().NoError(err)

				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"Fails - Multi-Key with messages added after signing",
			func() sdk.Tx {
				numKeys := 1
				privKeys, pubKeys := s.GenerateMultipleKeys(numKeys)
				pk := kmultisig.NewLegacyAminoPubKey(numKeys, pubKeys)

				msg := banktypes.NewMsgSend(
					sdk.AccAddress(pk.Address()),
					addr[:],
					sdk.NewCoins(
						sdk.NewCoin(
							"uatomz",
							sdkmath.NewInt(1),
						),
					),
				)

				txBuilder := s.CreateTestSignedMultisigTx(
					privKeys,
					signing.SignMode_SIGN_MODE_DIRECT,
					msg,
					ctx.ChainID(),
					2000,
					"EIP-712",
				)

				// Duplicate
				err := txBuilder.SetMsgs(msg, msg)
				s.Require().NoError(err)

				return txBuilder.GetTx()
			}, false, false, false,
		},
		{
			"Fails - Single-Signer EIP-712 with messages added after signing",
			func() sdk.Tx {
				msg := banktypes.NewMsgSend(
					sdk.AccAddress(privKey.PubKey().Address()),
					addr[:],
					sdk.NewCoins(
						sdk.NewCoin(
							"uatomz",
							sdkmath.NewInt(1),
						),
					),
				)

				txBuilder := s.CreateTestSingleSignedTx(
					privKey,
					signing.SignMode_SIGN_MODE_DIRECT,
					msg,
					ctx.ChainID(),
					2000,
					"EIP-712",
				)

				err := txBuilder.SetMsgs(msg, msg)
				s.Require().NoError(err)

				return txBuilder.GetTx()
			}, false, false, false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			setup()

			ctx = ctx.WithIsCheckTx(tc.checkTx).WithIsReCheckTx(tc.reCheckTx)
			anteHandler := s.GetAnteHandler()
			_, err := anteHandler(ctx, tc.txFn(), false)

			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *EvmAnteTestSuite) TestAnteHandlerWithDynamicTxFee() {
	addr, privKey := utiltx.NewAddrKey()
	to := utiltx.GenerateAddress()

	evmChainID := evmtypes.GetEthChainConfig().ChainID

	ethContractCreationTxParams := evmtypes.EvmTxArgs{
		ChainID:   evmChainID,
		Nonce:     0,
		Amount:    big.NewInt(10),
		GasLimit:  100000,
		GasFeeCap: big.NewInt(ethparams.InitialBaseFee + 1),
		GasTipCap: big.NewInt(1),
		Accesses:  &types.AccessList{},
	}

	ethTxParams := evmtypes.EvmTxArgs{
		ChainID:   evmChainID,
		Nonce:     0,
		Amount:    big.NewInt(10),
		GasLimit:  100000,
		GasFeeCap: big.NewInt(ethparams.InitialBaseFee + 1),
		GasTipCap: big.NewInt(1),
		Accesses:  &types.AccessList{},
		To:        &to,
	}

	testCases := []struct {
		name           string
		txFn           func() sdk.Tx
		enableLondonHF bool
		checkTx        bool
		reCheckTx      bool
		expPass        bool
	}{
		{
			"success - DeliverTx (contract)",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethContractCreationTxParams)
				s.Require().NoError(err)
				return tx
			},
			true,
			false, false, true,
		},
		{
			"success - CheckTx (contract)",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethContractCreationTxParams)
				s.Require().NoError(err)
				return tx
			},
			true,
			true, false, true,
		},
		{
			"success - ReCheckTx (contract)",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethContractCreationTxParams)
				s.Require().NoError(err)
				return tx
			},
			true,
			false, true, true,
		},
		{
			"success - DeliverTx",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethTxParams)
				s.Require().NoError(err)
				return tx
			},
			true,
			false, false, true,
		},
		{
			"success - CheckTx",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethTxParams)
				s.Require().NoError(err)
				return tx
			},
			true,
			true, false, true,
		},
		{
			"success - ReCheckTx",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethTxParams)
				s.Require().NoError(err)
				return tx
			},
			true,
			false, true, true,
		},
		{
			"success - CheckTx (cosmos tx not signed)",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethTxParams)
				s.Require().NoError(err)
				return tx
			},
			true,
			false, true, true,
		},
		{
			"fail - CheckTx (cosmos tx is not valid)",
			func() sdk.Tx {
				txBuilder := s.CreateTxBuilder(privKey, ethTxParams)
				// bigger than MaxGasWanted
				txBuilder.SetGasLimit(uint64(1 << 63))
				return txBuilder.GetTx()
			},
			true,
			true, false, false,
		},
		{
			"fail - CheckTx (memo too long)",
			func() sdk.Tx {
				txBuilder := s.CreateTxBuilder(privKey, ethTxParams)
				txBuilder.SetMemo(strings.Repeat("*", 257))
				return txBuilder.GetTx()
			},
			true,
			true, false, false,
		},
		{
			"fail - DynamicFeeTx without london hark fork",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethContractCreationTxParams)
				s.Require().NoError(err)
				return tx
			},
			false,
			false, false, false,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.WithFeemarketEnabled(true)
			s.WithLondonHardForkEnabled(tc.enableLondonHF)
			s.SetupTest() // reset
			ctx := s.GetNetwork().GetContext()
			acc := s.GetNetwork().App.GetAccountKeeper().NewAccountWithAddress(ctx, addr.Bytes())
			s.Require().NoError(acc.SetSequence(1))
			s.GetNetwork().App.GetAccountKeeper().SetAccount(ctx, acc)

			ctx = ctx.WithIsCheckTx(tc.checkTx).WithIsReCheckTx(tc.reCheckTx)
			err := s.GetNetwork().App.GetEVMKeeper().SetBalance(ctx, addr, uint256.NewInt((ethparams.InitialBaseFee+10)*100000))
			s.Require().NoError(err)

			anteHandler := s.GetAnteHandler()
			_, err = anteHandler(ctx, tc.txFn(), false)
			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
	s.WithFeemarketEnabled(false)
	s.WithLondonHardForkEnabled(true)
}

func (s *EvmAnteTestSuite) TestAnteHandlerWithParams() {
	addr, privKey := utiltx.NewAddrKey()
	to := utiltx.GenerateAddress()

	ethCfg := evmtypes.GetEthChainConfig()

	ethContractCreationTxParams := evmtypes.EvmTxArgs{
		ChainID:   ethCfg.ChainID,
		Nonce:     0,
		Amount:    big.NewInt(10),
		GasLimit:  100000,
		GasFeeCap: big.NewInt(ethparams.InitialBaseFee + 1),
		GasTipCap: big.NewInt(1),
		Input:     []byte("create bytes"),
		Accesses:  &types.AccessList{},
	}

	ethTxParams := evmtypes.EvmTxArgs{
		ChainID:   ethCfg.ChainID,
		Nonce:     0,
		Amount:    big.NewInt(10),
		GasLimit:  100000,
		GasFeeCap: big.NewInt(ethparams.InitialBaseFee + 1),
		GasTipCap: big.NewInt(1),
		Accesses:  &types.AccessList{},
		Input:     []byte("call bytes"),
		To:        &to,
	}

	testCases := []struct {
		name        string
		txFn        func() sdk.Tx
		permissions evmtypes.AccessControl
		expErr      error
	}{
		{
			"fail - Contract Creation Disabled",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethContractCreationTxParams)
				s.Require().NoError(err)
				return tx
			},
			evmtypes.AccessControl{
				Create: evmtypes.AccessControlType{
					AccessType:        evmtypes.AccessTypeRestricted,
					AccessControlList: evmtypes.DefaultCreateAllowlistAddresses,
				},
				Call: evmtypes.AccessControlType{
					AccessType:        evmtypes.AccessTypePermissionless,
					AccessControlList: evmtypes.DefaultCreateAllowlistAddresses,
				},
			},
			evmtypes.ErrCreateDisabled,
		},
		{
			"success - Contract Creation Enabled",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethContractCreationTxParams)
				s.Require().NoError(err)
				return tx
			},
			evmtypes.DefaultAccessControl,
			nil,
		},
		{
			"fail - EVM Call Disabled",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethTxParams)
				s.Require().NoError(err)
				return tx
			},
			evmtypes.AccessControl{
				Create: evmtypes.AccessControlType{
					AccessType:        evmtypes.AccessTypePermissionless,
					AccessControlList: evmtypes.DefaultCreateAllowlistAddresses,
				},
				Call: evmtypes.AccessControlType{
					AccessType:        evmtypes.AccessTypeRestricted,
					AccessControlList: evmtypes.DefaultCreateAllowlistAddresses,
				},
			},
			evmtypes.ErrCallDisabled,
		},
		{
			"success - EVM Call Enabled",
			func() sdk.Tx {
				tx, err := s.GetTxFactory().GenerateSignedEthTx(privKey, ethTxParams)
				s.Require().NoError(err)
				return tx
			},
			evmtypes.DefaultAccessControl,
			nil,
		},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.WithEvmParamsOptions(func(params *evmtypes.Params) {
				params.AccessControl = tc.permissions
			})
			// clean up the evmParamsOption
			defer s.ResetEvmParamsOptions()

			s.SetupTest() // reset

			ctx := s.GetNetwork().GetContext()
			acc := s.GetNetwork().App.GetAccountKeeper().NewAccountWithAddress(ctx, addr.Bytes())
			s.Require().NoError(acc.SetSequence(1))
			s.GetNetwork().App.GetAccountKeeper().SetAccount(ctx, acc)

			ctx = ctx.WithIsCheckTx(true)
			err := s.GetNetwork().App.GetEVMKeeper().SetBalance(ctx, addr, uint256.NewInt((ethparams.InitialBaseFee+10)*100000))
			s.Require().NoError(err)

			anteHandler := s.GetAnteHandler()
			_, err = anteHandler(ctx, tc.txFn(), false)
			if tc.expErr == nil {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
				s.Require().True(errors.Is(err, tc.expErr))
			}
		})
	}
	s.WithEvmParamsOptions(nil)
}

func (s *EvmAnteTestSuite) TestEthSigVerificationDecorator() {
	addr, privKey := utiltx.NewAddrKey()
	ethCfg := evmtypes.GetEthChainConfig()
	ethSigner := types.LatestSignerForChainID(ethCfg.ChainID)

	ethContractCreationTxParams := &evmtypes.EvmTxArgs{
		ChainID:  ethCfg.ChainID,
		Nonce:    1,
		Amount:   big.NewInt(10),
		GasLimit: 1000,
		GasPrice: big.NewInt(1),
	}
	signedTx := evmtypes.NewTx(ethContractCreationTxParams)
	signedTx.From = addr.Bytes()
	err := signedTx.Sign(ethSigner, utiltx.NewSigner(privKey))
	s.Require().NoError(err)

	unprotectedEthTxParams := &evmtypes.EvmTxArgs{
		Nonce:    1,
		Amount:   big.NewInt(10),
		GasLimit: 1000,
		GasPrice: big.NewInt(1),
	}
	unprotectedTx := evmtypes.NewTx(unprotectedEthTxParams)
	unprotectedTx.From = addr.Bytes()
	err = unprotectedTx.Sign(types.HomesteadSigner{}, utiltx.NewSigner(privKey))
	s.Require().NoError(err)

	testCases := []struct {
		name      string
		tx        sdk.Tx
		reCheckTx bool
		expPass   bool
	}{
		{"ReCheckTx", &utiltx.InvalidTx{}, true, false},
		{"invalid transaction type", &utiltx.InvalidTx{}, false, false},
		{
			"invalid sender",
			evmtypes.NewTx(&evmtypes.EvmTxArgs{
				To:       &addr,
				Nonce:    1,
				Amount:   big.NewInt(10),
				GasLimit: 1000,
				GasPrice: big.NewInt(1),
			}),
			false,
			false,
		},
		{"successful signature verification", signedTx, false, true},
		{"invalid, reject unprotected txs", unprotectedTx, false, false},
		{"successful, allow unprotected txs", unprotectedTx, false, true},
	}

	for _, tc := range testCases {
		s.Run(tc.name, func() {
			s.SetupTest()
			dec := ethante.NewEthSigVerificationDecorator(s.GetNetwork().App.GetEVMKeeper())
			_, err := dec.AnteHandle(s.GetNetwork().GetContext().WithIsReCheckTx(tc.reCheckTx), tc.tx, false, testutil.NoOpNextFn)

			if tc.expPass {
				s.Require().NoError(err)
			} else {
				s.Require().Error(err)
			}
		})
	}
}

func (s *EvmAnteTestSuite) TestSignatures() {
	s.WithFeemarketEnabled(false)
	s.SetupTest() // reset

	privKey := s.GetKeyring().GetPrivKey(0)
	to := utiltx.GenerateAddress()

	txArgs := evmtypes.EvmTxArgs{
		ChainID:  evmtypes.GetEthChainConfig().ChainID,
		Nonce:    0,
		To:       &to,
		Amount:   big.NewInt(10),
		GasLimit: 100000,
		GasPrice: big.NewInt(1),
	}

	// CreateTestTx will sign the msgEthereumTx but not sign the cosmos tx since we have signCosmosTx as false
	tx := s.CreateTxBuilder(privKey, txArgs).GetTx()
	sigs, err := tx.GetSignaturesV2()
	s.Require().NoError(err)

	// signatures of cosmos tx should be empty
	s.Require().Equal(len(sigs), 0)

	msg := tx.GetMsgs()[0]
	_, ok := msg.(*evmtypes.MsgEthereumTx)
	s.Require().True(ok)
}
