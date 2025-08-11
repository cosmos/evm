package erc20

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/mock"

	"github.com/cosmos/evm/contracts"
	"github.com/cosmos/evm/testutil/integration/evm/utils"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	"github.com/cosmos/evm/x/erc20/keeper"
	"github.com/cosmos/evm/x/erc20/types"
	erc20mocks "github.com/cosmos/evm/x/erc20/types/mocks"
	evmtypes "github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types"
)

const (
	contractMinterBurner = iota + 1
	contractDirectBalanceManipulation
	contractMaliciousDelayed
	contractBytes32Metadata
)

const (
	erc20Name          = "Coin Token"
	erc20Symbol        = "CTKN"
	erc20Decimals      = uint8(18)
	cosmosTokenBase    = "acoin"
	cosmosTokenDisplay = "coin"
	cosmosDecimals     = uint8(6)
	defaultExponent    = uint32(18)
	zeroExponent       = uint32(0)
	ibcBase            = "ibc/7B2A4F6E798182988D77B6B884919AF617A73503FDAC27C916CD7A69A69013CF"
)

var metadataIbc = banktypes.Metadata{
	Description: "ATOM IBC voucher (channel 14)",
	Base:        ibcBase,
	// NOTE: Denom units MUST be increasing
	DenomUnits: []*banktypes.DenomUnit{
		{
			Denom:    ibcBase,
			Exponent: 0,
		},
	},
	Name:    "ATOM channel-14",
	Symbol:  "ibcATOM-14",
	Display: ibcBase,
}

// setupRegisterERC20Pair deploys an ERC20 smart contract and
// registers it as ERC20.
func (s *KeeperTestSuite) setupRegisterERC20Pair(contractType int) (common.Address, error) {
	var (
		contract common.Address
		err      error
	)
	// Deploy contract
	switch contractType {
	case contractDirectBalanceManipulation:
		contract, err = s.DeployContractDirectBalanceManipulation()
	case contractMaliciousDelayed:
		contract, err = s.DeployContractMaliciousDelayed()
	case contractBytes32Metadata:
		contract, err = s.DeployBytes32MetadataTokenContract(erc20Name, erc20Symbol)
	default:
		contract, err = s.DeployContract(erc20Name, erc20Symbol, erc20Decimals)
	}

	if err != nil {
		return common.Address{}, err
	}
	if err := s.network.NextBlock(); err != nil {
		return common.Address{}, err
	}

	// submit gov proposal to register ERC20 token pair
	_, err = utils.RegisterERC20(s.factory, s.network, utils.ERC20RegistrationData{
		Addresses:    []string{contract.Hex()},
		ProposerPriv: s.keyring.GetPrivKey(0),
	})

	return contract, err
}

func (s *KeeperTestSuite) TestRegisterERC20() {
	var (
		ctx          sdk.Context
		contractAddr common.Address
		pair         types.TokenPair
	)
	testCases := []struct {
		name     string
		malleate func()
		signer   string
		expPass  bool
	}{
		{
			"token ERC20 already registered",
			func() {
				s.network.App.GetErc20Keeper().SetERC20Map(ctx, pair.GetERC20Contract(), pair.GetID())
			},
			s.keyring.GetAccAddr(0).String(),
			false,
		},
		{
			"denom already registered",
			func() {
				s.network.App.GetErc20Keeper().SetDenomMap(ctx, pair.Denom, pair.GetID())
			},
			s.keyring.GetAccAddr(0).String(),
			false,
		},
		{
			"meta data already stored",
			func() {
				s.network.App.GetErc20Keeper().CreateCoinMetadata(ctx, contractAddr) //nolint:errcheck
			},
			s.keyring.GetAccAddr(0).String(),
			false,
		},
		{
			"ok - governance, permissionless false",
			func() {
				s.network.App.GetErc20Keeper().SetPermissionlessRegistration(ctx, false)
			},
			authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			true,
		},
		{
			"ok - governance, permissionless true",
			func() {},
			authtypes.NewModuleAddress(govtypes.ModuleName).String(),
			true,
		},
		{
			"fail - non-governance, permissionless false",
			func() {
				s.network.App.GetErc20Keeper().SetPermissionlessRegistration(ctx, false)
			},
			s.keyring.GetAccAddr(0).String(),
			false,
		},
		{
			"ok - non-governance, permissionless true",
			func() {},
			s.keyring.GetAccAddr(0).String(),
			true,
		},
		{
			"force fail evm",
			func() {
				mockEVMKeeper := &erc20mocks.EVMKeeper{}

				transferKeeper := s.network.App.GetTransferKeeper()
				erc20Keeper := keeper.NewKeeper(
					s.network.App.GetKey("erc20"), s.network.App.AppCodec(),
					authtypes.NewModuleAddress(govtypes.ModuleName), s.network.App.GetAccountKeeper(),
					s.network.App.GetBankKeeper(), mockEVMKeeper, s.network.App.GetStakingKeeper(),
					&transferKeeper,
				)
				s.network.App.SetErc20Keeper(erc20Keeper)

				mockEVMKeeper.On("EstimateGasInternal", mock.Anything, mock.Anything, mock.Anything).Return(&evmtypes.EstimateGasResponse{Gas: uint64(200)}, nil)
				mockEVMKeeper.On("CallEVM", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced CallEVM error"))
				mockEVMKeeper.On("ApplyMessage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("forced ApplyMessage error"))
			},
			s.keyring.GetAccAddr(0).String(),
			false,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			var err error
			s.SetupTest() // reset

			contractAddr, err = s.factory.DeployContract(
				s.keyring.GetPrivKey(0),
				evmtypes.EvmTxArgs{},
				testutiltypes.ContractDeploymentData{
					Contract:        contracts.ERC20MinterBurnerDecimalsContract,
					ConstructorArgs: []interface{}{erc20Name, erc20Symbol, cosmosDecimals},
				},
			)
			s.Require().NoError(err, "failed to deploy contract")
			s.Require().NoError(s.network.NextBlock(), "failed to advance block")

			coinName := types.CreateDenom(contractAddr.String())
			pair = types.NewTokenPair(contractAddr, coinName, types.OWNER_EXTERNAL)

			ctx = s.network.GetContext()

			tc.malleate()

			_, err = s.network.App.GetErc20Keeper().RegisterERC20(ctx, &types.MsgRegisterERC20{
				Signer:         tc.signer,
				Erc20Addresses: []string{contractAddr.Hex()},
			})
			metadata, found := s.network.App.GetBankKeeper().GetDenomMetaData(ctx, coinName)
			if tc.expPass {
				s.Require().NoError(err, tc.name)
				// Metadata variables
				s.Require().True(found)
				s.Require().Equal(coinName, metadata.Base)
				s.Require().Equal(coinName, metadata.Name)
				s.Require().Equal(types.SanitizeERC20Name(erc20Name), metadata.Display)
				s.Require().Equal(erc20Symbol, metadata.Symbol)
				// Denom units
				s.Require().Equal(len(metadata.DenomUnits), 2)
				s.Require().Equal(coinName, metadata.DenomUnits[0].Denom)
				s.Require().Equal(zeroExponent, metadata.DenomUnits[0].Exponent)
				s.Require().Equal(types.SanitizeERC20Name(erc20Name), metadata.DenomUnits[1].Denom)
				// Custom exponent at contract creation matches coin with token
				s.Require().Equal(metadata.DenomUnits[1].Exponent, uint32(cosmosDecimals))
			} else {
				s.Require().Error(err, tc.name)
			}
		})
	}
}

func (s *KeeperTestSuite) TestToggleConverision() {
	var (
		ctx          sdk.Context
		err          error
		contractAddr common.Address
		id           []byte
		pair         types.TokenPair
	)

	testCases := []struct {
		name              string
		malleate          func()
		expPass           bool
		conversionEnabled bool
	}{
		{
			"token not registered",
			func() {
				contractAddr, err = s.factory.DeployContract(
					s.keyring.GetPrivKey(0),
					evmtypes.EvmTxArgs{},
					testutiltypes.ContractDeploymentData{
						Contract:        contracts.ERC20MinterBurnerDecimalsContract,
						ConstructorArgs: []interface{}{erc20Name, erc20Symbol, erc20Decimals},
					},
				)
				s.Require().NoError(err, "failed to deploy contract")
				s.Require().NoError(s.network.NextBlock(), "failed to advance block")

				pair = types.NewTokenPair(contractAddr, cosmosTokenBase, types.OWNER_MODULE)
			},
			false,
			false,
		},
		{
			"token not registered - pair not found",
			func() {
				contractAddr, err = s.factory.DeployContract(
					s.keyring.GetPrivKey(0),
					evmtypes.EvmTxArgs{},
					testutiltypes.ContractDeploymentData{
						Contract:        contracts.ERC20MinterBurnerDecimalsContract,
						ConstructorArgs: []interface{}{erc20Name, erc20Symbol, erc20Decimals},
					},
				)
				s.Require().NoError(err, "failed to deploy contract")
				s.Require().NoError(s.network.NextBlock(), "failed to advance block")

				pair = types.NewTokenPair(contractAddr, cosmosTokenBase, types.OWNER_MODULE)
				s.network.App.GetErc20Keeper().SetERC20Map(ctx, common.HexToAddress(pair.Erc20Address), pair.GetID())
			},
			false,
			false,
		},
		{
			"disable conversion",
			func() {
				contractAddr, err = s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id = s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
			},
			true,
			false,
		},
		{
			"disable and enable conversion",
			func() {
				contractAddr, err = s.setupRegisterERC20Pair(contractMinterBurner)
				s.Require().NoError(err, "failed to register pair")
				ctx = s.network.GetContext()
				id = s.network.App.GetErc20Keeper().GetTokenPairID(ctx, contractAddr.String())
				pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
				res, err := s.network.App.GetErc20Keeper().ToggleConversion(ctx, &types.MsgToggleConversion{Authority: authtypes.NewModuleAddress("gov").String(), Token: contractAddr.String()})
				s.Require().NoError(err)
				s.Require().NotNil(res)
				pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
			},
			true,
			true,
		},
	}
	for _, tc := range testCases {
		s.Run(fmt.Sprintf("Case %s", tc.name), func() {
			s.SetupTest() // reset
			ctx = s.network.GetContext()

			tc.malleate()

			_, err = s.network.App.GetErc20Keeper().ToggleConversion(ctx, &types.MsgToggleConversion{Authority: authtypes.NewModuleAddress("gov").String(), Token: contractAddr.String()})
			// Request the pair using the GetPairToken func to make sure that is updated on the db
			pair, _ = s.network.App.GetErc20Keeper().GetTokenPair(ctx, id)
			if tc.expPass {
				s.Require().NoError(err, tc.name)
				if tc.conversionEnabled {
					s.Require().True(pair.Enabled)
				} else {
					s.Require().False(pair.Enabled)
				}
			} else {
				s.Require().Error(err, tc.name)
			}
		})
	}
}
