package grapplemessaging

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/narration"
)

// RenderTemplate substitutes {controllerName} and {controlledName}
// in a template string and returns the rendered result. Caller is
// responsible for ANSI-wrapping or other formatting.
func RenderTemplate(template, controllerName, controlledName string) string {
	out := strings.ReplaceAll(template, "{controllerName}", controllerName)
	out = strings.ReplaceAll(out, "{controlledName}", controlledName)
	return out
}

// PickTemplate selects a template from the list, preferring ones
// not yet used in this grapple's cooldown map. The cooldown key is
// computed per template as `<keyPrefix>:<template>` so each unique
// rendered template can be tracked independently across rounds.
//
// When all templates in the list have been used at least once in
// this grapple, the cooldown map is reset and selection starts over
// — forcing variety within reasonable bounds without blocking
// templates forever.
//
// Empty `pool` returns a benign fallback string so callers can
// always send something (a missing template should not crash a
// round).
//
// The optional picker exists for the snapshot harness. Production passes none
// and gets narration.DefaultPicker, which routes through the engine's util.Rand
// seam. This file used stdlib math/rand directly until 2026-09-07, which made
// it unreachable by any seam and therefore unsnapshottable.
func PickTemplate(pool []string, cooldowns map[string]bool, keyPrefix string, picker ...narration.Picker) string {
	if len(pool) == 0 {
		return "(grapple messaging missing template)"
	}

	// Filter out templates whose cooldown key is marked.
	available := make([]string, 0, len(pool))
	for _, tmpl := range pool {
		ck := keyPrefix + ":" + tmpl
		if !cooldowns[ck] {
			available = append(available, tmpl)
		}
	}

	// If all are cooled-down, reset and start over.
	if len(available) == 0 {
		for _, tmpl := range pool {
			delete(cooldowns, keyPrefix+":"+tmpl)
		}
		available = pool
	}

	choose := narration.DefaultPicker
	if len(picker) > 0 && picker[0] != nil {
		choose = picker[0]
	}
	pick := available[choose(len(available))]
	cooldowns[keyPrefix+":"+pick] = true
	return pick
}
