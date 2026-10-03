package behaviortree

import (
	"context"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/npcidle"
)

// askedGen counts how often the idle-moment seam is consulted.
type askedGen struct{ asked int }

func (g *askedGen) Chance() int {
	g.asked++
	return 0 // never hits: the set line always runs
}
func (g *askedGen) Reserve(int) bool { return false }
func (g *askedGen) Generate(context.Context, int, npcidle.Request) (npcidle.Result, error) {
	return npcidle.Result{}, nil
}

// A tree's say and emote may be written fresh only on an idle tick
// (internal/npcidle); a greeting, an answer or any other event keeps its
// set line and never consults the seam.
func TestTreeSayAndEmoteAreReplaceableOnlyWhenIdle(t *testing.T) {
	m := &mobs.Mob{MobId: 60, InstanceId: 600, Character: *characters.New()}
	m.Character.Name = `a ferryman`
	t.Cleanup(mobs.SeedMobsForTest(map[int]*mobs.Mob{60: m}, map[int]*mobs.Mob{600: m}))
	g := &askedGen{}
	npcidle.SetGenerator(g)
	t.Cleanup(func() { npcidle.SetGenerator(nil); events.DrainQueuedInputsForTest(600) })
	events.DrainQueuedInputsForTest(600)

	for _, ev := range []string{"player_enter", "player_ask", "player_give"} {
		ctx := &EvalContext{InstanceId: 600, Event: EventContext{EventType: ev}}
		actEmote(map[string]any{"text": "nods"}, ctx)
		actSay(map[string]any{"text": "Welcome aboard."}, ctx)
	}
	if g.asked != 0 {
		t.Fatalf("only idle lines may be replaced; asked %d times", g.asked)
	}
	if got := events.DrainQueuedInputsForTest(600); len(got) != 6 {
		t.Fatalf("every set line ran: %q", got)
	}

	idle := &EvalContext{InstanceId: 600, Event: EventContext{EventType: "mob_idle"}}
	actEmote(map[string]any{"text": "coils a wet rope"}, idle)
	actSay(map[string]any{"text": "River's high."}, idle)
	if g.asked != 2 {
		t.Fatalf("each idle say and emote consults the seam: %d", g.asked)
	}
	if got := events.DrainQueuedInputsForTest(600); len(got) != 2 || got[0] != "emote coils a wet rope" {
		t.Fatalf("a miss runs the set line: %q", got)
	}
}
