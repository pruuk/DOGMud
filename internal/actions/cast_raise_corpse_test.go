package actions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/refusal"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
)

// mkRaiseCorpse builds a mob corpse for raise-selection tests. training feeds
// the stat-pool metric (one stat carries it); gold>0 makes HasLoot() true.
func mkRaiseCorpse(name string, charmed bool, gold, training int) rooms.Corpse {
	c := rooms.Corpse{MobId: 1, WasCharmed: charmed}
	c.Character.Name = name
	c.Character.Stats.Strength.Training = training
	if gold > 0 {
		c.Loot.Gold = gold
	}
	return c
}

func TestSelectRaiseCorpse_SkipsLootHolder_WithReason(t *testing.T) {
	corpses := []rooms.Corpse{mkRaiseCorpse("goblin", false, 50, 300)} // still holds gold
	idx, _, reason := SelectRaiseCorpse(corpses, "", 1, 0)
	if idx != -1 {
		t.Fatalf("idx=%d, want -1 (loot-holder skipped)", idx)
	}
	if reason != RaiseHasLoot {
		t.Errorf("reason=%v, want RaiseHasLoot", reason)
	}
}

func TestSelectRaiseCorpse_PicksValidAmongSkipped(t *testing.T) {
	corpses := []rooms.Corpse{
		mkRaiseCorpse("goblin", true, 0, 300),   // former companion -> skip
		mkRaiseCorpse("goblin", false, 10, 300), // still holds loot -> skip
		mkRaiseCorpse("goblin", false, 0, 300),  // valid
	}
	idx, pool, _ := SelectRaiseCorpse(corpses, "", 1, 0)
	if idx != 2 {
		t.Errorf("idx=%d, want 2 (the valid corpse)", idx)
	}
	if pool != 300 {
		t.Errorf("pool=%d, want 300", pool)
	}
}

func TestSelectRaiseCorpse_TooWeak(t *testing.T) {
	corpses := []rooms.Corpse{mkRaiseCorpse("goblin", false, 0, 50)} // pool 50 < min 120
	idx, _, reason := SelectRaiseCorpse(corpses, "", 1, 120)
	if idx != -1 || reason != RaiseTooWeak {
		t.Errorf("idx=%d reason=%v, want -1/RaiseTooWeak", idx, reason)
	}
}

func TestSelectRaiseCorpse_NameMismatchGivesNoNearReason(t *testing.T) {
	// A loot-holding corpse whose NAME doesn't match must not set a near-match
	// reason: the player asked for something that isn't here.
	corpses := []rooms.Corpse{mkRaiseCorpse("goblin", false, 50, 300)}
	idx, _, reason := SelectRaiseCorpse(corpses, "rat", 1, 0)
	if idx != -1 || reason != RaiseNoMatch {
		t.Errorf("idx=%d reason=%v, want -1/RaiseNoMatch", idx, reason)
	}
}

func TestSelectRaiseCorpse_LootReasonBeatsCharmed(t *testing.T) {
	corpses := []rooms.Corpse{
		mkRaiseCorpse("goblin", true, 0, 300),   // charmed
		mkRaiseCorpse("goblin", false, 50, 300), // has loot (more actionable)
	}
	_, _, reason := SelectRaiseCorpse(corpses, "", 1, 0)
	if reason != RaiseHasLoot {
		t.Errorf("reason=%v, want RaiseHasLoot (most actionable reason wins)", reason)
	}
}

func TestRaiseCorpseRefusal_LootIsActionable(t *testing.T) {
	msg := RaiseCorpseRefusal(RaiseHasLoot, "")
	if !strings.Contains(strings.ToLower(msg), "loot") {
		t.Errorf("RaiseHasLoot refusal should mention looting first; got %q", msg)
	}
	// The generic no-match keeps distinct named vs unnamed phrasing.
	if RaiseCorpseRefusal(RaiseNoMatch, "wolf") == RaiseCorpseRefusal(RaiseNoMatch, "") {
		t.Error("named and unnamed no-match refusals should differ")
	}
}

// "2.goblin" must filter on "goblin" and pick the second goblin. The filter
// used to keep the "2." prefix, so no corpse matched and the raise refused.
func TestFindRaiseCorpse_IndexedNameStripsTheIndex(t *testing.T) {
	room := &rooms.Room{Corpses: []rooms.Corpse{
		mkRaiseCorpse("goblin", false, 0, 300),
		mkRaiseCorpse("rat", false, 0, 300),
		mkRaiseCorpse("goblin", false, 0, 300),
	}}
	sd := &spells.SpellData{SummonRequiresCorpse: true, SummonMinCorpsePool: 120}

	idx, _, line := FindRaiseCorpse(room, nil, sd, "2.goblin")
	if idx != 2 || line != "" {
		t.Errorf("idx=%d line=%q, want 2 and no refusal", idx, line)
	}
	idx, _, _ = FindRaiseCorpse(room, nil, sd, "goblin#2")
	if idx != 2 {
		t.Errorf("goblin#2: idx=%d, want 2", idx)
	}
	idx, _, _ = FindRaiseCorpse(room, nil, sd, "2.corpse")
	if idx != 1 {
		t.Errorf("2.corpse: idx=%d, want 1 (generic word matches by index)", idx)
	}
}

func TestFindRaiseCorpse_EmptyRoomRefuses(t *testing.T) {
	sd := &spells.SpellData{SummonRequiresCorpse: true, SummonMinCorpsePool: 120}
	idx, _, line := FindRaiseCorpse(&rooms.Room{}, nil, sd, "")
	if idx != -1 || line != refusal.NoRemainsLine {
		t.Errorf("idx=%d line=%q, want -1 and NoRemainsLine", idx, line)
	}
	if idx, _, _ := FindRaiseCorpse(nil, nil, sd, ""); idx != -1 {
		t.Errorf("nil room: idx=%d, want -1", idx)
	}
}

// #587: below clear sight a corpse answers only to the generic word, as
// FindCorpseIndex has it for look, loot and get (#435). A caster who cannot
// see clearly and names a corpse gets the plain no-match line, never a
// refusal that confirms what lies there.
func TestFindRaiseCorpse_NameNeedsClearSight(t *testing.T) {
	t.Cleanup(rooms.SeedBiomesForTest(map[string]*rooms.BiomeInfo{
		"cave":    {BiomeId: "cave", SkyLight: rooms.SkyLightPtr(0.0)},
		"default": {BiomeId: "default"},
	}))
	room := &rooms.Room{RoomId: 9587, Biome: "cave", Corpses: []rooms.Corpse{
		mkRaiseCorpse("goblin", false, 50, 300), // still holds loot
	}}
	caster := characters.New()
	if s := messaging.ParticipantSight(caster, room); s == messaging.SightFull {
		t.Fatalf("fixture: an unlit cave must cost the caster clear sight, got %v", s)
	}
	sd := &spells.SpellData{SummonRequiresCorpse: true, SummonMinCorpsePool: 120}

	idx, _, line := FindRaiseCorpse(room, caster, sd, "goblin")
	if idx != -1 || line != refusal.NoRemainsMatchingLine {
		t.Errorf("named in the dark: idx=%d line=%q, want -1 and NoRemainsMatchingLine", idx, line)
	}
	// The generic word still reaches the corpse, as `look corpse` does.
	if _, _, line := FindRaiseCorpse(room, caster, sd, "corpse"); line != refusal.RemainsHoldLootLine {
		t.Errorf("generic in the dark: line=%q, want RemainsHoldLootLine", line)
	}
	// A nil caster (a system lookup) reads as clear sight.
	if _, _, line := FindRaiseCorpse(room, nil, sd, "goblin"); line != refusal.RemainsHoldLootLine {
		t.Errorf("nil caster: line=%q, want RemainsHoldLootLine", line)
	}
}
