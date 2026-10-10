package conditions

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yamlv2 "gopkg.in/yaml.v2"
	yamlv3 "gopkg.in/yaml.v3"
)

const casterTestConditionId = 9311

func seedCasterTestSpec(t *testing.T) {
	t.Helper()
	restore := SeedConditionsForTest(map[int]*ConditionSpec{
		casterTestConditionId: {ConditionId: casterTestConditionId, Name: "Caster Test", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
	})
	t.Cleanup(restore)
}

// Stamp writes the newest applier onto the held record: a re-application
// by someone else takes the record (spec section 1, "the newest application
// owns the record"), and a zero caster (a potion re-applying it) clears it.
func TestStamp_NewestApplicationOwnsTheRecord(t *testing.T) {
	seedCasterTestSpec(t)
	bs := New()
	require.True(t, bs.AddCondition(casterTestConditionId, false))

	bs.Stamp(casterTestConditionId, "spell", state.ActorRef{UserId: 7})
	got := bs.GetConditions(casterTestConditionId)
	require.Len(t, got, 1)
	assert.Equal(t, state.ActorRef{UserId: 7}, got[0].Caster)
	assert.Equal(t, "spell", got[0].Source)

	require.True(t, bs.AddCondition(casterTestConditionId, false))
	bs.Stamp(casterTestConditionId, "spell", state.ActorRef{MobInstanceId: 42})
	assert.Equal(t, state.ActorRef{MobInstanceId: 42}, bs.GetConditions(casterTestConditionId)[0].Caster)

	bs.Stamp(casterTestConditionId, "potion", state.ActorRef{})
	assert.True(t, bs.GetConditions(casterTestConditionId)[0].Caster.IsZero())
	assert.Equal(t, "potion", bs.GetConditions(casterTestConditionId)[0].Source)
}

// Stamp on a record that is not held does nothing and does not panic.
func TestStamp_UnheldIsANoOp(t *testing.T) {
	seedCasterTestSpec(t)
	bs := New()
	bs.Stamp(casterTestConditionId, "spell", state.ActorRef{UserId: 7})
	assert.Empty(t, bs.List)
}

// A player caster survives a save (user saves and copyover files are
// yaml.v2); a mob caster does not, because Mob.InstanceId is runtime only and
// would name a different creature after a restart (spec section 1,
// "Persistence"). A record with no caster writes no caster key at all.
func TestCaster_PlayerSurvivesASaveAndAMobDoesNot(t *testing.T) {
	list := []*Condition{
		{ConditionId: 1, TriggersLeft: 3, Caster: state.ActorRef{UserId: 7}},
		{ConditionId: 2, TriggersLeft: 3, Caster: state.ActorRef{MobInstanceId: 42}},
		{ConditionId: 3, TriggersLeft: 3},
	}

	for name, rt := range map[string]struct {
		marshal   func(any) ([]byte, error)
		unmarshal func([]byte, any) error
	}{
		"yaml.v2": {yamlv2.Marshal, yamlv2.Unmarshal},
		"yaml.v3": {yamlv3.Marshal, yamlv3.Unmarshal},
	} {
		t.Run(name, func(t *testing.T) {
			out, err := rt.marshal(list)
			require.NoError(t, err)
			assert.NotContains(t, string(out), "42", "a mob caster's instance id is never written")
			assert.Equal(t, 1, strings.Count(string(out), "caster:"),
				"only the player caster writes a caster key; a stripped mob caster and no caster write none")

			var back []*Condition
			require.NoError(t, rt.unmarshal(out, &back))
			require.Len(t, back, 3)
			assert.Equal(t, state.ActorRef{UserId: 7}, back[0].Caster)
			assert.True(t, back[1].Caster.IsZero(), "a mob caster reads as no caster after a load")
			assert.True(t, back[2].Caster.IsZero())
			assert.Equal(t, 3, back[1].TriggersLeft, "the rest of the record round-trips")
		})
	}
}

// A save written before a mob caster was stripped (a hand edit, an old
// copyover file) still loads with no caster.
func TestCaster_LoadDropsAMobCasterFromAnOldSave(t *testing.T) {
	src := "conditionid: 2\ntriggersleft: 3\ncaster:\n  userid: 0\n  mobinstanceid: 42\n"
	var c Condition
	require.NoError(t, yamlv2.Unmarshal([]byte(src), &c))
	assert.True(t, c.Caster.IsZero())
	assert.Equal(t, 2, c.ConditionId)
}
