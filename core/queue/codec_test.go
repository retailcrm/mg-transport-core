package queue_test

import (
	"strconv"
	"testing"

	"github.com/retailcrm/mg-transport-core/v2/core/queue"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFuncCodec(t *testing.T) {
	codec := queue.FuncCodec[int]{
		EncodeFunc: func(value int) ([]byte, error) { return []byte(strconv.Itoa(value)), nil },
		DecodeFunc: func(data []byte) (int, error) {
			value, err := strconv.Atoi(string(data))
			return value + 1, err
		},
	}
	data, err := codec.Encode(41)
	require.NoError(t, err)
	value, err := codec.Decode(data)
	require.NoError(t, err)
	assert.Equal(t, 42, value)
}

func TestFuncCodecValidatesFunctions(t *testing.T) {
	codec := queue.FuncCodec[int]{}
	_, err := codec.Encode(1)
	require.EqualError(t, err, "codec encode function is required")
	_, err = codec.Decode(nil)
	require.EqualError(t, err, "codec decode function is required")
}
