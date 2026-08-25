package combat

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/items"
)

// surpriseAttackBanner marks the ONE swing of an ambush round that carried the
// opening strike.
//
// It used to be computed once per round in calculateCombat and handed to every
// swing, which meant a four-swing ambush printed four identical banners and the
// player had no way to tell which line was the opener -- the only swing that
// crits on a won contest and pays the skullduggery-scaled bonus. The banner
// wording is unchanged on purpose: "surprise attack" is the term the hints, the
// skullduggery keyword list and the help files already teach.
const surpriseAttackBanner = `<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `

// openingStrikeDefendedLines narrates an opening strike the defender ANSWERED,
// naming the defence that won it.
//
// New in U10d: before the redesign the ambush could not be defended at all, so
// there was no such outcome to narrate. Now the opener contests normally, and a
// defended one that says nothing about the defence reads as the feature simply
// not firing.
//
// Only dodge, parry and block are worded. DefenceSetFor(ChannelMelee) returns
// exactly those three, so a quell or defy arm here would be an unreachable
// branch; DefenseNone keeps the same neutral "turns aside" fallback
// deflectedSwingLines uses, so an empty DefenseUsed cannot print an empty verb.
//
// dmgDesc is the GetDamageDescription band for the damage that got through, or
// "" when the defence stopped the swing outright. No raw numbers reach either
// line on either path.
func openingStrikeDefendedLines(defense DefenseType, sourceName, targetName, dmgDesc string) (toAttacker, toDefender items.ItemMessage) {

	verbYou, verbThey := "turn aside", "turns aside"
	switch defense {
	case DefenseDodge:
		verbYou, verbThey = "dodge", "dodges"
	case DefenseParry:
		verbYou, verbThey = "parry", "parries"
	case DefenseBlock:
		verbYou, verbThey = "block", "blocks"
	}

	if dmgDesc == "" {
		toAttacker = items.ItemMessage(fmt.Sprintf(
			`%s %s your surprise attack. Your one clean chance is gone.`, targetName, verbThey))
		toDefender = items.ItemMessage(fmt.Sprintf(
			`You %s %s's surprise attack before it can land!`, verbYou, sourceName))
		return toAttacker, toDefender
	}

	dmg := `(<ansi fg="damage">` + dmgDesc + `</ansi>)`
	toAttacker = items.ItemMessage(fmt.Sprintf(
		`%s %s your surprise attack, but part of it lands! %s`, targetName, verbThey, dmg))
	toDefender = items.ItemMessage(fmt.Sprintf(
		`You %s %s's surprise attack, but part of it lands! %s`, verbYou, sourceName, dmg))
	return toAttacker, toDefender
}
