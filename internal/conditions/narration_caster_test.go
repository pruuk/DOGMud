package conditions

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func wardSpecForNarration() *ConditionSpec {
	return &ConditionSpec{
		ConditionId:    502,
		Name:           "Test Ward",
		StartActorText: "A ward settles over {actee}.",
		StartUserText:  "A ward settles over you.",
		StartRoomText:  "A ward settles over {actee_plain}.",
	}
}

// start_actor is the caster's line (messaging M6 slice 1, section 2): the
// Actor variant of the start phase. No other phase has a caster line.
func TestNarration_StartActorIsTheCasterLine(t *testing.T) {
	s := wardSpecForNarration()
	v := s.Narration(PhaseStart)
	require.Len(t, v.Actor, 1)
	assert.Equal(t, "A ward settles over {actee}.", v.Actor[0])
	assert.Empty(t, s.Narration(PhaseEnd).Actor)
	assert.Empty(t, s.Narration(PhaseTrigger).Actor)
}

// The caster line obeys the same silences as the holder line: a secret,
// quiet or silent-start record says nothing at start.
func TestNarration_StartActorFollowsTheNoticeSilences(t *testing.T) {
	for _, mut := range []func(*ConditionSpec){
		func(s *ConditionSpec) { s.Secret = true },
		func(s *ConditionSpec) { s.Flags = []Flag{Quiet} },
		func(s *ConditionSpec) { s.Flags = []Flag{SilentStart} },
	} {
		s := wardSpecForNarration()
		mut(s)
		assert.Empty(t, s.StartActorNotice())
		assert.Empty(t, s.Narration(PhaseStart).Actor)
	}
}

// NarrateCast fills {actor} with the caster and {actee} with the holder in
// every start line; Narrate is NarrateCast with no caster.
func TestNarrateCast_FillsTheCasterAndTheHolder(t *testing.T) {
	s := wardSpecForNarration()
	s.StartUserText = "{actor} wraps a ward around you."
	roles := s.NarrateCast(PhaseStart, "Bob", "Bob", "Alice", "Alice")
	assert.Equal(t, "A ward settles over Bob.", roles.Actor)
	assert.Equal(t, "Alice wraps a ward around you.", roles.Actee)
	assert.Equal(t, "A ward settles over Bob.", roles.Observer)

	plain := wardSpecForNarration().Narrate(PhaseStart, "Bob", "Bob")
	assert.Equal(t, "A ward settles over you.", plain.Actee)
}

// NarratesCastStart is the rule that drops a spell's generic "takes effect"
// trio (R11): only when the condition's own start lines will reach each
// audience the trio served.
func TestNarratesCastStart(t *testing.T) {
	s := wardSpecForNarration()
	assert.True(t, s.NarratesCastStart(true), "self-cast: the holder line is the caster's line")
	assert.True(t, s.NarratesCastStart(false))

	noActor := wardSpecForNarration()
	noActor.StartActorText = ""
	assert.True(t, noActor.NarratesCastStart(true), "a self-cast needs no caster line")
	assert.False(t, noActor.NarratesCastStart(false), "a cast on another with no caster line keeps the trio")

	silent := wardSpecForNarration()
	silent.Flags = []Flag{SilentStart}
	assert.False(t, silent.NarratesCastStart(true))
	assert.False(t, silent.NarratesCastStart(false))

	noRoom := wardSpecForNarration()
	noRoom.StartRoomText = ""
	assert.False(t, noRoom.NarratesCastStart(true), "the room would lose its line")

	generic := wardSpecForNarration()
	generic.StartUserText = ""
	assert.False(t, generic.NarratesCastStart(true), "the generic takes-effect fallback is not authored text")
}

// {actor} names the caster, who is known only at start. Trigger and end lines
// are narrated with no caster, so the load refuses {actor} there rather than
// render an empty name.
func TestValidate_RefusesTheActorTokenOutsideTheStartPhase(t *testing.T) {
	for _, mut := range []func(*ConditionSpec){
		func(s *ConditionSpec) { s.TriggerUserText = "{actor}'s ward hums." },
		func(s *ConditionSpec) { s.TriggerRoomText = "{actor_plain}'s ward hums." },
		func(s *ConditionSpec) { s.EndUserText = "{actor}'s ward fades." },
		func(s *ConditionSpec) { s.EndRoomText = "{actor}'s ward fades." },
	} {
		s := wardSpecForNarration()
		mut(s)
		err := s.Validate()
		require.Error(t, err)
		assert.Contains(t, err.Error(), "{actor}")
	}
	assert.NoError(t, wardSpecForNarration().Validate())
}

// start_actor is checked like every other line: an unknown token fails the
// load.
func TestValidate_ChecksStartActorTokens(t *testing.T) {
	s := wardSpecForNarration()
	s.StartActorText = "A ward settles over {target}."
	assert.Error(t, s.Validate())
}
