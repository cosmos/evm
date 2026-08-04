package eip712

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"

	errorsmod "cosmossdk.io/errors"

	errortypes "github.com/cosmos/cosmos-sdk/types/errors"
)

type eip712MessagePayload struct {
	payload        gjson.Result
	numPayloadMsgs int
	message        map[string]interface{}
}

const (
	payloadMsgsField = "msgs"
)

// createEIP712MessagePayload generates the EIP-712 message payload
// corresponding to the input data.
func createEIP712MessagePayload(data []byte) (eip712MessagePayload, error) {
	basicPayload, err := unmarshalBytesToJSONObject(data)
	if err != nil {
		return eip712MessagePayload{}, err
	}

	basicPayload, err = stringifyJSONMsgFields(basicPayload)
	if err != nil {
		return eip712MessagePayload{}, errorsmod.Wrap(err, "failed to stringify JSON message fields")
	}

	payload, numPayloadMsgs, err := FlattenPayloadMessages(basicPayload)
	if err != nil {
		return eip712MessagePayload{}, errorsmod.Wrap(err, "failed to flatten payload JSON messages")
	}

	message, ok := payload.Value().(map[string]interface{})
	if !ok {
		return eip712MessagePayload{}, errorsmod.Wrap(errortypes.ErrInvalidType, "failed to parse JSON as map")
	}

	messagePayload := eip712MessagePayload{
		payload:        payload,
		numPayloadMsgs: numPayloadMsgs,
		message:        message,
	}

	return messagePayload, nil
}

// stringifyJSONMsgFields converts object- and array-valued "msg" fields to
// strings. Message fields conventionally contain opaque JSON (for example,
// CosmWasm contract messages), whose runtime shape cannot be represented by a
// stable EIP-712 type. Existing string values are left alone so ordinary
// string fields and already-stringified JSON are not changed.
//
// The conversion only affects the derived EIP-712 payload. The protobuf
// transaction still contains the original value used during execution.
func stringifyJSONMsgFields(value gjson.Result) (gjson.Result, error) {
	if !value.IsObject() && !value.IsArray() {
		return value, nil
	}

	updated := value.Raw
	var iterationErr error
	value.ForEach(func(key, child gjson.Result) bool {
		path := key.Str
		if value.IsArray() {
			path = strconv.FormatInt(key.Int(), 10)
		} else {
			path = escapeSJSONPath(path)
		}

		if !value.IsArray() && key.Str == "msg" && (child.IsObject() || child.IsArray()) {
			updated, iterationErr = sjson.Set(updated, path, child.Raw)
			return iterationErr == nil
		}

		if !child.IsObject() && !child.IsArray() {
			return true
		}

		var transformed gjson.Result
		transformed, iterationErr = stringifyJSONMsgFields(child)
		if iterationErr != nil {
			return false
		}

		updated, iterationErr = sjson.SetRaw(updated, path, transformed.Raw)
		return iterationErr == nil
	})
	if iterationErr != nil {
		return gjson.Result{}, iterationErr
	}

	return gjson.Parse(updated), nil
}

// escapeSJSONPath escapes object keys so they are treated as literal field
// names rather than SJSON path syntax.
func escapeSJSONPath(path string) string {
	path = strings.ReplaceAll(path, `\`, `\\`)
	path = strings.ReplaceAll(path, `.`, `\.`)
	return strings.ReplaceAll(path, `:`, `\:`)
}

// unmarshalBytesToJSONObject converts a bytestream into
// a JSON object, then makes sure the JSON is an object.
func unmarshalBytesToJSONObject(data []byte) (gjson.Result, error) {
	if !gjson.ValidBytes(data) {
		return gjson.Result{}, errorsmod.Wrap(errortypes.ErrJSONUnmarshal, "invalid JSON received")
	}

	payload := gjson.ParseBytes(data)

	if !payload.IsObject() {
		return gjson.Result{}, errorsmod.Wrap(errortypes.ErrJSONUnmarshal, "failed to JSON unmarshal data as object")
	}

	return payload, nil
}

// FlattenPayloadMessages flattens the input payload's messages, representing
// them as key-value pairs of "msg{i}": {Msg}, rather than as an array of Msgs.
// We do this to support messages with different schemas.
func FlattenPayloadMessages(payload gjson.Result) (gjson.Result, int, error) {
	flattened := payload
	var err error

	msgs, err := getPayloadMessages(payload)
	if err != nil {
		return gjson.Result{}, 0, err
	}

	for i, msg := range msgs {
		flattened, err = payloadWithNewMessage(flattened, msg, i)
		if err != nil {
			return gjson.Result{}, 0, err
		}
	}

	flattened, err = payloadWithoutMsgsField(flattened)
	if err != nil {
		return gjson.Result{}, 0, err
	}

	return flattened, len(msgs), nil
}

// getPayloadMessages processes and returns the payload messages as a JSON array.
func getPayloadMessages(payload gjson.Result) ([]gjson.Result, error) {
	rawMsgs := payload.Get(payloadMsgsField)

	if !rawMsgs.Exists() {
		return nil, errorsmod.Wrap(errortypes.ErrInvalidRequest, "no messages found in payload, unable to parse")
	}

	if rawMsgs.Type == gjson.Null {
		return []gjson.Result{}, nil
	}

	if !rawMsgs.IsArray() {
		return nil, errorsmod.Wrap(errortypes.ErrInvalidRequest, "expected type array of messages, cannot parse")
	}

	return rawMsgs.Array(), nil
}

// payloadWithNewMessage returns the updated payload object with the message
// set at the field corresponding to index.
func payloadWithNewMessage(payload gjson.Result, msg gjson.Result, index int) (gjson.Result, error) {
	field := msgFieldForIndex(index)

	if payload.Get(field).Exists() {
		return gjson.Result{}, errorsmod.Wrapf(
			errortypes.ErrInvalidRequest,
			"malformed payload received, did not expect to find key at field %v", field,
		)
	}

	if !msg.IsObject() {
		return gjson.Result{}, errorsmod.Wrapf(errortypes.ErrInvalidRequest, "msg at index %d is not valid JSON: %v", index, msg)
	}

	newRaw, err := sjson.SetRaw(payload.Raw, field, msg.Raw)
	if err != nil {
		return gjson.Result{}, err
	}

	return gjson.Parse(newRaw), nil
}

// msgFieldForIndex returns the payload field for a given message post-flattening.
// e.g. msgs[2] becomes 'msg2'
func msgFieldForIndex(i int) string {
	return fmt.Sprintf("msg%d", i)
}

// payloadWithoutMsgsField returns the updated payload without the "msgs" array
// field, which flattening makes obsolete.
func payloadWithoutMsgsField(payload gjson.Result) (gjson.Result, error) {
	newRaw, err := sjson.Delete(payload.Raw, payloadMsgsField)
	if err != nil {
		return gjson.Result{}, err
	}

	return gjson.Parse(newRaw), nil
}
