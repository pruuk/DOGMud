package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPallConditionId is a magnitude darkness, the shape of the shipped
// condition 131 Chrysalis Pall.
const testPallConditionId = 9741

func seedPallCondition() func() {
	return conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		testPallConditionId: {ConditionId: testPallConditionId, Name: "Test Pall", RoundInterval: 5, TriggerCount: 4,
			StartRoomText: "A pall of dark spores gathers around {actee_plain}.",
			Effects:       map[conditions.EffectKind]conditions.EffectValue{conditions.EffectDarknessStrength: {UsesMagnitude: true}},
			Flags:         []conditions.Flag{conditions.Adjustable, conditions.Cancellable}},
	})
}

// Ruling D6 as amended: a darkness's start line is judged against the room
// before it landed. The record is already held when the line goes out, so
// judged by the room as it now is, the observers the darkness has just
// blinded would miss it.
func TestDarknessStartLineIsSeenBeforeTheDarkFalls(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedPallCondition()
	defer restore()
	room := rooms.LoadRoom(1)
	room.Lamp = rooms.LampPtr(50)
	drainPlain(2)

	require.Equal(t, events.Continue, ApplyConditions(events.Condition{
		UserId: 1, ConditionId: testPallConditionId, Magnitude: 90, Triggers: 4}))
	require.Less(t, room.LightLevel(), 0, "the pall must actually have darkened the room below anyone's sight")
	assert.Equal(t, 1, countContaining(drainPlain(2), "A pall of dark spores gathers around Aliceia."),
		"the observer the pall just blinded must still see it gather")
}

// The darkness spell scales from its own trios through the shared seam
// (spec Rule 4): a mid caster (130, 30) darkens by 68 for 6 triggers.
func TestDarknessSpellAppliesAtTheCastersScale(t *testing.T) {
	configs.SetConfigForTest(t, configs.GetConfig())
	t.Cleanup(seedPallCondition())
	spell := &spells.SpellData{SpellId: "test-pall", PrimaryStat: "willpower"}
	caster := characters.New()
	caster.Stats.Willpower.ValueAdj = 130
	caster.SetSkill("spellcasting", 30)
	mag, trig, ok := magnitudeSpellApplication(spell, caster, testPallConditionId)
	if !ok || mag != 68 || trig != 6 {
		t.Errorf("mid caster: (%v, %d, %v), want (68, 6, true)", mag, trig, ok)
	}
}

// Owner rule, 2026-10-05: the start line is judged against the room as it was
// BEFORE the darkness landed. An observer already blind in a pitch-dark room
// (no sky, no lamp) learns nothing of who cast the pall; judged as lit, the
// line named the caster to them.
func TestDarknessStartLineIsNotSeenInARoomAlreadyDark(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	restore := seedPallCondition()
	defer restore()
	room := rooms.LoadRoom(1)
	zero := 0.0
	room.SkyLight = &zero
	room.Lamp = nil
	require.Less(t, room.LightLevel(), configs.GetLightingConfig().BlindBelow, "fixture: the room must be dark before the pall")
	drainPlain(2)

	require.Equal(t, events.Continue, ApplyConditions(events.Condition{
		UserId: 1, ConditionId: testPallConditionId, Magnitude: 90, Triggers: 4}))
	require.True(t, users.GetByUserId(1).Character.HasCondition(testPallConditionId), "fixture: the pall must have landed")
	assert.Zero(t, countContaining(drainPlain(2), "Aliceia"),
		"an observer already blind in the dark was told who cast the pall")
}
