// Package v2 handles the state migration from vm module consensus version 1 to 2.
//
// The Params proto schema changed between pushchain/evm v0.2.x and v0.3.x:
//
//	Old layout:                                New layout:
//	  evm_denom               = 1 (string)      evm_denom               = 1 (string)
//	  extra_eips              = 4 (int64[])      extra_eips              = 4 (int64[])
//	  chain_config            = 5 (message)      allow_unprotected_txs   = 5 (bool)
//	  allow_unprotected_txs   = 6 (bool)         evm_channels            = 7 (string[])
//	  evm_channels            = 8 (string[])     access_control          = 8 (message)
//	  access_control          = 9 (message)      active_static_precompiles = 9 (string[])
//	  active_static_precompiles = 10 (string[])
//
// Reading old bytes with the new decoder panics because field 5 is a
// length-delimited message but the new struct expects a varint bool.
package v2

import (
	storetypes "cosmossdk.io/store/types"

	"google.golang.org/protobuf/encoding/protowire"

	"github.com/cosmos/evm/x/vm/types"

	sdk "github.com/cosmos/cosmos-sdk/types"
)

// MigrateStore rewrites the vm Params KV entry from the v0.2.x proto layout
// to the v0.3.x layout.  It must run before any call to keeper.GetParams,
// which would otherwise panic on the mis-matched wire type at field 5.
func MigrateStore(ctx sdk.Context, storeKey storetypes.StoreKey) error {
	store := ctx.KVStore(storeKey)
	return migrateParams(store)
}

// oldParams holds the fields we care about from the v0.2.x Params encoding.
type oldParams struct {
	evmDenom                string
	extraEIPs               []int64
	allowUnprotectedTxs     bool
	evmChannels             []string
	accessControlRaw        []byte // raw bytes of the embedded AccessControl message
	activeStaticPrecompiles []string
}

// parseOldParams decodes raw protobuf bytes using the v0.2.x field-number layout.
// Unknown fields (e.g. chain_config at field 5) are skipped gracefully.
func parseOldParams(b []byte) (oldParams, error) {
	var p oldParams
	for len(b) > 0 {
		num, typ, n := protowire.ConsumeTag(b)
		if n < 0 {
			return p, protowire.ParseError(n)
		}
		b = b[n:]

		switch {
		// field 1: evm_denom
		case num == 1 && typ == protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return p, protowire.ParseError(n)
			}
			p.evmDenom = string(v)
			b = b[n:]

		// field 4: extra_eips — packed repeated int64
		case num == 4 && typ == protowire.BytesType:
			packed, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return p, protowire.ParseError(n)
			}
			b = b[n:]
			for len(packed) > 0 {
				v, n2 := protowire.ConsumeVarint(packed)
				if n2 < 0 {
					return p, protowire.ParseError(n2)
				}
				p.extraEIPs = append(p.extraEIPs, int64(v))
				packed = packed[n2:]
			}

		// field 5: chain_config (old) — skip the embedded-message bytes
		case num == 5 && typ == protowire.BytesType:
			_, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return p, protowire.ParseError(n)
			}
			b = b[n:]

		// field 6: allow_unprotected_txs (old position)
		case num == 6 && typ == protowire.VarintType:
			v, n := protowire.ConsumeVarint(b)
			if n < 0 {
				return p, protowire.ParseError(n)
			}
			p.allowUnprotectedTxs = v != 0
			b = b[n:]

		// field 8: evm_channels (old position) — each occurrence is one element
		case num == 8 && typ == protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return p, protowire.ParseError(n)
			}
			p.evmChannels = append(p.evmChannels, string(v))
			b = b[n:]

		// field 9: access_control (old position) — preserve raw bytes so we can
		// re-encode them unchanged at the new field number (8).
		case num == 9 && typ == protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return p, protowire.ParseError(n)
			}
			p.accessControlRaw = append([]byte(nil), v...)
			b = b[n:]

		// field 10: active_static_precompiles (old position)
		case num == 10 && typ == protowire.BytesType:
			v, n := protowire.ConsumeBytes(b)
			if n < 0 {
				return p, protowire.ParseError(n)
			}
			p.activeStaticPrecompiles = append(p.activeStaticPrecompiles, string(v))
			b = b[n:]

		default:
			// Skip any field we don't recognise
			n := protowire.ConsumeFieldValue(num, typ, b)
			if n < 0 {
				return p, protowire.ParseError(n)
			}
			b = b[n:]
		}
	}
	return p, nil
}

func migrateParams(store storetypes.KVStore) error {
	bz := store.Get(types.KeyPrefixParams)
	if bz == nil {
		// No params stored yet; nothing to migrate.
		return nil
	}

	old, err := parseOldParams(bz)
	if err != nil {
		return err
	}

	// Decode the AccessControl sub-message that was at field 9.
	var ac types.AccessControl
	if len(old.accessControlRaw) > 0 {
		if err := ac.Unmarshal(old.accessControlRaw); err != nil {
			return err
		}
	}

	// Start from DefaultParams so any newly-added fields get sensible values,
	// then overwrite with the values we recovered from the old encoding.
	newParams := types.DefaultParams()
	newParams.EvmDenom = old.evmDenom
	newParams.ExtraEIPs = old.extraEIPs
newParams.EVMChannels = old.evmChannels
	newParams.AccessControl = ac
	newParams.ActiveStaticPrecompiles = old.activeStaticPrecompiles

	newBz, err := newParams.Marshal()
	if err != nil {
		return err
	}

	store.Set(types.KeyPrefixParams, newBz)
	return nil
}
