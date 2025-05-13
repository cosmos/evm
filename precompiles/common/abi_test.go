// Portions of this file are derived from the go-ethereum project,
// Copyright 2017 The go-ethereum Authors
// (https://github.com/ethereum/go-ethereum).
// Used under the terms of the GNU Lesser General Public License v3.0.
//
// The modifications and additional code are licensed under the Apache License 2.0.
// See the accompanying LICENSE file for full terms.
//
// Original go-ethereum license: https://www.gnu.org/licenses/lgpl-3.0.html

package common

import (
	"bytes"
	"math"
	"math/big"
	"reflect"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
)

func TestMakeTopic(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   interface{}
		want    common.Hash
		wantErr bool
	}{
		{
			name:  "common.Hash",
			input: common.HexToHash("0x01020304"),
			want: func() common.Hash {
				return common.HexToHash("0x01020304")
			}(),
			wantErr: false,
		},
		{
			name:  "common.Address",
			input: common.HexToAddress("0x0102030405060708090a0b0c0d0e0f1011121314"),
			want: func() common.Hash {
				var h common.Hash
				addr := common.HexToAddress("0x0102030405060708090a0b0c0d0e0f1011121314")
				copy(h[common.HashLength-common.AddressLength:], addr.Bytes())
				return h
			}(),
			wantErr: false,
		},
		{
			name: "positive *big.Int",
			input: func() *big.Int {
				return big.NewInt(123456789)
			}(),
			want: func() common.Hash {
				var h common.Hash
				blob := big.NewInt(123456789).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name: "negative *big.Int (not sign-extended)",
			input: func() *big.Int {
				return big.NewInt(-1)
			}(),
			want: func() common.Hash {
				var h common.Hash
				// big.Int.Bytes() returns the absolute value; no two's complement
				blob := big.NewInt(-1).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "bool: true",
			input: true,
			want: func() common.Hash {
				var h common.Hash
				h[common.HashLength-1] = 1
				return h
			}(),
			wantErr: false,
		},
		{
			name:    "bool: false",
			input:   false,
			want:    common.Hash{},
			wantErr: false,
		},
		{
			name:  "int8",
			input: int8(-2),
			want: func() common.Hash {
				return [32]byte{
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 254,
				}
			}(),
			wantErr: false,
		},
		{
			name:  "int16",
			input: int16(-1234),
			want: func() common.Hash {
				var h common.Hash
				// genIntType(-1234, 2) => two's complement for int16
				// you can verify by using the same method as genIntType in your code
				gen := genIntType(int64(int16(-1234)), 2)
				copy(h[:], gen)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "int32",
			input: int32(-56789),
			want: func() common.Hash {
				var h common.Hash
				// genIntType(-56789, 4)
				gen := genIntType(int64(int32(-56789)), 4)
				copy(h[:], gen)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "int64",
			input: int64(-5),
			want: func() common.Hash {
				return [32]byte{
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 255,
					255, 255, 255, 255, 255, 255, 255, 251,
				}
			}(),
			wantErr: false,
		},
		{
			name:  "uint8",
			input: uint8(255),
			want: func() common.Hash {
				var h common.Hash
				blob := new(big.Int).SetUint64(uint64(uint8(255))).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "uint16",
			input: uint16(65535),
			want: func() common.Hash {
				var h common.Hash
				blob := new(big.Int).SetUint64(uint64(uint16(65535))).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "uint32",
			input: uint32(4294967295),
			want: func() common.Hash {
				var h common.Hash
				blob := new(big.Int).SetUint64(uint64(uint32(4294967295))).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "uint64",
			input: uint64(4294967296),
			want: func() common.Hash {
				var h common.Hash
				blob := new(big.Int).SetUint64(4294967296).Bytes()
				copy(h[common.HashLength-len(blob):], blob)
				return h
			}(),
			wantErr: false,
		},
		{
			name:  "string",
			input: "hello world",
			want: func() common.Hash {
				// Strings are hashed using keccak256
				return crypto.Keccak256Hash([]byte("hello world"))
			}(),
			wantErr: false,
		},
		{
			name:  "[]byte",
			input: []byte{0x01, 0x02, 0x03},
			want: func() common.Hash {
				// Byte slices are hashed using keccak256
				return crypto.Keccak256Hash([]byte{0x01, 0x02, 0x03})
			}(),
			wantErr: false,
		},
		{
			name:  "[5]byte (static byte array)",
			input: [5]byte{0x01, 0x02, 0x03, 0x04, 0x05},
			want: func() common.Hash {
				var h common.Hash
				copy(h[0:5], []byte{0x01, 0x02, 0x03, 0x04, 0x05})
				return h
			}(),
			wantErr: false,
		},
		{
			name:    "unsupported type",
			input:   struct{ Foo string }{"bar"},
			want:    common.Hash{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		// Capture range variable for parallel tests
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := MakeTopic(tt.input)
			if tt.wantErr {
				require.Error(t, err, "expected error but got nil")
				return
			}
			require.NoError(t, err, "expected no error but got error")

			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("MakeTopic(%v) = \n%#v\nwant \n%#v", tt.input, got, tt.want)
			}
		})
	}
}

func TestPackNumber(t *testing.T) {
	t.Parallel()
	tests := []struct {
		value  reflect.Value
		packed []byte
	}{
		// Protocol limits
		{reflect.ValueOf(0), common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000000")},
		{reflect.ValueOf(1), common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000001")},
		{reflect.ValueOf(-1), common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff")},

		// Type corner cases
		{reflect.ValueOf(uint8(math.MaxUint8)), common.Hex2Bytes("00000000000000000000000000000000000000000000000000000000000000ff")},
		{reflect.ValueOf(uint16(math.MaxUint16)), common.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000ffff")},
		{reflect.ValueOf(uint32(math.MaxUint32)), common.Hex2Bytes("00000000000000000000000000000000000000000000000000000000ffffffff")},
		{reflect.ValueOf(uint64(math.MaxUint64)), common.Hex2Bytes("000000000000000000000000000000000000000000000000ffffffffffffffff")},

		{reflect.ValueOf(int8(math.MaxInt8)), common.Hex2Bytes("000000000000000000000000000000000000000000000000000000000000007f")},
		{reflect.ValueOf(int16(math.MaxInt16)), common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000007fff")},
		{reflect.ValueOf(int32(math.MaxInt32)), common.Hex2Bytes("000000000000000000000000000000000000000000000000000000007fffffff")},
		{reflect.ValueOf(int64(math.MaxInt64)), common.Hex2Bytes("0000000000000000000000000000000000000000000000007fffffffffffffff")},

		{reflect.ValueOf(int8(math.MinInt8)), common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff80")},
		{reflect.ValueOf(int16(math.MinInt16)), common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff8000")},
		{reflect.ValueOf(int32(math.MinInt32)), common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffffffffffff80000000")},
		{reflect.ValueOf(int64(math.MinInt64)), common.Hex2Bytes("ffffffffffffffffffffffffffffffffffffffffffffffff8000000000000000")},
	}
	for i, tt := range tests {
		packed := PackNum(tt.value)
		if !bytes.Equal(packed, tt.packed) {
			t.Errorf("test %d: pack mismatch: have %x, want %x", i, packed, tt.packed)
		}
	}
}
