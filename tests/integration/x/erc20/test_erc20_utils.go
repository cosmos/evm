package erc20

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"

	abcitypes "github.com/cometbft/cometbft/abci/types"

	"github.com/cosmos/evm/precompiles/erc20"
	erc20testdata "github.com/cosmos/evm/precompiles/erc20/testdata"
	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func (s *KeeperTestSuite) MintERC20Token(contractAddr, to common.Address, amount *big.Int) (abcitypes.ExecTxResult, error) {
	res, err := s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{
			To: &contractAddr,
		},
		erc20testdata.NewMintCall(to, amount),
	)
	if err != nil {
		return res, err
	}

	return res, s.network.NextBlock()
}

func (s *KeeperTestSuite) BalanceOf(contract, account common.Address) (interface{}, error) {
	res, err := s.factory.ExecuteContractCall(
		s.keyring.GetPrivKey(0),
		evmtypes.EvmTxArgs{
			To: &contract,
		},
		erc20.NewBalanceOfCall(account),
	)
	if err != nil {
		return nil, err
	}

	ethRes, err := evmtypes.DecodeTxResponse(res.Data)
	if err != nil {
		return nil, err
	}

	var out erc20.BalanceOfReturn
	_, err = out.Decode(ethRes.Ret)
	if err != nil {
		return nil, err
	}

	return out.Field1, s.network.NextBlock()
}
