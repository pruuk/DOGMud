package hooks

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// seedGolemSpell registers a raise-golem-shaped summon spell so the backfill
// can resolve a base reserve from SummonMobId. Mirrors the live
// raise-golem.yaml numbers (mob 305, pet multiplier 1.00). The reserve is no
// longer authored: 1.00 x CompanionReserveDefault 280 derives it.
func seedGolemSpell() func() {
	return spells.SeedSpellsForTest(map[string]*spells.SpellData{
		"raise-golem": {
			SpellId:             "raise-golem",
			Name:                "Raise Flesh Golem",
			SummonMobId:         305,
			SummonPetMultiplier: 1.00,
		},
	})
}

// Companions saved before the Conviction economy shipped (2026-07-13) have no
// conviction_reserve in their save and load as 0 — they'd sustain for free
// forever. The login refresh must stamp them with what they'd cost today.
func TestRefreshCompanionReserves_LegacyGolem(t *testing.T) {
	cleanup := seedGolemSpell()
	defer cleanup()

	ch := characters.New()
	ch.Skills[string(skills.Manifestation)] = 48
	require.True(t, ch.AddCompanion(characters.CompanionInfo{
		MobId: 305, InstanceId: 1, SourceType: characters.CompanionRaised,
		Name: "a flesh golem", ConvictionReserve: 0, // legacy record
	}))
	require.True(t, ch.AddCompanion(characters.CompanionInfo{
		MobId: 305, InstanceId: 2, SourceType: characters.CompanionRaised,
		Name: "a flesh golem", ConvictionReserve: 0, // legacy record
	}))

	changed := refreshCompanionReserves(ch)

	assert.True(t, changed, "legacy companions present -> refresh must report a change")
	// Meirok calibration: base 280 * (1 - 48*0.01) * SkillCostMultiplier(48)
	// = 280 * 0.52 * 0.816 = 118.81 -> 119 each.
	assert.Equal(t, 119, ch.Companions[0].ConvictionReserve)
	assert.Equal(t, 119, ch.Companions[1].ConvictionReserve)
}

// D11 inverted this case. It used to assert "a snapshotted reserve must never be
// recomputed", which was right while the snapshot only ever had to survive a
// session and wrong across the U7b rebase: a returning veteran would keep their
// pre-rebase figure forever. Login is the ONE place the snapshot may move.
func TestRefreshCompanionReserves_StaleReserveIsRebased(t *testing.T) {
	cleanup := seedGolemSpell()
	defer cleanup()

	ch := characters.New()
	ch.Skills[string(skills.Manifestation)] = 48
	require.True(t, ch.AddCompanion(characters.CompanionInfo{
		MobId: 305, InstanceId: 1, Name: "a flesh golem", ConvictionReserve: 440,
	}))

	changed := refreshCompanionReserves(ch)

	assert.True(t, changed, "a stale pre-rebase reserve must be reported as changed")
	// Same Meirok calibration as the legacy case: base 280 * (1 - 48*0.01) *
	// SkillCostMultiplier(48) = 119.
	assert.Equal(t, 119, ch.Companions[0].ConvictionReserve,
		"the stale 440 must rebase to today's price")
}

func TestRefreshCompanionReserves_UnknownMobUsesDefault(t *testing.T) {
	cleanup := seedGolemSpell()
	defer cleanup()

	ch := characters.New()
	require.True(t, ch.AddCompanion(characters.CompanionInfo{
		MobId: 424242, InstanceId: 1, Name: "a mystery", ConvictionReserve: 0,
	}))

	changed := refreshCompanionReserves(ch)

	assert.True(t, changed)
	// No summon spell matches -> the unscaled CompanionReserveDefault (280).
	want := ch.CalcCompanionReserve(int(configs.GetBalanceConfig().CompanionReserveDefault))
	assert.Equal(t, want, ch.Companions[0].ConvictionReserve)
	assert.Greater(t, ch.Companions[0].ConvictionReserve, 0)
}

func TestCompanionBaseReserveFor_SpecialMobs(t *testing.T) {
	cleanup := seedGolemSpell()
	defer cleanup()

	cfg := configs.GetBalanceConfig()
	assert.Equal(t, 280, companionBaseReserveFor(305),
		"spell-summoned mob derives its base from the spell's pet multiplier")
	assert.Equal(t, int(cfg.HomunculusConvictionReserve), companionBaseReserveFor(homunculusMobId))
	assert.Equal(t, broodFloorReserve, companionBaseReserveFor(broodSpawnMobId))
	assert.Equal(t, int(cfg.CompanionReserveDefault), companionBaseReserveFor(424242))
}

// Owner decision 2026-08-15: the homunculus base drops from 1000 to 300. At 1000
// the ceiling would have made the apex unfieldable by exactly the crafter it is
// built for, while leaving it fieldable by a summoner who does not need it.
func TestHomunculusBaseReserveIs300(t *testing.T) {
	assert.Equal(t, 300, int(configs.GetBalanceConfig().HomunculusConvictionReserve))
}

// D11. Reserves are RECOMPUTED at login, not merely backfilled when zero.
// ConvictionReserve is frozen at summon time, so without this a returning
// veteran keeps their pre-U7b figures forever and never sees the rebase that is
// the entire reason no migration hurts.
func TestRefreshCompanionReserves_RecomputesAStaleSnapshot(t *testing.T) {
	cleanup := seedGolemSpell()
	defer cleanup()

	ch := characters.New()
	ch.ConvictionMax.Base = 500
	ch.Validate()
	ch.Companions = []characters.CompanionInfo{
		// A golem carrying the old flat 352-derived figure.
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised, ConvictionReserve: 158},
		// A legacy record that never had one at all.
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised, ConvictionReserve: 0},
	}

	if !refreshCompanionReserves(ch) {
		t.Fatalf("a stale snapshot must be reported as changed")
	}
	if ch.Companions[0].ConvictionReserve == 158 {
		t.Errorf("the stale 158 was not recomputed")
	}
	if ch.Companions[1].ConvictionReserve == 0 {
		t.Errorf("the legacy zero was not stamped")
	}
	if ch.Companions[0].ConvictionReserve != ch.Companions[1].ConvictionReserve {
		t.Errorf("two identical companions must recompute to the same reserve, got %d and %d",
			ch.Companions[0].ConvictionReserve, ch.Companions[1].ConvictionReserve)
	}
}

// Recomputing must be idempotent: a second login must not move the number
// again, or every login would drift a returning player's budget.
func TestRefreshCompanionReserves_IsIdempotent(t *testing.T) {
	cleanup := seedGolemSpell()
	defer cleanup()

	ch := characters.New()
	ch.ConvictionMax.Base = 500
	ch.Validate()
	ch.Companions = []characters.CompanionInfo{
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised},
	}

	refreshCompanionReserves(ch)
	first := ch.Companions[0].ConvictionReserve

	if refreshCompanionReserves(ch) {
		t.Errorf("a second refresh must report no change")
	}
	if ch.Companions[0].ConvictionReserve != first {
		t.Errorf("a second refresh moved the reserve from %d to %d", first, ch.Companions[0].ConvictionReserve)
	}
}

// D4 grandfathering at login. Recomputing must NEVER dismiss a companion, even
// if the recomputed total sits past the ceiling. Refuse additions, never force a
// removal.
func TestRefreshCompanionReserves_NeverDismisses(t *testing.T) {
	cleanup := seedGolemSpell()
	defer cleanup()

	ch := characters.New()
	ch.ConvictionMax.Base = 100 // tiny pool: any companion breaches
	ch.Validate()
	ch.Companions = []characters.CompanionInfo{
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised},
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised},
	}

	refreshCompanionReserves(ch)
	if len(ch.Companions) != 2 {
		t.Fatalf("login recompute dismissed a companion: %d left, want 2", len(ch.Companions))
	}
}

// D11 disclosure. A recompute that leaves the owner further past the ceiling
// must be SPOKEN, naming reservation. Companion price is partly manifestation,
// GetSkillLevel counts equipment stat mods, and skill_manifestation is in the
// loot affix pool, so this fires for a player whose only "action" was losing a
// piece of gear a session ago. Silent, that is indistinguishable from a bug.
func TestCompanionRebaseNotice_NamesReservationAndCarriesNoNumbers(t *testing.T) {
	cleanup := seedGolemSpell()
	defer cleanup()

	ch := characters.New()
	ch.ConvictionMax.Base = 300
	require.NoError(t, ch.Validate())
	ch.Companions = []characters.CompanionInfo{
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised},
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised},
	}
	require.True(t, refreshCompanionReserves(ch))
	ch.RecalculateStats()

	// Setup invariant: this owner really is past the ceiling now.
	require.Greater(t,
		ch.GetPoolReservation("conviction", ch.ConvictionMax.Value),
		ch.ReservationCap(characters.PoolConviction),
		"test setup: the recompute must have pushed this owner over")

	msg := companionRebaseNotice(ch)

	assert.Contains(t, msg, "reserve",
		"the notice must name reservation as the cause")
	assert.Contains(t, msg, "gear",
		"gear is the half of the price that can move without the player noticing")
	assert.NotContains(t, msg, "dismiss",
		"nothing is dismissed at login (D4); the notice must not imply otherwise")
	for _, digit := range "0123456789" {
		assert.NotContains(t, msg, string(digit), "player copy carries no raw numbers")
	}
	assert.False(t, strings.ContainsAny(msg, "–—"), "no en or em dashes in player copy")

	// And D4 still holds: nobody was dropped.
	assert.Len(t, ch.Companions, 2)
}

// The quiet case. A refresh that changes nothing, or that does not worsen the
// overage, must not nag on every login.
func TestRefreshCompanionReserves_QuietWhenNothingWorsens(t *testing.T) {
	cleanup := seedGolemSpell()
	defer cleanup()

	ch := characters.New()
	ch.ConvictionMax.Base = 5000
	require.NoError(t, ch.Validate())
	ch.Companions = []characters.CompanionInfo{
		{MobId: 305, Name: "Golem", SourceType: characters.CompanionRaised},
	}

	before := ch.ReservationOverages()
	require.True(t, refreshCompanionReserves(ch))
	ch.RecalculateStats()

	_, worse := before.Worsened(ch.ReservationOverages())
	assert.False(t, worse, "a companion well inside the ceiling must not trigger the notice")
}
