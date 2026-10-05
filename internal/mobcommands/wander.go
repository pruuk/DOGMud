package mobcommands

import (
	"errors"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// hasLootItems reports whether a floor holds anything a mob could scavenge.
// A fixture is part of the room, not loot (lighting 5e, R9).
func hasLootItems(floor []items.Item) bool {
	for _, it := range floor {
		if !it.IsFixture() {
			return true
		}
	}
	return false
}

// pickWanderExit chooses the exit a wandering mob should take.
//
// Finding 11: `wander loot` and `wander players` built a filtered candidate
// list and then threw it away, calling room.GetRandomExit() unconditionally,
// so both modes wandered at random and the filter did nothing at all.
//
// Filtered candidates win when present. An empty list means no adjacent room
// qualified, which is the normal case, so fall back to an ordinary random
// exit rather than standing still.
func pickWanderExit(exitOptions []string, room *rooms.Room) (string, int) {
	if len(exitOptions) > 0 {
		picked := exitOptions[util.Rand(len(exitOptions))]
		if ex, ok := room.Exits[picked]; ok {
			return picked, ex.RoomId
		}
	}
	return room.GetRandomExit()
}

func Wander(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	if mob.Character.IsCharmed() {
		return true, errors.New("friendly mobs don't wander")
	}

	// Stage 42.8: Pack roaming — scattered mobs skip wander
	if mob.ScatterRounds > 0 {
		return true, nil
	}

	// Stage 42.8: Pack roaming — non-alpha follows alpha's movement
	if mob.PackAlphaId > 0 && !mob.IsPackAlpha {
		return true, nil
	}

	// If they aren't home and need to go home, do it.
	if mob.Character.RoomId != mob.HomeRoomId {
		if mob.MaxWander > -1 { // -1 means they can wander forever and never go home. 0 means they never wander.
			if mob.WanderCount > mob.MaxWander {

				mob.Command(`pathto home`)

				return true, nil
			}
		}
	}

	// If MaxWander is zero, they don't wander.
	if mob.MaxWander == 0 {
		return true, nil
	}

	exitOptions := make([]string, 0)
	restrictZone := true

	// First only consider adjacent rooms with loot in them
	if rest == `loot` {
		for exitName, exit := range room.Exits {
			exitRoom := rooms.LoadRoom(exit.RoomId)
			if exitRoom == nil {
				continue
			}
			if hasLootItems(exitRoom.Items) || exitRoom.Gold > 0 {
				exitOptions = append(exitOptions, exitName)
			}
		}
	}

	// First only consider adjacent rooms with players in them
	if rest == `players` {
		for exitName, exit := range room.Exits {
			exitRoom := rooms.LoadRoom(exit.RoomId)
			if exitRoom == nil {
				continue
			}
			if exitRoom.PlayerCt() > 0 {
				exitOptions = append(exitOptions, exitName)
			}
		}
	}

	if exitName, roomId := pickWanderExit(exitOptions, room); exitName != `` {
		if r := rooms.LoadRoom(roomId); r != nil {
			if !restrictZone || r.Zone == mob.Character.Zone {

				// Movement parity 4b: a step the wanderer cannot pay is not
				// taken and not counted, and drags no followers.
				if !actions.QuoteMobStep(mob, exitName).OK() {
					return true, nil
				}

				// Stage 42.8: Capture mob list before alpha moves (for follower movement)
				var preMoveRoomMobs []int
				if mob.IsPackAlpha && mobs.PackRoamingEnabled() {
					preMoveRoomMobs = room.GetMobs(rooms.FindAll)
				}

				mob.WanderCount++
				mob.Command(fmt.Sprintf("go %s", exitName))

				// Stage 42.8: Move pack followers through the same exit. Each
				// pays its own step.
				if mob.IsPackAlpha && len(preMoveRoomMobs) > 0 {
					mobs.MovePackFollowers(mob, exitName, preMoveRoomMobs, func(f *mobs.Mob) bool {
						return actions.QuoteMobStep(f, exitName).OK()
					})
				}

			}
		}

	}

	return true, nil
}
