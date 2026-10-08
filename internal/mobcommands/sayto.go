package mobcommands

import (
	"fmt"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// sayToRoom sends a mob's addressed speech line (sayto, replyto) to everyone
// else in room. Like NPC say (actions.sendSpoken), every listener hears the
// words, and each name in the line, the speaker's and the addressee's, reads
// "a figure" at shapes and "someone" when the listener sees nothing (sight
// gates 5b ruling 3). Authored NPC speech, so never deafen-filtered (ruling 6).
func sayToRoom(room *rooms.Room, line string, names []string, excludeUserIds ...int) {
	room.SendTextHidingNames(messaging.CategorySpeech, line, names, messaging.HideSpeakerNames, excludeUserIds...)
}

// sayToUser sends the addressee its own line, the speaker's name hidden at the
// addressee's sight: a blinded player spoken to hears "Someone says to you".
func sayToUser(toUser *users.UserRecord, room *rooms.Room, line, speaker string) {
	toUser.SendText(messaging.CategorySpeech,
		messaging.HideSpeakerNames(line, []string{speaker}, messaging.ParticipantSight(toUser.Character, room)))
}

func SayTo(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Don't bother if no players are present
	if room.PlayerCt() < 1 {
		return true, nil
	}

	args := util.SplitButRespectQuotes(strings.ToLower(rest))
	if len(args) < 2 {
		return true, nil
	}

	target, err := actions.ResolveTargetActor(room, args[0])
	if err != nil {
		return true, nil
	}
	if target.IsPlayer() {

		toUser := target.(*actions.UserActor).User

		rest = strings.TrimSpace(rest[len(args[0]):])
		isSneaking := mob.Character.IsHidden()

		if isSneaking {
			toUser.SendText(messaging.CategorySpeech, fmt.Sprintf(`someone says to you, "<ansi fg="saytext-mob">%s</ansi>"`, rest))

			events.AddToQueue(events.Communication{
				SourceMobInstanceId: mob.InstanceId,
				TargetUserId:        toUser.UserId,
				CommType:            `say`,
				Name:                mob.Character.Name,
				Message:             rest,
				SpeakerHidden:       true,
			})

		} else {
			sayToUser(toUser, room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> says to you, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, rest), mob.Character.Name)
			sayToRoom(room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> says to <ansi fg="username">%s</ansi>, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, toUser.Character.Name, rest),
				[]string{mob.Character.Name, toUser.Character.Name}, toUser.UserId)

			events.AddToQueue(events.Communication{
				SourceMobInstanceId: mob.InstanceId,
				CommType:            `say`,
				Name:                mob.Character.Name,
				Message:             rest,
			})
		}
	} else {

		toMob := target.(*actions.MobActor).Mob

		rest = strings.TrimSpace(rest[len(args[0]):])
		isSneaking := mob.Character.IsHidden()

		if !isSneaking {
			sayToRoom(room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> says to <ansi fg="mobname">%s</ansi>, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, toMob.Character.Name, rest),
				[]string{mob.Character.Name, toMob.Character.Name})

			events.AddToQueue(events.Communication{
				SourceMobInstanceId: mob.InstanceId,
				CommType:            `say`,
				Name:                mob.Character.Name,
				Message:             rest,
			})

		}
	}

	return true, nil
}

func SayToOnly(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Don't bother if no players are present
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

	rest = strings.TrimSpace(rest[len(args[0]):])
	isSneaking := mob.Character.IsHidden()

	if isSneaking {
		toUser.SendText(messaging.CategorySpeech, fmt.Sprintf(`someone says to you, "<ansi fg="saytext-mob">%s</ansi>"`, rest))
	} else {
		sayToUser(toUser, room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> says to you, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, rest), mob.Character.Name)
	}

	events.AddToQueue(events.Communication{
		SourceMobInstanceId: mob.InstanceId,
		TargetUserId:        toUser.UserId,
		CommType:            `say`,
		Name:                mob.Character.Name,
		Message:             rest,
		SpeakerHidden:       isSneaking,
	})

	return true, nil
}

func ReplyTo(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// Don't bother if no players are present
	if room.PlayerCt() < 1 {
		return true, nil
	}

	args := util.SplitButRespectQuotes(strings.ToLower(rest))
	if len(args) < 2 {
		return true, nil
	}

	target, err := actions.ResolveTargetActor(room, args[0])
	if err != nil {
		return true, nil
	}
	if target.IsPlayer() {

		toUser := target.(*actions.UserActor).User

		rest = strings.TrimSpace(rest[len(args[0]):])
		isSneaking := mob.Character.IsHidden()

		if isSneaking {
			toUser.SendText(messaging.CategorySpeech, fmt.Sprintf(`someone replies to you, "<ansi fg="saytext-mob">%s</ansi>"`, rest))
		} else {
			sayToUser(toUser, room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> replies to you, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, rest), mob.Character.Name)
			sayToRoom(room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> replies to <ansi fg="username">%s</ansi>, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, toUser.Character.Name, rest),
				[]string{mob.Character.Name, toUser.Character.Name}, toUser.UserId)
		}
	} else {

		toMob := target.(*actions.MobActor).Mob

		rest = strings.TrimSpace(rest[len(args[0]):])
		isSneaking := mob.Character.IsHidden()

		if !isSneaking {
			sayToRoom(room, fmt.Sprintf(`<ansi fg="mobname">%s</ansi> replies to <ansi fg="mobname">%s</ansi>, "<ansi fg="saytext-mob">%s</ansi>"`, mob.Character.Name, toMob.Character.Name, rest),
				[]string{mob.Character.Name, toMob.Character.Name})
		}
	}

	return true, nil
}
