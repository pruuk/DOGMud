package combat

import (
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// DamageChannel is NOT one of the four authored axes. It is the damage
// pipeline's pool: which scale knob, which mitigation curve, which stat
// toughens on a defensive crit. Nothing declares it in data; these functions
// derive it from combatvocab (owner ruling 2026-09-18), replacing three
// switches that used to encode the same facts by hand.

// scaleChannelFor is the table behind ScaleChannelFor, with the ok the guard
// test wants.
func scaleChannelFor(at combatvocab.AttackType) (DamageChannel, bool) {
	switch at {
	case combatvocab.AttackMelee, combatvocab.AttackRanged, combatvocab.AttackThrown:
		return ChannelPhysical, true
	case combatvocab.AttackSpell:
		return ChannelMagical, true
	case combatvocab.AttackRhetoric:
		return ChannelConviction, true
	}
	return ChannelPhysical, false
}

// ScaleChannelFor returns the pool an attack SCALES on (CalcRawDamage,
// DamageScale) and the pool whose stat TOUGHENS on a defensive crit against
// it. Both are properties of how the attack is delivered, not of what it
// does: a physical spell is dodged but is still cast off willpower, so it
// scales magically and toughens willpower. (The old flattened-channel
// mapping warned that mapping spell-physical to "physical" would toughen the
// wrong stat; this keeps that mapping.)
//
// AttackNone never reaches the pipeline. It answers Physical here only so a
// caller that does reach it cannot divide by a zero scale; the error log is
// the tell.
func ScaleChannelFor(at combatvocab.AttackType) DamageChannel {
	ch, ok := scaleChannelFor(at)
	if !ok {
		mudlog.Error("ScaleChannelFor", "attack_type", string(at), "error", "no scale pool; a non-harm attack reached the damage pipeline")
	}
	return ch
}

// MitigationChannelFor returns the pool that MITIGATES a damage type: the
// defender's physical mitigation against physical harm, magical against
// mental, conviction against social. non_harm has none and returns false;
// a non-harm cast takes the uncontested path before any damage helper runs.
func MitigationChannelFor(dt combatvocab.DamageType) (DamageChannel, bool) {
	switch dt {
	case combatvocab.DamagePhysical:
		return ChannelPhysical, true
	case combatvocab.DamageMental:
		return ChannelMagical, true
	case combatvocab.DamageSocial:
		return ChannelConviction, true
	}
	return ChannelPhysical, false
}

// ToughenChannelFor returns the pool whose stat toughens on a DEFENSIVE crit
// against this attack. It is the scale pool, except that a social attack
// toughens the defender's conviction whoever delivers it: on master the
// social channel (taunt AND charm) toughened charisma, and this keeps that.
// Distinct from ScaleChannelFor because charm scales magically (it is a
// spell) but is defied socially.
func ToughenChannelFor(shape combatvocab.Attack) DamageChannel {
	if shape.Damage == combatvocab.DamageSocial {
		return ChannelConviction
	}
	return ScaleChannelFor(shape.Type)
}

// ToughenName is the string characters.ToughenStatFor expects. characters
// cannot import combat, so the string crosses that boundary; this is the one
// place it is spelt on this side.
func (c DamageChannel) ToughenName() string {
	switch c {
	case ChannelPhysical:
		return "physical"
	case ChannelMagical:
		return "magical"
	case ChannelConviction:
		return "conviction"
	}
	return ""
}
