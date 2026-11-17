package logger

import (
	"encoding/json"

	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

// maskingEncoder uses zap's encoder cloning to keep rules local to each child.
// Ordinary fields retain their eager With and lazy WithLazy evaluation semantics.
type maskingEncoder struct {
	zapcore.Encoder
	rules maskRules

	// Console output contains a plain-text header followed by a JSON object.
	// A context-free encoder renders the header once without parsing the message.
	consoleHeader zapcore.Encoder
	config        zapcore.EncoderConfig
}

func (enc *maskingEncoder) Clone() zapcore.Encoder {
	clone := *enc
	clone.Encoder = enc.Encoder.Clone()

	return &clone
}

func (enc *maskingEncoder) AddReflected(key string, value any) error {
	if directive, ok := value.(maskDirective); ok {
		enc.rules = enc.rules.with(directive.secrets)

		return nil
	}

	return enc.Encoder.AddReflected(key, value)
}

func (enc *maskingEncoder) EncodeEntry(entry zapcore.Entry, fields []zapcore.Field) (*buffer.Buffer, error) {
	rules, fields := enc.entryRules(fields)

	if enc.consoleHeader != nil {
		return enc.encodeConsole(entry, fields, rules)
	}

	if rules.replacer == nil {
		return enc.Encoder.EncodeEntry(entry, fields)
	}

	encoded, err := enc.Encoder.EncodeEntry(entry, fields)
	if err != nil {
		return nil, err
	}

	defer encoded.Free()

	data := encoded.Bytes()[:encoded.Len()-len(enc.config.LineEnding)]
	masked, err := maskJSONStrings(data, rules)
	if err != nil {
		return nil, err
	}

	masked.AppendString(enc.config.LineEnding)

	return masked, nil
}

func (enc *maskingEncoder) entryRules(fields []zapcore.Field) (maskRules, []zapcore.Field) {
	var secrets []string
	var filtered []zapcore.Field

	for i, field := range fields {
		directive, ok := field.Interface.(maskDirective)

		if ok && field.Type == zapcore.ReflectType {
			if filtered == nil {
				filtered = make([]zapcore.Field, 0, len(fields))
				filtered = append(filtered, fields[:i]...)
			}

			secrets = append(secrets, directive.secrets...)
		} else if filtered != nil {
			filtered = append(filtered, field)
		}
	}

	if filtered == nil {
		return enc.rules, fields
	}

	if len(secrets) == 0 {
		return enc.rules, filtered
	}

	return enc.rules.with(secrets), filtered
}

// maskJSONStrings rewrites string values only, preserving keys, number tokens,
// duplicate keys, whitespace and the surrounding JSON structure.
func maskJSONStrings(data []byte, rules maskRules) (*buffer.Buffer, error) {
	result := GetBufferPool()
	start := 0

	for i := 0; i < len(data); i++ {
		if data[i] != '"' {
			continue
		}

		end := jsonStringEnd(data, i)

		if !jsonStringKey(data, end) {
			var value string

			if err := json.Unmarshal(data[i:end], &value); err != nil {
				result.Free()

				return nil, err
			}

			if masked := rules.replace(value); masked != value {
				result.AppendBytes(data[start:i])

				encoded, err := json.Marshal(masked)
				if err != nil {
					result.Free()

					return nil, err
				}

				result.AppendBytes(encoded)
				start = end
			}
		}

		i = end - 1
	}

	result.AppendBytes(data[start:])

	return result, nil
}

func jsonStringEnd(data []byte, start int) int {
	for i := start + 1; i < len(data); i++ {
		switch data[i] {
		case '\\':
			i++
		case '"':
			return i + 1
		}
	}

	return len(data)
}

func jsonStringKey(data []byte, end int) bool {
	for _, char := range data[end:] {
		switch char {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return char == ':'
		}
	}

	return false
}
