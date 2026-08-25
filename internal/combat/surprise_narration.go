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
// crits on a won contest and pays the skullduggery-scaled bonus. The wording is
// unchanged on purpose: "surprise attack" is the term the hints, the
// skullduggery keyword list and the help files already teach.
//
// WHERE IT IS APPLIED, and why it is not applied everywhere: the banner exists
// because the swing it marks is narrated by the GENERIC weapon message pool
// ("You slash the bandit!"), which says nothing about an ambush. Wherever the
// prose already names the opening blow -- openingStrikeDefendedLines below, and
// the shoot wrapper's from-cover lines -- the banner is redundant, and it is
// not free: it costs 20 rendered columns. An 80-column line cannot carry a
// 20-column marker, two names AND a damage description, so on those lines the
// marker loses and the meaning stays. Colour is not a substitute (the playtest
// harness strips it), which is why the landed opener keeps a TEXT marker.
const surpriseAttackBanner = `<ansi fg="magenta-bold">*[SURPRISE ATTACK]*</ansi> `

// openingStrikeDefendedLines narrates an opening strike the defender ANSWERED,
// naming the defence that won it.
//
// New in U10d: before the redesign the ambush could not be defended at all, so
// there was no such outcome to narrate. Now the opener contests normally, and a
// defended one that says nothing about the defence reads as the feature simply
// not firing.
//
// SCOPE. This covers the DEFLECTION path only, because that is the only path
// that sets hitResolution.defended (one assignment site; the field's own
// docstring records that it is false on every clean-win, fumble and
// defensive-crit path). A defensive crit still narrates through
// sendDefenseMessages, which keeps its personal lines on that path and already
// names the defence -- so the "answered ambush" outcome is spoken there too,
// just by the older seam.
//
// Only dodge, parry and block are worded. DefenceSetFor(ChannelMelee) returns
// exactly those three, so a quell or defy arm here would be unreachable.
// DefenseNone keeps a neutral fallback so an empty DefenseUsed cannot print an
// empty verb; "deflect" rather than deflectedSwingLines' "turn aside" because
// it is four columns cheaper and these lines are width-critical.
//
// dmgDesc is the GetDamageDescription band for the damage that got through, or
// "" when the deflection let nothing through at all. No raw numbers reach any
// line on either path.
func openingStrikeDefendedLines(defense DefenseType, sourceName, targetName, dmgDesc string) (toAttacker, toDefender, toRoom items.ItemMessage) {

	verbYou, verbThey := "deflect", "deflects"
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
			`%s %s your opening blow. You had one chance.`, targetName, verbThey))
		toDefender = items.ItemMessage(fmt.Sprintf(
			`You %s %s's opening blow before it lands!`, verbYou, sourceName))
		toRoom = items.ItemMessage(fmt.Sprintf(
			`%s %s %s's opening blow!`, targetName, verbThey, sourceName))
		return toAttacker, toDefender, toRoom
	}

	dmg := `(<ansi fg="damage">` + dmgDesc + `</ansi>)`
	toAttacker = items.ItemMessage(fmt.Sprintf(
		`%s %s most of your opening blow! %s`, targetName, verbThey, dmg))
	toDefender = items.ItemMessage(fmt.Sprintf(
		`You %s most of %s's opening blow! %s`, verbYou, sourceName, dmg))
	// Room lines carry no damage description, matching every other room line in
	// this package.
	toRoom = items.ItemMessage(fmt.Sprintf(
		`%s %s most of %s's opening blow!`, targetName, verbThey, sourceName))
	return toAttacker, toDefender, toRoom
}
