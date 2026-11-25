package rooms

import (
	"iter"
	"maps"
	"sort"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/users"
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
// through it (ExitKnownTo and IsSecretExit through GetExitInfo, the same
// rule for one name; NPC loops through PermanentExitsKnownTo), so a secret
// mutator exit over a plain room exit of the same name is hidden from all of
// them alike.
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

// PermanentExitsKnownTo yields every room and mutator exit of r that char
// knows, by the name permanentExits settles on: ExitKnownTo's rule over the
// whole set, with the room's mutators resolved once per call rather than once
// per exit. Temporary exits are not included. An NPC choice point that loops
// over a room's exits on its own (wandering, a walk-away, a forager's roam, a
// planner's flee) iterates it instead of r.Exits (owner ruling 13).
func (r *Room) PermanentExitsKnownTo(char *characters.Character) iter.Seq2[string, exit.RoomExit] {
	return func(yield func(string, exit.RoomExit) bool) {
		for name, ex := range r.permanentExits() {
			if !r.exitKnown(char, name, ex) {
				continue
			}
			if !yield(name, ex) {
				return
			}
		}
	}
}

// ExitTo is one exit of a room leading to a given room (ExitsTo).
type ExitTo struct {
	Key       string // the map key: what `go` and `shoot` match
	Name      string // what a line prints: Key, or a temporary exit's Title
	Secret    bool
	Temporary bool
}

// exitToRank is ExitsTo's order: plain room or mutator exits, then temporary
// ones, then secret ones.
func exitToRank(e ExitTo) int {
	switch {
	case e.Secret:
		return 2
	case e.Temporary:
		return 1
	}
	return 0
}

// ExitsTo is every exit of r that leads to roomId, best first: a plain room
// or mutator exit (permanentExits, so a mutator exit takes the place of the
// room exit of its name), then a temporary exit, then a secret one, A to Z by
// key within each. It is the one ranking of "the way from r to roomId":
// FindExitTo prints the first, and actions.FollowExitTo picks among them by
// what a leader knows (#781).
func (r *Room) ExitsTo(roomId int) []ExitTo {
	var ways []ExitTo
	for key, ex := range r.permanentExits() {
		if ex.RoomId == roomId {
			ways = append(ways, ExitTo{Key: key, Name: key, Secret: ex.Secret})
		}
	}
	for key, ex := range r.ExitsTemp {
		if ex.RoomId == roomId {
			name := ex.Title
			if name == "" {
				name = key
			}
			ways = append(ways, ExitTo{Key: key, Name: name, Temporary: true})
		}
	}
	sort.Slice(ways, func(i, j int) bool {
		if ri, rj := exitToRank(ways[i]), exitToRank(ways[j]); ri != rj {
			return ri < rj
		}
		return ways[i].Key < ways[j].Key
	})
	return ways
}

// FindKnownExitTo is FindExitTo among the exits char knows (ExitKnownTo): the
// exit of r that leads to roomId, or "" when the only way there is a secret
// exit char has not found. A room exit a mutator exit has taken the place of
// leads nowhere. An NPC heading for a room on its own uses it (owner ruling
// 13). Every caller puts the answer in a command (`go <exit>`, `shoot
// <target> <exit>`), so a temporary exit answers with its key, which is what
// those commands match, never its Title: a rift's title ("x-7 rift") or a
// coloured portal title matches nothing (#781, Task 5c). The pick is the
// first of ExitsTo that char knows: plain, then temporary, then a found
// secret exit.
func (r *Room) FindKnownExitTo(char *characters.Character, roomId int) string {
	for _, way := range r.ExitsTo(roomId) {
		if !way.Secret || r.ExitKnownTo(char, way.Key) {
			return way.Key
		}
	}
	return ""
}

// HeardFromNearby is what a listener reads in place of the name of a secret
// exit they have not found, for a sound that came through it: "(From
// nearby)", "Someone shouts from nearby" (#781, owner ruling 16).
const HeardFromNearby = `nearby`

// HeardThroughSecretExit splits the players in r for a sound that came
// through exitName of r (#781, owner ruling 16): knowers read the exit's name,
// strangers read HeardFromNearby, and nothing is learned from hearing. It is
// the one split for every heard line: a sender that sends one message per room
// excludes by it, and the GMCP shout sender reads it per listener. secret is
// false when exitName is not a secret
// exit, and then nothing is split: everyone reads the name, and the check
// costs one exit lookup, with no work per listener.
func (r *Room) HeardThroughSecretExit(exitName string) (knowers, strangers []int, secret bool) {
	if !r.IsSecretExit(exitName) {
		return nil, nil, false
	}
	for _, uid := range r.GetPlayers() {
		u := users.GetByUserId(uid)
		if u != nil && r.ExitKnownTo(u.Character, exitName) {
			knowers = append(knowers, uid)
		} else {
			strangers = append(strangers, uid)
		}
	}
	return knowers, strangers, true
}
