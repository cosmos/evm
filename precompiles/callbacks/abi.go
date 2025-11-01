package callbacks

//go:generate go run github.com/yihuang/go-abi/cmd -input abi.json -module callback -external-tuples Coin=cmn.Coin,Dec=cmn.Dec,DecCoin=cmn.DecCoin,PageRequest=cmn.PageRequest -imports cmn=github.com/cosmos/evm/precompiles/common
