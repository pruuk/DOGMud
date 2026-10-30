package mobs

import (
	"math"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// IdleClass is which of the two idle rolls an idle command belongs to (#317).
type IdleClass int

const (
	// IdleChatter is anything seen or heard in place: emote, say, consume,
	// cast and the rest.
	IdleChatter IdleClass = iota
	// IdleMovement is the movement roll: wander and portal, which leave the
	// room, and sneak. Sneak hides in place, but it rides the movement roll
	// so the chatter scale does not change how often thieves hide. It is
	// not a move, and ambient telemetry never counts it as one.
	IdleMovement
)

// idleVerbClass classifies every idle verb shipped content uses. A verb not
// listed rolls as chatter; internal/guards/idle_verb_class_guard_test.go
// fails on one in shipped content, so a new verb is added here first.
var idleVerbClass = map[string]IdleClass{
	"emote":   IdleChatter,
	"say":     IdleChatter,
	"consume": IdleChatter,
	"cast":    IdleChatter,
	"wear":    IdleChatter,
	"alchemy": IdleChatter,
	"wander":  IdleMovement,
	"sneak":   IdleMovement,
	"portal":  IdleMovement,
}

// IdleVerb is an idle command's verb: its first word, lower case, cut at a
// ';' so "wander;wander" reads as "wander". A blank entry has none.
func IdleVerb(entry string) string {
	fields := strings.Fields(entry)
	if len(fields) == 0 {
		return ""
	}
	verb := strings.ToLower(fields[0])
	if i := strings.Index(verb, ";"); i >= 0 {
		verb = verb[:i]
	}
	return verb
}

// ClassifyIdleVerb returns verb's class and whether the table knows it. An
// unknown verb rolls as chatter.
func ClassifyIdleVerb(verb string) (IdleClass, bool) {
	c, ok := idleVerbClass[verb]
	if !ok {
		return IdleChatter, false
	}
	return c, true
}

// IdlePool is an idle pool split by class. Size counts every entry, blanks
// included: blank entries are the authors' rate dial.
type IdlePool struct {
	Chatter  []string
	Movement []string
	Size     int
}

// SplitIdlePool splits entries into chatter and movement.
func SplitIdlePool(entries []string) IdlePool {
	p := IdlePool{Size: len(entries)}
	for _, e := range entries {
		verb := IdleVerb(e)
		if verb == "" {
			continue
		}
		if c, _ := ClassifyIdleVerb(verb); c == IdleMovement {
			p.Movement = append(p.Movement, e)
		} else {
			p.Chatter = append(p.Chatter, e)
		}
	}
	return p
}

// IdleRollInput carries the multipliers of one idle round: the two global
// scales (Balance.MobIdleChatterScale, MobIdleMovementScale), the mob's room
// zone's ambientmultiplier and wandermultiplier, and the mob's own
// wandermultiplier. 1.0 everywhere is the authored rate.
type IdleRollInput struct {
	ChatterScale  float64
	MovementScale float64
	ZoneAmbient   float64
	ZoneWander    float64
	MobWander     float64
}

// IdleChances is each class's chance per round (0 to 1), weighted by its
// share of the pool so that at every multiplier 1.0 a mob speaks and moves
// as often as the old single roll let it (#317):
//
//	chatter  = AL x ChatterScale x ZoneAmbient x chatter entries / pool size
//	movement = AL x MovementScale x MobWander x ZoneWander x movement entries / pool size
func IdleChances(activityLevel int, p IdlePool, in IdleRollInput) (chatter, movement float64) {
	if p.Size == 0 || activityLevel <= 0 {
		return 0, 0
	}
	al := float64(activityLevel) / 100
	size := float64(p.Size)
	chatter = al * in.ChatterScale * in.ZoneAmbient * float64(len(p.Chatter)) / size
	movement = al * in.MovementScale * in.MobWander * in.ZoneWander * float64(len(p.Movement)) / size
	return clampChance(chatter), clampChance(movement)
}

func clampChance(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 1 {
		return 1
	}
	return p
}

// idleRollScale is the resolution of one idle roll: a chance is drawn out of
// a million so a fraction such as 0.03125 is kept.
const idleRollScale = 1_000_000

func rollChance(p float64, randN func(int) int) bool {
	if p <= 0 {
		return false
	}
	if p >= 1 {
		return true
	}
	return randN(idleRollScale) < int(math.Round(p*idleRollScale))
}

// RollIdle makes this round's two independent idle rolls and returns the
// chatter command and the movement command each picked ("" when that roll
// failed or its class is empty). A mob may now speak and move in one round;
// the caller looks for trouble only when both are "". ActivityLevel is read,
// never written: combat AI reads it too. randN is util.Rand in production.
func (m *Mob) RollIdle(in IdleRollInput, randN func(int) int) (chatter, movement string) {
	pool := SplitIdlePool(m.IdleCommands)
	pc, pm := IdleChances(m.ActivityLevel, pool, in)
	if rollChance(pc, randN) {
		chatter = pool.Chatter[randN(len(pool.Chatter))]
	}
	if rollChance(pm, randN) {
		movement = pool.Movement[randN(len(pool.Movement))]
	}
	return chatter, movement
}

// WanderScale is the mob's wandermultiplier: 1.0 when unset.
func (m *Mob) WanderScale() float64 {
	if m.WanderMultiplier == nil {
		return 1.0
	}
	return *m.WanderMultiplier
}

// CommandNoSplit queues inputTxt as ONE command, never split on ';'. The
// idle handler sends an ambient emote or say this way (#317): authored text
// may hold a ';' ("...into the quench trough; steam rises"), and Command
// would cut it into a second, unknown command and lose the rest.
func (m *Mob) CommandNoSplit(inputTxt string) {
	readyTurn := util.GetTurnCount()
	if readyTurn > m.lastCommandTurn {
		m.lastCommandTurn = readyTurn
	}
	events.AddToQueue(events.Input{
		MobInstanceId: m.InstanceId,
		InputText:     inputTxt,
		ReadyTurn:     m.lastCommandTurn,
	})
}
