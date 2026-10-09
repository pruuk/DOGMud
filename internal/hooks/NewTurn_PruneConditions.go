package hooks

import (
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
)

//
// Prune all conditions that have expired.
//

func PruneConditions(e events.Event) events.ListenerReturn {

	/*
		evt, typeOk := e.(events.NewTurn)
		if !typeOk {
			mudlog.Error("Event", "Expected Type", "NewTurn", "Actual Type", e.Type())
			return events.Cancel
		}
	*/

	roomsWithPlayers := rooms.GetRoomsWithPlayers()
	for _, roomId := range roomsWithPlayers {
		// Get rooom
		if room := rooms.LoadRoom(roomId); room != nil {

			// Handle outstanding player conditions
			logOff := false
			for _, uId := range room.GetPlayers(rooms.FindWithConditions) {

				user := users.GetByUserId(uId)

				logOff = false
				if conditionsToPrune := user.Character.Conditions.Prune(); len(conditionsToPrune) > 0 {
					for _, conditionInfo := range conditionsToPrune {
						// Send the end notice (authored, or the generic line;
						// a secret condition is silent).
						endConditionSpec := conditions.GetConditionSpec(conditionInfo.ConditionId)
						if endConditionSpec != nil && endConditionSpec.Narration(conditions.PhaseEnd).Len() > 0 {
							roles := endConditionSpec.Narrate(conditions.PhaseEnd,
								user.Character.GetCharacterName(true),
								user.Character.GetCharacterName(false))
							if roles.Actee != "" {
								user.SendText(messaging.CategoryConditionExpire, roles.Actee)
							}
							if roles.Observer != "" {
								if r := rooms.LoadRoom(user.Character.RoomId); r != nil {
									sendConditionEndRoomText(r, rooms.TakeEndLineSnapshot(conditionInfo, r.RoomId), roles.Observer,
										[]string{user.Character.GetCharacterName(false)}, user.UserId)
								}
							}
						}

						if conditionInfo.ConditionId == 0 { // Log them out // logoff // logout
							if !user.Character.HasAdjective(`zombie`) { // if they are currently a zombie, we don't log them out from this condition being removed
								logOff = true
							}
						}
					}

					user.Character.Validate()

					// Push a Char update so the web client's Status &
					// Conditions panel refreshes immediately on condition
					// expiry. GMCP listens to ConditionsTriggered and queues
					// Char.Conditions; reusing it here (on
					// the removal batch) avoids a stale panel until some
					// unrelated Char event fires. Player-only — the mob
					// prune branch below has no UserId and is skipped.
					prunedIds := make([]int, 0, len(conditionsToPrune))
					for _, conditionInfo := range conditionsToPrune {
						prunedIds = append(prunedIds, conditionInfo.ConditionId)
					}
					events.AddToQueue(events.ConditionsTriggered{UserId: user.UserId, ConditionIds: prunedIds})

					if logOff {
						mudlog.Info("MEDITATION LOGOFF")
						events.AddToQueue(events.System{Command: "logoff", Data: uId})
					}
				}

			}
		}
	}

	// Handle outstanding mob conditions
	for _, mobInstanceId := range mobs.GetAllMobInstanceIds() {

		mob := mobs.GetInstance(mobInstanceId)

		if conditionsToPrune := mob.Character.Conditions.Prune(); len(conditionsToPrune) > 0 {
			for _, conditionInfo := range conditionsToPrune {
				// Send YAML end text (if defined).
				endConditionSpec := conditions.GetConditionSpec(conditionInfo.ConditionId)
				if endConditionSpec != nil && len(endConditionSpec.Narration(conditions.PhaseEnd).Observer) > 0 {
					// The mob tag, not the player one: see Condition_ApplyConditions.go.
					// Visual, not audio, for the same reason as start text. The
					// holder line is rendered and dropped: a mob has no client.
					holderName := mob.Character.GetCharacterName(true)
					if r := rooms.LoadRoom(mob.Character.RoomId); r != nil {
						holderName = messaging.StripNameAdjectives(mobDisplayName(mob, r, 0)) // #453
					}
					roles := endConditionSpec.Narrate(conditions.PhaseEnd,
						holderName, mobPlainName(mob))
					if roles.Observer != "" {
						if r := rooms.LoadRoom(mob.Character.RoomId); r != nil {
							sendConditionEndRoomText(r, rooms.TakeEndLineSnapshot(conditionInfo, r.RoomId), roles.Observer,
								[]string{mob.Character.GetCharacterName(false)})
						}
					}
				}
			}

			mob.Character.Validate()
		}

	}

	// Every snapshot this prune did not narrate is stale: its record was
	// revived, pruned by some other path, or silent. See rooms.endLineSnapshots.
	rooms.ClearEndLineSnapshots()

	return events.Continue

}

// sendConditionEndRoomText sends a condition's end room line on the visual
// channel. With a snapshot (a light or darkness record that ran out on the
// round tick or was cancelled, from rooms.TakeEndLineSnapshot) the line is
// judged against the room as it was just before the record ended: the owner
// rule of 2026-10-05, a line announcing a change is judged by the state
// before it. So a light's end line reaches the watchers who saw by it and not
// one who never could, and a darkness's does not reach a watcher who was
// blind in it. Every other end line is judged by the room as it is: right
// for a condition that is no light or darkness, and for a holder who moved
// since; for a light or darkness removed by a path that keeps no snapshot it
// is a known gap (see rooms.endLineSnapshots).
//
// names is the holder's PLAIN name. An end line may author a bare
// {actee_plain} (shipped conditions 1 and 9 both do), which tag-based
// Anonymize cannot see, so the name must be handed to HideNames explicitly.
// This is the End-phase twin of sendConditionStartRoomText.
func sendConditionEndRoomText(r *rooms.Room, snap rooms.VisualSnapshot, msg string, names []string, skip ...int) {
	if snap != nil {
		r.SendTextVisualToSnapshot(snap, messaging.CategoryConditionExpire, msg, names, skip...)
		return
	}
	r.SendTextVisualHidingNames(messaging.CategoryConditionExpire, msg, names, skip...)
}
