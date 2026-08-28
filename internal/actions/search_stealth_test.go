package actions

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
)

// hiddenMobRoom builds a room holding one hidden mob with a given skullduggery
// rank, so a Search resolves exactly one hidden-entity check.
func hiddenMobRoom(t *testing.T, roomId, instId, sneakRank int) *rooms.Room {
	t.Helper()
	room := &rooms.Room{RoomId: roomId}
	m := newSearchTestMob(instId, "Lurker", roomId)
	m.Character.Skills = map[string]int{"skullduggery": sneakRank}
	m.Character.Stats.Dexterity.Base = 100
	m.Character.Stats.Dexterity.Recalculate()
	m.Character.Buffs = buffs.New()
	// IsHidden() reads the Awareness FSM, not a buff. Concealing then resolving
	// is the only route into awareness.Hidden.
	m.Character.Awareness = awareness.NewMachine()
	reason := state.TransitionReason{Trigger: "search_stealth_test"}
	if err := m.Character.Awareness.TransitionToConcealing(awareness.ConcealingData{}, reason); err != nil {
		t.Fatalf("concealing the test mob: %v", err)
	}
	m.Character.Awareness.ResolveConcealment(true, reason)
	if !m.Character.IsHidden() {
		t.Fatal("fixture mob is not hidden; the search would skip it entirely")
	}
	mobs.SetInstanceForTest(instId, m)
	t.Cleanup(func() { mobs.SetInstanceForTest(instId, nil) })
	room.AddMob(instId)
	return room
}

// spotRate reports how often a searcher with the given perception finds a
// hidden mob of the given sneak rank, over independent attempts.
//
// Fresh actor per trial: Search is cooldown-gated per character.
func spotRate(t *testing.T, perception, sneakRank, trials int) float64 {
	t.Helper()
	found := 0
	for i := 0; i < trials; i++ {
		room := hiddenMobRoom(t, 9500+i, 8500+i, sneakRank)
		actor := newSearchFakeActor("StealthSearcher", room, false, 0)
		actor.char.Stats.Perception.ValueAdj = perception
		actor.char.Stats.Perception.Base = perception
		actor.char.Stats.Perception.Recalculate()
		if len(Search(actor, SearchOptions{}).HiddenMobsFound) > 0 {
			found++
		}
	}
	return float64(found) / float64(trials)
}

// TestHiddenDetectionReadsTheHidersSkill is the substance of spec 5.2, and the
// slice's ONE deliberate behaviour change.
//
// The flat 135 threshold NEVER read the hider's score, while
// usercommands/go.go resolved the identical question as an opposed contest. A
// hider's skill decided the outcome in one path and was ignored in the other,
// and mobs reached the broken path too via behaviortree actTrySearch.
//
// Asserted as an ORDERING rather than exact rates, because the contest is
// stochastic and the point is simply that hiding well now works.
func TestHiddenDetectionReadsTheHidersSkill(t *testing.T) {
	const trials = 800

	novice := spotRate(t, 100, 0, trials)
	expert := spotRate(t, 100, 45, trials)

	if expert >= novice {
		t.Fatalf("a skullduggery-45 hider was spotted %.1f%% of the time against "+
			"a rank-0 hider's %.1f%%. The hider's skill must REDUCE the spot "+
			"rate; equal rates mean the flat 135 threshold survived and the "+
			"hider is still being ignored.", expert*100, novice*100)
	}

	// Guard against a change that technically orders the two but is too small
	// to matter to a player. The old form produced EXACTLY equal rates.
	if novice-expert < 0.10 {
		t.Errorf("skill 0 spotted %.1f%%, skill 45 spotted %.1f%% — a gap of "+
			"only %.1f points. Stealth investment should be clearly visible.",
			novice*100, expert*100, (novice-expert)*100)
	}
}
