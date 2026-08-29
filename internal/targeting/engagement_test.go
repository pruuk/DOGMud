package targeting

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/combatphase"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
	assert.True(t, c.CombatPhase.OpeningUnspent())
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

// TestEngagementOf_SpellCastTargetsAreNotDropped closes the defect an
// adversarial review found in the first draft: characters.SetCast builds an
// Aggro whose UserId/MobInstanceId stay ZERO and whose real targets live in
// SpellInfo. Reading only Target reported "no target" for a caster that
// IsAggro reports as aggro'd.
func TestEngagementOf_SpellCastTargetsAreNotDropped(t *testing.T) {
	c := characters.New()
	c.SetCast(2, characters.SpellAggroInfo{
		SpellId:              "burst",
		TargetUserIds:        []int{7},
		TargetMobInstanceIds: []int{88},
	})

	e := EngagementOf(c)

	assert.True(t, e.Target.IsZero(),
		"SetCast leaves the plain ids zero; this documents that, it is not a wish")
	assert.Len(t, e.SpellTargets, 2)
	assert.True(t, e.IsAimedAt(state.ActorRef{UserId: 7}))
	assert.True(t, e.IsAimedAt(state.ActorRef{MobInstanceId: 88}))
}

// The seam must agree with the production predicate it is replacing.
func TestEngagementOf_IsAimedAtAgreesWithIsAggro(t *testing.T) {
	cases := []struct {
		name  string
		setup func(*characters.Character)
		ref   state.ActorRef
	}{
		{"melee mob target", func(c *characters.Character) {
			c.SetAggro(0, 88, characters.DefaultAttack)
		}, state.ActorRef{MobInstanceId: 88}},
		{"melee player target", func(c *characters.Character) {
			c.SetAggro(7, 0, characters.DefaultAttack)
		}, state.ActorRef{UserId: 7}},
		{"spellcast player target", func(c *characters.Character) {
			c.SetCast(2, characters.SpellAggroInfo{TargetUserIds: []int{7}})
		}, state.ActorRef{UserId: 7}},
		{"spellcast mob target", func(c *characters.Character) {
			c.SetCast(2, characters.SpellAggroInfo{TargetMobInstanceIds: []int{88}})
		}, state.ActorRef{MobInstanceId: 88}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := characters.New()
			tc.setup(c)

			assert.Equal(t,
				c.IsAggro(tc.ref.UserId, tc.ref.MobInstanceId),
				EngagementOf(c).IsAimedAt(tc.ref),
				"IsAimedAt must agree with characters.IsAggro")
		})
	}
}

func TestEngagementOf_IsAimedAtRejectsTheZeroRef(t *testing.T) {
	c := characters.New()
	c.SetAggro(0, 88, characters.DefaultAttack)

	assert.False(t, EngagementOf(c).IsAimedAt(state.ActorRef{}),
		"the zero ref means nobody and must never match")
}

// Ranged is a DERIVED field and was asserted by nothing in the first draft, so
// an inverted or mis-keyed derivation would have shipped green.
func TestEngagementOf_RangedTracksTheEquippedWeapon(t *testing.T) {
	c := characters.New()

	assert.False(t, EngagementOf(c).Ranged,
		"an empty weapon slot is not ranged")

	c.Equipment.Weapon = items.Item{
		ItemId: 7,
		Spec: &items.ItemSpec{
			ItemId: 7, Name: "sword", Type: items.Weapon,
			Subtype: items.Slashing, Hands: 1,
		},
	}
	assert.False(t, EngagementOf(c).Ranged, "a melee weapon is not ranged")

	c.Equipment.Weapon = items.Item{
		ItemId: 9,
		Spec: &items.ItemSpec{
			ItemId: 9, Name: "bow", Type: items.Weapon,
			Subtype: items.Shooting, Hands: 2,
		},
	}
	assert.True(t, EngagementOf(c).Ranged, "a Shooting-subtype weapon is ranged")
}

// Casting is the other DERIVED field the first draft never asserted true.
func TestEngagementOf_CastingTracksTheActivityMachine(t *testing.T) {
	c := characters.New()
	require.NotNil(t, c.Activity, "characters.New must build an activity machine")

	assert.False(t, EngagementOf(c).Casting, "a fresh character is not casting")

	c.SetCast(2, characters.SpellAggroInfo{SpellId: "burst"})

	assert.Equal(t, c.Activity.IsCasting(), EngagementOf(c).Casting,
		"Casting must track the activity machine, not the aggro type")
}
