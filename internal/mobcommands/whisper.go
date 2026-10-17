package mobcommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Whisper speaks to one player alone: `whisper <target> <text>`, where the
// target is a player in the room, usually named by id ("@<userId>"). The line
// is framed in asterisks the way a player's whisper is, so it stands out from
// a wall of combat text; quest npc_say lines marked `whisper: true` use it
// (quest 28's effigy fight, owner ruling 2026-10-17). No one else in the room
// hears it. The speaker's name follows the addressee's sight, and a hidden
// mob whispers as "Someone". NPC lines are authored content, so a deafened
// player still receives it (ruling 6).
func Whisper(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	if room.PlayerCt() < 1 {
		return true, nil
	}

	args := util.SplitButRespectQuotes(strings.ToLower(rest))
	if len(args) < 2 {
		return true, nil
	}

	target, err := actions.ResolveTargetActor(room, args[0])
	if err != nil || !target.IsPlayer() {
		return true, nil
	}
	toUser := target.(*actions.UserActor).User

	text := strings.TrimSpace(rest[len(args[0]):])
	if text == `` {
		return true, nil
	}

	sight := messaging.ParticipantSight(toUser.Character, room)
	if mob.Character.IsHidden() {
		sight = messaging.SightNone
	}

	line := fmt.Sprintf(`<ansi fg="white">***</ansi> <ansi fg="mobname">%s</ansi> whispers to you, "<ansi fg="saytext-mob">%s</ansi>" <ansi fg="white">***</ansi>`,
		mob.Character.Name, text)
	line = messaging.HideSpeakerNames(line, []string{mob.Character.Name}, sight)
	toUser.SendText(messaging.CategoryWhisper, util.SplitStringNL(line, 80))

	sender := mob.Character.Name
	if sight != messaging.SightFull {
		noun := messaging.SpeakerNoun(sight)
		sender = strings.ToUpper(noun[:1]) + noun[1:]
	}
	events.AddToQueue(events.Communication{
		SourceMobInstanceId: mob.InstanceId,
		TargetUserId:        toUser.UserId,
		CommType:            `whisper`,
		Name:                sender,
		Message:             text,
		SpeakerHidden:       mob.Character.IsHidden(),
	})

	return true, nil
}
