package types_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"
)

func newIntPtr(i int64) *sdkmath.Int {
	v := sdkmath.NewInt(i)
	return &v
}

func TestChainConfigValidate(t *testing.T) {
	testCases := []struct {
		name     string
		config   types.ChainConfig
		expError bool
	}{
		{"default", *types.DefaultChainConfig(0), false},
		{
			"valid",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(0),
				MuirGlacierBlock:    newIntPtr(0),
				BerlinBlock:         newIntPtr(0),
				LondonBlock:         newIntPtr(0),
				CancunTime:          newIntPtr(0),
				ShanghaiTime:        newIntPtr(0),
				PragueTime:          newIntPtr(0),
				VerkleTime:          newIntPtr(0),
			},
			false,
		},
		{
			"valid with nil values",
			types.ChainConfig{
				HomesteadBlock:      nil,
				DAOForkBlock:        nil,
				EIP150Block:         nil,
				EIP155Block:         nil,
				EIP158Block:         nil,
				ByzantiumBlock:      nil,
				ConstantinopleBlock: nil,
				PetersburgBlock:     nil,
				IstanbulBlock:       nil,
				MuirGlacierBlock:    nil,
				BerlinBlock:         nil,
				LondonBlock:         nil,
				CancunTime:          nil,
				ShanghaiTime:        nil,
			},
			false,
		},
		{
			"empty",
			types.ChainConfig{},
			false,
		},
		{
			"invalid HomesteadBlock",
			types.ChainConfig{
				HomesteadBlock: newIntPtr(-1),
			},
			true,
		},
		{
			"invalid DAOForkBlock",
			types.ChainConfig{
				HomesteadBlock: newIntPtr(0),
				DAOForkBlock:   newIntPtr(-1),
			},
			true,
		},
		{
			"invalid EIP150Block",
			types.ChainConfig{
				HomesteadBlock: newIntPtr(0),
				DAOForkBlock:   newIntPtr(0),
				EIP150Block:    newIntPtr(-1),
			},
			true,
		},
		{
			"invalid EIP155Block",
			types.ChainConfig{
				HomesteadBlock: newIntPtr(0),
				DAOForkBlock:   newIntPtr(0),
				EIP150Block:    newIntPtr(0),
				EIP155Block:    newIntPtr(-1),
			},
			true,
		},
		{
			"invalid EIP158Block",
			types.ChainConfig{
				HomesteadBlock: newIntPtr(0),
				DAOForkBlock:   newIntPtr(0),
				EIP150Block:    newIntPtr(0),
				EIP155Block:    newIntPtr(0),
				EIP158Block:    newIntPtr(-1),
			},
			true,
		},
		{
			"invalid ByzantiumBlock",
			types.ChainConfig{
				HomesteadBlock: newIntPtr(0),
				DAOForkBlock:   newIntPtr(0),
				EIP150Block:    newIntPtr(0),
				EIP155Block:    newIntPtr(0),
				EIP158Block:    newIntPtr(0),
				ByzantiumBlock: newIntPtr(-1),
			},
			true,
		},
		{
			"invalid ConstantinopleBlock",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(-1),
			},
			true,
		},
		{
			"invalid PetersburgBlock",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(-1),
			},
			true,
		},
		{
			"invalid IstanbulBlock",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(-1),
			},
			true,
		},
		{
			"invalid MuirGlacierBlock",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(0),
				MuirGlacierBlock:    newIntPtr(-1),
			},
			true,
		},
		{
			"invalid BerlinBlock",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(0),
				MuirGlacierBlock:    newIntPtr(0),
				BerlinBlock:         newIntPtr(-1),
			},
			true,
		},
		{
			"invalid LondonBlock",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(0),
				MuirGlacierBlock:    newIntPtr(0),
				BerlinBlock:         newIntPtr(0),
				LondonBlock:         newIntPtr(-1),
			},
			true,
		},
		{
			"invalid ArrowGlacierBlock",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(0),
				MuirGlacierBlock:    newIntPtr(0),
				BerlinBlock:         newIntPtr(0),
				LondonBlock:         newIntPtr(0),
				ArrowGlacierBlock:   newIntPtr(-1),
			},
			true,
		},
		{
			"invalid GrayGlacierBlock",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(0),
				MuirGlacierBlock:    newIntPtr(0),
				BerlinBlock:         newIntPtr(0),
				LondonBlock:         newIntPtr(0),
				ArrowGlacierBlock:   newIntPtr(0),
				GrayGlacierBlock:    newIntPtr(-1),
			},
			true,
		},
		{
			"invalid MergeNetsplitBlock",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(0),
				MuirGlacierBlock:    newIntPtr(0),
				BerlinBlock:         newIntPtr(0),
				LondonBlock:         newIntPtr(0),
				ArrowGlacierBlock:   newIntPtr(0),
				GrayGlacierBlock:    newIntPtr(0),
				MergeNetsplitBlock:  newIntPtr(-1),
			},
			true,
		},
		{
			"invalid fork order - skip HomesteadBlock",
			types.ChainConfig{
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(0),
				MuirGlacierBlock:    newIntPtr(0),
				BerlinBlock:         newIntPtr(0),
				LondonBlock:         newIntPtr(0),
			},
			true,
		},
		{
			"invalid ShanghaiTime",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(0),
				MuirGlacierBlock:    newIntPtr(0),
				BerlinBlock:         newIntPtr(0),
				LondonBlock:         newIntPtr(0),
				ArrowGlacierBlock:   newIntPtr(0),
				GrayGlacierBlock:    newIntPtr(0),
				MergeNetsplitBlock:  newIntPtr(0),
				ShanghaiTime:        newIntPtr(-1),
			},
			true,
		},
		{
			"invalid CancunTime",
			types.ChainConfig{
				HomesteadBlock:      newIntPtr(0),
				DAOForkBlock:        newIntPtr(0),
				EIP150Block:         newIntPtr(0),
				EIP155Block:         newIntPtr(0),
				EIP158Block:         newIntPtr(0),
				ByzantiumBlock:      newIntPtr(0),
				ConstantinopleBlock: newIntPtr(0),
				PetersburgBlock:     newIntPtr(0),
				IstanbulBlock:       newIntPtr(0),
				MuirGlacierBlock:    newIntPtr(0),
				BerlinBlock:         newIntPtr(0),
				LondonBlock:         newIntPtr(0),
				ArrowGlacierBlock:   newIntPtr(0),
				GrayGlacierBlock:    newIntPtr(0),
				MergeNetsplitBlock:  newIntPtr(0),
				ShanghaiTime:        newIntPtr(0),
				CancunTime:          newIntPtr(-1),
			},
			true,
		},
	}

	for _, tc := range testCases {
		err := tc.config.Validate()

		if tc.expError {
			require.Error(t, err, tc.name)
		} else {
			require.NoError(t, err, tc.name)
		}
	}
}
