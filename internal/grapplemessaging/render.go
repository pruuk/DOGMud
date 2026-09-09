package grapplemessaging

import (
	"strconv"
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

// RenderedTriad is one grapple exchange as its three audiences are told it.
// All three fields come from the SAME variant index.
type RenderedTriad struct {
	Controller string
	Controlled string
	Observers  string
}

// PickIndex is PickTemplate expressed over INDICES instead of template strings,
// so a whole triad can share one pick.
//
// That distinction is the entire point. Filtering each role's own pool by its
// own cooldowns removes DIFFERENT entries from each, so position N stops
// meaning the same moment across roles and the triad cannot be coordinated.
// Cooling an index instead keeps all three aligned.
//
// Returns -1 for an empty pool. A nil cooldown map is tolerated: selection
// still works, it simply remembers nothing.
func PickIndex(n int, cooldowns map[string]bool, keyPrefix string, picker ...narration.Picker) int {
	if n <= 0 {
		return -1
	}

	available := make([]int, 0, n)
	for i := 0; i < n; i++ {
		if !cooldowns[cooldownKey(keyPrefix, i)] {
			available = append(available, i)
		}
	}

	// Every variant used: clear and start over, exactly as PickTemplate did.
	// Falling silent here would drop the event entirely.
	if len(available) == 0 {
		for i := 0; i < n; i++ {
			delete(cooldowns, cooldownKey(keyPrefix, i))
		}
		available = available[:0]
		for i := 0; i < n; i++ {
			available = append(available, i)
		}
	}

	choose := narration.DefaultPicker
	if len(picker) > 0 && picker[0] != nil {
		choose = picker[0]
	}
	// No modulo. The Picker contract is [0,n), and PickTemplate indexed
	// straight into its slice, so a picker that violates the contract PANICKED
	// and was found. Wrapping the result here would silently paper over the
	// next such mistake.
	idx := available[choose(len(available))]
	if cooldowns != nil {
		cooldowns[cooldownKey(keyPrefix, idx)] = true
	}
	return idx
}

func cooldownKey(keyPrefix string, i int) string {
	return keyPrefix + ":" + strconv.Itoa(i)
}

// RenderTriad renders one grapple exchange for all three audiences from a
// SINGLE coordinated variant index.
//
// ONE INDEX FOR ALL THREE ROLES IS THE POINT. internal/hooks used to call
// PickTemplate once per role, so the controller, the controlled and the room
// each drew an INDEPENDENT index and were told three different moments of one
// exchange. The authored data pairs by index and does so carefully: in
// clinch_to_mount, variant 1 is "a snap-down sets it up" for the controller and
// "You feel the snap-down TOO LATE" for the controlled, the same instant from
// two sides. With three variants per role the three agreed one time in nine.
//
// Roles are ALIASED onto the core's vocabulary here rather than in the core:
// a controller ACTS and the controlled is ACTED UPON, so controller is the
// Actor and controlled is the Actee.
//
// Renders nothing unless all three roles are present and equal length. Every
// one of the store's 41 keys is equal-length today, so an unequal triad means
// the data broke, and telling some audiences while leaving others silent is
// the defect this migration exists to prevent.
func RenderTriad(tri TemplateTriad, controllerName, controlledName string,
	cooldowns map[string]bool, keyPrefix string, picker ...narration.Picker) RenderedTriad {

	// AGREEMENT IS CHECKED BEFORE PICKING, and the order matters. PickIndex
	// both draws from the engine's global randomness and writes a cooldown
	// key, so picking first and discovering the mismatch afterwards would
	// shift every subsequent draw in the process and strand a cooldown entry
	// for an event nobody was ever told about.
	if !rolesAgree(len(tri.Controller), len(tri.Controlled), len(tri.Observers)) {
		return RenderedTriad{}
	}

	idx := PickIndex(len(tri.Controller), cooldowns, keyPrefix, picker...)
	if idx < 0 {
		return RenderedTriad{}
	}

	// The store has already chosen the index, so the core's own picker must not
	// consume a draw from the engine's global randomness. Its result is
	// discarded by the override.
	roles := narration.Render(
		narration.Variants{
			Actor:    tri.Controller,
			Actee:    tri.Controlled,
			Observer: tri.Observers,
		},
		map[string]string{
			"{controllerName}": controllerName,
			"{controlledName}": controlledName,
		},
		func(int) int { return 0 },
		idx,
	)

	return RenderedTriad{
		Controller: roles.Actor,
		Controlled: roles.Actee,
		Observers:  roles.Observer,
	}
}

// RenderedGradient is one gradient beat as its three audiences are told it.
// All three fields come from the SAME variant index.
type RenderedGradient struct {
	Self      string
	Partner   string
	Observers string
}

// RenderGradient is RenderTriad for the gradient pools, whose authored roles
// are named self/partner/observers rather than controller/controlled/observers.
//
// This is the AUDIENCE ALIASING the arc spec named for this slice. The two
// vocabularies describe the same three seats: whoever the beat is about is the
// Actor, the other participant is the Actee, and the room observes. Resolving
// that here rather than in the core is what lets grapple_outcomes.yaml keep
// both spellings, which are meaningful to whoever authors it.
//
// All four gradient keys are equal-length today; an unequal one renders
// nothing rather than telling some audiences and not others.
func RenderGradient(tri GradientTriad, selfName, partnerName string,
	cooldowns map[string]bool, keyPrefix string, picker ...narration.Picker) RenderedGradient {

	// Checked before picking, for the reason RenderTriad states.
	if !rolesAgree(len(tri.Self), len(tri.Partner), len(tri.Observers)) {
		return RenderedGradient{}
	}

	idx := PickIndex(len(tri.Self), cooldowns, keyPrefix, picker...)
	if idx < 0 {
		return RenderedGradient{}
	}

	roles := narration.Render(
		narration.Variants{
			Actor:    tri.Self,
			Actee:    tri.Partner,
			Observer: tri.Observers,
		},
		map[string]string{
			"{controllerName}": selfName,
			"{controlledName}": partnerName,
		},
		func(int) int { return 0 },
		idx,
	)

	return RenderedGradient{
		Self:      roles.Actor,
		Partner:   roles.Actee,
		Observers: roles.Observer,
	}
}

// rolesAgree reports whether every role holds the same non-zero number of
// variants, which is the precondition for a coordinated index meaning the same
// moment to all three audiences.
//
// ValidateCompleteness rejects a mismatch at LOAD time, so reaching here with
// one is a bug rather than a content state. This guard exists so that bug is a
// silent no-op rather than a wasted draw plus a stranded cooldown key.
func rolesAgree(a, b, c int) bool {
	return a > 0 && a == b && a == c
}
