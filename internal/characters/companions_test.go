package characters

import (
	"testing"

	"gopkg.in/yaml.v2"

	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── GetMaxCompanions ────────────────────────────────────────────────────────

func TestGetMaxCompanions_SoftBackstop(t *testing.T) {
	seedMut := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"broodqueen": {
			MutationId: "broodqueen", Name: "Brood Queen", Rarity: 8, Pole: "belief",
			Pros: []mutations.MutationEffect{{Type: "flag", Target: "companion-cap-raise"}},
		},
	})
	defer seedMut()

	// No manifestation investment still returns the soft cap — the real gate is
	// the Conviction reservation ceiling, not this count.
	c := New()
	assert.Equal(t, 5, c.GetMaxCompanions())

	// The apex flag raises the backstop.
	c.Mutations = map[string]int{"broodqueen": 1}
	assert.Equal(t, 7, c.GetMaxCompanions())
}

// ─── AddCompanion ────────────────────────────────────────────────────────────

func TestAddCompanion_Success(t *testing.T) {
	c := New()
	// Give skill 19 → cap of 1
	c.Skills[string(skills.Manifestation)] = 19

	comp := CompanionInfo{
		MobId:      1001,
		InstanceId: 5,
		SourceType: CompanionSummoned,
		Name:       "Spirit Wolf",
	}

	ok := c.AddCompanion(comp)
	assert.True(t, ok, "expected AddCompanion to succeed when under cap")
	require.Len(t, c.Companions, 1)
	assert.Equal(t, comp, c.Companions[0])
}

func TestAddCompanion_AtCap(t *testing.T) {
	c := New()
	// Soft backstop is 5 (the real limit is the Conviction budget, tested
	// separately). Filling to the backstop and rejecting the next verifies it.
	for i := 0; i < 5; i++ {
		ok := c.AddCompanion(CompanionInfo{MobId: 1000 + i, InstanceId: i + 1, Name: "Wolf"})
		assert.True(t, ok, "add under the soft cap should succeed")
	}
	ok := c.AddCompanion(CompanionInfo{MobId: 2000, InstanceId: 99, Name: "Wolf Too Many"})
	assert.False(t, ok, "add at the soft cap should fail")
	assert.Len(t, c.Companions, 5, "companion count should remain at the soft cap")
}

// ─── RemoveCompanion ─────────────────────────────────────────────────────────

func TestRemoveCompanion_ByInstanceId(t *testing.T) {
	c := New()
	// cap = 2
	c.Skills[string(skills.Manifestation)] = 38

	alpha := CompanionInfo{MobId: 1001, InstanceId: 10, Name: "Alpha Wolf"}
	beta := CompanionInfo{MobId: 1002, InstanceId: 20, Name: "Beta Bear"}

	c.AddCompanion(alpha)
	c.AddCompanion(beta)
	require.Len(t, c.Companions, 2)

	removed := c.RemoveCompanion(10)
	require.NotNil(t, removed, "expected a non-nil return for valid instance ID")
	assert.Equal(t, 10, removed.InstanceId)
	assert.Equal(t, "Alpha Wolf", removed.Name)

	// Beta should still be present
	require.Len(t, c.Companions, 1)
	assert.Equal(t, 20, c.Companions[0].InstanceId)
}

// ─── GetCompanion (partial name) ─────────────────────────────────────────────

func TestGetCompanion_PartialMatch(t *testing.T) {
	c := New()
	c.Skills[string(skills.Manifestation)] = 19

	comp := CompanionInfo{MobId: 1001, InstanceId: 7, Name: "Steppe Spirit Wolf"}
	c.AddCompanion(comp)

	found := c.GetCompanion("wolf")
	require.NotNil(t, found, "expected to find companion via partial match 'wolf'")
	assert.Equal(t, "Steppe Spirit Wolf", found.Name)
}

func TestGetCompanion_NotFound(t *testing.T) {
	c := New()
	c.Skills[string(skills.Manifestation)] = 19

	comp := CompanionInfo{MobId: 1001, InstanceId: 7, Name: "Steppe Spirit Wolf"}
	c.AddCompanion(comp)

	found := c.GetCompanion("dragon")
	assert.Nil(t, found, "expected nil for name not matching any companion")
}

// ─── GetCompanionByInstanceId ─────────────────────────────────────────────────

func TestGetCompanionByInstanceId(t *testing.T) {
	c := New()
	c.Skills[string(skills.Manifestation)] = 38

	alpha := CompanionInfo{MobId: 1001, InstanceId: 42, Name: "Alpha"}
	beta := CompanionInfo{MobId: 1002, InstanceId: 99, Name: "Beta"}
	c.AddCompanion(alpha)
	c.AddCompanion(beta)

	found := c.GetCompanionByInstanceId(42)
	require.NotNil(t, found)
	assert.Equal(t, 42, found.InstanceId)
	assert.Equal(t, "Alpha", found.Name)

	notFound := c.GetCompanionByInstanceId(999)
	assert.Nil(t, notFound)
}

// ─── ConvictionReserve field ─────────────────────────────────────────────────

func TestCompanionInfo_ConvictionReserveField(t *testing.T) {
	c := New()
	c.Skills[string(skills.Manifestation)] = 19
	comp := CompanionInfo{MobId: 1001, InstanceId: 5, Name: "Spirit Wolf", ConvictionReserve: 333}
	require.True(t, c.AddCompanion(comp))
	assert.Equal(t, 333, c.Companions[0].ConvictionReserve)
}

// ─── CalcCompanionReserve ─────────────────────────────────────────────────────

// The base a companion reserves is now CompanionReserveDefault scaled by the
// pet's multiplier (D9), so the ongoing budget tracks pet POWER rather than
// being a flat two-tier charge shared across both families.
func TestCompanionReserveBase_TracksThePetMultiplier(t *testing.T) {
	for _, tt := range []struct {
		name       string
		multiplier float64
		want       int
	}{
		{"magma", 1.25, 350},
		{"earth", 1.05, 294},
		{"fire / golem", 1.00, 280},
		{"vampire", 0.83, 232},
		{"water / spectre / steppe spirit", 0.75, 210},
		{"zombie", 0.67, 188},
		{"wraith", 0.58, 162},
		{"skeleton", 0.50, 140},
		{"hive swarm", 0.30, 84},
		// Charm has no authored pet, so it reserves the unscaled default. Its
		// price therefore does not move in U7b.
		{"charm (no pet)", 1.00, 280},
		// The paths with no pet tier at all (charm, the brood floor, the
		// homunculus) pass 0, which means "unscaled" rather than "free".
		{"unscaled (multiplier 0)", 0, 280},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CompanionReserveBase(tt.multiplier))
		})
	}
}

// CalcCompanionReserve composes the U7 inverse-skill band ON TOP OF the existing
// manifestation reduction (D10 §4.1). Compose, never replace: the U7 curve
// bottoms at 0.40 while the existing reduction already reaches 0.45 at
// manifestation 55 and 0.21 with the Manifester mutation, so a replacement would
// make companions DEARER for everyone, the exact opposite of intent.
func TestCalcCompanionReserve_ComposesTheInverseSkillRider(t *testing.T) {
	seedMut := mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"broodmaster": {
			MutationId: "broodmaster", Name: "Broodmaster", Rarity: 5, Pole: "belief",
			Pros: []mutations.MutationEffect{{Type: "companion_reserve_reduction"}},
		},
	})
	defer seedMut()

	// Rank 0: the rider's PENALTY half applies, deliberately and consistently
	// with the item side. A rank-0 summoner pays 1.10x, so 280 -> 308.
	novice := New()
	novice.Skills[string(skills.Manifestation)] = 0
	assert.Equal(t, 308, novice.CalcCompanionReserve(280))

	// Rank 25 is the rider's neutral point (1.00x), and the existing skill
	// reduction is 0.25, so 280 * 0.75 * 1.00 = 210.
	mid := New()
	mid.Skills[string(skills.Manifestation)] = 25
	assert.Equal(t, 210, mid.CalcCompanionReserve(280))

	// The composed curve must never be worse than the existing reduction alone
	// at any rank past the rider's neutral point -- that is the property that
	// makes composition safe.
	for rank := 25; rank <= 100; rank++ {
		c := New()
		c.Skills[string(skills.Manifestation)] = rank
		composed := c.CalcCompanionReserve(280)
		if composed > 210 {
			t.Fatalf("manifestation %d: composed reserve %d exceeds the rank-25 figure 210; "+
				"the curve must be monotonically non-increasing past neutral", rank, composed)
		}
	}
}

// The reserve must never round to zero: a free companion is an unbounded one.
func TestCalcCompanionReserve_FloorsAtOne(t *testing.T) {
	c := New()
	c.Skills[string(skills.Manifestation)] = 100
	if got := c.CalcCompanionReserve(1); got < 1 {
		t.Fatalf("CalcCompanionReserve(1) = %d, want at least 1", got)
	}
}

// D5: the cap subsumes CanAffordCompanion, which is removed rather than kept
// alongside. Two ceilings on the same pool means the weaker one never fires.
func TestCanAffordCompanionIsGone(t *testing.T) {
	// This test exists only as a tombstone. If someone reintroduces the method
	// the compiler will not complain, so state the intent in prose: companion
	// affordability is now WouldBreachReservationCap(PoolConviction, reserve)
	// plus the GetMaxCompanions count backstop, checked at the call site.
	t.Skip("tombstone: see WouldBreachReservationCap and GetMaxCompanions")
}

// ─── CalcCompanionPool ───────────────────────────────────────────────────────

// The numbers here are the spec's own expected-outcome table, which is
// internally consistent with B = 406 (Charisma 166 + manifestation 48 x 5).
//
// Rounding is math.Round, half away from zero. Three of the four skeleton rows
// in the spec's table land on a .5 and round UP, which is what pins it; the
// magma crossover confirms it independently (406 x 1.25 = 507.5 -> 508, the
// spec's stated conjure-magma figure). The spec's "126" for the 100-pool
// skeleton is an arithmetic slip and should read 127.
func TestCalcCompanionPool(t *testing.T) {
	const cha, manifest = 166, 48 // B = 166 + 240 = 406

	tests := []struct {
		name       string
		charisma   int
		manifest   int
		multiplier float64
		corpsePool int
		want       int
	}{
		// Conjures: no corpse, so the multiplier applies to B directly.
		{"conjure magma", cha, manifest, 1.25, 0, 508},
		{"conjure earth", cha, manifest, 1.05, 0, 426},
		{"conjure fire", cha, manifest, 1.00, 0, 406},
		{"conjure water", cha, manifest, 0.75, 0, 305}, // 304.5 -> 305
		{"hive swarm", cha, manifest, 0.30, 0, 122},    // 121.8 -> 122

		// Raises: the multiplier applies AFTER the corpse average, which is the
		// whole point of the reshape. Every tier stays proportionally separated
		// at every corpse size.
		{"golem on a trash corpse", cha, manifest, 1.00, 100, 253},
		{"skeleton on a trash corpse", cha, manifest, 0.50, 100, 127},
		{"golem on a boss corpse", cha, manifest, 1.00, 400, 403},
		{"skeleton on a boss corpse", cha, manifest, 0.50, 400, 202},
		{"golem on a rich corpse", cha, manifest, 1.00, 1000, 703},
		{"skeleton on a rich corpse", cha, manifest, 0.50, 1000, 352},
		{"golem on the Core Guardian", cha, manifest, 1.00, 2800, 1603},
		{"skeleton on the Core Guardian", cha, manifest, 0.50, 2800, 802},

		// A fresh summoner: no manifestation investment at all.
		{"novice conjures fire", 100, 0, 1.00, 0, 100},
		{"novice raises a skeleton", 100, 0, 0.50, 60, 40},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalcCompanionPool(tt.charisma, tt.manifest, tt.multiplier, tt.corpsePool)
			assert.Equal(t, tt.want, got)
		})
	}
}

// Every pet tier must stay proportionally separated however big the corpse is.
// This is the property the reshape exists to deliver, and it is the one a future
// "simplification" back to a pre-average multiplier would silently destroy.
func TestCalcCompanionPool_TiersStaySeparatedAtEveryCorpseSize(t *testing.T) {
	const cha, manifest = 166, 48
	for _, corpse := range []int{0, 100, 500, 1000, 2800, 10000} {
		golem := CalcCompanionPool(cha, manifest, 1.00, corpse)
		skeleton := CalcCompanionPool(cha, manifest, 0.50, corpse)
		ratio := float64(golem) / float64(skeleton)
		if ratio < 1.99 || ratio > 2.01 {
			t.Errorf("corpse %d: golem/skeleton = %.4f, want 2.0 at every corpse size "+
				"(the multiplier must be applied AFTER the average, not before)", corpse, ratio)
		}
	}
}

// A zero or negative multiplier must not silently field a pool-zero companion.
func TestCalcCompanionPool_FloorsAtOne(t *testing.T) {
	if got := CalcCompanionPool(100, 0, 0, 0); got != 1 {
		t.Errorf("a zero multiplier = %d, want 1 (never a pool-zero companion)", got)
	}
	if got := CalcCompanionPool(0, 0, 1.0, 0); got != 1 {
		t.Errorf("a zero-charisma novice = %d, want 1", got)
	}
}

// ─── CalcSpawnPoolFromBase ───────────────────────────────────────────────────

// The behaviour-tree add scaler keeps the OLD shape and is renamed to say so.
// It is not the player companion formula and must not be confused for one: its
// callers are authored boss encounters whose base_pool values (50 for the Core
// Guardian's repair frames, 300 for the Sentinel) were tuned against exactly
// this curve.
func TestCalcSpawnPoolFromBase(t *testing.T) {
	// Config defaults apply when no config is loaded:
	//   ManifestStatScaleChaFactor   = 150
	//   ManifestStatScaleSkillFactor = 0.02
	// scale = 1.0 + 100/150 + 0*0.02 = 1.667  ->  50 * 1.667 = 83
	assert.Equal(t, 83, CalcSpawnPoolFromBase(50, 100, 0))
	// scale = 1.667  ->  300 * 1.667 = 500
	assert.Equal(t, 500, CalcSpawnPoolFromBase(300, 100, 0))
}

// ─── CompanionInfo YAML persistence ──────────────────────────────────────────

func TestCompanionInfo_YAMLPersistence(t *testing.T) {
	original := CompanionInfo{
		MobId:      2345,
		InstanceId: 77, // should NOT survive round-trip (yaml:"-")
		SourceType: CompanionSummoned,
		Name:       "Hive Swarm",
		AutoAssist: true,
		StatTraining: map[string]int{
			"strength": 3,
		},
		Skills: map[string]int{
			"unarmed-combat": 5,
		},
		SkillUseCount: map[string]int{
			"unarmed-combat": 12,
		},
		Mutations: map[string]int{
			"carapace": 1,
		},
		SpellBook: map[string]int{
			"sparks": 1,
		},
		MutationProgress: 0.75,
	}

	data, err := yaml.Marshal(original)
	require.NoError(t, err)

	var restored CompanionInfo
	err = yaml.Unmarshal(data, &restored)
	require.NoError(t, err)

	// InstanceId must NOT survive (yaml:"-")
	assert.Equal(t, 0, restored.InstanceId, "InstanceId should not be persisted")

	// All other fields must survive
	assert.Equal(t, original.MobId, restored.MobId)
	assert.Equal(t, original.SourceType, restored.SourceType)
	assert.Equal(t, original.Name, restored.Name)
	assert.Equal(t, original.AutoAssist, restored.AutoAssist)
	assert.Equal(t, original.StatTraining, restored.StatTraining)
	assert.Equal(t, original.Skills, restored.Skills)
	assert.Equal(t, original.SkillUseCount, restored.SkillUseCount)
	assert.Equal(t, original.Mutations, restored.Mutations)
	assert.Equal(t, original.SpellBook, restored.SpellBook)
	assert.InDelta(t, original.MutationProgress, restored.MutationProgress, 1e-9)
}
