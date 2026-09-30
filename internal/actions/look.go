package actions

import (
	"strings"

	"github.com/GoMudEngine/GoMud/internal/keywords"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/rooms"
)

// LookKind is what a look resolved to.
type LookKind int

const (
	LookDark        LookKind = iota // the looker sees nothing at all here
	LookRoom                        // no target: the room itself
	LookCreature                    // a creature the looker perceives, at clear sight
	LookExit                        // an exit the looker can see through
	LookExitTooDark                 // an exit, but too dark to see through it
	LookExitLocked                  // an exit that is locked
	LookOther                       // anything else: each wrapper's own objects, in its own order
)

// LookResolution carries every sight rule of both looks (slice 5a).
type LookResolution struct {
	Kind  LookKind
	Sight messaging.SightDecision
	// NamesCreatures is Sight == SightFull: only then is a creature or a pet
	// named, so a shapes-only looker cannot confirm who is standing there.
	NamesCreatures bool
	Target         Actor  // LookCreature
	LookAt         string // the target, after a direction alias resolved to an exit
	ExitName       string // the LookExit kinds
	ExitRoomId     int    // LookExit
	// PetUserId is the owner of a pet the looker may name here (clear sight
	// only), 0 otherwise. It is not a kind because the player resolves the
	// pet AFTER carried items and room nouns; each wrapper reads it at its
	// own pet step.
	PetUserId int
}

// ResolveLook is the shared look resolution for players and mobs. Order is
// the player's: sight, no target, creature (resolved with the looker as
// viewer, so a creature it does not perceive cannot be named), then a sealed
// crate or a known container defers to the wrapper (LookOther) before any
// exit does, then the exit (with direction aliases), through-sight, lock.
// It has no side effects.
func ResolveLook(actor Actor, lookAt string) LookResolution {
	char := actor.GetCharacter()
	room := actor.GetRoom()
	res := LookResolution{Sight: messaging.ParticipantSight(char, room), LookAt: lookAt}
	res.NamesCreatures = res.Sight == messaging.SightFull

	if res.Sight == messaging.SightNone {
		res.Kind = LookDark
		return res
	}
	if lookAt == `` {
		res.Kind = LookRoom
		return res
	}

	if res.NamesCreatures {
		if target, err := ResolveTargetActor(room, lookAt, ResolveTargetOptions{Viewer: char}); err == nil {
			res.Kind, res.Target = LookCreature, target
			return res
		}
		res.PetUserId = room.FindByPetName(lookAt)
		if res.PetUserId == 0 && lookAt == `pet` && actor.IsPlayer() && char.Pet.Exists() {
			res.PetUserId = actor.GetUserId()
		}
	}

	if lookNamesAnObject(actor, lookAt) {
		res.Kind = LookOther
		return res
	}

	exitName, exitRoomId := room.FindExitByName(lookAt)
	if exitName == `` {
		if alias := keywords.TryDirectionAlias(lookAt); alias != lookAt {
			if exitName, exitRoomId = room.FindExitByName(alias); exitName != `` {
				res.LookAt = alias
			}
		}
	}
	// A routed exit (a housing door) leads each player somewhere different,
	// so there is no one room to peer into. Looking at it looks at the door
	// itself: drop the exit match and let the room's noun answer.
	if exitName != `` && rooms.IsRoutedExit(actor.GetUserId(), room.RoomId, exitName) {
		exitName = ``
	}
	if exitName == `` {
		res.Kind = LookOther
		return res
	}
	res.ExitName, res.ExitRoomId = exitName, exitRoomId

	// Seeing THROUGH an exit needs more light than seeing the room you are
	// standing in (messaging.SeesThroughExit; infra reach does not help).
	if !messaging.SeesThroughExit(char, room) {
		res.Kind = LookExitTooDark
		return res
	}
	if info, _ := room.GetExitInfo(exitName); info.Lock.IsLocked() {
		res.Kind = LookExitLocked
		return res
	}
	res.Kind = LookExit
	return res
}

// lookNamesAnObject reports a sealed crate or a room container the looker
// knows of: the player resolves those between the creature and the exit.
func lookNamesAnObject(actor Actor, lookAt string) bool {
	room := actor.GetRoom()
	if room.MatchesSealedCrate(strings.ToLower(lookAt)) {
		return true
	}
	name := room.FindContainerByName(lookAt)
	if name == `` {
		return false
	}
	if c, ok := room.Containers[name]; ok && c.Hidden {
		return actor.GetCharacter().HasDiscovery(room.RoomId, name)
	}
	return true
}
