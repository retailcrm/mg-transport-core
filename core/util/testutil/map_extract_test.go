package testutil

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestMapValue_DifferentKeyTypes(t *testing.T) {
	mString := map[string]interface{}{"key": 1}
	mInt := map[int]interface{}{2: 1}
	mInt8 := map[int8]int{2: 1}
	mInt16 := map[int16]int{2: 1}
	mInt32 := map[int32]int{2: 1}
	mInt64 := map[int64]int{2: 1}
	mUInt := map[uint]interface{}{2: 1}
	mUInt8 := map[uint8]int{2: 1}
	mUInt16 := map[uint16]int{2: 1}
	mUInt32 := map[uint32]int{2: 1}
	mUInt64 := map[uint64]int{2: 1}
	mUIntptr := map[uintptr]int{2: 1}
	mFloat32 := map[float32]int{2.5: 1}
	mFloat64 := map[float64]int{2.5: 1}

	assert.Equal(t, 1, MustMapValue(mString, "key").(int))
	assert.Equal(t, 1, MustMapValue(mInt, "2").(int))
	assert.Equal(t, 1, MustMapValue(mInt8, "2").(int))
	assert.Equal(t, 1, MustMapValue(mInt16, "2").(int))
	assert.Equal(t, 1, MustMapValue(mInt32, "2").(int))
	assert.Equal(t, 1, MustMapValue(mInt64, "2").(int))
	assert.Equal(t, 1, MustMapValue(mUInt, "2").(int))
	assert.Equal(t, 1, MustMapValue(mUInt8, "2").(int))
	assert.Equal(t, 1, MustMapValue(mUInt16, "2").(int))
	assert.Equal(t, 1, MustMapValue(mUInt32, "2").(int))
	assert.Equal(t, 1, MustMapValue(mUInt64, "2").(int))
	assert.Equal(t, 1, MustMapValue(mUIntptr, "2").(int))
	assert.Equal(t, 1, MustMapValue(mFloat32, "2,5").(int))
	assert.Equal(t, 1, MustMapValue(mFloat64, "2,5").(int))
}

func TestMapValue_ErrorUnsupportedKeyType(t *testing.T) {
	_, err := MapValue(map[complex64]interface{}{}, "key")
	assert.Error(t, err)
	assert.Equal(t, "unsupported reflect.Kind: complex64", err.Error())
}

func TestMapValue_Nested(t *testing.T) {
	assert.Equal(t, "value", MustMapValue(map[string]map[string]interface{}{
		"key1": {
			"key2": "value",
		},
	}, "key1.key2").(string))

	m := map[string]interface{}{
		"key1": map[string]map[string]interface{}{
			"key2": {
				"key3": "value",
			},
		},
	}
	assert.Equal(t, "value", MustMapValue(m, "key1.key2.key3").(string))
	assert.Equal(t, "value", MustMapValue(m, "key1.key2").(map[string]interface{})["key3"].(string))
}

func TestMapValue_ErrorNotAMap(t *testing.T) {
	_, err := MapValue(1, "key")
	assert.Error(t, err)
	assert.Equal(t, "value at path '' is not a map", err.Error())

	_, err = MapValue(map[string]int{"key": 1}, "key.key2")
	assert.Error(t, err)
	assert.Equal(t, "value at path 'key' is not a map", err.Error())
}

func TestMapValue_ErrorKeyNotFound(t *testing.T) {
	_, err := MapValue(map[string]int{"key": 1}, "key2")
	assert.Error(t, err)
	assert.Equal(t, "key 'key2' not found at path ''", err.Error())

	_, err = MapValue(map[string]map[string]int{"key": {"key2": 1}}, "key.key3")
	assert.Error(t, err)
	assert.Equal(t, "key 'key3' not found at path 'key'", err.Error())
}

func TestMustMapValue_Panics(t *testing.T) {
	assert.Panics(t, func() {
		MustMapValue(map[string]int{"key": 1}, "key2")
	})
}
