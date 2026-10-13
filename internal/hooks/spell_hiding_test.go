package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobcommands"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/targeting"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Messaging M6 slice 4, owner ruling R6: starting a cast never breaks
// hiding; a harmful spell landing does, for every caster.
//
// hiddenHolderFixture (hidden_holder_condition_lines_test.go) seeds lit room
// 1 with users 1 and 2 and the Skeleton (instance 100), and seeds stealth
// record 9 with its shipped flags (hidden, cancel-on-combat) and end line.

func hidingSpikeSpell() *spells.SpellData {
	return &spells.SpellData{SpellId: "test-spike", Name: "Test Spike", AttackType: combatvocab.AttackSpell,
		DamageType: combatvocab.DamageMental, Targeting: combatvocab.TargetSingle, EffectType: "damage"}
}

func startCastingForHidingTest(t *testing.T, c *characters.Character) {
	t.Helper()
	require.NoError(t, c.Activity.TransitionToCasting(activity.CastingData{
		SpellId: "test-spike", FoldsNeeded: 4, FoldsPerRound: 1, TotalConvictionCost: 10,
		TargetUserIds: []int{1},
	}, state.TransitionReason{Trigger: activity.TriggerCastBegin}))
	t.Cleanup(func() {
		_ = c.Activity.TransitionToFree(state.TransitionReason{Trigger: activity.TriggerCastCancel})
	})
}

// A mob's harmful cast commits it at cast start only so its fold steps in
// the combat loop (#242). That commit carries its own trigger, and the
// Awareness cascade does not reveal on it.
func TestCastFoldCommitLeavesTheCasterHidden(t *testing.T) {
	_, _, m := hiddenHolderFixture(t)
	sneakHide(t, &m.Character)
	startCastingForHidingTest(t, &m.Character)
	t.Cleanup(func() { targeting.Release(&m.Character, targeting.ReasonDisengage) })

	require.True(t, targeting.Commit(&m.Character, state.ActorRef{UserId: 1}, targeting.ReasonCastFold))
	require.True(t, m.Character.IsHidden(), "a fold-only combat entry must not reveal the caster")
}

// Null probe for the test above, and the guard against a casting-based
// skip: a hidden caster mid-fold that enters combat for any other reason
// (turning on a player who attacked it, say) is revealed as before.
func TestAttackCommitRevealsAHiddenCasterMidFold(t *testing.T) {
	_, _, m := hiddenHolderFixture(t)
	sneakHide(t, &m.Character)
	startCastingForHidingTest(t, &m.Character)
	t.Cleanup(func() { targeting.Release(&m.Character, targeting.ReasonDisengage) })

	require.True(t, targeting.Commit(&m.Character, state.ActorRef{UserId: 1}, targeting.ReasonAttack))
	require.False(t, m.Character.IsHidden(), "an ordinary commit reveals, casting or not")
}

// A harmful landing reveals a hidden caster explicitly, even one already in
// combat, which makes no Idle to Engaging transition of its own. The room
// reads the stealth record's end line.
func TestHarmfulLandingRevealsAHiddenMobCaster(t *testing.T) {
	seer, watcher, m := hiddenHolderFixture(t)
	room := rooms.LoadRoom(1)
	require.True(t, targeting.Commit(&m.Character, state.ActorRef{UserId: seer.UserId}, targeting.ReasonAttack))
	t.Cleanup(func() { targeting.Release(&m.Character, targeting.ReasonDisengage) })
	sneakHide(t, &m.Character) // fixture: hidden while already fighting
	events.DrainQueuedMessagesForTest(watcher.UserId)

	c := newSpellEffectCtx(&m.Character, actions.NewMobActorInRoom(m, room),
		actions.NewUserActorInRoom(seer, room), room, hidingSpikeSpell(), 100, combat.ChannelDefenceResult{})
	commitHarmfulSpellAggro(c, false)

	require.False(t, m.Character.IsHidden(), "the landing must reveal its hidden caster")
	PruneConditions(events.NewTurn{TurnNumber: 1})
	got := drainPlain(watcher.UserId)
	require.Equal(t, 1, countContaining(got, "Skeleton emerges from the shadows."), "%v", got)
}

// The strike line is judged against the room as the spell found it, before
// the landing's reveal: a hidden mob caster's result line keeps "Something"
// (owner ruling 2026-10-10), and the stealth end line follows it.
func TestHarmfulLandingLineIsJudgedBeforeTheReveal(t *testing.T) {
	target, watcher, m := hiddenHolderFixture(t)
	room := rooms.LoadRoom(1)
	require.True(t, targeting.Commit(&m.Character, state.ActorRef{UserId: target.UserId}, targeting.ReasonCastFold))
	t.Cleanup(func() {
		targeting.Release(&m.Character, targeting.ReasonDisengage)
		targeting.Release(target.Character, targeting.ReasonDisengage)
	})
	sneakHide(t, &m.Character)
	events.DrainQueuedMessagesForTest(watcher.UserId)

	applySpellDamage(newSpellEffectCtx(&m.Character, actions.NewMobActorInRoom(m, room),
		actions.NewUserActorInRoom(target, room), room, hidingSpikeSpell(), 10, spellContestAttackWin()))
	PruneConditions(events.NewTurn{TurnNumber: 1})

	require.False(t, m.Character.IsHidden())
	got := drainPlain(watcher.UserId)
	strike := indexContaining(got, "Something's Test Spike strikes Aliceia!")
	emerge := indexContaining(got, "Skeleton emerges from the shadows.")
	require.NotEqual(t, -1, strike, "%v", got)
	require.NotEqual(t, -1, emerge, "%v", got)
	require.Less(t, strike, emerge, "the strike is told before the reveal: %v", got)
	require.Zero(t, countContaining(got, "Skeleton's Test Spike"), "%v", got)
}

// R6 is for every caster: a hidden player's harmful spell on a mob reveals
// the player when it lands.
func TestHarmfulLandingRevealsAHiddenPlayerCaster(t *testing.T) {
	caster, _, m := hiddenHolderFixture(t)
	room := rooms.LoadRoom(1)
	sneakHide(t, caster.Character)
	t.Cleanup(func() {
		targeting.Release(caster.Character, targeting.ReasonDisengage)
		targeting.Release(&m.Character, targeting.ReasonDisengage)
	})

	c := newSpellEffectCtx(caster.Character, actions.NewUserActorInRoom(caster, room),
		actions.NewMobActorInRoom(m, room), room, hidingSpikeSpell(), 100, combat.ChannelDefenceResult{})
	commitHarmfulSpellAggro(c, true)

	require.False(t, caster.Character.IsHidden(), "the landing must reveal its hidden player caster")
}

// End to end through the mob's own cast command and the combat round: a
// hidden mob's harmful cast leaves it hidden through its fold rounds (the
// round's cancel-on-combat strip must not reveal a mob still folding), and
// it is out of hiding once the cast is over.
func TestHiddenMobHarmfulCast_StaysHiddenWhileFolding(t *testing.T) {
	_, _, caster := hiddenHolderFixture(t)
	room := rooms.LoadRoom(1)
	spike := hidingSpikeSpell()
	spike.MobOnly = true
	spike.MinCastRounds = 2 // two held rounds after the start round
	spike.IgnoreMoveCooldown = true
	t.Cleanup(spells.SeedSpellsForTest(map[string]*spells.SpellData{"test-spike": spike}))
	caster.Character.ConvictionMax.Value, caster.Character.Conviction = 1000, 1000
	caster.Character.Cooldowns = nil

	rat := &mobs.Mob{MobId: 2, InstanceId: 101, Character: characters.Character{
		Name: "Rat", RoomId: room.RoomId, Health: 1000, Conditions: conditions.New()}}
	rat.Character.HealthMax.Value = 1000
	mobs.SetInstanceForTest(101, rat)
	t.Cleanup(func() { mobs.SetInstanceForTest(101, nil) })
	room.AddMob(101)
	t.Cleanup(func() { room.RemoveMob(101) })
	t.Cleanup(func() { targeting.Release(&caster.Character, targeting.ReasonDisengage) })

	sneakHide(t, &caster.Character)
	pinScheduleRoundForTest(t, 9000)
	_, err := mobcommands.Cast("test-spike rat", caster, room)
	require.NoError(t, err)
	require.True(t, caster.Character.IsCasting(), "fixture: the cast began")
	require.True(t, caster.Character.IsInCombat(), "fixture: the harmful cast committed the caster (#242)")
	require.True(t, caster.Character.IsHidden(), "starting a cast must not reveal the caster")

	handleMobCombat(events.NewRound{RoundNumber: 9000})
	util.SetRoundCountForTest(9001)
	handleMobCombat(events.NewRound{RoundNumber: 9001})
	require.True(t, caster.Character.IsCasting(), "fixture: still folding")
	require.True(t, caster.Character.IsHidden(), "a fold round must not reveal the caster")

	for round := uint64(9002); round <= 9006 && caster.Character.IsCasting(); round++ {
		util.SetRoundCountForTest(round)
		handleMobCombat(events.NewRound{RoundNumber: round})
	}
	require.False(t, caster.Character.IsCasting(), "fixture: the cast resolved")
	// Landed, it revealed the caster; fumbled, the caster is still in combat
	// and the next round's strip reveals it before it can swing.
	util.SetRoundCountForTest(9007)
	handleMobCombat(events.NewRound{RoundNumber: 9007})
	require.False(t, caster.Character.IsHidden(), "a caster whose cast is over is out of hiding")
}
