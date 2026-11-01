package behaviortree

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

// #690: the Straw Effigy's vitality is set very high so a new player cannot
// drop it, so the fight prompt reads [healthy] however hard it is hit, while
// a critical swing still reads DEVASTATING (combat words a crit by the
// swing, not by the target's health). Its tree says why: now and then, when
// struck, it settles back into shape.
func TestStrawEffigySettlesWhenHurt(t *testing.T) {
	var emoted []string
	chance := Success
	origEmote, origChance := actionRegistry["emote"], conditionRegistry["random_chance"]
	actionRegistry["emote"] = func(params map[string]any, ctx *EvalContext) Result {
		emoted = append(emoted, getStringParam(params, "text"))
		return Success
	}
	conditionRegistry["random_chance"] = func(map[string]any, *EvalContext) Result { return chance }
	t.Cleanup(func() {
		actionRegistry["emote"] = origEmote
		conditionRegistry["random_chance"] = origChance
	})

	_, here, _, ok := runtime.Caller(0)
	require.True(t, ok)
	tree := filepath.Join(filepath.Dir(here), "..", "..", "_datafiles", "world", "dogmud",
		"behaviors", "newcomer_antechamber", "9614-straw_effigy.yaml")
	require.NoError(t, GetEngine().LoadTree(9614, tree))
	t.Cleanup(func() { GetEngine().EvictTree(9614) })

	GetEngine().EvaluateEvent(9614, 96140, EventContext{EventType: "mob_hurt"}, NewBehaviorState())
	require.Len(t, emoted, 1, "a blow that passes the chance sets it settling")
	require.Contains(t, emoted[0], "no worse for the blow")

	GetEngine().EvaluateEvent(9614, 96140, EventContext{EventType: "mob_idle"}, NewBehaviorState())
	require.Len(t, emoted, 1, "only a blow sets it settling")

	chance = Failure
	GetEngine().EvaluateEvent(9614, 96140, EventContext{EventType: "mob_hurt"}, NewBehaviorState())
	require.Len(t, emoted, 1, "the line is paced by its chance, not every hit")
}
