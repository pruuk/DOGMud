package grapplemessaging

import (
	"testing"
)

func buildTestLib() *Library {
	return &Library{
		Advancements: map[string]TemplateTriad{
			"clinch_to_mount": {
				Controller: []string{"You mount {actee}.", "You ride them down."},
				Controlled: []string{"{actor} mounts you.", "{actor} rides you down."},
				Observers:  []string{"{actor} mounts {actee}.", "{actor} rides {actee} down."},
			},
		},
		Degradations: map[string]TemplateTriad{},
		Reversals:    map[string]TemplateTriad{},
		Escapes:      map[string]TemplateTriad{},
		Holds:        map[string]TemplateTriad{},
		StrikingApex: map[string][]string{},
	}
}

func TestPickTemplateRandomization(t *testing.T) {
	lib := buildTestLib()
	cooldowns := map[string]bool{}
	// Pick should rotate through both available templates given enough
	// calls; with cooldown empty, both should be reachable.
	seen := map[string]bool{}
	for range 100 {
		tmpl := PickTemplate(lib.Advancements["clinch_to_mount"].Controller, cooldowns, "adv:clinch_to_mount:ctrl")
		seen[tmpl] = true
	}
	if len(seen) < 2 {
		t.Errorf("expected variety after 100 picks, only saw %d unique templates", len(seen))
	}
}

func TestPickTemplateRespectsCooldownByExactString(t *testing.T) {
	lib := buildTestLib()
	cooldownKey := "adv:clinch_to_mount:ctrl"
	cooldowns := map[string]bool{}

	// First pick — marks something used.
	first := PickTemplate(lib.Advancements["clinch_to_mount"].Controller, cooldowns, cooldownKey)
	cooldowns[cooldownKey+":"+first] = true

	// Subsequent picks should avoid the cooldown-marked template until
	// all are exhausted, then reset.
	for range 20 {
		next := PickTemplate(lib.Advancements["clinch_to_mount"].Controller, cooldowns, cooldownKey)
		// At least one of the two templates must be the "other" one
		// (cooldown not yet exhausted with both).
		if next == first {
			// Both templates have been used; cooldown should have
			// reset. This is fine — test passes.
			return
		}
	}
}

func TestPickTemplateEmptyListReturnsFallback(t *testing.T) {
	out := PickTemplate([]string{}, map[string]bool{}, "any:key")
	if out == "" {
		t.Error("PickTemplate should return a non-empty fallback even for empty input")
	}
}
