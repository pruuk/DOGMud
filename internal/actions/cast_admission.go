package actions

import (
	"github.com/GoMudEngine/GoMud/internal/combatvocab"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// castAim is what a player caster's sight lets a targeted cast aim at.
type castAim struct {
	// targetName is the name to resolve. A shape is rewritten to "@<userId>"
	// or "#<mobInstanceId>", forms FindByName already resolves.
	targetName string
	// ownFoeOnly is set when the caster makes out shapes only: a no-name
	// harmful cast may aim at the caster's own foe, not the party leader's.
	ownFoeOnly bool
}

// admitCastAim applies follow-up slice A's sight rules to a player's targeted
// cast (single-target harm or help, or multi-target harm). It returns refused=true after
// telling the caster why; nothing has been spent at that point.
//
//	clear sight:  names, your foe then your party leader's foe, shapes
//	shapes only:  your own foe, shapes; a typed name gets a hint
//	no sight:     every targeted cast refused
//
// The sight rule itself is AimBySight's (sight_aim.go, #454), shared with
// every other command that names a creature; this wrapper adds what is the
// cast's own: which spells it covers, the self-cast exemption, and what no
// name means at shapes.
//
// A self-cast (a help spell with no name, or the caster's own name) needs no
// sight. Area, help-multi and neutral casts are not affected.
func admitCastAim(actor Actor, spellInfo *spells.SpellData, targetName string) (castAim, bool) {
	aim := castAim{targetName: targetName}
	switch {
	case spellInfo.Targeting == combatvocab.TargetSingle:
	case spellInfo.IsHarm() && spellInfo.Targeting == combatvocab.TargetMulti:
	default:
		return aim, false
	}
	room := actor.GetRoom()
	if room == nil {
		return aim, false
	}
	if !spellInfo.IsHarm() && spellInfo.Targeting == combatvocab.TargetSingle && castsAtSelf(actor, targetName) {
		return aim, false
	}

	char := actor.GetCharacter()
	sight := messaging.ParticipantSight(char, room)
	name, refusal := aimAtSight(char, actor.GetUserId(), room, sight, targetName, `cast `+spellInfo.SpellId)
	if refusal != `` {
		actor.SendText(messaging.CategorySystem, refusal)
		return aim, true
	}
	aim.targetName = name
	aim.ownFoeOnly = sight == messaging.SightShapes && targetName == ``
	return aim, false
}

// castsAtSelf reports whether a help spell with this target name is aimed at
// the caster. Ruling 4: a self-cast needs no sight, so `cast heal caster` and
// `cast heal cast` must work in the dark, not just the exact spelling of the
// name. It matches the way the rest of the game matches names. Only the
// caster.s own name is a candidate, so a close match here means the typed noun
// matched nothing but themselves.
func castsAtSelf(actor Actor, targetName string) bool {
	if targetName == `` {
		return true
	}
	match, closeMatch := util.FindMatchIn(targetName, actor.GetName())
	return match != `` || closeMatch != ``
}
