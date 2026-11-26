package mobcommands

import (
	"errors"
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/ambient"
	"github.com/GoMudEngine/GoMud/internal/characters"
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
//
// The fallback takes a secret exit only once the mover found it (#781, owner
// ruling 13: wandering is the NPC's own choice).
func pickWanderExit(exitOptions []string, room *rooms.Room, mover *characters.Character) (string, int) {
	if len(exitOptions) > 0 {
		picked := exitOptions[util.Rand(len(exitOptions))]
		// GetExitInfo, so a candidate means the exit wanderCandidates judged
		// (an active mutator's takes the place of the room's).
		if ex, ok := room.GetExitInfo(picked); ok {
			return picked, ex.RoomId
		}
	}
	return room.GetRandomKnownExit(mover)
}

// wanderCandidates is the exits of room whose far room wants accepts, among
// the exits mover knows: a secret exit only once found (#781, ruling 13,
// Room.PermanentExitsKnownTo, which resolves the room's mutators once).
func wanderCandidates(room *rooms.Room, mover *characters.Character, wants func(dest *rooms.Room) bool) []string {
	out := []string{}
	for exitName, ex := range room.PermanentExitsKnownTo(mover) {
		dest := rooms.LoadRoom(ex.RoomId)
		if dest == nil || !wants(dest) {
			continue
		}
		out = append(out, exitName)
	}
	return out
}

// packFollowerStep is MovePackFollowers' canStep for packmates following their
// alpha through exitName of room: each pays its own step (movement parity 4b),
// and one that goes learns a secret exit the alpha took (#781, ruling 2's NPC
// analogue, D17). Whether the exit is secret is asked once per alpha step, so
// an open exit costs the followers nothing more.
func packFollowerStep(room *rooms.Room, exitName string) func(f *mobs.Mob) bool {
	secretExit := room.IsSecretExit(exitName)
	return func(f *mobs.Mob) bool {
		if !actions.QuoteMobStep(f, exitName).OK() {
			return false
		}
		if secretExit {
			actions.LearnSecretExitFollowing(&f.Character, room, exitName)
		}
		ambient.CountMove(room.RoomId, ambient.MoveIdle)
		return true
	}
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

	switch rest {
	case `loot`: // first only consider adjacent rooms with loot in them
		exitOptions = wanderCandidates(room, &mob.Character, func(dest *rooms.Room) bool {
			return hasLootItems(dest.Items) || dest.Gold > 0
		})
	case `players`: // first only consider adjacent rooms with players in them
		exitOptions = wanderCandidates(room, &mob.Character, func(dest *rooms.Room) bool {
			return dest.PlayerCt() > 0
		})
	}

	if exitName, roomId := pickWanderExit(exitOptions, room, &mob.Character); exitName != `` {
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
				ambient.CountMove(room.RoomId, ambient.MoveIdle)
				mob.Command(fmt.Sprintf("go %s", exitName))

				// Stage 42.8: Move pack followers through the same exit. Each
				// pays its own step. canStep is MovePackFollowers' last check
				// before it moves a follower, so a true here is a follower
				// that goes, and telemetry counts it.
				if mob.IsPackAlpha && len(preMoveRoomMobs) > 0 {
					mobs.MovePackFollowers(mob, exitName, preMoveRoomMobs, packFollowerStep(room, exitName))
				}

			}
		}

	}

	return true, nil
}
