package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// #242, owner ruling R4, closing playtest 2026-10-08: a boss-interrupt spell
// that collapses a mob's cast is a disruption, so a reader who sees nothing
// hears it, as every other disruption is heard. The room line ("X's spell
// collapses!") travelled on the trio's visual observer line only.
func TestSpellInterrupt_IsHeardByAReaderWhoSeesNothing(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	t.Cleanup(seedNarrationConditions())
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(90)
	m := mobs.GetInstance(100)
	require.NotNil(t, m)
	m.Character.Validate()
	saved := m.Character.Activity
	t.Cleanup(func() { m.Character.Activity = saved })
	setMobCastingForSpellTest(m, "core-discharge")

	bob := users.GetByUserId(2)
	bob.Character.Perception = perception.NewMachine()
	require.NoError(t, bob.Character.Perception.TransitionTo(perception.Blinded, state.TransitionReason{Trigger: "test"}))
	require.Equal(t, messaging.SightNone, messaging.ParticipantSight(bob.Character, room))
	events.DrainQueuedMessagesForTest(1)
	events.DrainQueuedMessagesForTest(2)

	caster := users.GetByUserId(1)
	spell := &spells.SpellData{SpellId: "neural-stun", Name: "Neural Stun", EffectType: "damage"}
	interruptSpellTarget(newSpellEffectCtx(caster.Character, actions.NewUserActorInRoom(caster, room),
		actions.NewMobActorInRoom(m, room), room, spell, 10, spellContestAttackWin()))

	require.NotZero(t, countContaining(drainPlain(1), "scrambles"), "the caster reads its own line")
	got := drainPlain(2)
	require.Equal(t, 1, countContaining(got, messaging.SoundChantBreaksOff), "%v", got)
	require.Zero(t, countContaining(got, "Skeleton"), "%v", got)
	require.Zero(t, countContaining(got, "collapses"), "the seen line is for readers who see: %v", got)
}
