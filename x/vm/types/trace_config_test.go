package types_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"

	evmtypes "github.com/cosmos/evm/x/vm/types"
)

func TestTraceConfig_UnmarshalJSON_ObjectFormat(t *testing.T) {
	// Ethereum standard format - tracerConfig as JSON object
	input := `{"tracer":"callTracer","tracerConfig":{"onlyTopCall":true}}`

	var tc evmtypes.TraceConfig
	err := json.Unmarshal([]byte(input), &tc)
	require.NoError(t, err)
	require.Equal(t, "callTracer", tc.Tracer)
	require.Equal(t, `{"onlyTopCall":true}`, tc.TracerJsonConfig)
}

func TestTraceConfig_UnmarshalJSON_ComplexObjectFormat(t *testing.T) {
	// More complex tracerConfig object
	input := `{"tracer":"callTracer","tracerConfig":{"onlyTopCall":true,"withLog":false,"diffMode":true}}`

	var tc evmtypes.TraceConfig
	err := json.Unmarshal([]byte(input), &tc)
	require.NoError(t, err)
	require.Equal(t, "callTracer", tc.Tracer)
	require.Equal(t, `{"onlyTopCall":true,"withLog":false,"diffMode":true}`, tc.TracerJsonConfig)
}

func TestTraceConfig_UnmarshalJSON_StringFormat(t *testing.T) {
	// Legacy format - tracerConfig as escaped string (backwards compatibility)
	input := `{"tracer":"callTracer","tracerConfig":"{\"onlyTopCall\":true}"}`

	var tc evmtypes.TraceConfig
	err := json.Unmarshal([]byte(input), &tc)
	require.NoError(t, err)
	require.Equal(t, "callTracer", tc.Tracer)
	// When input is a string, json.RawMessage preserves it as a quoted string
	require.Equal(t, `"{\"onlyTopCall\":true}"`, tc.TracerJsonConfig)
}

func TestTraceConfig_UnmarshalJSON_NoTracerConfig(t *testing.T) {
	input := `{"tracer":"callTracer","disableStack":true}`

	var tc evmtypes.TraceConfig
	err := json.Unmarshal([]byte(input), &tc)
	require.NoError(t, err)
	require.Equal(t, "callTracer", tc.Tracer)
	require.True(t, tc.DisableStack)
	require.Empty(t, tc.TracerJsonConfig)
}

func TestTraceConfig_UnmarshalJSON_AllFields(t *testing.T) {
	input := `{
		"tracer": "callTracer",
		"timeout": "10s",
		"reexec": 128,
		"disableStack": true,
		"disableStorage": true,
		"debug": true,
		"limit": 1000,
		"enableMemory": true,
		"enableReturnData": true,
		"tracerConfig": {"onlyTopCall": true}
	}`

	var tc evmtypes.TraceConfig
	err := json.Unmarshal([]byte(input), &tc)
	require.NoError(t, err)
	require.Equal(t, "callTracer", tc.Tracer)
	require.Equal(t, "10s", tc.Timeout)
	require.Equal(t, uint64(128), tc.Reexec)
	require.True(t, tc.DisableStack)
	require.True(t, tc.DisableStorage)
	require.True(t, tc.Debug)
	require.Equal(t, int32(1000), tc.Limit)
	require.True(t, tc.EnableMemory)
	require.True(t, tc.EnableReturnData)
	require.Equal(t, `{"onlyTopCall": true}`, tc.TracerJsonConfig)
}

func TestTraceConfig_UnmarshalJSON_EmptyObject(t *testing.T) {
	input := `{}`

	var tc evmtypes.TraceConfig
	err := json.Unmarshal([]byte(input), &tc)
	require.NoError(t, err)
	require.Empty(t, tc.Tracer)
	require.Empty(t, tc.TracerJsonConfig)
}

func TestTraceConfig_UnmarshalJSON_NullTracerConfig(t *testing.T) {
	input := `{"tracer":"callTracer","tracerConfig":null}`

	var tc evmtypes.TraceConfig
	err := json.Unmarshal([]byte(input), &tc)
	require.NoError(t, err)
	require.Equal(t, "callTracer", tc.Tracer)
	// null is preserved as "null" in json.RawMessage
	require.Equal(t, "null", tc.TracerJsonConfig)
}

func TestTraceConfig_MarshalJSON(t *testing.T) {
	tc := evmtypes.TraceConfig{
		Tracer:           "callTracer",
		TracerJsonConfig: `{"onlyTopCall":true}`,
	}

	data, err := json.Marshal(tc)
	require.NoError(t, err)

	// Verify it outputs as raw JSON object, not escaped string
	require.Contains(t, string(data), `"tracerConfig":{"onlyTopCall":true}`)
	require.NotContains(t, string(data), `"tracerConfig":"{`)
}

func TestTraceConfig_MarshalJSON_NoTracerConfig(t *testing.T) {
	tc := evmtypes.TraceConfig{
		Tracer: "callTracer",
	}

	data, err := json.Marshal(tc)
	require.NoError(t, err)

	// tracerConfig should be omitted when empty
	require.NotContains(t, string(data), `"tracerConfig"`)
}

func TestTraceConfig_MarshalJSON_AllFields(t *testing.T) {
	tc := evmtypes.TraceConfig{
		Tracer:           "callTracer",
		Timeout:          "10s",
		Reexec:           128,
		DisableStack:     true,
		DisableStorage:   true,
		Debug:            true,
		Limit:            1000,
		EnableMemory:     true,
		EnableReturnData: true,
		TracerJsonConfig: `{"onlyTopCall":true}`,
	}

	data, err := json.Marshal(tc)
	require.NoError(t, err)

	// Unmarshal to verify structure
	var result map[string]interface{}
	err = json.Unmarshal(data, &result)
	require.NoError(t, err)

	require.Equal(t, "callTracer", result["tracer"])
	require.Equal(t, "10s", result["timeout"])
	require.Equal(t, float64(128), result["reexec"])
	require.Equal(t, true, result["disableStack"])
	require.Equal(t, true, result["disableStorage"])
	require.Equal(t, true, result["debug"])
	require.Equal(t, float64(1000), result["limit"])
	require.Equal(t, true, result["enableMemory"])
	require.Equal(t, true, result["enableReturnData"])

	// tracerConfig should be a map, not a string
	tracerConfig, ok := result["tracerConfig"].(map[string]interface{})
	require.True(t, ok, "tracerConfig should be an object, not a string")
	require.Equal(t, true, tracerConfig["onlyTopCall"])
}

func TestTraceConfig_RoundTrip(t *testing.T) {
	original := evmtypes.TraceConfig{
		Tracer:           "callTracer",
		Timeout:          "5s",
		DisableStack:     true,
		EnableMemory:     true,
		TracerJsonConfig: `{"onlyTopCall":true,"withLog":false}`,
	}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var restored evmtypes.TraceConfig
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	require.Equal(t, original.Tracer, restored.Tracer)
	require.Equal(t, original.Timeout, restored.Timeout)
	require.Equal(t, original.DisableStack, restored.DisableStack)
	require.Equal(t, original.EnableMemory, restored.EnableMemory)
	require.Equal(t, original.TracerJsonConfig, restored.TracerJsonConfig)
}

func TestTraceConfig_RoundTrip_FromObjectInput(t *testing.T) {
	// Start with JSON object format (Ethereum standard)
	input := `{"tracer":"callTracer","tracerConfig":{"onlyTopCall":true,"withLog":false}}`

	var tc evmtypes.TraceConfig
	err := json.Unmarshal([]byte(input), &tc)
	require.NoError(t, err)

	// Marshal back
	data, err := json.Marshal(tc)
	require.NoError(t, err)

	// Unmarshal again
	var restored evmtypes.TraceConfig
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	require.Equal(t, tc.Tracer, restored.Tracer)
	require.Equal(t, tc.TracerJsonConfig, restored.TracerJsonConfig)
}
