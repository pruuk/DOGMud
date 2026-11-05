package aicompanion

import (
	"fmt"
	"time"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/companionai"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Trips (F11.5 to F11.7, F7.10). A trip is a route over the companion's own
// map, walked one ordinary `go <exit>` at a time. Each step waits for the
// companion to actually arrive before the next is issued. The owner moving
// pulls the companion along (the engine's companion transport), which
// cancels an errand; a blocked or wrong exit is recorded and the route is
// planned again from wherever the companion now stands.

// travelPlan is a trip in progress.
type travelPlan struct {
	Dest       int
	DestName   string
	Purpose    string // errand, explore, return
	Steps      []step
	Next       int
	FromRoom   int    // room the current step was issued from
	Expect     int    // room the current step should reach; -1 any new room; 0 none issued
	Issued     uint64 // round the current step was issued
	Replans    int
	TriedKey   bool   // a locked way on this step has already had the key tried
	Authorized bool   // the owner asked for this errand
	Errand     string // what she set out to do there, in her own words
	Resting    bool   // already told her mind she stopped to rest on this trip
}

const (
	stepTimeoutRounds = 3
	maxReplans        = 2
	returnMaxSteps    = 60
)

// tooTiredLine is what a companion remembers when it stops to rest mid-trip.
const tooTiredLine = `You are too tired to go on, and stop to catch your breath.`

// stepQuote prices a step through exitName for the companion right now
// (movement parity 4b): OK when she can pay, Never when she could not pay it
// even fully rested. A variable so tests can pin it.
var stepQuote = func(mob *mobs.Mob, exitName string) actions.MoveCharge {
	return actions.QuoteMobStep(mob, exitName)
}

// startTravel plans a route and begins a trip. It returns a reason when the
// trip cannot start.
func (m *AICompanionModule) startTravel(c *controller, mob *mobs.Mob, dest int, purpose string, authorized bool) string {
	from := mob.Character.RoomId
	if from == dest {
		return `already there`
	}
	maxSteps := m.cfg.MaxErrandSteps
	if purpose == `return` {
		maxSteps = returnMaxSteps
	}
	path, ok := findPath(c.mind.Map, from, dest, maxSteps)
	if !ok || len(path) == 0 {
		return `you know no way there`
	}
	name := fmt.Sprintf(`room %d`, dest)
	if rec := c.mind.Map[dest]; rec != nil && rec.Title != `` {
		name = rec.Title
	}
	c.travel = &travelPlan{
		Dest: dest, DestName: name, Purpose: purpose, Steps: path,
		FromRoom: from, Authorized: authorized,
	}
	return ``
}

// startExplore begins a one-step trip through an exit the companion has not
// walked.
func (m *AICompanionModule) startExplore(c *controller, mob *mobs.Mob, exitName string, authorized bool) string {
	rec := c.mind.Map[mob.Character.RoomId]
	if rec == nil {
		return `you have not taken stock of this place yet`
	}
	// `go` takes a mob through a room_gate, so an errand stops at one here.
	if room := rooms.LoadRoom(mob.Character.RoomId); room != nil {
		if info, ok := room.GetExitInfo(exitName); ok && actions.RoomGateGuards(info.RoomId) {
			return `the way ` + exitName + ` is closed to you`
		}
	}
	for _, name := range unexploredExits(rec) {
		if name == exitName {
			c.travel = &travelPlan{
				Dest: 0, DestName: `whatever lies ` + exitName, Purpose: `explore`,
				Steps: []step{{Exit: exitName, To: 0}}, FromRoom: mob.Character.RoomId, Authorized: authorized,
			}
			return ``
		}
	}
	return `no unexplored way called ` + exitName + ` here`
}

// endTravel stops a trip and tells the companion why.
func (m *AICompanionModule) endTravel(c *controller, why string, tellModel bool) {
	c.travel = nil
	c.mind.addLine(Line{Kind: `event`, Text: why}, m.cfg.WorkingMemoryLines)
	c.dirty = true
	if tellModel {
		c.push(stimulus{Kind: `trip`, Text: why})
	}
}

// endTooWeak ends a trip whose next step the companion could not pay for even
// fully rested (the quote's Never), the way any failed trip ends. Nothing is
// charged to the map: the exit is not at fault.
func (m *AICompanionModule) endTooWeak(c *controller) {
	p := c.travel
	m.endTravel(c, `You are too weak to go on toward `+p.DestName+`, however long you rest.`, p.Purpose != `return`)
}

// advanceTravel moves a trip on by at most one step. Called every round.
func (m *AICompanionModule) advanceTravel(c *controller, mob *mobs.Mob, owner *users.UserRecord, round uint64) {
	p := c.travel
	cur := mob.Character.RoomId

	if mob.Character.IsInCombat() {
		m.endTravel(c, `A fight cut your trip to `+p.DestName+` short.`, false)
		return
	}

	if p.Expect != 0 {
		arrived := cur == p.Expect || (p.Expect == -1 && cur != p.FromRoom)
		switch {
		case arrived:
			p.Next++
			p.Expect = 0
			p.FromRoom = cur

		case cur != p.FromRoom:
			// Somewhere unexpected: pulled back by the owner, or a guessed
			// exit led somewhere else (recordRoom has already corrected the
			// map from the exit actually used).
			if owner != nil && cur == owner.Character.RoomId && p.Purpose != `return` {
				m.endTravel(c, `You were called back to `+owner.Character.Name+` before you reached `+p.DestName+`.`, false)
				return
			}
			if !m.replan(c, mob) {
				return
			}

		case round >= p.Issued+stepTimeoutRounds:
			// Movement parity 4b: a step that did not move because she
			// could not pay for it is a rest, not a bad exit. Re-quote next
			// round; never charge exhaustion to the map.
			if q := stepQuote(mob, p.Steps[p.Next].Exit); !q.OK() {
				if q.Never {
					m.endTooWeak(c)
					return
				}
				p.Expect = 0
				return
			}
			// Did not move. A locked door she has the key for is worth one
			// try before the way is written off.
			if !p.TriedKey && m.tryUnlock(c, mob, p.Steps[p.Next].Exit) {
				p.TriedKey = true
				p.Issued = round
				mob.Command(`go ` + util.EscapeAnsiTags(p.Steps[p.Next].Exit))
				return
			}
			if rec := c.mind.Map[p.FromRoom]; rec != nil {
				if er, ok := rec.Exits[p.Steps[p.Next].Exit]; ok {
					er.Fails++
				}
			}
			if p.Purpose == `explore` || !m.replan(c, mob) {
				if c.travel != nil {
					m.endTravel(c, `The way `+p.Steps[p.Next].Exit+` would not let you through.`, p.Purpose != `return`)
				}
				return
			}

		default:
			return // still waiting on the step
		}
	}

	if p.Next >= len(p.Steps) {
		c.travel = nil
		c.dirty = true
		if p.Purpose == `return` {
			name := `your companion`
			if owner != nil && owner.Character != nil {
				name = owner.Character.Name
			}
			if m.mayRemember(c) {
				c.mind.addLine(Line{Kind: `event`, Text: `You made your way back to ` + name + `.`}, m.cfg.WorkingMemoryLines)
			}
			return
		}
		here := p.DestName
		if rec := c.mind.Map[cur]; rec != nil && rec.Title != `` {
			here = rec.Title
		}
		c.mind.addLine(Line{Kind: `event`, Text: `You reached ` + here + `.`}, m.cfg.WorkingMemoryLines)
		c.apartSince = round
		// A merchant here has their stock read on arrival, so the wares
		// carry [s] refs in the very first decision she makes: otherwise a
		// trip to buy something spends its whole visit browsing.
		if room := rooms.LoadRoom(cur); room != nil {
			now := time.Now().Unix()
			for _, l := range browseShops(room) {
				c.mind.rememberShop(l, cur, now)
			}
		}
		c.lastErrand = p.Errand
		c.push(stimulus{Kind: `arrived`, Text: here, Authorized: p.Authorized, Errand: p.Errand})
		return
	}

	st := p.Steps[p.Next]
	if !safeExitName(st.Exit) {
		m.endTravel(c, `You could not find the way on.`, false)
		return
	}
	// Movement parity 4b: quote before issuing. Tired, she waits without
	// starting the step clock and tells her mind once per trip (a new trip is
	// a new plan, so the flag starts clear). A step she could not pay for
	// even fully rested ends the trip instead of waiting forever.
	if q := stepQuote(mob, st.Exit); !q.OK() {
		if q.Never {
			m.endTooWeak(c)
			return
		}
		if !p.Resting {
			p.Resting = true
			c.mind.addLine(Line{Kind: `event`, Text: tooTiredLine}, m.cfg.WorkingMemoryLines)
			c.dirty = true
		}
		return
	}
	mob.Command(`go ` + util.EscapeAnsiTags(st.Exit))
	p.FromRoom = cur
	p.Issued = round
	p.Expect = st.To
	if st.To == 0 {
		p.Expect = -1
	}
}

// replan finds a new route from where the companion stands. It ends the
// trip and returns false when there is none, or after too many tries.
func (m *AICompanionModule) replan(c *controller, mob *mobs.Mob) bool {
	p := c.travel
	if p.Dest == 0 || p.Replans >= maxReplans {
		m.endTravel(c, `You lost your way to `+p.DestName+`.`, p.Purpose != `return`)
		return false
	}
	maxSteps := m.cfg.MaxErrandSteps
	if p.Purpose == `return` {
		maxSteps = returnMaxSteps
	}
	path, ok := findPath(c.mind.Map, mob.Character.RoomId, p.Dest, maxSteps)
	if !ok {
		m.endTravel(c, `You lost your way to `+p.DestName+`.`, p.Purpose != `return`)
		return false
	}
	p.Steps, p.Next, p.Expect, p.FromRoom = path, 0, 0, mob.Character.RoomId
	p.Replans++
	return true
}

// headBack returns a companion that has been apart from its owner for
// ErrandLingerRounds, by its own map. If it knows no way back for RescueRounds
// it rejoins through the engine's companion transport, as though it had
// followed, rather than stay lost.
func (m *AICompanionModule) headBack(c *controller, mob *mobs.Mob, u *users.UserRecord, round uint64) {
	if c.apartSince == 0 {
		c.apartSince = round
		return
	}
	if c.inFlight || len(c.pending) > 0 || c.pendingAct != nil {
		return // still doing whatever she went for
	}
	apart := round - c.apartSince
	linger := uint64(m.cfg.ErrandLingerRounds)
	if c.lastErrand == `` {
		linger = 2 // she only wandered; there is nothing to stay for
	}
	if apart < linger {
		return
	}
	if reason := m.startTravel(c, mob, u.Character.RoomId, `return`, false); reason == `` {
		c.lastErrand = ``
		if m.mayRemember(c) {
			c.mind.addLine(Line{Kind: `event`, Text: `You started back toward ` + u.Character.Name + `.`}, m.cfg.WorkingMemoryLines)
			c.dirty = true
		}
		return
	}
	// She could not work out a way back on her own. Try again each round:
	// her owner may move somewhere she does know. Only when she has been
	// lost a long while does the engine put her back beside them, because a
	// companion that blinks across the world is not a companion, it is a
	// convenience.
	if apart >= uint64(m.cfg.RescueRounds) && companionai.Rejoin(u.UserId) {
		c.apartSince = 0
		if m.mayRemember(c) {
			c.mind.addLine(Line{Kind: `event`, Text: `You were lost for a long while before you found ` + u.Character.Name + ` again.`},
				m.cfg.WorkingMemoryLines)
			c.dirty = true
		}
		mudlog.Info(`aicompanion`, `action`, `rescue`, `owner`, u.UserId, `roundsLost`, apart)
	}
}

// tryUnlock opens a door on the route when the companion is carrying the
// key for it. It reports whether the way is now open.
func (m *AICompanionModule) tryUnlock(c *controller, mob *mobs.Mob, exitName string) bool {
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil || !safeExitName(exitName) {
		return false
	}
	info, ok := room.GetExitInfo(exitName)
	if !ok || !info.Lock.IsLocked() {
		return false
	}
	mob.Command(cmdCompanionUnlock + ` ` + util.EscapeAnsiTags(exitName))
	c.mind.addLine(Line{Kind: `event`, Text: `You tried your keys on the ` + exitName + ` door.`}, m.cfg.WorkingMemoryLines)
	c.dirty = true
	return true
}
