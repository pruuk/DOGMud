package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// RetargetNotice builds the "You turn your attention to X!" line for a
// player who was just given a new target, with X hidden by that player's
// sight in room: "something" for a reader who cannot see, "a figure" for
// infrared. ok is false when the target no longer resolves, in which case
// say nothing.
//
// The notice is not suppressed in the dark. Each caller picks the new target
// from whoever is already attacking the reader (or one of their
// companions/allies), and melee in the dark still swings, so the honest line
// is that their attention turned to something.
//
// Three call sites, four calls (the mob-departure retarget has a
// player-target and a companion-target branch) share this builder: the round
// driver's two retarget points (hooks.emitRetargetMessage and the
// NewRound_DoCombat.go validate-aggro block) and the mob-departure retarget
// in mobcommands.clearRoomAggroOnDeparture.
func RetargetNotice(room *rooms.Room, userId int, target state.ActorRef) (string, bool) {
	var name, line string
	if mob := mobs.GetInstance(target.MobInstanceId); target.MobInstanceId > 0 && mob != nil {
		name = mob.Character.Name
		line = fmt.Sprintf(`You turn your attention to <ansi fg="mobname">%s</ansi>!`, name)
	} else if u := users.GetByUserId(target.UserId); target.UserId > 0 && u != nil {
		name = u.Character.Name
		line = fmt.Sprintf(`You turn your attention to <ansi fg="username">%s</ansi>!`, name)
	} else {
		return "", false
	}
	if room == nil {
		return line, true
	}
	return messaging.HideNames(line, []string{name}, room.ParticipantSight(userId)), true
}
