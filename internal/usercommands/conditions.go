package usercommands

import (
	"slices"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// Conditions lists everything currently affecting the player: one entry per
// held, unexpired, non-hidden buff record, with its visible name, its visible
// description and a duration.
//
// There is one loop because there is one source. This command used to print
// the buff list and then a second list of combat conditions from an enum with
// its own tick, which is why warcry and rally needed a mirror flag to keep
// them out of the first list. The enum is gone; records are the conditions.
func Conditions(rest string, user *users.UserRecord, room *rooms.Room, flags events.EventFlag) (bool, error) {

	type buffInfo struct {
		Name        string
		Description string
		RoundsLeft  int
		PermaBuff   bool
	}

	afflictions := []buffInfo{}

	charBuffs := user.Character.GetBuffs()
	for _, buff := range charBuffs {

		spec := buffs.GetBuffSpec(buff.BuffId)
		if spec == nil {
			continue
		}

		// Buffs that hide you (Empathic Shroud, the Hidden stealth state, etc.)
		// are deliberately NOT shown: if you can't know who spotted you, you
		// shouldn't be told you're hidden in the first place. Mirrors the
		// no-end-message design on the hidden buff itself.
		if slices.Contains(spec.Flags, buffs.Hidden) {
			continue
		}

		roundsLeft, _ := buffs.GetDurations(buff, spec)

		// VisibleNameDesc, not spec.Name/spec.Description: a secret record
		// shows the player the cover story it was authored with.
		name, desc := spec.VisibleNameDesc()

		afflictions = append(afflictions, buffInfo{
			Name:        name,
			Description: desc,
			RoundsLeft:  roundsLeft,
			PermaBuff:   buff.PermaBuff,
		})
	}

	tplTxt, _ := templates.Process("character/conditions", afflictions, user.UserId)
	user.SendText(messaging.CategorySystem, tplTxt)

	return true, nil
}
