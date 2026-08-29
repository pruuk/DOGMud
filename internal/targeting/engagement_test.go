package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/assert"
)

func TestEngagementOf_IdleCharacterIsZero(t *testing.T) {
	c := characters.New()

	e := EngagementOf(c)

	assert.Equal(t, combatphase.Idle, e.Phase)
	assert.True(t, e.Target.IsZero())
	assert.False(t, e.OpeningUnspent)
	assert.False(t, e.Casting)
}

func TestEngagementOf_ReportsTargetAfterAggro(t *testing.T) {
	c := characters.New()
	c.SetAggro(0, 77, characters.DefaultAttack)

	e := EngagementOf(c)

	assert.Equal(t, 77, e.Target.MobInstanceId)
	assert.Equal(t, 0, e.Target.UserId)
}

func TestEngagementOf_OpeningUnspentTracksSurpriseAttack(t *testing.T) {
	c := characters.New()
	c.SetAggro(0, 77, characters.SurpriseAttack)

	assert.True(t, EngagementOf(c).OpeningUnspent)

	c.SetAggro(0, 77, characters.DefaultAttack)

	assert.False(t, EngagementOf(c).OpeningUnspent)
}

// TestEngagementOf_IsPure is the guard for the design's central rule: today
// the read IS the write (calculateCombat reads Aggro.Type and demotes it in
// the same breath). If EngagementOf ever inherits that, every caller asking
// "is this an ambush?" silently spends the ambush.
func TestEngagementOf_IsPure(t *testing.T) {
	c := characters.New()
	c.SetAggro(0, 77, characters.SurpriseAttack)

	for i := 0; i < 5; i++ {
		assert.True(t, EngagementOf(c).OpeningUnspent,
			"EngagementOf must not consume the opening strike (call %d)", i+1)
	}
	assert.Equal(t, characters.SurpriseAttack, c.Aggro.Type)
}

func TestEngagementOf_NilCharacterIsZero(t *testing.T) {
	e := EngagementOf(nil)

	assert.Equal(t, combatphase.Idle, e.Phase)
	assert.True(t, e.Target.IsZero())
}

func TestConsumeOpeningStrike_SpendsExactlyOnce(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 9}, ReasonSurprise)

	assert.True(t, ConsumeOpeningStrike(c), "first call spends the opening")
	assert.False(t, ConsumeOpeningStrike(c), "second call must find it spent")
	assert.False(t, EngagementOf(c).OpeningUnspent)
}

func TestConsumeOpeningStrike_KeepsTheTarget(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 9}, ReasonSurprise)

	ConsumeOpeningStrike(c)

	assert.Equal(t, 9, EngagementOf(c).Target.MobInstanceId,
		"spending the opening must not end the engagement")
}

func TestConsumeOpeningStrike_FalseWhenNothingArmed(t *testing.T) {
	c := characters.New()
	Commit(c, state.ActorRef{MobInstanceId: 9}, ReasonAttack)

	assert.False(t, ConsumeOpeningStrike(c))
	assert.False(t, ConsumeOpeningStrike(nil))
}
