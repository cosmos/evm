package common

import (
	"fmt"
	"math/big"
	"reflect"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// TrueValue is the byte array representing a true value in solidity.
var TrueValue = []byte{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1}

// ICS20Allocation defines the spend limit for a particular port and channel.
// We need this to be able to unpack to big.Int instead of math.Int.
type ICS20Allocation struct {
	SourcePort        string
	SourceChannel     string
	SpendLimit        []Coin
	AllowList         []string
	AllowedPacketData []string
}

// Coin defines a struct that stores all needed information about a coin
// in types native to the EVM.
type Coin struct {
	Denom  string
	Amount *big.Int
}

// DecCoin defines a struct that stores all needed information about a decimal coin
// in types native to the EVM.
type DecCoin struct {
	Denom     string
	Amount    *big.Int
	Precision uint8
}

// Dec defines a struct that represents a decimal number of a given precision
// in types native to the EVM.
type Dec struct {
	Value     *big.Int
	Precision uint8
}

// ToSDKType converts the Coin to the Cosmos SDK representation.
func (c Coin) ToSDKType() sdk.Coin {
	return sdk.NewCoin(c.Denom, math.NewIntFromBigInt(c.Amount))
}

// NewCoinsResponse converts a response to an array of Coin.
func NewCoinsResponse(amount sdk.Coins) []Coin {
	// Create a new output for each coin and add it to the output array.
	outputs := make([]Coin, len(amount))
	for i, coin := range amount {
		outputs[i] = Coin{
			Denom:  coin.Denom,
			Amount: coin.Amount.BigInt(),
		}
	}
	return outputs
}

// NewDecCoinsResponse converts a response to an array of DecCoin.
func NewDecCoinsResponse(amount sdk.DecCoins) []DecCoin {
	// Create a new output for each coin and add it to the output array.
	outputs := make([]DecCoin, len(amount))
	for i, coin := range amount {
		outputs[i] = DecCoin{
			Denom:     coin.Denom,
			Amount:    coin.Amount.TruncateInt().BigInt(),
			Precision: math.LegacyPrecision,
		}
	}
	return outputs
}

// SafeAdd adds two integers and returns a boolean if an overflow occurs to avoid panic.
// TODO: Upstream this to the SDK math package.
func SafeAdd(a, b math.Int) (res *big.Int, overflow bool) {
	res = a.BigInt().Add(a.BigInt(), b.BigInt())
	return res, res.BitLen() > math.MaxBitLen
}

// ToCoins converts a value returned from the ABI to a slice of Coin.
func ToCoins(v interface{}) ([]Coin, error) {
	// Fast-path: if ABI already returned []Coin (e.g. in tests) just cast.
	if coins, ok := v.([]Coin); ok {
		return coins, nil
	}

	// Slow-path: reflect over anonymous struct slice.
	rv := reflect.ValueOf(v)
	if rv.Kind() != reflect.Slice {
		return nil, fmt.Errorf("expected slice, got %T", v)
	}

	out := make([]Coin, rv.Len())
	for i := 0; i < rv.Len(); i++ {
		item := rv.Index(i)
		denomField := item.FieldByName("Denom")
		amountField := item.FieldByName("Amount")

		// Field lookup failure would panic → treat as programmer error.
		if !denomField.IsValid() || !amountField.IsValid() {
			return nil, fmt.Errorf("coin tuple does not have expected fields")
		}

		denom, ok1 := denomField.Interface().(string)
		amount, ok2 := amountField.Interface().(*big.Int)
		if !ok1 || !ok2 || amount == nil || denom == "" {
			return nil, fmt.Errorf("invalid coin at index %d", i)
		}

		out[i] = Coin{Denom: denom, Amount: amount}
	}
	return out, nil
}

// NewSdkCoinsFromCoins converts a slice of Coin to sdk.Coins.
func NewSdkCoinsFromCoins(coins []Coin) (sdk.Coins, error) {
	sdkCoins := make(sdk.Coins, len(coins))
	for i, coin := range coins {
		sdkCoin := sdk.Coin{
			Denom:  coin.Denom,
			Amount: math.NewIntFromBigInt(coin.Amount),
		}
		if err := sdkCoin.Validate(); err != nil {
			return nil, err
		}

		sdkCoins[i] = sdkCoin
	}
	return sdkCoins.Sort(), nil
}
