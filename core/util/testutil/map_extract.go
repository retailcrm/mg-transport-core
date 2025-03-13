package testutil

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"testing"
)

// MapValue extracts nested map values using dot notation. Keys are separated by dots, slices and arrays can be
// accessed by integer indexes.
// Example:
//
//	MapValue(m, "key1") // Access value with key "key1"
//	MapValue(m, "key1.key2") // Access nested value with key "key2"
//	MapValue(m, "key1.key2.key3") // Access nested value with key "key3"
//	MapValue(m, "key1.key2.key3.0") // Access the first slice / array element in the nested map.
func MapValue(data interface{}, path string) (interface{}, error) {
	if path == "" {
		return data, nil
	}

	parts := strings.Split(path, ".")
	current := data

	for i, part := range parts {
		v := reflect.ValueOf(current)

		if v.Kind() == reflect.Map {
			converted, err := convertKeyToKind(part, v.Type().Key().Kind())
			if err != nil {
				return nil, err
			}

			keyValue := reflect.ValueOf(converted)
			valueValue := v.MapIndex(keyValue)

			if !valueValue.IsValid() {
				return nil, fmt.Errorf("key '%s' not found at path '%s'",
					part, strings.Join(parts[:i], "."))
			}

			current = valueValue.Interface()

			continue
		}

		if v.Kind() == reflect.Slice || v.Kind() == reflect.Array {
			index, err := strconv.Atoi(part)
			if err != nil {
				return nil, fmt.Errorf("'%s' is not a valid slice / array index", part)
			}

			if index < 0 || index >= v.Len() {
				return nil, fmt.Errorf("index %d out of bounds for %s of length %d at path '%s'",
					index, v.Kind().String(), v.Len(), strings.Join(parts[:i], "."))
			}

			current = v.Index(index).Interface()

			continue
		}

		return nil, fmt.Errorf("value at path '%s' is not a map, slice or array",
			strings.Join(parts[:i], "."))
	}

	return current, nil
}

// AssertMapValue is the MapValue variant useful in tests.
func AssertMapValue(t *testing.T, data interface{}, path string) interface{} {
	val, err := MapValue(data, path)
	if err != nil {
		t.Error(err)
	}
	return val
}

// MustMapValue is the same as MapValue but it panics in case of error.
func MustMapValue(data interface{}, path string) interface{} {
	val, err := MapValue(data, path)
	if err != nil {
		panic(err)
	}
	return val
}

// convertKeyToKind converts a string to the given kind.
func convertKeyToKind(part string, kind reflect.Kind) (interface{}, error) {
	switch kind {
	case reflect.String:
		return part, nil
	case reflect.Bool:
		return part == "true" || part == "1", nil
	case reflect.Int:
		return strconv.Atoi(part)
	case reflect.Int8:
		val, err := strconv.Atoi(part)
		return int8(val), err
	case reflect.Int16:
		val, err := strconv.Atoi(part)
		return int16(val), err
	case reflect.Int32:
		val, err := strconv.Atoi(part)
		return int32(val), err
	case reflect.Int64:
		val, err := strconv.ParseInt(part, 10, 64)
		return val, err
	case reflect.Uint:
		val, err := strconv.ParseUint(part, 10, 32)
		return uint(val), err
	case reflect.Uint8:
		val, err := strconv.ParseUint(part, 10, 8)
		return uint8(val), err
	case reflect.Uint16:
		val, err := strconv.ParseUint(part, 10, 16)
		return uint16(val), err
	case reflect.Uint32:
		val, err := strconv.ParseUint(part, 10, 32)
		return uint32(val), err
	case reflect.Uint64:
		val, err := strconv.ParseUint(part, 10, 64)
		return val, err
	case reflect.Uintptr:
		val, err := strconv.ParseUint(part, 10, 64)
		return uintptr(val), err
	case reflect.Float32:
		val, err := strconv.ParseFloat(strings.Replace(part, ",", ".", 1), 32)
		return float32(val), err
	case reflect.Float64:
		val, err := strconv.ParseFloat(strings.Replace(part, ",", ".", 1), 64)
		return val, err
	default:
		return nil, fmt.Errorf("unsupported reflect.Kind: %s", kind)
	}
}
