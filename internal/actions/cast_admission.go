package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// shapeWord is what a player who makes out shapes aims at: `shape`, `2.shape`,
// `shape#2`. No dogmud mob, item, noun or alias uses the word.
const shapeWord = "shape"

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
// cast (harmsingle, harmmulti, helpsingle). It returns refused=true after
// telling the caster why; nothing has been spent at that point.
//
//	clear sight:  names, your foe then your party leader's foe, shapes
//	shapes only:  your own foe, shapes; a typed name gets a hint
//	no sight:     every targeted cast refused
//
// A self-cast (a help spell with no name, or the caster's own name) needs no
// sight. Mob casters, area, help-multi and neutral casts are not affected.
func admitCastAim(actor Actor, spellInfo *spells.SpellData, targetName string) (castAim, bool) {
	aim := castAim{targetName: targetName}
	switch spellInfo.Type {
	case spells.HarmSingle, spells.HarmMulti, spells.HelpSingle:
	default:
		return aim, false
	}
	room := actor.GetRoom()
	if !actor.IsPlayer() || room == nil {
		return aim, false
	}
	if spellInfo.Type == spells.HelpSingle && (targetName == `` || targetName == actor.GetName()) {
		return aim, false
	}

	char := actor.GetCharacter()
	shape := castShapeIndex(targetName)

	switch messaging.ParticipantSight(char, room) {
	case messaging.SightNone:
		if targetName == `` || shape > 0 {
			actor.SendText(messaging.CategorySystem, `You can't see anything to aim at.`)
		} else {
			actor.SendText(messaging.CategorySystem, `You don't see them here.`)
		}
		return aim, true
	case messaging.SightShapes:
		if targetName == `` {
			aim.ownFoeOnly = true
			return aim, false
		}
		if shape == 0 {
			actor.SendText(messaging.CategorySystem, fmt.Sprintf(
				`You can only make out shapes here. Try <ansi fg="command">cast %s shape</ansi> or <ansi fg="command">cast %s 2.shape</ansi>.`,
				spellInfo.SpellId, spellInfo.SpellId))
			return aim, true
		}
	}

	if shape > 0 {
		figures := castFigures(char, actor.GetUserId(), room)
		if shape > len(figures) {
			actor.SendText(messaging.CategorySystem, `You don't see them here.`)
			return aim, true
		}
		aim.targetName = figures[shape-1]
	}
	return aim, false
}

// castShapeIndex is N for `shape`, `N.shape` or `shape#N`, and 0 otherwise.
func castShapeIndex(targetName string) int {
	if targetName == `` {
		return 0
	}
	word, n := util.GetMatchNumber(targetName)
	if word != shapeWord || n < 1 {
		return 0
	}
	return n
}

// castFigures lists what the caster makes out as shapes: the other players
// they perceive, in room order, then the mobs they perceive, in room order.
// Each is a name FindByName resolves: "@<userId>" or "#<mobInstanceId>".
func castFigures(viewer *characters.Character, selfUserId int, room *rooms.Room) []string {
	figures := []string{}
	for _, uid := range room.GetPlayers() {
		if uid == selfUserId {
			continue
		}
		if u := users.GetByUserId(uid); u != nil && viewer.Perceives(u.Character) {
			figures = append(figures, fmt.Sprintf(`@%d`, uid))
		}
	}
	for _, mid := range room.GetMobs() {
		if m := mobs.GetInstance(mid); m != nil && viewer.Perceives(&m.Character) {
			figures = append(figures, fmt.Sprintf(`#%d`, mid))
		}
	}
	return figures
}
