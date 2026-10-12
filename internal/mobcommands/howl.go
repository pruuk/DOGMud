package mobcommands

import (
	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// howlLegacy is what a howl sends when the taunt store is empty (unit tests
// that never load world data).
var howlLegacy = mobTauntLegacy{
	Fumble:       `<ansi fg="mobname">%s</ansi> lets out a pitiful howl that trails off weakly.`,
	HitPersonal:  `<ansi fg="mobname">%s</ansi>'s menacing howl shakes your resolve! (<ansi fg="damage">%s</ansi>)`,
	HitRoom:      `<ansi fg="mobname">%s</ansi> throws back its head and lets out a bone-chilling howl at <ansi fg="username">%s</ansi>!`,
	MissPersonal: `<ansi fg="mobname">%s</ansi> howls, but you steel yourself against the sound.`,
	MissRoom:     `<ansi fg="mobname">%s</ansi> howls menacingly at <ansi fg="username">%s</ansi>, but it has no effect.`,
}

// Howl is a wolf conviction attack, a taunt reskin for mob use. It always
// narrates from the wordless pool: a howl is a sound, whatever the species.
func Howl(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	if !mob.Character.IsInCombat() {
		return true, nil
	}

	actor := &actions.MobActor{Mob: mob, Room: room}
	result := executeTauntAction(actor)
	if result.Cost.Status == characters.CostRefused {
		return true, nil
	}

	if !result.Executed {
		return true, nil
	}

	return narrateMobTaunt(mob, room, result, combat.TauntPoolWordless, "howl", howlLegacy, howlAggroLine)
}
