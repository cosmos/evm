package erc20

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	abcitypes "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/contracts"
	testutiltypes "github.com/cosmos/evm/testutil/types"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func (s *KeeperTestSuite) MintERC20Token(contractAddr, to common.Address, amount *big.Int) (abcitypes.ExecTxResult, error) {
	res, err := s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{
			To: &contractAddr,
		},
		testutiltypes.CallArgs{
			ContractABI: contracts.ERC20MinterBurnerDecimalsContract.ABI,
			MethodName:  "mint",
			Args:        []interface{}{to, amount},
		},
	)
	if err != nil {
		return res, err
	}

	return res, s.network.NextBlock()
}

func (s *KeeperTestSuite) BalanceOf(contract, account common.Address) (interface{}, error) {
	erc20 := contracts.ERC20MinterBurnerDecimalsContract.ABI

	res, err := s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{
			To: &contract,
		},
		testutiltypes.CallArgs{
			ContractABI: erc20,
			MethodName:  "balanceOf",
			Args:        []interface{}{account},
		},
	)
	if err != nil {
		return nil, err
	}

	ethRes, err := evmtypes.DecodeTxResponse(res.Data)
	if err != nil {
		return nil, err
	}

	unpacked, err := erc20.Unpack("balanceOf", ethRes.Ret)
	if err != nil {
		return nil, err
	}
	if len(unpacked) == 0 {
		return nil, errors.New("nothing unpacked from response")
	}

	return unpacked[0], s.network.NextBlock()
}
