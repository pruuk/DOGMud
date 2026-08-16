package spells

import (
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v2"
)

// The pet multiplier is the single dial: it drives the companion's pool, and
// (via CompanionReserveDefault) its ongoing reservation. summon_base_pool,
// summon_scaling_divisor and summon_conviction_reserve are gone -- the first
// replaced, the second never read by anything, the third now derived.
func TestSpellData_SummonPetMultiplierParses(t *testing.T) {
	var sd SpellData
	in := "spellid: test-summon\nsummon_mob_id: 300\nsummon_pet_multiplier: 0.5\n"
	if err := yaml.Unmarshal([]byte(in), &sd); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if sd.SummonPetMultiplier != 0.5 {
		t.Fatalf("SummonPetMultiplier = %v, want 0.5", sd.SummonPetMultiplier)
	}
}

// A summon spell with no multiplier is an authoring error the loader must warn
// about, exactly as it used to warn about a missing base pool.
func TestSpellData_ValidateWarnsOnMissingPetMultiplier(t *testing.T) {
	sd := SpellData{SpellId: "test-summon", SummonMobId: 300}
	if err := sd.Validate(); err != nil {
		t.Fatalf("Validate returned an error, want nil: %v", err)
	}
	// Validate warns rather than failing; this test pins that it does not panic
	// and does not start returning an error, which would take the server down at
	// boot on an authoring slip that has always been a warning.
}

// The retired fields must be gone from the struct. A YAML file still carrying
// them parses (the loader is non-strict) but must not populate anything.
func TestSpellData_RetiredSummonFieldsAreGone(t *testing.T) {
	src := yamlTagsOf(SpellData{})
	for _, dead := range []string{"summon_base_pool", "summon_scaling_divisor", "summon_conviction_reserve"} {
		if strings.Contains(src, dead) {
			t.Errorf("SpellData still declares %q; it must be deleted, not left unread", dead)
		}
	}
}

func yamlTagsOf(v any) string {
	t := reflect.TypeOf(v)
	var b strings.Builder
	for i := 0; i < t.NumField(); i++ {
		b.WriteString(string(t.Field(i).Tag))
		b.WriteString("\n")
	}
	return b.String()
}
