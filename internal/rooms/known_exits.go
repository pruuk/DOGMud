package rooms

import (
	"maps"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
)

// ExitKind sorts an exit for the touch controls (#781): the six directions of
// the inverted T are compass and vertical; diagonals, named and temporary
// exits sit behind the other-exits button.
type ExitKind string

const (
	ExitCompass   ExitKind = "compass"   // north, south, east, west
	ExitVertical  ExitKind = "vertical"  // up, down
	ExitDiagonal  ExitKind = "diagonal"  // northeast, southeast, southwest, northwest
	ExitNamed     ExitKind = "named"     // any other permanent exit: out, shrine, crack
	ExitTemporary ExitKind = "temporary" // a gangplank, a return portal, a tree or mob portal
)

// exitOrder is the order KnownExits lists exits in: the six T directions,
// then the diagonals clockwise from northeast (spec 2b, other exits). Every
// other exit follows, A to Z.
var exitOrder = map[string]int{
	"north": 0, "east": 1, "south": 2, "west": 3, "up": 4, "down": 5,
	"northeast": 6, "southeast": 7, "southwest": 8, "northwest": 9,
}

// ExitKindOf is the kind of the exit named exitName; temporary marks an
// ExitsTemp entry, which is temporary whatever its name.
func ExitKindOf(exitName string, temporary bool) ExitKind {
	if temporary {
		return ExitTemporary
	}
	switch exitName {
	case "north", "south", "east", "west":
		return ExitCompass
	case "up", "down":
		return ExitVertical
	case "northeast", "southeast", "southwest", "northwest":
		return ExitDiagonal
	}
	return ExitNamed
}

// KnownExit is one exit a character knows in a room.
type KnownExit struct {
	Name      string        // what the player types: the Exits, mutator or ExitsTemp key
	Kind      ExitKind      // see ExitKind
	LockedNow bool          // Lock.IsLocked() now, not merely HasLock (owner ruling 3)
	RoomId    int           // the destination, for the server's own use
	Exit      exit.RoomExit // the room or mutator exit; zero for a temporary exit
	Temp      exit.TemporaryRoomExit
}

// KnownExitList is what a character knows of a room's exits. Dark is true
// when the character sees nothing there; then Exits is empty.
type KnownExitList struct {
	Dark  bool
	Exits []KnownExit
}

// KnownExits is the one answer to "which exits of r does c know" (#781,
// owner ruling 6). Every lister reads it: the exits line (GetDetails),
// Room.Info and the touch buttons. It covers r.Exits, active mutator exits (a
// mutator's exit takes the place of a room exit of the same name, as in
// GetExitInfo) and ExitsTemp. A secret exit is listed once c found it
// (Character.KnowsSecretExit, ruling 1). At no sight (messaging.
// ParticipantSight) the list is dark and empty. A nil c sees fully and knows
// no secret exit. Exits come in exitOrder, then A to Z.
func KnownExits(r *Room, c *characters.Character) KnownExitList {
	return knownExitsAt(r, c, messaging.ParticipantSight(c, r))
}

// knownExitsAt is KnownExits for a caller that already holds c's sight in r
// (GetDetails reads it once for the roster too).
func knownExitsAt(r *Room, c *characters.Character, sight messaging.SightDecision) KnownExitList {
	list := KnownExitList{Exits: []KnownExit{}}
	if sight == messaging.SightNone {
		list.Dark = true
		return list
	}
	for name, ex := range r.permanentExits() {
		if !r.exitKnown(c, name, ex) {
			continue
		}
		list.Exits = append(list.Exits, KnownExit{
			Name:      name,
			Kind:      ExitKindOf(name, false),
			LockedNow: ex.Lock.IsLocked(),
			RoomId:    ex.RoomId,
			Exit:      ex,
		})
	}
	for name, t := range r.ExitsTemp {
		list.Exits = append(list.Exits, KnownExit{Name: name, Kind: ExitTemporary, RoomId: t.RoomId, Temp: t})
	}
	sort.SliceStable(list.Exits, func(i, j int) bool { return knownExitLess(list.Exits[i], list.Exits[j]) })
	return list
}

// knownExitLess orders two known exits for KnownExits: by knownExitRank,
// then A to Z ignoring case, and a room or mutator exit before a temporary
// exit of the same name.
func knownExitLess(a, b KnownExit) bool {
	ra, rb := knownExitRank(a), knownExitRank(b)
	if ra != rb {
		return ra < rb
	}
	la, lb := strings.ToLower(a.Name), strings.ToLower(b.Name)
	if la != lb {
		return la < lb
	}
	return a.Kind != ExitTemporary && b.Kind == ExitTemporary
}

// knownExitRank is k's place in exitOrder; a temporary exit, and any name
// exitOrder does not hold, ranks after all of them.
func knownExitRank(k KnownExit) int {
	if k.Kind != ExitTemporary {
		if rank, ok := exitOrder[k.Name]; ok {
			return rank
		}
	}
	return len(exitOrder)
}

// exitKnown is the one test every known-exit filter applies to a room or
// mutator exit, after permanentExits (or GetExitInfo) has settled which exit
// a name means: not secret, or a secret one char found.
func (r *Room) exitKnown(char *characters.Character, exitName string, ex exit.RoomExit) bool {
	return !ex.Secret || char.KnowsSecretExit(r.RoomId, exitName)
}

// permanentExits is every room and mutator exit of r by name. The first
// active mutator holding a name takes the place of the room exit and of later
// mutators, GetExitInfo's rule. Every known-exit filter resolves a name
// through it, so a secret mutator exit over a plain room exit of the same
// name is hidden from all of them alike.
func (r *Room) permanentExits() map[string]exit.RoomExit {
	all := make(map[string]exit.RoomExit, len(r.Exits))
	maps.Copy(all, r.Exits)
	fromMutator := map[string]bool{}
	for mut := range r.ActiveMutators {
		spec := mut.GetSpec()
		if spec == nil {
			continue
		}
		for name, ex := range spec.Exits {
			if fromMutator[name] {
				continue
			}
			fromMutator[name] = true
			all[name] = ex
		}
	}
	return all
}

// IsSecretExit reports whether exitName is a secret room or mutator exit of r
// (GetExitInfo, so a mutator's exit takes the place of the room's). No such
// exit, or a temporary one: false. A caller asks it before paying for a
// snapshot only a secret exit needs (actions.LearnSecretExitWatching).
func (r *Room) IsSecretExit(exitName string) bool {
	ex, ok := r.GetExitInfo(exitName)
	return ok && ex.Secret
}

// ExitKnownTo reports whether char knows the exit exitName of r: any
// temporary exit, any room or mutator exit that is not secret, and a secret
// one once found (Character.KnowsSecretExit). No such exit: false. NPC choice
// points ask it (owner ruling 13).
func (r *Room) ExitKnownTo(char *characters.Character, exitName string) bool {
	if ex, ok := r.GetExitInfo(exitName); ok {
		return r.exitKnown(char, exitName, ex)
	}
	_, ok := r.ExitsTemp[exitName]
	return ok
}

// FindKnownExitTo is FindExitTo among the exits char knows (ExitKnownTo): the
// exit of r that leads to roomId, or "" when the only way there is a secret
// exit char has not found. A room exit a mutator exit has taken the place of
// leads nowhere. An NPC heading for a room on its own uses it (owner ruling
// 13).
func (r *Room) FindKnownExitTo(char *characters.Character, roomId int) string {
	for exitName, ex := range r.permanentExits() {
		if ex.RoomId == roomId && r.exitKnown(char, exitName, ex) {
			return exitName
		}
	}
	for _, ex := range r.ExitsTemp {
		if ex.RoomId == roomId {
			return ex.Title
		}
	}
	return ""
}
