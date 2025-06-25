package types

import (
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/suite"

	"github.com/cosmos/evm/crypto/ethsecp256k1"
)

type GenesisTestSuite struct {
	suite.Suite

	address string
	hash    common.Hash
	code    string
}

func (suite *GenesisTestSuite) SetupTest() {
	priv, err := ethsecp256k1.GenerateKey()
	suite.Require().NoError(err)

	suite.address = common.BytesToAddress(priv.PubKey().Address().Bytes()).String()
	suite.hash = common.BytesToHash([]byte("hash"))
	suite.code = common.Bytes2Hex([]byte{1, 2, 3})
}

func TestGenesisTestSuite(t *testing.T) {
	suite.Run(t, new(GenesisTestSuite))
}

func (suite *GenesisTestSuite) TestValidateGenesisAccount() {
	testCases := []struct {
		name           string
		genesisAccount GenesisAccount
		expPass        bool
	}{
		{
			"valid genesis account",
			GenesisAccount{
				Address: suite.address,
				Code:    suite.code,
				Storage: Storage{
					NewState(suite.hash, suite.hash),
				},
			},
			true,
		},
		{
			"empty account address bytes",
			GenesisAccount{
				Address: "",
				Code:    suite.code,
				Storage: Storage{
					NewState(suite.hash, suite.hash),
				},
			},
			false,
		},
		{
			"empty code bytes",
			GenesisAccount{
				Address: suite.address,
				Code:    "",
				Storage: Storage{
					NewState(suite.hash, suite.hash),
				},
			},
			true,
		},
	}

	for _, tc := range testCases {
		err := tc.genesisAccount.Validate()
		if tc.expPass {
			suite.Require().NoError(err, tc.name)
		} else {
			suite.Require().Error(err, tc.name)
		}
	}
}

func (suite *GenesisTestSuite) TestValidateGenesis() {
	defaultGenesis := DefaultGenesisState()

	testCases := []struct {
		name     string
		genState *GenesisState
		expPass  bool
	}{
		{
			name:     "default",
			genState: DefaultGenesisState(),
			expPass:  true,
		},
		{
			name: "valid genesis",
			genState: &GenesisState{
				Accounts: []GenesisAccount{
					{
						Address: suite.address,

						Code: suite.code,
						Storage: Storage{
							{Key: suite.hash.String()},
						},
					},
				},
				Params: DefaultParams(),
				Preinstalls: []Preinstall{
					{
						Address: "0x4e59b44847b379578588920ca78fbf26c0b4956c",
						Code:    "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3",
					},
				},
			},
			expPass: true,
		},
		{
			name:     "copied genesis",
			genState: NewGenesisState(defaultGenesis.Params, defaultGenesis.Accounts, defaultGenesis.Preinstalls),
			expPass:  true,
		},
		{
			name: "invalid genesis account",
			genState: &GenesisState{
				Accounts: []GenesisAccount{
					{
						Address: "123456",

						Code: suite.code,
						Storage: Storage{
							{Key: suite.hash.String()},
						},
					},
				},
				Params: DefaultParams(),
			},
			expPass: false,
		},
		{
			name: "duplicated genesis account",
			genState: &GenesisState{
				Accounts: []GenesisAccount{
					{
						Address: suite.address,

						Code: suite.code,
						Storage: Storage{
							NewState(suite.hash, suite.hash),
						},
					},
					{
						Address: suite.address,

						Code: suite.code,
						Storage: Storage{
							NewState(suite.hash, suite.hash),
						},
					},
				},
			},
			expPass: false,
		},
		{
			name: "invalid preinstall",
			genState: &GenesisState{
				Accounts: []GenesisAccount{},
				Params:   DefaultParams(),
				Preinstalls: []Preinstall{
					{
						Address: "invalid-address",
						Code:    "0x123",
					},
				},
			},
			expPass: false,
		},
		{
			name: "duplicated preinstall address",
			genState: &GenesisState{
				Accounts: []GenesisAccount{},
				Params:   DefaultParams(),
				Preinstalls: []Preinstall{
					{
						Address: "0x4e59b44847b379578588920ca78fbf26c0b4956c",
						Code:    "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3",
					},
					{
						Address: "0x4e59b44847b379578588920ca78fbf26c0b4956c",
						Code:    "0x123456",
					},
				},
			},
			expPass: false,
		},
		{
			name: "preinstall address conflicts with genesis account",
			genState: &GenesisState{
				Accounts: []GenesisAccount{
					{
						Address: suite.address,
						Code:    suite.code,
						Storage: Storage{
							{Key: suite.hash.String()},
						},
					},
				},
				Params: DefaultParams(),
				Preinstalls: []Preinstall{
					{
						Address: suite.address,
						Code:    "0x7fffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffe03601600081602082378035828234f58015156039578182fd5b8082525050506014600cf3",
					},
				},
			},
			expPass: false,
		},
	}

	for _, tc := range testCases {
		suite.Run(tc.name, func() {
			err := tc.genState.Validate()
			if tc.expPass {
				suite.Require().NoError(err, tc.name)
			} else {
				suite.Require().Error(err, tc.name)
			}
		})
	}
}
