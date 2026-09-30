package mobcommands

import (
	"fmt"
	"strconv"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// sendMovementMessage sends a visual movement message to players who can see
// and a sound-based fallback to players in darkness without night vision.
//
// visualCat tags the visual (entry/exit) line; the audio soundMsg uses
// CategorySystem since it's an environment-cue ("you hear footsteps").
// sendMovementMessage shows visualMsg to whoever can see and soundMsg to
// whoever cannot.
//
// It now delegates to Room.SendTextVisualWithAudio rather than deciding for
// itself. The hand-rolled version it replaced tested ONLY
// conditions.NightVision, which meant it ignored blindness, sleep and infrared: a
// BLINDED player who happened to carry night vision was shown the named line,
// and a player with infrared got the sound cue when they should have got
// shapes. The shared primitive reads the same perception predicates the rest
// of the pipeline does.
//
// This was the third hand-rolled darkness check in this package. canSeeInDark
// is gone (M4e-1 Task 9: every visual reader now gets ParticipantSight +
// HideNames, the three-tier verdict, instead of a binary lit-or-nightvision
// check). The last one, darkness.go's two-tier sendAudioRoomText, was deleted
// by sight gates slice 5b: speech, rally, warcry, howl and taunt now hide
// names through rooms.Room.SendTextHidingNames at every tier.
func sendMovementMessage(room *rooms.Room, visualCat messaging.Category, visualMsg string, soundMsg string) {
	room.SendTextVisualWithAudio(visualCat, visualMsg, soundMsg)
}

func Go(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	// If has a condition that prevents combat, skip the player
	if mob.Character.HasConditionFlag(conditions.NoMovement) {
		return true, nil
	}

	// Special behavior allowed for mobs to travel to specific rooms, even if disconnected.
	if forceRoomId, err := strconv.Atoi(rest); err == nil {

		foundRoomExit := false
		for exitName, exitInfo := range room.Exits {
			if exitInfo.RoomId == forceRoomId {
				rest = exitName
				foundRoomExit = true
			}
		}

		if !foundRoomExit {
			c := configs.GetTextFormatsConfig()

			if forceRoomId == room.RoomId {
				return true, nil
			}

			destRoom := rooms.LoadRoom(forceRoomId)
			if destRoom == nil {
				return true, nil
			}

			// Captured before the move, matching ruling D1: a sneaking mob's
			// forced relocation (callforhelp's `go <roomId>`) is as quiet as
			// its ordinary step through actions.RelocateMob.
			sneaking := actions.MobIsSneaking(mob)

			room.RemoveMob(mob.InstanceId)
			actions.ClearRoomAggroOnDeparture(room, mob.InstanceId)
			destRoom.AddMob(mob.InstanceId)

			if !sneaking {
				// Tell the old room they are leaving
				sendMovementMessage(room, messaging.CategoryRoomExit,
					fmt.Sprintf(string(c.ExitRoomMessageWrapper),
						fmt.Sprintf(`<ansi fg="mobname">%s</ansi> runs off suddenly.`, mob.Character.Name),
					),
					`You hear hurried footsteps receding.`)

				// Tell the new room they have arrived
				sendMovementMessage(destRoom, messaging.CategoryRoomEntry,
					fmt.Sprintf(string(c.EnterRoomMessageWrapper),
						fmt.Sprintf(`<ansi fg="mobname">%s</ansi> enters from nearby.`, mob.Character.Name),
					),
					`You hear footsteps approaching.`)
			}

			return true, nil

		}
	}

	if rest == `home` {
		mob.Command(`pathto home`)
		return true, nil
	}

	exitResult := actions.FindExit(room, rest)
	exitName := exitResult.ExitName
	goRoomId := exitResult.RoomId

	exitInfo, _ := room.GetExitInfo(exitName)
	if exitInfo.Lock.IsLocked() {

		mob.Command(fmt.Sprintf(`emote tries to go the <ansi fg="exit">%s</ansi> exit, but it's locked.`, exitName))

		return true, nil
	}

	if exitName != `` {

		// Load current room details
		destRoom := rooms.LoadRoom(goRoomId)
		if destRoom == nil {
			return false, fmt.Errorf(`room %d not found`, goRoomId)
		}

		// Entering through the far side of a locked door would unlock it; for
		// now mobs do not do that.
		if back := destRoom.FindExitTo(room.RoomId); back != `` {
			if backInfo, _ := destRoom.GetExitInfo(back); backInfo.Lock.IsLocked() {
				return true, nil
			}
		}

		// Movement parity 4b, owner ruling 2: a mob pays the player's step
		// price, after the lock gates, silently on refusal (a mob has no one
		// to tell).
		mover := actions.NewMobActorInRoom(mob, room)
		if !actions.ChargeMove(mover, destRoom).OK() {
			return true, nil
		}

		sneaking := actions.MobIsSneaking(mob)

		actions.RelocateMob(mob, room, exitName, destRoom, sneaking)

		// The rare Search roll and hidden detection on entry, both ways
		// (owner ruling 3), shared with players.
		arrived := actions.NewMobActorInRoom(mob, destRoom)
		actions.TrainSearchOnMove(arrived)
		actions.EntryDetection(arrived, destRoom, sneaking)

		// We want the `waypoint` onPath event triggered right after they enter the room.
		if currentStep := mob.Path.Current(); currentStep != nil && currentStep.Waypoint() {

			// Anytime a mob reaches a waypoint, introduce a 1 second delay before they can perform any additional commands.
			// This gives a more natural feel to mob behavior, and gives those following a moment to catch up before the mob does something.
			mob.Command("noop", 1)
		}

		return true, nil
	}

	return false, nil
}
