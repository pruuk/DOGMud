package mobcommands

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/mapper"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// pathtoRefusedRounds is how many rounds a found-only route that found no way
// is not searched again for the same start and targets (#781). A planner
// resends its `pathto found <id>` every idle tick, and a target reached only
// through a secret exit the mob has not found fails the uncached search each
// time (mapper.GetPathKnown), which can cross every zone. No balance knob
// paces a mob's path retries, so this is a named constant, not a knob.
const pathtoRefusedRounds = 10

// pathtoRefusedKey is the mob temp data key holding the last refused
// found-only route (pathtoRefusal), beside `home-impossible`.
const pathtoRefusedKey = `pathto-refused`

// errPathtoRefusedRecently answers a found-only pathto that pathtoRefusal
// still covers: no search was run.
var errPathtoRefusedRecently = errors.New("pathto: found-only route refused recently")

// pathtoRefusal is a found-only route that found no way: from fromRoomId to
// toRoomIds, refused on round, while the mob knew knownCount discoveries.
// Learning a secret exit adds a discovery, so the record stops covering the
// route at once; a new start or target never matches it.
type pathtoRefusal struct {
	fromRoomId int
	toRoomIds  []int
	knownCount int
	round      uint64
}

func (r pathtoRefusal) covers(fromRoomId int, toRoomIds []int, knownCount int, round uint64) bool {
	return r.fromRoomId == fromRoomId && r.knownCount == knownCount &&
		slices.Equal(r.toRoomIds, toRoomIds) && round < r.round+pathtoRefusedRounds
}

// discoveryCount is how many discoveries c holds, the fingerprint
// pathtoRefusal compares; it is read only on a found-only route.
func discoveryCount(c *characters.Character) int {
	n := 0
	for _, keys := range c.Discoveries {
		n += len(keys)
	}
	return n
}

func Pathto(rest string, mob *mobs.Mob, room *rooms.Room) (bool, error) {

	toRoomIds, foundOnly, homeOnly := pathtoTargets(rest, mob.HomeRoomId)

	// If only going home, check whether a path home was already tried and marked as impossible.
	if homeOnly {
		cantGoHome := mob.GetTempData(`home-impossible`)
		if cantGoHome != nil && cantGoHome.(bool) == true {
			// If can't go home, slowly lose health (10%)
			// This helps to clean up mobs that get stuck in a weird location, which can
			// happen for any number of reasons, like players dragging them through portals
			// Attrition, not an attack: it does not interrupt a craft (#514 R2).
			mob.Character.ApplyAttrition(characters.PoolHealth,
				int(math.Ceil(float64(mob.Character.HealthMax.Value)/10)),
				state.ActorRef{})
			return true, nil
		}
	}

	if len(toRoomIds) < 1 || toRoomIds[0] == mob.Character.RoomId {
		return true, nil
	}

	// A planner's errand (`pathto found ...`) takes only the secret exits
	// this mob found; any other pathto is a scripted route and takes them all
	// (#781, owner ruling 13).
	var known mapper.SecretExitKnown = mapper.AnySecretExit
	knownCount := 0
	if foundOnly {
		known = mapper.SecretExitsKnownTo(&mob.Character)
		knownCount = discoveryCount(&mob.Character)
		if r, ok := mob.GetTempData(pathtoRefusedKey).(pathtoRefusal); ok &&
			r.covers(mob.Character.RoomId, toRoomIds, knownCount, util.GetRoundCount()) {
			return false, errPathtoRefusedRecently
		}
	}
	path, err := mapper.GetPathKnown(mob.Character.RoomId, known, toRoomIds...)
	if err != nil {
		if homeOnly {
			mob.SetTempData(`home-impossible`, true)
			mob.Character.SetAdjective(`lost`, true)
		}
		if foundOnly {
			mob.SetTempData(pathtoRefusedKey, pathtoRefusal{
				fromRoomId: mob.Character.RoomId,
				toRoomIds:  toRoomIds,
				knownCount: knownCount,
				round:      util.GetRoundCount(),
			})
		}
		return false, err
	}

	if homeOnly && len(path) == 0 {
		mob.SetTempData(`home-impossible`, true)
		mob.Character.SetAdjective(`lost`, true)
		return true, nil
	}

	newPath := make([]mobs.PathRoom, len(path))

	// Copy everything over
	for idx, p := range path {
		newPath[idx] = p
	}

	mob.Path.SetPath(newPath, foundOnly)

	return true, nil
}

// pathtoTargets reads pathto's words (mobs.PathtoCommand builds them): room
// ids, `home` (homeRoomId), and mobs.PathtoFoundOnly, which marks an NPC's
// own errand. homeOnly is a route whose one target is the word `home`, with
// or without the found mark: the route `home-impossible` is about.
func pathtoTargets(rest string, homeRoomId int) (toRoomIds []int, foundOnly bool, homeOnly bool) {
	toRoomIds = []int{}
	homeWords := 0
	for _, roomIdStr := range util.SplitButRespectQuotes(strings.ToLower(rest)) {
		if roomIdStr == mobs.PathtoFoundOnly {
			foundOnly = true
			continue
		}
		if roomIdStr == `home` {
			toRoomIds = append(toRoomIds, homeRoomId)
			homeWords++
			continue
		}
		if roomIdInt, err := strconv.Atoi(roomIdStr); err == nil {
			toRoomIds = append(toRoomIds, roomIdInt)
		}
	}
	return toRoomIds, foundOnly, homeWords == 1 && len(toRoomIds) == 1
}
