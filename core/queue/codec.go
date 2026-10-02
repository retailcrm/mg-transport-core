package queue

import (
	json "encoding/json/v2"
	"errors"
)

// Codec converts items to bytes and back for drivers that persist envelopes. The Decode
// implementation must be deterministic across process restarts, so runtime-only dependencies should be
// restored with FuncCodec rather than serialized.
type Codec[T any] interface {
	Encode(T) ([]byte, error)
	Decode([]byte) (T, error)
}

// FuncCodec adapts functions to Codec. It is useful when decoding also needs
// to hydrate dependencies which are deliberately omitted from persisted data.
type FuncCodec[T any] struct {
	EncodeFunc func(T) ([]byte, error)
	DecodeFunc func([]byte) (T, error)
}

func (c FuncCodec[T]) Encode(value T) ([]byte, error) {
	if c.EncodeFunc == nil {
		return nil, errors.New("codec encode function is required")
	}
	return c.EncodeFunc(value)
}

func (c FuncCodec[T]) Decode(data []byte) (T, error) {
	if c.DecodeFunc == nil {
		var zero T
		return zero, errors.New("codec decode function is required")
	}
	return c.DecodeFunc(data)
}

// JSONCodec serializes items with the encoding/json/v2 package. It suits most persisted types; use
// FuncCodec when decoding requires additional runtime context.
type JSONCodec[T any] struct{}

func (JSONCodec[T]) Encode(value T) ([]byte, error) {
	return json.Marshal(value)
}

func (JSONCodec[T]) Decode(data []byte) (T, error) {
	var value T
	err := json.Unmarshal(data, &value)
	return value, err
}

// BytesCodec is a pass-through codec for payloads that are already encoded ([]byte items).
type BytesCodec struct{}

func (BytesCodec) Encode(value []byte) ([]byte, error) {
	return append([]byte(nil), value...), nil
}

func (BytesCodec) Decode(data []byte) ([]byte, error) {
	return append([]byte(nil), data...), nil
}
