package logger

import (
	"cmp"
	"slices"
	"strings"

	"go.uber.org/zap"
)

const (
	maskReplacement = "****"
	maskPairSize    = 2
)

// maskDirective keeps secrets private even if the field reaches an ordinary JSON encoder.
type maskDirective struct {
	secrets []string
}

// Mask registers literal, case-sensitive secrets to replace with "****" in messages
// and structured string values. Use it with With or WithLazy to mask a child logger
// and its descendants without modifying the parent or siblings. Multiple secrets
// are registered in one operation; empty strings are ignored.
//
// Mask is supported by this package's NewDefault and NewZap constructors and
// NewJSONWithContextEncoder. As a log-site field, it applies only to that entry.
// Encoded variants such as Go-quoted strings, URL escaping or base64 must be registered separately.
func Mask(secrets ...string) zap.Field {
	return zap.Reflect("", maskDirective{secrets: slices.Clone(secrets)})
}

type maskRules struct {
	secrets  []string
	replacer *strings.Replacer
}

func (r maskRules) with(secrets []string) maskRules {
	all := make([]string, 0, len(r.secrets)+len(secrets))
	all = append(all, r.secrets...)

	for _, secret := range secrets {
		if secret != "" {
			all = append(all, secret)
		}
	}

	if len(all) == 0 {
		return r
	}

	// Prefer the longest match at the same position, independently of argument order.
	slices.SortFunc(all, func(a, b string) int {
		if n := cmp.Compare(len(b), len(a)); n != 0 {
			return n
		}

		return strings.Compare(a, b)
	})

	all = slices.Compact(all)
	pairs := make([]string, 0, maskPairSize*len(all))

	for _, secret := range all {
		pairs = append(pairs, secret, maskReplacement)
	}

	return maskRules{secrets: all, replacer: strings.NewReplacer(pairs...)}
}

func (r maskRules) replace(value string) string {
	if r.replacer == nil {
		return value
	}

	return r.replacer.Replace(value)
}
