package types

import "encoding/json"

// traceConfigJSON is an auxiliary struct for custom JSON unmarshaling.
// The key difference is TracerConfig uses json.RawMessage to accept both
// JSON objects and strings, enabling Ethereum JSON-RPC standard compliance.
type traceConfigJSON struct {
	Tracer           string          `json:"tracer,omitempty"`
	Timeout          string          `json:"timeout,omitempty"`
	Reexec           uint64          `json:"reexec,omitempty"`
	DisableStack     bool            `json:"disableStack"`
	DisableStorage   bool            `json:"disableStorage"`
	Debug            bool            `json:"debug,omitempty"`
	Limit            int32           `json:"limit,omitempty"`
	Overrides        *ChainConfig    `json:"overrides,omitempty"`
	EnableMemory     bool            `json:"enableMemory"`
	EnableReturnData bool            `json:"enableReturnData"`
	TracerConfig     json.RawMessage `json:"tracerConfig,omitempty"`
}

// UnmarshalJSON implements custom JSON unmarshaling for TraceConfig.
// This enables the tracerConfig field to accept both:
//   - JSON objects (Ethereum standard): {"tracerConfig": {"onlyTopCall": true}}
//   - Escaped JSON strings (legacy): {"tracerConfig": "{\"onlyTopCall\": true}"}
//
// The Ethereum JSON-RPC standard expects tracerConfig as a JSON object,
// but the protobuf-generated struct has it as a string field. This custom
// unmarshaler bridges that gap by accepting json.RawMessage and storing
// it as a string in TracerJsonConfig.
func (tc *TraceConfig) UnmarshalJSON(data []byte) error {
	var aux traceConfigJSON
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	tc.Tracer = aux.Tracer
	tc.Timeout = aux.Timeout
	tc.Reexec = aux.Reexec
	tc.DisableStack = aux.DisableStack
	tc.DisableStorage = aux.DisableStorage
	tc.Debug = aux.Debug
	tc.Limit = aux.Limit
	tc.Overrides = aux.Overrides
	tc.EnableMemory = aux.EnableMemory
	tc.EnableReturnData = aux.EnableReturnData

	if len(aux.TracerConfig) > 0 {
		tc.TracerJsonConfig = string(aux.TracerConfig)
	}

	return nil
}

// MarshalJSON implements custom JSON marshaling for TraceConfig.
// Outputs tracerConfig as raw JSON (not an escaped string) for
// Ethereum JSON-RPC standard compliance.
func (tc TraceConfig) MarshalJSON() ([]byte, error) {
	aux := struct {
		Tracer           string          `json:"tracer,omitempty"`
		Timeout          string          `json:"timeout,omitempty"`
		Reexec           uint64          `json:"reexec,omitempty"`
		DisableStack     bool            `json:"disableStack"`
		DisableStorage   bool            `json:"disableStorage"`
		Debug            bool            `json:"debug,omitempty"`
		Limit            int32           `json:"limit,omitempty"`
		Overrides        *ChainConfig    `json:"overrides,omitempty"`
		EnableMemory     bool            `json:"enableMemory"`
		EnableReturnData bool            `json:"enableReturnData"`
		TracerConfig     json.RawMessage `json:"tracerConfig,omitempty"`
	}{
		Tracer:           tc.Tracer,
		Timeout:          tc.Timeout,
		Reexec:           tc.Reexec,
		DisableStack:     tc.DisableStack,
		DisableStorage:   tc.DisableStorage,
		Debug:            tc.Debug,
		Limit:            tc.Limit,
		Overrides:        tc.Overrides,
		EnableMemory:     tc.EnableMemory,
		EnableReturnData: tc.EnableReturnData,
	}

	if tc.TracerJsonConfig != "" {
		aux.TracerConfig = json.RawMessage(tc.TracerJsonConfig)
	}

	return json.Marshal(aux)
}
