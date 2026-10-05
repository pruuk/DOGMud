package hooks

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Gear wear in a fight (wilderness trades). Melee rounds call gearWearOnCrit
// from dispatchCritAndMessaging; skill moves and shots reach it through
// combat.OnCritLanded. Only players' gear wears: mobs keep theirs as
// authored.

func init() {
	combat.OnCritLanded = gearWearOnCrit
}

// roundLandedCrit reports whether any swing this round was a critical hit
// that landed. AttackResult.Crit is reset per swing, so the per-weapon
// record is the one to read.
func roundLandedCrit(res *combat.AttackResult) bool {
	if res == nil || !res.Hit {
		return false
	}
	if res.Crit {
		return true
	}
	for _, wh := range res.WeaponHits {
		if wh.Crit && wh.Hit {
			return true
		}
	}
	return false
}

// gearWearOnMeleeCrit wears, for a melee round that landed a critical hit,
// each weapon whose swings landed one (not the other hand, not a bare fist)
// and one piece of the defender's armour.
func gearWearOnMeleeCrit(attacker, defender *characters.Character, res *combat.AttackResult) {
	if !roundLandedCrit(res) {
		return
	}
	if attacker != nil && attacker.GetUserId() != 0 {
		for _, wh := range res.WeaponHits {
			if !wh.Crit || !wh.Hit || wh.Weapon.ItemId < 1 {
				continue
			}
			if name, broke := attacker.CritWearStriker(attacker.EquippedItemPtr(wh.Weapon)); broke {
				tellGearBroke(attacker.GetUserId(), name)
			}
		}
	}
	wearDefenderArmour(defender)
}

// gearWearOnCrit wears the item a skill move struck with (nil for a kick or
// a bite: nothing of the attacker's wears) and the defender's armour, each on
// its own chance, and tells a player whose gear broke.
func gearWearOnCrit(attacker, defender *characters.Character, strikeWith *items.Item) {
	if attacker != nil && attacker.GetUserId() != 0 {
		if name, broke := attacker.CritWearStriker(strikeWith); broke {
			tellGearBroke(attacker.GetUserId(), name)
		}
	}
	wearDefenderArmour(defender)
}

// wearDefenderArmour rolls a player defender's armour wear for a critical
// hit taken.
func wearDefenderArmour(defender *characters.Character) {
	if defender != nil && defender.GetUserId() != 0 {
		if name, broke := defender.CritWearArmor(); broke {
			tellGearBroke(defender.GetUserId(), name)
		}
	}
}

// tellGearBroke tells a player their gear has broken.
func tellGearBroke(userId int, name string) {
	u := users.GetByUserId(userId)
	if u == nil {
		return
	}
	u.SendText(messaging.CategoryWarning, fmt.Sprintf(
		`<ansi fg="red">Your <ansi fg="itemname">%s</ansi> is broken!</ansi> It will serve you poorly until it is repaired. (<ansi fg="command">help repair</ansi>)`, name))
}
