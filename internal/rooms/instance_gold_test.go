package rooms

import (
	"math"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Owner ruling 2026-10-17: in a paid instance a mob's gold scales with the
// entry gold. Regular mobs carry 2 gold per 100 paid, tough mobs 3, bosses 5,
// rolled uniformly within plus or minus InstanceGoldVariancePct (25) and
// rounded to whole gold. The tier is the spawn's stat-pool multiplier
// (1 regular, 2 tough, 4 boss), which ScaleSpawnStatPools overwrites with the
// scaled pool, so the spawn must carry it to spawn time.

const instGoldMobId = 7861

// pinInstanceGoldConfig pins the knobs these tests depend on, so a change
// to a Go default cannot move the expected bands.
func pinInstanceGoldConfig(t *testing.T, statPoolCap int) {
	t.Helper()
	cfg := configs.GetConfig()
	cfg.Balance.InstanceStatPoolCap = configs.ConfigInt(statPoolCap)
	cfg.Balance.InstanceGoldPer100Regular = 2
	cfg.Balance.InstanceGoldPer100Tough = 3
	cfg.Balance.InstanceGoldPer100Boss = 5
	cfg.Balance.InstanceGoldVariancePct = 25
	configs.SetConfigForTest(t, cfg)
}

// spawnGold spawns one mob through Room.Prepare and returns its gold. mult is
// the authored stat-pool multiplier; goldPaid 0 means an ordinary room.
func spawnGold(t *testing.T, mult, goldPaid, templateGold int, seed int64) int {
	t.Helper()
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
		instGoldMobId: {MobId: instGoldMobId, Zone: "Instance Arena",
			Character: characters.Character{Name: "Gold Probe", Gold: templateGold}},
	}, map[int]*mobs.Mob{}))

	room := &Room{RoomId: 7862, SpawnInfo: []SpawnInfo{{MobId: instGoldMobId, StatPool: mult}}}
	if goldPaid > 0 {
		ScaleSpawnStatPools(room.SpawnInfo, goldPaid, int(configs.GetBalanceConfig().InstanceStatPoolCap))
		room.SetTempData("gold_paid", goldPaid)
	}

	restore := util.SetRandForTest(seed)
	room.Prepare(false)
	restore()

	ids := room.GetMobs()
	require.Len(t, ids, 1, "the room should spawn exactly one mob")
	m := mobs.GetInstance(ids[0])
	require.NotNil(t, m)
	return m.Character.Gold
}

// requireGoldBand spawns over many seeds and checks every result lands in the
// rounded +-25% band around base. When wantSpread is set it also requires the
// roll to produce more than one value, proving the variance is live.
func requireGoldBand(t *testing.T, mult, goldPaid int, base float64, wantSpread bool) {
	t.Helper()
	lo := int(math.Round(base * 0.75))
	hi := int(math.Round(base * 1.25))
	if lo < 1 && base >= 1 {
		lo = 1
	}
	minSeen, maxSeen := math.MaxInt, math.MinInt
	for seed := int64(1); seed <= 60; seed++ {
		g := spawnGold(t, mult, goldPaid, 7, seed)
		if g < lo || g > hi {
			t.Fatalf("mult %d, paid %d, seed %d: gold %d outside band [%d, %d] around %.2f",
				mult, goldPaid, seed, g, lo, hi, base)
		}
		minSeen = min(minSeen, g)
		maxSeen = max(maxSeen, g)
	}
	if wantSpread && minSeen == maxSeen {
		t.Errorf("mult %d, paid %d: every seed rolled %d; the variance roll is not live", mult, goldPaid, minSeen)
	}
}

func TestInstanceMobGold_100GoldEntry(t *testing.T) {
	pinInstanceGoldConfig(t, 50000)
	requireGoldBand(t, 1, 100, 2, false) // regular
	requireGoldBand(t, 2, 100, 3, true)  // tough
	requireGoldBand(t, 4, 100, 5, true)  // boss
}

func TestInstanceMobGold_1000GoldEntry(t *testing.T) {
	pinInstanceGoldConfig(t, 50000)
	requireGoldBand(t, 1, 1000, 20, true)
	requireGoldBand(t, 2, 1000, 30, true)
	requireGoldBand(t, 4, 1000, 50, true)
}

// A multiplier that is not exactly 1, 2 or 4 (Crash Site Interior's Warden
// Prime spawns at 5) takes the nearest rule: 2-3 tough, 4 or more boss.
func TestInstanceMobGold_OddMultipliersMapToTiers(t *testing.T) {
	pinInstanceGoldConfig(t, 50000)
	requireGoldBand(t, 3, 1000, 30, true)
	requireGoldBand(t, 5, 1000, 50, true)
}

// Outside a paid instance the template gold is left alone.
func TestInstanceMobGold_OrdinaryRoomKeepsTemplateGold(t *testing.T) {
	pinInstanceGoldConfig(t, 50000)
	for seed := int64(1); seed <= 10; seed++ {
		if g := spawnGold(t, 4, 0, 7, seed); g != 7 {
			t.Fatalf("seed %d: an ordinary-room mob carries %d gold, want its template 7", seed, g)
		}
	}
}

// When InstanceStatPoolCap clamps a mob's pool, its gold stops rising at the
// entry the cap allows: a boss at 20000 paid with a 50000 cap grows as if
// 12500 were paid, so 625 gold, not 1000. A regular mob at the same entry is
// under the cap (20000 < 50000) and keeps the full 400.
func TestInstanceMobGold_StatPoolCapClampsGold(t *testing.T) {
	pinInstanceGoldConfig(t, 50000)
	requireGoldBand(t, 4, 20000, 625, true)
	requireGoldBand(t, 1, 20000, 400, true)
}

// A base of at least 1 never rolls below 1, even at full variance: a regular
// mob at 50 paid has a base of exactly 1, and a 100% roll spans 0 to 2.
func TestInstanceMobGold_BaseOfOneNeverRollsZero(t *testing.T) {
	pinInstanceGoldConfig(t, 50000)
	cfg := configs.GetConfig()
	cfg.Balance.InstanceGoldVariancePct = 100
	configs.SetConfigForTest(t, cfg)

	sawTwo := false
	for seed := int64(1); seed <= 200; seed++ {
		restore := util.SetRandForTest(seed)
		g := InstanceMobGold(50, 1)
		restore()
		if g < 1 || g > 2 {
			t.Fatalf("seed %d: base 1 at 100%% variance rolled %d, want 1 or 2", seed, g)
		}
		sawTwo = sawTwo || g == 2
	}
	if !sawTwo {
		t.Error("200 seeds never rolled the top of the band; the roll is not uniform")
	}
}

// The Planar Oasis builds its spawns in GenerateOasisCube (trash 1, tough 2,
// boss 4) and CreateZoneInstanceWithOpts scales them with ScaleSpawnStatPools
// like any cloned zone. The tier must survive that path too.
func TestInstanceMobGold_OasisCubeCarriesTier(t *testing.T) {
	pinInstanceGoldConfig(t, 50000)
	threshold := &Room{
		RoomId:    5003,
		Zone:      "Instance Planar Oasis",
		Exits:     make(map[string]exit.RoomExit),
		ExitsTemp: make(map[string]exit.TemporaryRoomExit),
	}
	t.Cleanup(SeedRoomsForTest(map[int]*Room{5003: threshold}, map[string]*ZoneConfig{}))
	t.Cleanup(func() {
		for i := range ephemeralRoomChunks {
			ephemeralRoomChunks[i] = nil
		}
		originalRoomIdLookups = map[int]int{}
	})

	const paid = 1000
	roomIds, _, err := GenerateOasisCube(5003, "Instance Planar Oasis", paid, 1, true, "rejoin")
	require.NoError(t, err)

	// Step 5 of CreateZoneInstanceWithOpts, verbatim in effect.
	pickByMult := map[int]*Room{}
	for _, rid := range roomIds {
		r := LoadRoom(rid)
		require.NotNil(t, r)
		mult := r.SpawnInfo[0].StatPool
		ScaleSpawnStatPools(r.SpawnInfo, paid, int(configs.GetBalanceConfig().InstanceStatPoolCap))
		if _, ok := pickByMult[mult]; !ok {
			pickByMult[mult] = r
		}
	}
	require.Len(t, pickByMult, 3, "the cube should hold trash (1), tough (2) and boss (4) spawns")

	want := map[int]float64{1: 20, 2: 30, 4: 50}
	for mult, r := range pickByMult {
		mobId := r.SpawnInfo[0].MobId
		t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{
			mobId: {MobId: mobs.MobId(mobId), Zone: "Instance Planar Oasis",
				Character: characters.Character{Name: "Oasis Probe"}},
		}, map[int]*mobs.Mob{}))
		restore := util.SetRandForTest(int64(mult))
		r.Prepare(false)
		restore()

		ids := r.GetMobs()
		require.Len(t, ids, 1, "mult %d room should spawn one mob", mult)
		m := mobs.GetInstance(ids[0])
		require.NotNil(t, m)
		base := want[mult]
		lo, hi := int(math.Round(base*0.75)), int(math.Round(base*1.25))
		if g := m.Character.Gold; g < lo || g > hi {
			t.Errorf("oasis mult %d mob carries %d gold, want within [%d, %d] around %.0f", mult, g, lo, hi, base)
		}
	}
}
