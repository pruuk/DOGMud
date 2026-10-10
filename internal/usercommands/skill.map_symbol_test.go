package usercommands

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #253: the mob map symbol must survive ASCII conversion, or the cell
// vanishes and the row's frame shifts.
func TestMobMapSymbolSurvivesAscii(t *testing.T) {
	if got := mobMapSymbol(false); got != '☠' {
		t.Errorf("utf-8 symbol = %q, want ☠", got)
	}
	sym := mobMapSymbol(true)
	if sym >= 0x80 {
		t.Fatalf("ascii symbol %q is not ASCII", sym)
	}
	if got := util.ConvertToAscii(string(sym)); got != string(sym) {
		t.Errorf("ascii symbol changed by conversion: %q", got)
	}
}

// #253 follow-up: the markers the map command draws convert to one ASCII
// character each, so an ASCII map cell keeps its width.
func TestMapCodeMarkersConvertToOneAsciiCharacter(t *testing.T) {
	for name, r := range map[string]rune{
		"you": '@', "player, npc, party member": '☺', "friend": '☹', "mob": mobMapSymbol(true),
	} {
		got := util.ConvertToAscii(string(r))
		if len(got) != 1 || got[0] >= 0x80 {
			t.Errorf("%s: %q converts to %q, not one ASCII character", name, r, got)
		}
	}
}

// #253 follow-up: the NPC, Player and Mob markers sat under skillLevel > 4,
// which no Perception tier reaches, so no map drew them although `help map`
// lists them (owner call 2026-10-10: level 4).
func TestMapOccupantMarkers_DrawnFromLevelFour(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	// Room 1 holds Aliceia, Bobrick and the hostile Skeleton; a peaceful
	// Merchant joins room 2.
	merchant := &mobs.Mob{MobId: 2, InstanceId: 9253, HomeRoomId: 2}
	merchant.Character.Name = "Merchant"
	merchant.Character.Conditions = conditions.New()
	t.Cleanup(mobs.SeedMobsForTest(nil, map[int]*mobs.Mob{100: mobs.GetInstance(100), 9253: merchant}))
	rooms.LoadRoom(2).AddMob(9253)

	for _, level := range []int{1, 2, 3} {
		c := mapper.Config{}
		addOccupantMarkers(&c, level, false)
		_, npc := c.SymbolOverrideAt(2)
		_, player := c.SymbolOverrideAt(1)
		assert.False(t, npc || player, "a level-%d map draws no occupant marker", level)
	}

	c := mapper.Config{}
	addOccupantMarkers(&c, 4, false)
	npc, ok := c.SymbolOverrideAt(2)
	require.True(t, ok, "a level-4 map marks the room holding the Merchant")
	assert.Equal(t, mapper.SymbolOverride{Symbol: '☺', Legend: "NPC"}, npc)
	player, ok := c.SymbolOverrideAt(1)
	require.True(t, ok, "a level-4 map marks the room holding players")
	assert.Equal(t, mapper.SymbolOverride{Symbol: '☺', Legend: "Player"}, player,
		"players are marked last, so they win over the Skeleton in the same room")
}

// A client that reports a tiny or zero-height screen must never shrink the
// map below the standard size (it panicked on a negative height).
func TestMapSize_WideNeverBelowStandard(t *testing.T) {
	for _, tc := range []struct{ sw, sh int }{{80, 0}, {10, 5}, {0, 0}, {80, 24}} {
		w, h := mapSizeFor(4, true, tc.sw, tc.sh)
		assert.GreaterOrEqual(t, w, 65, "screen %dx%d width", tc.sw, tc.sh)
		assert.GreaterOrEqual(t, h, 21, "screen %dx%d height", tc.sw, tc.sh)
	}
}

// `help map` promises `map wide` at level 4; the screen-size sizing sat under
// the same unreachable skillLevel > 4 (owner call 2026-10-10).
func TestMapSize_WideFromLevelFour(t *testing.T) {
	const defW, defH = 65, 21
	w, h := mapSizeFor(4, true, 100, 40)
	assert.Equal(t, 100-mapBorderWidth, w, "level-4 map wide fits the screen width")
	assert.Equal(t, 34, h, "level-4 map wide fits the screen height, kept even")

	for _, tc := range []struct {
		level int
		wide  bool
	}{{4, false}, {3, true}, {1, true}} {
		w, h := mapSizeFor(tc.level, tc.wide, 100, 40)
		assert.Equal(t, defW, w, "level %d wide=%v keeps the standard width", tc.level, tc.wide)
		assert.Equal(t, defH, h, "level %d wide=%v keeps the standard height", tc.level, tc.wide)
	}
}
