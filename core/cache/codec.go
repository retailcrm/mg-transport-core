package cache

import (
	"bytes"
	"encoding/base64"
	json "encoding/json/v2"
)

// Codec converts cache values to bytes and back for drivers that persist entries.
type Codec[T any] interface {
	Encode(T) ([]byte, error)
	Decode([]byte) (T, error)
}

// JSONCodec serializes values with the encoding/json/v2 package.
type JSONCodec[T any] struct{}

func (JSONCodec[T]) Encode(value T) ([]byte, error) {
	return json.Marshal(value)
}

func (JSONCodec[T]) Decode(data []byte) (T, error) {
	var value T
	err := json.Unmarshal(data, &value)
	return value, err
}

// BytesCodec is a pass-through codec for values that are already encoded ([]byte values).
type BytesCodec struct{}

func (BytesCodec) Encode(value []byte) ([]byte, error) {
	return bytes.Clone(value), nil
}

func (BytesCodec) Decode(data []byte) ([]byte, error) {
	return bytes.Clone(data), nil
}

// KeyEncoder converts typed cache keys into the string keys required by persistent drivers.
type KeyEncoder[K comparable] interface {
	EncodeKey(K) (string, error)
}

// KeyDecoder converts a persistent driver key back to its typed form.
type KeyDecoder[K comparable] interface {
	DecodeKey(string) (K, error)
}

// KeyCodec converts typed keys in both directions.
type KeyCodec[K comparable] interface {
	KeyEncoder[K]
	KeyDecoder[K]
}

// StringKeyEncoder passes string keys through unchanged.
type StringKeyEncoder struct{}

func (StringKeyEncoder) EncodeKey(key string) (string, error) {
	return key, nil
}

// DecodeKey passes string keys through unchanged.
func (StringKeyEncoder) DecodeKey(key string) (string, error) {
	return key, nil
}

// JSONKeyEncoder serializes arbitrary comparable keys with encoding/json/v2 and base64-encodes the
// result, keeping the encoded form safe for subject-style key spaces.
type JSONKeyEncoder[K comparable] struct{}

func (JSONKeyEncoder[K]) EncodeKey(key K) (string, error) {
	data, err := json.Marshal(key)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(data), nil
}

// DecodeKey reverses the JSON and base64 encoding applied by EncodeKey.
func (JSONKeyEncoder[K]) DecodeKey(key string) (K, error) {
	var value K
	data, err := base64.RawURLEncoding.DecodeString(key)
	if err != nil {
		return value, err
	}
	err = json.Unmarshal(data, &value)
	return value, err
}
