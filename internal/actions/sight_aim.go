package actions

import (
	"fmt"
	"strconv"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// shapeWord is what a player who makes out shapes aims at: `shape`, `2.shape`,
// `shape#2`. No dogmud mob, item, noun or alias uses the word.
const shapeWord = "shape"

// The refusals a player's sight gives a typed name (#454). Neither echoes the
// name, so a creature that is there and a name that matches nothing read the
// same.
const (
	// AimNothingLine answers a player who sees nothing and named a shape, or
	// named nothing at all.
	AimNothingLine = `You can't see anything to aim at.`
	// AimNotHereLine answers a name a player cannot see to pick out.
	AimNotHereLine = `You don't see them here.`
	// aimShapesLine opens the hint a player who makes out shapes only reads
	// when they typed a name.
	aimShapesLine = `You can only make out shapes here.`
	// aimHintWidth is the widest the hint's second line may be, in visible
	// columns (the 80-column rule, dogmud-player-copy).
	aimHintWidth = 80
)

// AimBySight judges a creature name a player typed against what the player
// makes out of room (#454, lifted from the cast's own rule, follow-up slice A):
//
//	clear sight:  every name, as typed
//	shapes only:  `shape`, `N.shape`, `shape#N`, or an id form (`@N`, `#N`);
//	              a typed name gets a hint naming verb
//	no sight:     nothing
//
// It returns the name to resolve, with a shape rewritten to "@<userId>" or
// "#<mobInstanceId>" (forms FindByName already resolves), and refusal: the
// line to tell the player, or "" when the name is admitted. verb is what the
// player types before the name, e.g. "kick" or "give torch"; the hint repeats
// it. An empty name is admitted at shapes (the caller decides what no name
// means) and refused with no sight.
//
// Player actors only: a mob acts on shapes and needs no hint (D8). An id form
// is admitted at shapes because it is what a shape becomes, and what the
// game's own party auto-assist types (`attack #N`).
func AimBySight(viewer *characters.Character, selfUserId int, room *rooms.Room, name, verb string) (string, string) {
	if room == nil {
		return name, ``
	}
	return aimAtSight(viewer, selfUserId, room, messaging.ParticipantSight(viewer, room), name, verb)
}

// aimAtSight is AimBySight at a sight the caller already judged.
func aimAtSight(viewer *characters.Character, selfUserId int, room *rooms.Room, sight messaging.SightDecision, name, verb string) (string, string) {
	shape := aimShapeIndex(name)
	switch sight {
	case messaging.SightNone:
		if name == `` || aimNamesAShape(name) {
			return name, AimNothingLine
		}
		return name, AimNotHereLine
	case messaging.SightShapes:
		if name == `` || aimIdForm(name) {
			return name, ``
		}
		if shape == 0 {
			return name, AimShapesHint(verb)
		}
	}

	if shape > 0 {
		figures := aimFigures(viewer, selfUserId, room)
		if shape > len(figures) {
			return name, AimNotHereLine
		}
		return figures[shape-1], ``
	}
	return name, ``
}

// AimShapesHint is the hint a player who makes out shapes only reads when
// they typed a name: two lines, the second naming verb with a shape after it.
// A verb too long for the second line to fit 80 columns drops out of it.
func AimShapesHint(verb string) string {
	try := fmt.Sprintf(`Try <ansi fg="command">%s shape</ansi> or <ansi fg="command">%s 2.shape</ansi>.`, verb, verb)
	if verb == `` || len(`Try  shape or  2.shape.`)+2*len([]rune(verb)) > aimHintWidth {
		try = `Use <ansi fg="command">shape</ansi> or <ansi fg="command">2.shape</ansi> in place of a name.`
	}
	return aimShapesLine + "\n" + try
}

// aimNamesAShape reports whether the player typed the shape word at all,
// including `all.shape`, which names no single figure. aimShapeIndex returns 0
// for that, and without this the refusal would call it a typed name.
func aimNamesAShape(name string) bool {
	if name == `` {
		return false
	}
	word, _ := util.GetMatchNumber(name)
	return word == shapeWord
}

// aimShapeIndex is N for `shape`, `N.shape` or `shape#N`, and 0 otherwise.
func aimShapeIndex(name string) int {
	if name == `` {
		return 0
	}
	word, n := util.GetMatchNumber(name)
	if word != shapeWord || n < 1 {
		return 0
	}
	return n
}

// aimIdForm reports whether name is "@<userId>" or "#<mobInstanceId>", the
// form a shape is rewritten to. FindByNameSeenBy still applies the viewer's
// Perceives to it, so it reaches exactly the figures a shape reaches.
func aimIdForm(name string) bool {
	if len(name) < 2 || (name[0] != '@' && name[0] != '#') {
		return false
	}
	n, err := strconv.Atoi(name[1:])
	return err == nil && n > 0
}

// aimFigures lists what the viewer makes out as shapes: the other players
// they perceive, in room order, then the mobs they perceive, in room order.
// Each is a name FindByName resolves: "@<userId>" or "#<mobInstanceId>".
func aimFigures(viewer *characters.Character, selfUserId int, room *rooms.Room) []string {
	figures := []string{}
	for _, uid := range room.GetPlayers() {
		if uid == selfUserId {
			continue
		}
		if u := users.GetByUserId(uid); u != nil && viewer.Perceives(u.Character) {
			figures = append(figures, fmt.Sprintf(`@%d`, uid))
		}
	}
	for _, mid := range room.GetMobs() {
		if m := mobs.GetInstance(mid); m != nil && viewer.Perceives(&m.Character) {
			figures = append(figures, fmt.Sprintf(`#%d`, mid))
		}
	}
	return figures
}

// UserActorAtSight is the actor a player command that names a creature hands
// to its action (#454). At clear sight, or with no sight (where nothing
// resolves), it is &UserActor{User: user, Room: room}. A player who makes out
// shapes only aimed at a shape and never learned the name, so every line the
// action tells them anonymizes the identity tags it carries ("a figure"), as
// a room line already does for them.
func UserActorAtSight(user *users.UserRecord, room *rooms.Room) Actor {
	actor := &UserActor{User: user, Room: room}
	if room == nil || messaging.ParticipantSight(user.Character, room) != messaging.SightShapes {
		return actor
	}
	return &shapesUserActor{Actor: actor}
}

// shapesUserActor is a UserActor whose own lines name no one (see
// UserActorAtSight).
type shapesUserActor struct {
	Actor
}

func (a *shapesUserActor) SendText(cat messaging.Category, msg string) {
	a.Actor.SendText(cat, messaging.Anonymize(msg))
}
