package eip712

import (
	"testing"

	"github.com/ethereum/go-ethereum/signer/core/apitypes"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestCreateEIP712MessagePayloadStringifiesOpaqueJSONMsg(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"account_number":"0",
		"chain_id":"evm-1",
		"fee":{"amount":[],"gas":"200000"},
		"memo":"",
		"msgs":[{
			"type":"wasm/MsgExecuteContract",
			"value":{
				"contract":"cosmos1contract",
				"funds":[],
				"msg":{"transfer":{"amount":"10","recipient":"cosmos1recipient"}},
				"sender":"cosmos1sender"
			}
		}],
		"sequence":"0"
	}`)

	payload, err := createEIP712MessagePayload(data)
	require.NoError(t, err)

	msg := payload.payload.Get("msg0.value.msg")
	require.Equal(t, gjson.String, msg.Type)
	require.JSONEq(t, `{"transfer":{"amount":"10","recipient":"cosmos1recipient"}}`, msg.Str)

	types, err := createEIP712Types(payload)
	require.NoError(t, err)
	require.Contains(t, types["TypeValue0"], apitypes.Type{Name: "msg", Type: ethString})
	require.NotContains(t, types, "TypeValueMsg0")
}

func TestCreateEIP712MessagePayloadHandlesNestedAndArrayJSONMsg(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"account_number":"0",
		"chain_id":"evm-1",
		"fee":{"amount":[],"gas":"200000"},
		"memo":"",
		"msgs":[{
			"type":"cosmos.authz.v1beta1/MsgExec",
			"value":{"msgs":[{"type":"wasm/MsgExecuteContract","value":{"msg":[{"mint":{}},2]}}]}
		}],
		"sequence":"0"
	}`)

	payload, err := createEIP712MessagePayload(data)
	require.NoError(t, err)

	msg := payload.payload.Get("msg0.value.msgs.0.value.msg")
	require.Equal(t, gjson.String, msg.Type)
	require.JSONEq(t, `[{"mint":{}},2]`, msg.Str)
}

func TestCreateEIP712MessagePayloadPreservesStringMsg(t *testing.T) {
	t.Parallel()

	for _, value := range []string{
		`"ordinary text"`,
		`"{\"already\":\"stringified\"}"`,
	} {
		data := []byte(`{
			"account_number":"0",
			"chain_id":"evm-1",
			"fee":{"amount":[],"gas":"200000"},
			"memo":"",
			"msgs":[{"type":"example/Msg","value":{"msg":` + value + `}}],
			"sequence":"0"
		}`)

		original := gjson.ParseBytes(data).Get("msgs.0.value.msg").Str
		payload, err := createEIP712MessagePayload(data)
		require.NoError(t, err)
		require.Equal(t, original, payload.payload.Get("msg0.value.msg").Str)
	}
}

func TestCreateEIP712MessagePayloadPreservesTypedObjects(t *testing.T) {
	t.Parallel()

	data := []byte(`{
		"account_number":"0",
		"chain_id":"evm-1",
		"fee":{"amount":[],"gas":"200000"},
		"memo":"",
		"msgs":[{"type":"example/Msg","value":{"details":{"enabled":true},"msg":"text"}}],
		"sequence":"0"
	}`)

	payload, err := createEIP712MessagePayload(data)
	require.NoError(t, err)
	require.True(t, payload.payload.Get("msg0.value.details").IsObject())
}
