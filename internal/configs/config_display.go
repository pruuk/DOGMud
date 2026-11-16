package configs

import (
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"slices"
	"strings"
)

// RedactedValue is what every human-facing view of the config prints in
// place of a secret. ConfigSecret.String returns it too.
const RedactedValue = `*** REDACTED ***`

// ErrRedactedValue refuses the redaction marker as a new value: a prompt that
// offers the displayed value as its default must not write the marker over
// the real secret.
var ErrRedactedValue = errors.New("value is the redaction marker, not a real value")

// secretNameRule matches a single dotted-path segment (or a map key found
// while deep-scanning a value) that names a credential even when it is not
// typed ConfigSecret. Module config lives in the untyped Modules map, so
// Modules.aicompanion.APIKey arrives as a plain string and only its name
// marks it. Exact or suffix match, case-insensitive, checked one path
// element (or map key) at a time: ClientSecret and OpenAIAPIKey match,
// APIKeyEnv (a variable NAME, not a secret) does not, and the plural or
// compound token counters (MaxCompletionTokens, DailyTokenBudget,
// StrangerDailyTokens, DailyTokensPerCompanion) do not, because their
// suffix is "Tokens" or "Budget", never the literal word "token". Checked
// against shipped keys (_datafiles/config.yaml and every internal/configs
// yaml tag) on 2026-09-28: only APIKey and WebhookUrl match, both secrets.
// secretkey and privatekey are added (Task 5 review) as their own suffixes,
// not by adding a bare "key" suffix: a bare "key" would also catch ordinary
// lookup keys (map key names, cache keys) that are not credentials.
var secretNameRule = regexp.MustCompile(`(?i)(apikey|api_key|secret|password|token|webhookurl|secretkey|privatekey)$`)

// DisplayConfigData is AllConfigData for anything a person reads: the boot
// log, the server set listing, the server config menu and /viewconfig. A
// value reads RedactedValue when it is a ConfigSecret, when any element of
// its dotted path names a credential (secretNameRule), or when the value
// itself is a slice or map that a deep scan finds carries one: buildDotPaths
// stores a slice whole at its parent path (never recursing into elements),
// so a secret nested inside a slice, or inside a map nested inside a slice,
// never earns its own dotted path and only a deep scan of the stored value
// catches it. AllConfigData stays raw because the key and type lookups are
// built from it.
func (c Config) DisplayConfigData(excludeStrings ...string) map[string]any {
	out := c.AllConfigData(excludeStrings...)
	for name, value := range out {
		if isSecretConfigValue(name, value) {
			out[name] = RedactedValue
		}
	}
	return out
}

func isSecretConfigValue(name string, value any) bool {
	return pathHasSecretSegment(name) || containsSecret(value)
}

// pathHasSecretSegment reports whether any dotted-path element (not just the
// leaf) matches secretNameRule, so Modules.x.password.db is caught by the
// mid-path "password" segment even though its leaf ("db") is innocent.
func pathHasSecretSegment(path string) bool {
	return slices.ContainsFunc(strings.Split(path, `.`), secretNameRule.MatchString)
}

// maxContainsSecretDepth caps how far containsSecret descends into a value.
// Modules values come from a YAML decode, which never produces a cycle, so
// the cap exists only to bound a pathological or hand-built value (this
// package's own test builds a self-referential slice, and any code with
// access to a Config could do the same). Past the cap containsSecret fails
// closed: it reports the value as secret rather than risk showing one.
const maxContainsSecretDepth = 32

// containsSecret deep-scans a value that AllConfigData stored whole (a slice
// or a map, at any nesting depth, reached through maps with string or any
// keys, slices, arrays, pointers and interfaces) and reports whether it is or
// carries a ConfigSecret, or a map key matching secretNameRule. It descends
// only maps, slices, arrays, pointers and interfaces, never struct fields,
// because every value it is called on (a leaf AllConfigData/buildDotPaths
// stored whole, always from Modules) comes from a YAML decode, which never
// produces a struct. A match redacts the entire stored value, since
// DisplayConfigData cannot redact just the offending piece without
// decomposing a value AllConfigData never decomposed.
func containsSecret(value any) bool {
	return containsSecretAtDepth(value, 0)
}

func containsSecretAtDepth(value any, depth int) bool {
	if depth >= maxContainsSecretDepth {
		return true
	}
	if value == nil {
		return false
	}
	if _, ok := value.(ConfigSecret); ok {
		return true
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Interface, reflect.Pointer:
		if rv.IsNil() {
			return false
		}
		return containsSecretAtDepth(rv.Elem().Interface(), depth+1)
	case reflect.Map:
		for _, key := range rv.MapKeys() {
			if secretNameRule.MatchString(fmt.Sprintf(`%v`, key.Interface())) {
				return true
			}
			if containsSecretAtDepth(rv.MapIndex(key).Interface(), depth+1) {
				return true
			}
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if containsSecretAtDepth(rv.Index(i).Interface(), depth+1) {
				return true
			}
		}
	}
	return false
}
