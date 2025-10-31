package vm

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	ethtypes "github.com/ethereum/go-ethereum/core/types"
	"github.com/stretchr/testify/require"

	testconstants "github.com/cosmos/evm/testutil/constants"
	utiltx "github.com/cosmos/evm/testutil/tx"
	vmkeeper "github.com/cosmos/evm/x/vm/keeper"
	"github.com/cosmos/evm/x/vm/keeper/testdata"
	"github.com/cosmos/evm/x/vm/types"

	sdkmath "cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

func SetupContract(b *testing.B) (*KeeperTestSuite, common.Address) {
	b.Helper()
	suite := KeeperTestSuite{}
	suite.SetupTest()

	amt := sdk.Coins{sdk.NewInt64Coin(testconstants.ExampleAttoDenom, 1000000000000000000)}
	err := suite.Network.App.GetBankKeeper().MintCoins(suite.Network.GetContext(), types.ModuleName, amt)
	require.NoError(b, err)
	err = suite.Network.App.GetBankKeeper().SendCoinsFromModuleToAccount(suite.Network.GetContext(), types.ModuleName, suite.Keyring.GetAddr(0).Bytes(), amt)
	require.NoError(b, err)

	contractAddr := suite.DeployTestContract(b, suite.Network.GetContext(), suite.Keyring.GetAddr(0), sdkmath.NewIntWithDecimal(1000, 18).BigInt())
	err = suite.Network.NextBlock()
	require.NoError(b, err)

	return &suite, contractAddr
}

func SetupTestMessageCall(b *testing.B) (*KeeperTestSuite, common.Address) {
	b.Helper()
	suite := KeeperTestSuite{}
	suite.SetupTest()

	amt := sdk.Coins{sdk.NewInt64Coin(testconstants.ExampleAttoDenom, 1000000000000000000)}
	err := suite.Network.App.GetBankKeeper().MintCoins(suite.Network.GetContext(), types.ModuleName, amt)
	require.NoError(b, err)
	err = suite.Network.App.GetBankKeeper().SendCoinsFromModuleToAccount(suite.Network.GetContext(), types.ModuleName, suite.Keyring.GetAddr(0).Bytes(), amt)
	require.NoError(b, err)

	contractAddr := suite.DeployTestMessageCall(b)
	err = suite.Network.NextBlock()
	require.NoError(b, err)

	return &suite, contractAddr
}

type TxBuilder func(suite *KeeperTestSuite, contract common.Address) *types.MsgEthereumTx

func DoBenchmark(b *testing.B, txBuilder TxBuilder) {
	b.Helper()
	suite, contractAddr := SetupContract(b)

	krSigner := utiltx.NewSigner(suite.Keyring.GetPrivKey(0))
	msg := txBuilder(suite, contractAddr)
	msg.From = suite.Keyring.GetAddr(0).Bytes()
	err := msg.Sign(ethtypes.LatestSignerForChainID(types.GetEthChainConfig().ChainID), krSigner)
	require.NoError(b, err)

	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		ctx, _ := suite.Network.GetContext().CacheContext()

		fees := sdk.Coins{sdk.NewCoin(suite.EvmDenom(), sdkmath.NewIntFromBigInt(msg.GetFee()))}
		err = vmkeeper.DeductFees(suite.Network.App.GetBankKeeper(), suite.Network.App.GetEVMKeeper(), suite.Network.GetContext(), suite.Network.App.GetAccountKeeper().GetAccount(ctx, msg.GetFrom()), fees)
		require.NoError(b, err)

		rsp, err := suite.Network.App.GetEVMKeeper().EthereumTx(ctx, msg)
		require.NoError(b, err)
		require.False(b, rsp.Failed())
	}
}

func BenchmarkTokenTransfer(b *testing.B) {
	erc20Contract, err := testdata.LoadERC20Contract()
	require.NoError(b, err, "failed to load erc20 contract")

	DoBenchmark(b, func(suite *KeeperTestSuite, contract common.Address) *types.MsgEthereumTx {
		input, err := erc20Contract.ABI.Pack("transfer", common.HexToAddress("0x378c50D9264C63F3F92B806d4ee56E9D86FfB3Ec"), big.NewInt(1000))
		require.NoError(b, err)
		nonce := suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), suite.Keyring.GetAddr(0))
		ethTxParams := &types.EvmTxArgs{
			ChainID:  types.GetEthChainConfig().ChainID,
			Nonce:    nonce,
			To:       &contract,
			Amount:   big.NewInt(0),
			GasLimit: 410000,
			GasPrice: big.NewInt(1),
			Input:    input,
		}
		return types.NewTx(ethTxParams)
	})
}

func BenchmarkEmitLogs(b *testing.B) {
	erc20Contract, err := testdata.LoadERC20Contract()
	require.NoError(b, err, "failed to load erc20 contract")

	DoBenchmark(b, func(suite *KeeperTestSuite, contract common.Address) *types.MsgEthereumTx {
		input, err := erc20Contract.ABI.Pack("benchmarkLogs", big.NewInt(1000))
		require.NoError(b, err)
		nonce := suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), suite.Keyring.GetAddr(0))
		ethTxParams := &types.EvmTxArgs{
			ChainID:  types.GetEthChainConfig().ChainID,
			Nonce:    nonce,
			To:       &contract,
			Amount:   big.NewInt(0),
			GasLimit: 4100000,
			GasPrice: big.NewInt(1),
			Input:    input,
		}
		return types.NewTx(ethTxParams)
	})
}

func BenchmarkTokenTransferFrom(b *testing.B) {
	erc20Contract, err := testdata.LoadERC20Contract()
	require.NoError(b, err)

	DoBenchmark(b, func(suite *KeeperTestSuite, contract common.Address) *types.MsgEthereumTx {
		input, err := erc20Contract.ABI.Pack("transferFrom", suite.Keyring.GetAddr(0), common.HexToAddress("0x378c50D9264C63F3F92B806d4ee56E9D86FfB3Ec"), big.NewInt(0))
		require.NoError(b, err)
		nonce := suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), suite.Keyring.GetAddr(0))
		ethTxParams := &types.EvmTxArgs{
			ChainID:  types.GetEthChainConfig().ChainID,
			Nonce:    nonce,
			To:       &contract,
			Amount:   big.NewInt(0),
			GasLimit: 410000,
			GasPrice: big.NewInt(1),
			Input:    input,
		}
		return types.NewTx(ethTxParams)
	})
}

func BenchmarkTokenMint(b *testing.B) {
	erc20Contract, err := testdata.LoadERC20Contract()
	require.NoError(b, err, "failed to load erc20 contract")

	DoBenchmark(b, func(suite *KeeperTestSuite, contract common.Address) *types.MsgEthereumTx {
		input, err := erc20Contract.ABI.Pack("mint", common.HexToAddress("0x378c50D9264C63F3F92B806d4ee56E9D86FfB3Ec"), big.NewInt(1000))
		require.NoError(b, err)
		nonce := suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), suite.Keyring.GetAddr(0))
		ethTxParams := &types.EvmTxArgs{
			ChainID:  types.GetEthChainConfig().ChainID,
			Nonce:    nonce,
			To:       &contract,
			Amount:   big.NewInt(0),
			GasLimit: 410000,
			GasPrice: big.NewInt(1),
			Input:    input,
		}
		return types.NewTx(ethTxParams)
	})
}

func BenchmarkMessageCall(b *testing.B) {
	suite, contract := SetupTestMessageCall(b)

	messageCallContract, err := testdata.LoadMessageCallContract()
	require.NoError(b, err, "failed to load message call contract")

	input, err := messageCallContract.ABI.Pack("benchmarkMessageCall", big.NewInt(10000))
	require.NoError(b, err)
	nonce := suite.Network.App.GetEVMKeeper().GetNonce(suite.Network.GetContext(), suite.Keyring.GetAddr(0))
	ethCfg := types.GetEthChainConfig()
	ethTxParams := &types.EvmTxArgs{
		ChainID:  ethCfg.ChainID,
		Nonce:    nonce,
		To:       &contract,
		Amount:   big.NewInt(0),
		GasLimit: 25000000,
		GasPrice: big.NewInt(1),
		Input:    input,
	}
	msg := types.NewTx(ethTxParams)

	msg.From = suite.Keyring.GetAddr(0).Bytes()
	krSigner := utiltx.NewSigner(suite.Keyring.GetPrivKey(0))
	err = msg.Sign(ethtypes.LatestSignerForChainID(ethCfg.ChainID), krSigner)
	require.NoError(b, err)

	b.ResetTimer()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		ctx, _ := suite.Network.GetContext().CacheContext()

		fees := sdk.Coins{sdk.NewCoin(suite.EvmDenom(), sdkmath.NewIntFromBigInt(msg.GetFee()))}
		err = vmkeeper.DeductFees(suite.Network.App.GetBankKeeper(), suite.Network.App.GetEVMKeeper(), suite.Network.GetContext(), suite.Network.App.GetAccountKeeper().GetAccount(ctx, msg.GetFrom()), fees)
		require.NoError(b, err)

		rsp, err := suite.Network.App.GetEVMKeeper().EthereumTx(ctx, msg)
		require.NoError(b, err)
		require.False(b, rsp.Failed())
	}
}
