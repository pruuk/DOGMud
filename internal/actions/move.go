package actions

import (
	"fmt"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/combat"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/contest"
	"github.com/GoMudEngine/GoMud/internal/messaging"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/state"
	"github.com/GoMudEngine/GoMud/internal/state/awareness"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// Movement parity 4b. The price of one room step, and the hidden-detection
// contests on arrival, are shared by players and mobs here. The command
// wrappers (usercommands.Go, mobcommands.Go) keep their own gates, lock
// handling and narration and call these once, right before relocating.

// MoveRefusal says why a step was not paid for.
type MoveRefusal int

const (
	MoveOK MoveRefusal = iota
	MoveRefuseEncumbered
	MoveRefuseTired
	MoveRefuseExhausted
)

// MoveCharge reports one priced step.
type MoveCharge struct {
	Refusal     MoveRefusal
	ActionCost  int     // 10, or 50 over carry capacity
	StaminaCost float64 // fractional stamina price, banked through the carry
	Winded      bool    // paid, and stamina is now under a quarter of its reachable max
	Never       bool    // refused, and this actor could not pay the step even fully rested
}

// OK reports whether the step was (or, for a quote, would be) paid.
func (m MoveCharge) OK() bool { return m.Refusal == MoveOK }

// The hardcoded step prices (fact V2). Not knobs; the spec keeps them as they
// are.
const (
	moveActionCost           = 10
	moveEncumberedActionCost = 50
)

// MovePrice is the price of one step into dest for c: action points, and the
// fractional stamina cost. It reads the destination biome by NAME
// (rooms.GetBiome(dest.Biome)), exactly as the player path always has.
func MovePrice(c *characters.Character, dest *rooms.Room) (int, float64) {
	actionCost := moveActionCost
	if c.GetCarriedWeight() > c.CarryCapacity() {
		actionCost = moveEncumberedActionCost
	}
	terrain := 1.0
	if biome, _ := rooms.GetBiome(dest.Biome); biome != nil {
		terrain = biome.GetMovementCost()
	}
	stamina := c.GetMovementStaminaCost(terrain)
	if mutations.IsFlying(c.Mutations) {
		// Winged Flight glides over terrain: movement barely tires you.
		stamina *= float64(configs.GetBalanceConfig().FlightMoveStaminaMult)
	}
	return actionCost, stamina
}

// settleMoverActionPoints brings a mob's points up to date. Players are
// credited per turn by hooks.ActionPoints and must never be settled here.
func settleMoverActionPoints(actor Actor) {
	if actor.IsPlayer() {
		return
	}
	actor.GetCharacter().SettleActionPoints(util.GetTurnCount())
}

func apRefusal(actionCost int) MoveRefusal {
	if actionCost == moveEncumberedActionCost {
		return MoveRefuseEncumbered
	}
	return MoveRefuseTired
}

// QuoteMove prices a step without paying for it, for callers that must decide
// before issuing one (the path walker, wander, behaviour-tree steps, the AI
// companion). Settling a mob's points is bookkeeping, not a charge.
func QuoteMove(actor Actor, dest *rooms.Room) MoveCharge {
	c := actor.GetCharacter()
	settleMoverActionPoints(actor)
	ap, st := MovePrice(c, dest)
	q := MoveCharge{ActionCost: ap, StaminaCost: st}
	switch {
	case c.ActionPoints < ap:
		q.Refusal = apRefusal(ap)
		q.Never = ap > c.ActionPointsMax.Value
	case !c.CanAffordCostFloat(characters.PoolStamina, st):
		q.Refusal = MoveRefuseExhausted
		q.Never = st > float64(c.EffectivePoolMax(characters.PoolStamina))
	}
	return q
}

// ChargeMove pays for one step into dest: action points first, then stamina,
// refunding the points when stamina refuses. Wrappers call it AFTER their
// lock checks and any exit-message requeue, so a door that stays locked costs
// nothing and a requeued step is charged once (fact V4).
func ChargeMove(actor Actor, dest *rooms.Room) MoveCharge {
	c := actor.GetCharacter()
	settleMoverActionPoints(actor)
	ap, st := MovePrice(c, dest)
	m := MoveCharge{ActionCost: ap, StaminaCost: st}
	if !c.DeductActionPoints(ap) {
		m.Refusal = apRefusal(ap)
		m.Never = ap > c.ActionPointsMax.Value
		return m
	}
	if !c.ApplyCostFloatOrRefuse(characters.PoolStamina, st) {
		c.ActionPoints += ap
		m.Refusal = MoveRefuseExhausted
		m.Never = st > float64(c.EffectivePoolMax(characters.PoolStamina))
		return m
	}
	// EffectivePoolMax, not the raw max: current stamina is reserve-clamped,
	// so a raw denominator nags a reserved character at a full pool.
	m.Winded = c.Stamina < c.EffectivePoolMax(characters.PoolStamina)/4
	return m
}

// QuoteMobStep quotes the step a mob would take through exitName from the room
// it stands in. A step that does not resolve (no room, no such exit, no
// destination, or `home`) quotes as affordable, so the caller's existing
// handling of a bad step is unchanged.
func QuoteMobStep(mob *mobs.Mob, exitName string) MoveCharge {
	room := rooms.LoadRoom(mob.Character.RoomId)
	if room == nil {
		return MoveCharge{}
	}
	ex := FindExit(room, exitName)
	if !ex.Found {
		return MoveCharge{}
	}
	dest := rooms.LoadRoom(ex.RoomId)
	if dest == nil {
		return MoveCharge{}
	}
	return QuoteMove(NewMobActorInRoom(mob, room), dest)
}

// movementTrainsSearch reports whether this move should record a search use.
// Moved unchanged from usercommands/go.go by movement parity 4b; the long
// rationale lives in internal/actions/context.md ("Movement").
//
// A zero or negative MovementSearchTrainChance switches the feature off.
func movementTrainsSearch() bool {
	chance := float64(configs.GetBalanceConfig().MovementSearchTrainChance)
	if chance <= 0 {
		return false
	}
	// Resolving against 100,000 keeps a knob as small as 0.00001 meaningful.
	const resolution = 100000
	return util.Rand(resolution) < int(chance*resolution)
}

// TrainSearchOnMove is the rare Search training a completed step earns. Walking
// is not a contest, so won is always true; the rarity gate is the rule.
func TrainSearchOnMove(actor Actor) {
	if movementTrainsSearch() {
		actor.AwardResolved(true, actor.GetCharacter().CandidateFor(string(skills.Search)))
	}
}

// EntryDetectionResult reports what arrival detection left behind.
type EntryDetectionResult struct {
	// StillSneaking is false once a sneaking mover has been spotted.
	StillSneaking bool
}

// EntryDetection runs the hidden-detection contests for a mover that has just
// arrived in dest, the same for a player or a mob (movement parity 4b, owner
// ruling 3: symmetric both ways).
//
// A sneaking mover is rolled against every observer in dest, players first
// (the one who spots it is told) and then mobs (silent), skipping the mover's
// allies (alliesOf). Once it is not sneaking, whether it never was or was just
// spotted, the mover rolls to spot every hidden player and mob in dest (a mob
// mover skipping its allies) and earns a Search award on BOTH outcomes
// (U10b-2).
//
// Moved from usercommands.Go. The contests keep their shape exactly; lines to
// players go through each player's own actor and room lines through
// SendTextVisual, so a mob mover's lines differ only in its name colour.
func EntryDetection(mover Actor, dest *rooms.Room, sneaking bool) EntryDetectionResult {
	// The light is invariant across every occupant, so it is composed once.
	light := messaging.FixedLight(dest.LightLevel())

	if sneaking && sneakerSpotted(mover, dest, light) {
		mc := mover.GetCharacter()
		// Drive the Awareness FSM out of Hidden; the mirror cascade in
		// Awareness_Cascades.go clears the Hidden condition. Calling
		// CancelConditionsWithFlag directly would expire the condition but
		// leave the FSM in Hidden. Silent to the mover: if the observer is
		// itself hidden, naming it leaks what the mover cannot see; the
		// Hidden condition's own end text is the signal.
		_ = mc.Awareness.TransitionToRevealing(
			state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
		mc.SetMiscData(`sneaking`, nil)
		sneaking = false
	}

	if !sneaking {
		newcomerSpots(mover, dest, light)
	}

	return EntryDetectionResult{StillSneaking: sneaking}
}

// moverName is the mover's name in its identity colour.
func moverName(mover Actor) string {
	if mover.IsPlayer() {
		return fmt.Sprintf(`<ansi fg="username">%s</ansi>`, mover.GetName())
	}
	return fmt.Sprintf(`<ansi fg="mobname">%s</ansi>`, mover.GetName())
}

// moverAllies is the mover's side: allies do not expose a sneaker, and a mob
// newcomer does not roll to spot them.
type moverAllies struct {
	users map[int]bool
	mobs  map[int]bool
}

// addParty marks every member of p, players and mobs.
func (a moverAllies) addParty(p *parties.Party) {
	if p == nil {
		return
	}
	for _, uid := range p.GetMembers() {
		a.users[uid] = true
	}
	for _, member := range p.Members {
		if member.IsPlayer() {
			a.users[member.GetUserId()] = true
		} else if id := member.GetMobInstanceId(); id != 0 {
			a.mobs[id] = true
		}
	}
}

// alliesOf is a player mover's side: its party's players (as on master) and
// its own charmed mobs and companions (every companion path tracks its mob in
// the charm ids). For a mob mover it is its NPC party, and when charmed its
// owner, the owner's party and the owner's other charmed mobs and companions.
func alliesOf(mover Actor) moverAllies {
	a := moverAllies{users: map[int]bool{}, mobs: map[int]bool{}}
	if mover.IsPlayer() {
		if p := parties.Get(mover.GetUserId()); p != nil {
			for _, uid := range p.GetMembers() {
				a.users[uid] = true
			}
		}
		for _, id := range mover.GetCharacter().GetCharmIds() {
			a.mobs[id] = true
		}
		return a
	}
	a.addParty(parties.GetByMobInstanceId(mover.GetMobInstanceId()))
	if ownerId := mover.GetCharacter().GetCharmedUserId(); ownerId > 0 {
		a.users[ownerId] = true
		// parties.Get also returns a party the owner is only invited to.
		if p := parties.Get(ownerId); p != nil && p.IsMember(ownerId) {
			a.addParty(p)
		}
		if owner := users.GetByUserId(ownerId); owner != nil {
			for _, id := range owner.Character.GetCharmIds() {
				a.mobs[id] = true
			}
		}
	}
	return a
}

// sneakerSpotted rolls a sneaking mover against dest's observers. The sneak
// score is computed per observer so a nightvision observer applies the right
// light modifier.
func sneakerSpotted(mover Actor, dest *rooms.Room, light messaging.RoomVisibility) bool {
	mc := mover.GetCharacter()
	allies := alliesOf(mover)

	for _, pId := range dest.GetPlayers() {
		if pId == mover.GetUserId() || allies.users[pId] {
			continue
		}
		p := users.GetByUserId(pId)
		if p == nil {
			continue
		}
		sneakScore := CalcSneakScoreVsObserver(mc, p.Character, light)
		observerScore := CalcDetectionScore(p.Character, dest)
		if !combat.RunContest(sneakScore, []contest.Entry{{Score: observerScore}}).Success {
			NewUserActor(p).SendText(messaging.CategorySystem, fmt.Sprintf(
				`%s slips into the room but you notice them.`, moverName(mover)))
			return true
		}
	}

	for _, mId := range dest.GetMobs() {
		if mId == mover.GetMobInstanceId() || allies.mobs[mId] {
			continue
		}
		m := mobs.GetInstance(mId)
		if m == nil {
			continue
		}
		sneakScore := CalcSneakScoreVsObserver(mc, &m.Character, light)
		observerScore := CalcDetectionScore(&m.Character, dest)
		if !combat.RunContest(sneakScore, []contest.Entry{{Score: observerScore}}).Success {
			return true
		}
	}

	return false
}

// newcomerSpots rolls the arriving mover to spot every hidden player and mob
// in dest. Neither side learns a name it cannot see. A mob mover skips its
// allies (alliesOf), so a pet or party member following a hidden leader never
// reveals it or trains Search on it. A player mover skips only itself, as on
// master.
func newcomerSpots(mover Actor, dest *rooms.Room, light messaging.RoomVisibility) {
	mc := mover.GetCharacter()
	// The newcomer now stands in dest: that is the light their eyes meet.
	observerScore := CalcDetectionScore(mc, dest)
	allies := moverAllies{}
	if !mover.IsPlayer() {
		allies = alliesOf(mover)
	}

	for _, pId := range dest.GetPlayers() {
		if pId == mover.GetUserId() || allies.users[pId] {
			continue
		}
		hiddenP := users.GetByUserId(pId)
		if hiddenP == nil || !hiddenP.Character.IsHidden() {
			continue
		}
		hiddenScore := CalcSneakScoreVsObserver(hiddenP.Character, mc, light)
		success := combat.RunContest(observerScore, []contest.Entry{{Score: hiddenScore}}).Success
		if success {
			_ = hiddenP.Character.Awareness.TransitionToRevealing(
				state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
			hiddenP.Character.SetMiscData(`sneaking`, nil)
			hider := NewUserActor(hiddenP)
			if messaging.CanSeeClearly(hiddenP.Character, dest) {
				hider.SendText(messaging.CategorySystem, fmt.Sprintf(
					`%s enters the room and notices you!`, moverName(mover)))
			} else {
				hider.SendText(messaging.CategorySystem,
					`Someone enters the room and notices you!`)
			}
			if messaging.CanSeeClearly(mc, dest) {
				mover.SendText(messaging.CategorySystem, fmt.Sprintf(
					`You notice <ansi fg="username">%s</ansi> lurking in the shadows.`,
					hiddenP.Character.Name))
			} else {
				mover.SendText(messaging.CategorySystem,
					`You notice someone lurking in the shadows.`)
			}
		}
		// U10b-2: the Search award fires on BOTH outcomes, full on a win and
		// partial on a resolved loss. Outside the success branch on purpose:
		// a win-only award is the defect the firing convention removes. Still
		// opportunity-gated: no hidden occupant, no contest, no award.
		mover.AwardResolved(success, mc.CandidateFor(string(skills.Search)))
	}

	for _, mId := range dest.GetMobs(rooms.FindAll) {
		if mId == mover.GetMobInstanceId() || allies.mobs[mId] {
			continue
		}
		m := mobs.GetInstance(mId)
		if m == nil || !m.Character.IsHidden() {
			continue
		}
		hiddenScore := CalcSneakScoreVsObserver(&m.Character, mc, light)
		success := combat.RunContest(observerScore, []contest.Entry{{Score: hiddenScore}}).Success
		if success {
			_ = m.Character.Awareness.TransitionToRevealing(
				state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
			// Spotting something is not the same as identifying it.
			if messaging.CanSeeClearly(mc, dest) {
				mover.SendText(messaging.CategorySystem, fmt.Sprintf(
					`You notice <ansi fg="mobname">%s</ansi> lurking in the shadows!`,
					m.Character.Name))
			} else {
				mover.SendText(messaging.CategorySystem,
					`You notice something lurking in the shadows!`)
			}
			// SendTextVisual, not SendText: a sight event. The audio channel
			// bypasses the sight gate and the anonymizer; Visual gets each
			// bystander the version their eyes allow, or nothing.
			dest.SendTextVisual(messaging.CategorySystem, fmt.Sprintf(
				`%s spots <ansi fg="mobname">%s</ansi> hiding in the shadows!`,
				moverName(mover), m.Character.Name),
				mover.GetUserId())
		}
		mover.AwardResolved(success, mc.CandidateFor(string(skills.Search)))
	}
}

// SpotHiddenPlayer is a watcher standing in room looking for one hidden
// player: the same contest entry detection rolls (the watcher's detection
// score against the hider's sneak score, in the room's light). On success the
// player is revealed (the Awareness machine, as when a newcomer spots them)
// and told so; true. A player not hidden is already seen: true, no roll.
// Used by watchers that stay put and keep looking (a rift's hunter).
func SpotHiddenPlayer(watcher Actor, room *rooms.Room, userId int) bool {
	u := users.GetByUserId(userId)
	if u == nil || room == nil || u.Character.RoomId != room.RoomId {
		return false
	}
	if !u.Character.IsHidden() {
		return true
	}
	wc := watcher.GetCharacter()
	light := messaging.FixedLight(room.LightLevel())
	observerScore := CalcDetectionScore(wc, room)
	hiddenScore := CalcSneakScoreVsObserver(u.Character, wc, light)
	success := combat.RunContest(observerScore, []contest.Entry{{Score: hiddenScore}}).Success
	watcher.AwardResolved(success, wc.CandidateFor(string(skills.Search)))
	if !success {
		return false
	}
	_ = u.Character.Awareness.TransitionToRevealing(
		state.TransitionReason{Trigger: awareness.TriggerObserverSearch})
	u.Character.SetMiscData(`sneaking`, nil)
	hider := NewUserActor(u)
	if messaging.CanSeeClearly(u.Character, room) {
		hider.SendText(messaging.CategorySystem, fmt.Sprintf(`%s turns, and its attention settles on you.`, moverName(watcher)))
	} else {
		hider.SendText(messaging.CategorySystem, `Something in the dark turns, and its attention settles on you.`)
	}
	return true
}
