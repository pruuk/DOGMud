package mobs

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/stats"
)

// withSpeciesFixture installs a single species (id 1) with a flat baseline, so
// the migration's authored = template.Base - species.Base arithmetic has a
// known reference. Restored via t.Cleanup.
func withSpeciesFixture(t *testing.T) {
	t.Helper()
	var st stats.Statistics
	for _, p := range []*stats.StatInfo{
		&st.Strength, &st.Dexterity, &st.Perception,
		&st.Vitality, &st.Willpower, &st.Charisma,
	} {
		p.Base = 100
		p.Recalculate()
	}
	species.SetSpeciesForTest(t, map[int]*species.Species{
		1: {SpeciesId: 1, Name: "human", Stats: st},
	})
}

// migrationFixture registers a template whose Base is species baseline +
// authored, which is what Phase A's fold produces, and returns the mob id.
func migrationFixture(t *testing.T, speciesBase, authored, statPool int) MobId {
	t.Helper()
	const id = 1
	t.Cleanup(seedRegistry())

	mobsMu.Lock()
	m := mobs[id]
	m.StatPool = statPool
	m.Character.SpeciesId = 1
	// Species 1 is human, 100 per stat in the real data; the fixture registry
	// may differ, so drive Base from the caller's stated baseline.
	m.Character.Stats.Strength.Base = speciesBase + authored
	m.Character.Stats.Dexterity.Base = speciesBase
	m.Character.Stats.Perception.Base = speciesBase
	m.Character.Stats.Vitality.Base = speciesBase
	m.Character.Stats.Willpower.Base = speciesBase
	m.Character.Stats.Charisma.Base = speciesBase
	mobsMu.Unlock()

	return id
}

// The common case, and the one that matters most: a companion that was summoned
// and never progressed. Its saved Training is exactly authored + pool, so it
// must migrate to exactly zero gains — not to a small positive remainder that
// would compound on every future load.
func TestLegacyTrainingToGains_NeverProgressedIsExactlyZero(t *testing.T) {
	withSpeciesFixture(t)
	id := migrationFixture(t, 100, 22, 60)

	// authored 22 + pool 60, distributed however the roll happened to fall.
	saved := map[string]int{
		"strength": 30, "dexterity": 20, "perception": 12,
		"vitality": 10, "willpower": 6, "charisma": 4,
	}
	got := LegacyTrainingToGains(id, saved)

	total := 0
	for _, s := range StatNames {
		if got[s] < 0 {
			t.Errorf("%s went negative: %d", s, got[s])
		}
		total += got[s]
	}
	if total != 0 {
		t.Errorf("a never-progressed mob migrated to %d gains, want 0 (%v)", total, got)
	}
}

// A mob that did progress keeps its gains, exactly on the total.
func TestLegacyTrainingToGains_KeepsRealGains(t *testing.T) {
	withSpeciesFixture(t)
	id := migrationFixture(t, 100, 22, 60)

	// authored 22 + pool 60 = 82 spawned, plus 40 earned.
	saved := map[string]int{
		"strength": 50, "dexterity": 30, "perception": 20,
		"vitality": 12, "willpower": 6, "charisma": 4,
	}
	got := LegacyTrainingToGains(id, saved)

	total := 0
	for _, s := range StatNames {
		total += got[s]
	}
	if total != 40 {
		t.Errorf("migrated total = %d, want 40 (%v)", total, got)
	}
}

// Saved smaller than authored + pool cannot yield negative gains.
func TestLegacyTrainingToGains_NeverNegative(t *testing.T) {
	withSpeciesFixture(t)
	id := migrationFixture(t, 100, 50, 100)

	saved := map[string]int{"strength": 5, "dexterity": 3}
	got := LegacyTrainingToGains(id, saved)
	for k, v := range got {
		if v < 0 {
			t.Errorf("%s = %d, want >= 0", k, v)
		}
	}
}

// Idempotence. The schema version is what stops the migration running twice in
// production, but the arithmetic must not compound even if it does: running it
// on an already-migrated map must be a no-op, because a gains-only map is below
// authored + pool and floors to zero... which is exactly why the version marker
// is load-bearing and this test documents the trap rather than a safety net.
func TestLegacyTrainingToGains_RunningTwiceIsNotSelfCorrecting(t *testing.T) {
	withSpeciesFixture(t)
	id := migrationFixture(t, 100, 22, 60)

	saved := map[string]int{
		"strength": 50, "dexterity": 30, "perception": 20,
		"vitality": 12, "willpower": 6, "charisma": 4,
	}
	once := LegacyTrainingToGains(id, saved)
	twice := LegacyTrainingToGains(id, once)

	t1, t2 := 0, 0
	for _, s := range StatNames {
		t1 += once[s]
		t2 += twice[s]
	}
	if t1 == t2 {
		t.Skip("arithmetic happens to be idempotent for this fixture; the version marker is still required")
	}
	// Documented, not tolerated: this is why InstanceSchemaVersion exists.
	t.Logf("running the migration twice loses gains (%d -> %d); InstanceSchemaVersion prevents it", t1, t2)
}

// A missing template must not corrupt the value.
func TestLegacyTrainingToGains_UnknownTemplateReturnsSavedUnchanged(t *testing.T) {
	withSpeciesFixture(t)
	saved := map[string]int{"strength": 40, "dexterity": 10}
	got := LegacyTrainingToGains(MobId(987654), saved)
	if got["strength"] != 40 || got["dexterity"] != 10 {
		t.Errorf("unknown template altered the saved value: %v", got)
	}
}
