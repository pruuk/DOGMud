package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/state/activity"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/assert"
)

// These replace TestApplyPlayerEffect_PurgeSelf, _HealSelf and _BuffSelf,
// which called applyPlayerEffect(u, u, ...) and asserted NOTHING, so they could
// never have caught that a self-caster was told about themselves twice and in
// the third person, or that the room read "Aliceia's Heal envelops Aliceia".
//
// A help spell with no target is a self-cast by default (actions/cast.go:227),
// and an area spell puts the caster in its own target list, so this is the
// common path, not an edge case.

func TestSelfCastPurge_OneLineToCaster_RoomNamesCasterOnce(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	u.Character.AddCondition(characters.ConditionPoisoned, 10, 5.0, "test")
	spell := &spells.SpellData{SpellId: "cleansing-wave", Name: "Cleansing Wave", EffectType: "purge"}
	applyPlayerEffect(u, u, room, spell, 10, spellContestAttackWin())

	caster, observer := drainPlain(1), drainPlain(2)
	assert.Equal(t, 1, countContaining(caster, "You purge the afflictions from your body."))
	assert.Equal(t, 0, countContaining(caster, "cleanses Aliceia"),
		"a self-caster must not be told about themselves in the third person")
	assert.Equal(t, 1, countContaining(observer, "Cleansing Wave cleanses Aliceia of afflictions."))
	assert.Equal(t, 0, countContaining(observer, "Aliceia's Cleansing Wave cleanses Aliceia"))
}

func TestSelfCastHeal_OneLineToCaster_RoomNamesCasterOnce(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "heal", Name: "Heal", EffectType: "heal", EffectMagnitude: 3}
	applyPlayerEffect(u, u, room, spell, 3, spellContestAttackWin())

	caster, observer := drainPlain(1), drainPlain(2)
	assert.Equal(t, 1, countContaining(caster, "A warm glow of healing magic envelops you."))
	assert.Equal(t, 0, countContaining(caster, "restorative magic around Aliceia"))
	assert.Equal(t, 1, countContaining(observer, "Aliceia channels restorative magic."))
	assert.Equal(t, 0, countContaining(observer, "envelops Aliceia in healing light"))
}

func TestSelfCastBuff_OneLineToCaster_RoomNamesCasterOnce(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "bless", Name: "Bless", EffectType: "buff", BuffIds: []int{100}}
	applyPlayerEffect(u, u, room, spell, 0, spellContestAttackWin())

	caster, observer := drainPlain(1), drainPlain(2)
	assert.Equal(t, 1, countContaining(caster, "Your Bless takes effect."))
	assert.Equal(t, 0, countContaining(caster, "takes effect on Aliceia"))
	// "Bless settles over Aliceia." is a substring of the old broken line, so
	// the second assertion is the one that proves the fix.
	assert.Equal(t, 1, countContaining(observer, "Bless settles over Aliceia."))
	assert.Equal(t, 0, countContaining(observer, "Aliceia's Bless settles over Aliceia"))
}

func TestSelfCastDefault_NamesNoOneInTheThirdPerson(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)

	spell := &spells.SpellData{SpellId: "curiosity", Name: "Curiosity", EffectType: "curiosity"}
	applyPlayerEffect(u, u, room, spell, 0, spellContestAttackWin())

	caster := drainPlain(1)
	assert.Equal(t, 1, countContaining(caster, "Your Curiosity takes effect."))
	assert.Equal(t, 0, countContaining(caster, "takes effect on Aliceia"))
}

// TestCrossCast_WordingUnchanged is a regression guard, not a red test: casting
// on someone else must keep today's lines exactly.
func TestCrossCast_WordingUnchanged(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	caster := users.GetByUserId(1)
	target := users.GetByUserId(2)
	room := rooms.LoadRoom(1)

	cases := []struct {
		spell      *spells.SpellData
		casterLine string
		targetLine string
	}{
		{&spells.SpellData{SpellId: "purge", Name: "Purge", EffectType: "purge"},
			"Your Purge cleanses Bobrick of afflictions.", "Aliceia's Purge purges the toxins from your body."},
		{&spells.SpellData{SpellId: "heal", Name: "Heal", EffectType: "heal", EffectMagnitude: 3},
			"You weave restorative magic around Bobrick.", "Aliceia's Heal envelops you in healing energy."},
		{&spells.SpellData{SpellId: "bless", Name: "Bless", EffectType: "buff", BuffIds: []int{100}},
			"Your Bless takes effect on Bobrick!", "Aliceia's Bless takes effect on you!"},
	}
	for _, c := range cases {
		drainPlain(1)
		drainPlain(2)
		applyPlayerEffect(caster, target, room, c.spell, 3, spellContestAttackWin())
		assert.Equal(t, 1, countContaining(drainPlain(1), c.casterLine), c.spell.Name)
		assert.Equal(t, 1, countContaining(drainPlain(2), c.targetLine), c.spell.Name)
	}
}

// TestAreaHeal_CasterIsTheirOwnTarget drives the real HelpArea fill, which puts
// every room player in the target list, caster included.
func TestAreaHeal_CasterIsTheirOwnTarget(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "mass-mend", Name: "Mass Mend", Type: spells.HelpArea,
		EffectType: "heal", EffectMagnitude: 3}
	resolveSpell(u, activity.CastingData{SpellId: "mass-mend"}, spell, room)

	caster, observer := drainPlain(1), drainPlain(2)
	assert.Equal(t, 1, countContaining(caster, "A warm glow of healing magic envelops you."))
	assert.Equal(t, 0, countContaining(caster, "restorative magic around Aliceia"))
	assert.Equal(t, 1, countContaining(caster, "restorative magic around Bobrick"))
	assert.Equal(t, 1, countContaining(observer, "Aliceia's Mass Mend envelops you in healing energy."))
	assert.Equal(t, 1, countContaining(observer, "Aliceia channels restorative magic."))
}

func TestAreaPurge_CasterIsTheirOwnTarget(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	u := users.GetByUserId(1)
	room := rooms.LoadRoom(1)
	drainPlain(1)
	drainPlain(2)

	spell := &spells.SpellData{SpellId: "cleansing-wave", Name: "Cleansing Wave", Type: spells.HelpArea,
		EffectType: "purge"}
	resolveSpell(u, activity.CastingData{SpellId: "cleansing-wave"}, spell, room)

	caster, observer := drainPlain(1), drainPlain(2)
	assert.Equal(t, 1, countContaining(caster, "You purge the afflictions from your body."))
	assert.Equal(t, 0, countContaining(caster, "cleanses Aliceia"))
	assert.Equal(t, 1, countContaining(caster, "Your Cleansing Wave cleanses Bobrick of afflictions."))
	assert.Equal(t, 1, countContaining(observer, "Cleansing Wave cleanses Aliceia of afflictions."))
	assert.Equal(t, 1, countContaining(observer, "purges the toxins from your body."))
}
