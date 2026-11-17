package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/buffer"
	"go.uber.org/zap/zapcore"
)

func init() {
	err := zap.RegisterEncoder("console-with-masking", func(config zapcore.EncoderConfig) (zapcore.Encoder, error) {
		return newMaskingConsoleEncoder(config), nil
	})

	if err != nil {
		panic(err)
	}
}

func newMaskingConsoleEncoder(config zapcore.EncoderConfig) zapcore.Encoder {
	if config.ConsoleSeparator == "" {
		config.ConsoleSeparator = "\t"
	}

	if config.SkipLineEnding {
		config.LineEnding = ""
	} else if config.LineEnding == "" {
		config.LineEnding = zapcore.DefaultLineEnding
	}

	// Encode context separately so metadata callbacks run only once per entry.
	contextConfig := config
	contextConfig.MessageKey, contextConfig.LevelKey, contextConfig.TimeKey = "", "", ""
	contextConfig.NameKey, contextConfig.CallerKey, contextConfig.FunctionKey = "", "", ""
	contextConfig.StacktraceKey = ""

	return &maskingEncoder{
		Encoder:       zapcore.NewConsoleEncoder(contextConfig),
		consoleHeader: zapcore.NewConsoleEncoder(config),
		config:        config,
	}
}

func (enc *maskingEncoder) encodeConsole(
	entry zapcore.Entry, fields []zapcore.Field, rules maskRules,
) (*buffer.Buffer, error) {
	stack := entry.Stack
	entry.Stack = ""

	header, err := enc.consoleHeader.EncodeEntry(entry, nil)
	if err != nil {
		return nil, err
	}

	defer header.Free()

	encoded, err := enc.Encoder.EncodeEntry(zapcore.Entry{}, fields)
	if err != nil {
		return nil, err
	}

	defer encoded.Free()

	prefix := header.Bytes()[:header.Len()-len(enc.config.LineEnding)]

	context := encoded.Bytes()[:encoded.Len()-len(enc.config.LineEnding)]
	if rules.replacer != nil && len(context) > 0 {
		masked, maskErr := maskJSONStrings(context, rules)
		if maskErr != nil {
			return nil, maskErr
		}

		defer masked.Free()

		context = masked.Bytes()
	}

	result := GetBufferPool()
	result.AppendString(rules.replace(string(prefix)))

	if len(context) > 0 {
		if len(prefix) > 0 {
			result.AppendString(enc.config.ConsoleSeparator)
		}

		result.AppendBytes(context)
	}

	if stack != "" && enc.config.StacktraceKey != "" {
		result.AppendByte('\n')
		result.AppendString(rules.replace(stack))
	}

	result.AppendString(enc.config.LineEnding)

	return result, nil
}
