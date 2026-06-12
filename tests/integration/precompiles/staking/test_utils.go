package staking

import (
	"encoding/base64"
	"math/big"
	"slices"
	"time"

	"github.com/ethereum/go-ethereum/common"
	gethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"

	//nolint:revive // dot imports are fine for Ginkgo
	. "github.com/onsi/gomega"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
	"github.com/cosmos/evm/precompiles/staking"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	"cosmossdk.io/math"

	"github.com/cosmos/cosmos-sdk/crypto/keys/ed25519"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	stakingkeeper "github.com/cosmos/cosmos-sdk/x/staking/keeper"
	stakingtypes "github.com/cosmos/cosmos-sdk/x/staking/types"
)

// assertValidatorsResponse asserts all the fields on the validators response
func (s *PrecompileTestSuite) assertValidatorsResponse(validators []staking.ValidatorInfo, expLen int) {
	// returning order can change
	valOrder := []int{0, 1}
	varAddr := sdk.ValAddress(common.HexToAddress(validators[0].OperatorAddress).Bytes()).String()
	vals := s.network.GetValidators()

	if varAddr != vals[0].OperatorAddress {
		valOrder = []int{1, 0}
	}
	for i := 0; i < expLen; i++ {
		j := valOrder[i]

		val := s.network.GetValidators()[j]
		s.Require().Equal(val.OperatorAddress, sdk.ValAddress(common.HexToAddress(validators[i].OperatorAddress).Bytes()).String())
		s.Require().Equal(uint8(val.Status), validators[i].Status) //#nosec G115
		s.Require().Equal(val.Tokens.Uint64(), validators[i].Tokens.Uint64())
		s.Require().Equal(val.DelegatorShares.BigInt(), validators[i].DelegatorShares)
		s.Require().Equal(val.Jailed, validators[i].Jailed)
		s.Require().Equal(val.UnbondingHeight, validators[i].UnbondingHeight)
		s.Require().Equal(int64(0), validators[i].UnbondingTime)
		s.Require().Equal(math.LegacyNewDecWithPrec(5, 2).BigInt(), validators[i].Commission)
		s.Require().Equal(int64(0), validators[i].MinSelfDelegation.Int64())
		s.Require().Equal(validators[i].ConsensusPubkey, staking.FormatConsensusPubkey(val.ConsensusPubkey))
	}
}

// assertRedelegation asserts the redelegationOutput struct and its fields
func (s *PrecompileTestSuite) assertRedelegationsOutput(data []byte, redelTotalCount uint64, expAmt *big.Int, expCreationHeight int64, hasPagination bool) {
	var redOut staking.RedelegationsOutput
	err := s.precompile.UnpackIntoInterface(&redOut, staking.RedelegationsMethod, data)
	s.Require().NoError(err, "failed to unpack output")

	s.Require().Len(redOut.Response, 1)
	// check pagination - total count should be 2
	s.Require().Equal(redelTotalCount, redOut.PageResponse.Total)
	if hasPagination {
		s.Require().NotEmpty(redOut.PageResponse.NextKey)
	} else {
		s.Require().Empty(redOut.PageResponse.NextKey)
	}
	// check redelegation entry
	// order may change, one redelegation has 2 entries
	// and the other has one
	if len(redOut.Response[0].Entries) == 2 {
		s.assertRedelegation(redOut.Response[0],
			2,
			s.network.GetValidators()[0].OperatorAddress,
			s.network.GetValidators()[1].OperatorAddress,
			expAmt,
			expCreationHeight,
		)
	} else {
		s.assertRedelegation(redOut.Response[0],
			1,
			s.network.GetValidators()[0].OperatorAddress,
			s.network.GetValidators()[2].OperatorAddress,
			expAmt,
			expCreationHeight,
		)
	}
}

// assertRedelegation asserts all the fields on the redelegations response
// should specify the amount of entries expected and the expected amount for this
// the same amount is considered for all entries
func (s *PrecompileTestSuite) assertRedelegation(res staking.RedelegationResponse, entriesCount int, expValSrcAddr, expValDstAddr string, expAmt *big.Int, expCreationHeight int64) {
	// check response
	s.Require().Equal(res.Redelegation.DelegatorAddress, s.keyring.GetAccAddr(0).String())
	s.Require().Equal(res.Redelegation.ValidatorSrcAddress, expValSrcAddr)
	s.Require().Equal(res.Redelegation.ValidatorDstAddress, expValDstAddr)
	// check redelegation entries - should be empty
	s.Require().Empty(res.Redelegation.Entries)
	// check response entries, should be 2
	s.Require().Len(res.Entries, entriesCount)
	// check redelegation entries
	for _, e := range res.Entries {
		s.Require().Equal(e.Balance, expAmt)
		s.Require().True(e.RedelegationEntry.CompletionTime > 1600000000)
		s.Require().Equal(expCreationHeight, e.RedelegationEntry.CreationHeight)
		s.Require().Equal(e.RedelegationEntry.InitialBalance, expAmt)
	}
}

// setupRedelegations setups 2 entries for redelegation from validator[0]
// to validator[1], and a redelegation from validator[0] to validator[2]
func (s *PrecompileTestSuite) setupRedelegations(ctx sdk.Context, redelAmt *big.Int) error {
	ctx = ctx.WithBlockTime(time.Now())
	vals := s.network.GetValidators()

	msg := stakingtypes.MsgBeginRedelegate{
		DelegatorAddress:    s.keyring.GetAccAddr(0).String(),
		ValidatorSrcAddress: vals[0].OperatorAddress,
		ValidatorDstAddress: vals[1].OperatorAddress,
		Amount:              sdk.NewCoin(s.bondDenom, math.NewIntFromBigInt(redelAmt)),
	}

	msgSrv := stakingkeeper.NewMsgServerImpl(s.network.App.GetStakingKeeper())
	// create 2 entries for same redelegation
	for i := 0; i < 2; i++ {
		if _, err := msgSrv.BeginRedelegate(ctx, &msg); err != nil {
			return err
		}
	}

	// create a redelegation from validator[0] to validator[2]
	msg.ValidatorDstAddress = vals[2].OperatorAddress
	_, err := msgSrv.BeginRedelegate(ctx, &msg)
	return err
}

// CheckValidatorOutput checks that the given validator output
func (s *PrecompileTestSuite) CheckValidatorOutput(valOut staking.ValidatorInfo) {
	vals := s.network.GetValidators()
	validatorAddrs := make([]string, len(vals))
	for i, v := range vals {
		validatorAddrs[i] = v.OperatorAddress
	}

	operatorAddress := sdk.ValAddress(common.HexToAddress(valOut.OperatorAddress).Bytes()).String()

	Expect(slices.Contains(validatorAddrs, operatorAddress)).To(BeTrue(), "operator address not found in test suite validators")
	Expect(valOut.DelegatorShares).To(Equal(big.NewInt(1e18)), "expected different delegator shares")
}

// GenerateBase64PubKey generates the Base64 encoded PubKey associated with a PrivKey generated with
// the ed25519 algorithm used in CometBFT nodes.
func GenerateBase64PubKey() string {
	privKey := ed25519.GenPrivKey()
	pubKey := privKey.PubKey().(*ed25519.PubKey)
	return base64.StdEncoding.EncodeToString(pubKey.Bytes())
}

// delegateAccountToContract delegates the given account to the given contract address using
// the EIP-7702 SetCodeTx.
func (s *PrecompileTestSuite) delegateAccountToContract(privKey cryptotypes.PrivKey, accountAddr, contractAddr common.Address) {
	ethPriv, ok := privKey.(*ethsecp256k1.PrivKey)
	Expect(ok).To(BeTrue(), "expected ethsecp256k1 private key")
	ecdsaPriv, err := ethPriv.ToECDSA()
	Expect(err).To(BeNil(), "error converting to ECDSA private key")

	chainID := evmtypes.GetChainConfig().GetChainId()
	accResp, err := s.grpcHandler.GetEvmAccount(accountAddr)
	Expect(err).To(BeNil(), "error while getting the EVM account")

	nonce := accResp.GetNonce()

	authorization := gethtypes.SetCodeAuthorization{
		ChainID: *uint256.NewInt(chainID),
		Address: contractAddr,
		Nonce:   nonce + 1,
	}

	signedAuth, err := gethtypes.SignSetCode(ecdsaPriv, authorization)
	Expect(err).To(BeNil(), "error while signing the SetCodeAuthorization")

	txArgs := evmtypes.EvmTxArgs{
		To:       &common.Address{},
		GasLimit: 500_000,
		AuthorizationList: []gethtypes.SetCodeAuthorization{
			signedAuth,
		},
	}

	_, err = s.factory.ExecuteEthTx(privKey, txArgs)
	Expect(err).To(BeNil(), "error while executing the SetCode transaction")
	Expect(s.network.NextBlock()).To(BeNil(), "failed to advance block")

	codeHash := s.network.App.GetEVMKeeper().GetCodeHash(s.network.GetContext(), accountAddr)
	code := s.network.App.GetEVMKeeper().GetCode(s.network.GetContext(), codeHash)
	_, delegated := gethtypes.ParseDelegation(code)
	Expect(delegated).To(BeTrue(), "expected account to be delegated to the contract address")
}
