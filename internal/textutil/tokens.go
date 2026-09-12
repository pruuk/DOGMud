package textutil

import (
	"regexp"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

// TokenContext holds actor names for substitution in YAML text fields.
type TokenContext struct {
	SourceName      string // ANSI-tagged display name
	SourcePlainName string // Plain name (for possessives)
	TargetName      string // ANSI-tagged display name (empty if no target)
	TargetPlainName string // Plain name (empty if no target)
}

// Tokens is the vocabulary as the narration core takes it. All four keys are
// always present, so an absent target substitutes to an empty string, which
// is what SubstituteTokens has always done.
func (ctx TokenContext) Tokens() map[string]string {
	return map[string]string{
		`{source}`:       ctx.SourceName,
		`{target}`:       ctx.TargetName,
		`{source_plain}`: ctx.SourcePlainName,
		`{target_plain}`: ctx.TargetPlainName,
	}
}

// SubstituteTokens replaces known tokens in text with values from ctx.
// Unknown tokens are left as-is. Empty string input returns empty string.
//
// Since M3 item 5b it is a one-variant Narrate, so one engine substitutes
// for every store. The result is identical to the former four-pair
// strings.NewReplacer: no token is a prefix of another (each ends in "}"),
// so pair order cannot change the output.
func SubstituteTokens(text string, ctx TokenContext) string {
	if text == "" {
		return ""
	}
	return Narrate(narration.Variants{Actor: Pool(text)}, ctx).Actor
}

var tokenPattern = regexp.MustCompile(`\{[a-z_]+\}`)

var knownTokens = map[string]bool{
	`{source}`:       true,
	`{target}`:       true,
	`{source_plain}`: true,
	`{target_plain}`: true,
}

// ValidateTokens scans text for {token} patterns and returns warnings
// for any that are not in the known set.
func ValidateTokens(text string) []string {
	if text == "" {
		return nil
	}
	var warnings []string
	matches := tokenPattern.FindAllString(text, -1)
	for _, m := range matches {
		if !knownTokens[m] {
			warnings = append(warnings, "unknown token: "+m)
		}
	}
	return warnings
}
