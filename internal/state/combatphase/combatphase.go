// Package combatphase defines the Combat Phase state machine,
// the first consumer of internal/state. It replaces the
// Character.Aggro field as the source of truth for "who am I
// attacking?" and "am I in combat?".
package combatphase

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/state"
)

// State is the Combat Phase state enum.
type State int

const (
	Idle State = iota
	Engaging
	Engaged
	Disengaging
)

// String for logging / debugging.
func (s State) String() string {
	switch s {
	case Idle:
		return "Idle"
	case Engaging:
		return "Engaging"
	case Engaged:
		return "Engaged"
	case Disengaging:
		return "Disengaging"
	}
	return "Unknown"
}

// EngagingData is the state-data type for the Engaging state.
//
// U12c-2 deleted a `Reason state.TransitionReason` field from here. It had
// ZERO readers anywhere and was never set by the one production construction
// site, so it settled U10d's deferred either/or with the second branch: it was
// dead, so it went.
//
// ⚠️ The `r state.TransitionReason` PARAMETER on TransitionToEngaging is LIVE
// (it reaches m.inner.TransitionTo, and OpeningUnspent is keyed on its
// Trigger). Do not confuse the two, and do NOT repurpose either as a home for
// an engagement-kind enum: that moves the demotion bug this slice removed
// rather than killing it.
type EngagingData struct {
	Target      state.ActorRef
	RoundsUntil int // weapon WaitRounds before swing
}

// EngagedData is the state-data type for the Engaged state.
type EngagedData struct {
	Target      state.ActorRef
	NextSwingAt int // round number for next swing
}

// DisengagingData is the state-data type for the Disengaging state.
type DisengagingData struct {
	LastTarget state.ActorRef // target at time of flee
	FleeRound  int            // round flee was initiated
}

// vetoChain holds the registered veto functions. Each
// function returns true if the transition is OK, false if
// it should be vetoed.
type vetoChain struct {
	combatantSelf   func() bool               // self.Combatant
	activitySelf    func() bool               // self.Activity == Free
	lifeSelf        func() bool               // self.Life == Alive
	positionSelf    func() bool               // self.Position == Standing (for flee only)
	targetCombatant func(state.ActorRef) bool // target.Combatant
	targetLife      func(state.ActorRef) bool // target.Life == Alive
	targetPresence  func(state.ActorRef) bool // target.Presence available
}

// TWO round counters live on this machine, and they are NOT the same thing.
// Nothing said so before U12c-2, so each one read like the only one.
//
//	RoundsUntil    (EngagingData.RoundsUntil) is the ENGAGEMENT WIND-UP: how
//	               many rounds before the engagement becomes active.
//	               OnRoundTick decrements it and calls advanceToEngaged() at
//	               zero, which is also what fires the mob_engaged
//	               behaviour-tree event. It exists ONLY in Engaging.
//
//	roundsWaiting  (the Machine field) is the ACTOR'S ROUND BUDGET: how many
//	               rounds before this actor may act again.
//	               handleCombatWaitRound decrements it LATER IN THE SAME
//	               ROUND, and emits the wait messages.
//
// They are seeded identically by the commit path, so during wind-up they march
// in lockstep. That is a coincidence of seeding, not shared identity.
//
// They diverge in Engaged ON PURPOSE: RoundsUntil does not exist there, while
// the ~20 special-move `= 1` writes need a counter that still works once
// engaged. That is why roundsWaiting is a MACHINE field and not state data.
//
// OnRoundTick's Engaged branch is a DELIBERATE no-op. Making it decrement is
// the first step of unifying the two counters, not a bug fix.
//
// ⚠️ Unifying them is DEFERRED with a written reason (U12 spec §6.3.1): the two
// decrements happen at different moments in the round (OnRoundTick fires FIRST,
// from the round driver; handleCombatWaitRound runs later during resolution),
// so one counter shortens every weapon wind-up and every special-move recovery
// by one round unless compensated by seeding 2 where the code says 1. That is a
// balance change wearing a refactor's clothes. It is its own post-arc slice,
// and it must also relocate advanceToEngaged() and verify mob_engaged still
// fires at the same point.

// Machine wraps state.Machine[State] with Combat-Phase-specific
// API including per-state data storage and Attackers tracking.
//
// Per-state data and inbound attacker tracking are populated by
// the transition methods that arrive in Tasks 5-8. This Task 3
// only establishes the type with empty data slots and the basic
// State() / Inner() accessors.
type Machine struct {
	inner                    *state.Machine[State]
	self                     state.ActorRef // own identity, set by RegisterMachine
	engaging                 *EngagingData
	engaged                  *EngagedData
	disengaging              *DisengagingData
	attackers                []state.ActorRef // inbound attacker list
	attackersChangeListeners []func([]state.ActorRef)
	vetoes                   vetoChain
	tickEventListeners       []func(name string, r state.TransitionReason)
	roundsWaiting            int // see the two-counter note above
	openingUnspent           bool
}

// OpeningUnspent reports whether this engagement still carries its ambush
// opening -- the ONE swing of a surprise attack that crits on a clean win.
//
// U12c-2: this was Aggro.Type == SurpriseAttack, a value calculateCombat read
// and DEMOTED in the same breath. Splitting the query (here) from the
// consumption (SpendOpening) is what stops a casual reader spending an ambush
// by asking about it, and is why AttackResult.WasSurpriseAttack had to exist.
func (m *Machine) OpeningUnspent() bool { return m.openingUnspent }

// SpendOpening consumes the ambush opening and reports whether it was there to
// spend. Exactly ONE caller: the swing loop, on the swing that is THROWN.
//
// The engagement itself survives; only the opening is spent.
func (m *Machine) SpendOpening() bool {
	if !m.openingUnspent {
		return false
	}
	m.openingUnspent = false
	return true
}

// RoundsWaiting reports the actor's remaining round budget.
func (m *Machine) RoundsWaiting() int { return m.roundsWaiting }

// SetRoundsWaiting sets the actor's round budget. Negative values clamp to
// zero; every caller means "wait at least this long", never "act early".
func (m *Machine) SetRoundsWaiting(n int) {
	if n < 0 {
		n = 0
	}
	m.roundsWaiting = n
}

// ConsumeRoundWaiting decrements the budget by one and reports whether this
// round was consumed by the wait. False means the actor is free to act.
//
// Replaces the `if Aggro.RoundsWaiting <= 0 { return false }; RoundsWaiting--`
// pair in handleCombatWaitRound, so the guard and the decrement can no longer
// drift apart.
func (m *Machine) ConsumeRoundWaiting() bool {
	if m.roundsWaiting <= 0 {
		return false
	}
	m.roundsWaiting--
	return true
}

// NewMachine returns a Combat Phase machine in Idle.
func NewMachine() *Machine {
	return &Machine{
		inner: state.NewMachine(Idle, validTransitions),
	}
}

// State returns the current state.
func (m *Machine) State() State { return m.inner.State() }

// EngagingData returns the Engaging state's data if currently Engaging.
func (m *Machine) EngagingData() (EngagingData, bool) {
	if m.State() != Engaging || m.engaging == nil {
		return EngagingData{}, false
	}
	return *m.engaging, true
}

// EngagedData returns the Engaged state's data if currently Engaged.
func (m *Machine) EngagedData() (EngagedData, bool) {
	if m.State() != Engaged || m.engaged == nil {
		return EngagedData{}, false
	}
	return *m.engaged, true
}

// DisengagingData returns the Disengaging state's data if currently
// Disengaging.
func (m *Machine) DisengagingData() (DisengagingData, bool) {
	if m.State() != Disengaging || m.disengaging == nil {
		return DisengagingData{}, false
	}
	return *m.disengaging, true
}

// Attackers returns the inbound attacker list — characters
// currently Engaging or Engaged with this character as their
// target. Framework-maintained; do not mutate directly.
func (m *Machine) Attackers() []state.ActorRef {
	out := make([]state.ActorRef, len(m.attackers))
	copy(out, m.attackers)
	return out
}

// IsEngaged returns true if Combat Phase is Engaged.
func (m *Machine) IsEngaged() bool {
	return m.State() == Engaged
}

// IsInCombat returns true if Combat Phase is anything other
// than Idle. (Engaging, Engaged, and Disengaging all count.)
func (m *Machine) IsInCombat() bool {
	return m.State() != Idle
}

// CurrentTarget returns the ActorRef of the current target if
// any state has one (Engaging, Engaged, Disengaging), else zero.
func (m *Machine) CurrentTarget() state.ActorRef {
	switch m.State() {
	case Engaging:
		if m.engaging != nil {
			return m.engaging.Target
		}
	case Engaged:
		if m.engaged != nil {
			return m.engaged.Target
		}
	case Disengaging:
		if m.disengaging != nil {
			return m.disengaging.LastTarget
		}
	}
	return state.ActorRef{}
}

// Inner returns the underlying state.Machine — used by rules.go
// (Task 5+) to register vetoes/cascades. Not part of the stable
// API; do not depend on it from outside this package.
func (m *Machine) Inner() *state.Machine[State] {
	return m.inner
}

// === Machine registry ===
// Cross-character lookups for inbound attacker tracking, target-death
// cascades, etc. Real engine integration (Task 10) wires this from
// Character setup.

var (
	registryMu      sync.Mutex
	machineRegistry = map[state.ActorRef]*Machine{}
)

// RegisterMachine binds an ActorRef to its Machine.
func RegisterMachine(ref state.ActorRef, m *Machine) {
	registryMu.Lock()
	defer registryMu.Unlock()
	m.self = ref
	machineRegistry[ref] = m
}

// UnregisterMachine removes a binding (e.g. on logout or despawn).
func UnregisterMachine(ref state.ActorRef) {
	registryMu.Lock()
	defer registryMu.Unlock()
	delete(machineRegistry, ref)
}

// lookupMachine returns the registered Machine for ref, or nil.
func lookupMachine(ref state.ActorRef) *Machine {
	registryMu.Lock()
	defer registryMu.Unlock()
	return machineRegistry[ref]
}

// === Task 5 implementations ===

// TransitionToEngaging is the primary entry point into combat.
// Vetoes run before the inner framework transition.
// On success, stores EngagingData and notifies target's inbound list.
func (m *Machine) TransitionToEngaging(d EngagingData, r state.TransitionReason) error {
	// Vetoes — run before the framework's transition.
	if m.vetoes.combatantSelf != nil && !m.vetoes.combatantSelf() {
		return &state.VetoError{HandlerName: "combatant_self", Reason: "non-combatant"}
	}
	if m.vetoes.activitySelf != nil && !m.vetoes.activitySelf() {
		return &state.VetoError{HandlerName: "activity_self", Reason: "busy with activity"}
	}
	if m.vetoes.lifeSelf != nil && !m.vetoes.lifeSelf() {
		return &state.VetoError{HandlerName: "life_self", Reason: "not alive"}
	}
	if m.vetoes.targetCombatant != nil && !m.vetoes.targetCombatant(d.Target) {
		return &state.VetoError{HandlerName: "target_combatant", Reason: "target is non-combatant"}
	}
	if m.vetoes.targetLife != nil && !m.vetoes.targetLife(d.Target) {
		return &state.VetoError{HandlerName: "target_life", Reason: "target not alive"}
	}
	if m.vetoes.targetPresence != nil && !m.vetoes.targetPresence(d.Target) {
		return &state.VetoError{HandlerName: "target_presence", Reason: "target unavailable"}
	}

	if err := m.inner.TransitionTo(Engaging, r); err != nil {
		return err
	}

	// U12c-2: an ambush arms its opening here, on the transition that starts
	// the engagement. Keyed on the TRIGGER, not on a stored kind: a retarget
	// into an ordinary attack disarms it, which is what dropping the surprise
	// aggro type used to do.
	m.openingUnspent = r.Trigger == TriggerSurpriseAttack

	// U12c-0: this transition is now reachable from Engaged (a retarget), so
	// the superseded state data must go. The public accessors are state-gated
	// and would hide a stale value, which is exactly why leaving it would be a
	// trap for the next accessor that is not.
	//
	// prevTarget is captured BEFORE the clear, because CurrentTarget() reads
	// the state data.
	prevTarget := m.CurrentTarget()
	m.engaged = nil
	m.disengaging = nil

	m.engaging = &d

	// Move our inbound-attacker entry off the previous target. Inert today —
	// lookupMachine returns nil because combatphase.RegisterMachine has no
	// production callers — but without it a retarget would leak an entry on
	// the old target the day that registry is wired up.
	selfRef := r.Actor
	if selfRef.IsZero() {
		selfRef = m.self
	}
	if !prevTarget.IsZero() && prevTarget != d.Target {
		if prev := lookupMachine(prevTarget); prev != nil && !selfRef.IsZero() {
			prev.RemoveInboundAttacker(selfRef)
		}
	}

	if target := lookupMachine(d.Target); target != nil {
		target.RecordInboundAttacker(r.Actor)
	}
	return nil
}

// TransitionToDisengaging starts a flee/disengage attempt.
// Flee while grappled/clinched/grounded is vetoed via positionSelf.
func (m *Machine) TransitionToDisengaging(r state.TransitionReason) error {
	// Flee while grappled/clinched/grounded is vetoed.
	if r.Trigger == TriggerFleeCommand &&
		m.vetoes.positionSelf != nil && !m.vetoes.positionSelf() {
		return &state.VetoError{HandlerName: "position_self", Reason: "grappled"}
	}
	if err := m.inner.TransitionTo(Disengaging, r); err != nil {
		return err
	}
	target := state.ActorRef{}
	if m.engaged != nil {
		target = m.engaged.Target
	}
	m.disengaging = &DisengagingData{LastTarget: target}
	return nil
}

// OnRoundTick advances per-state round counters and fires any
// state transitions whose round has come. Called once per round
// per character by the round driver.
func (m *Machine) OnRoundTick() {
	switch m.State() {
	case Engaging:
		if m.engaging == nil {
			return
		}
		if m.engaging.RoundsUntil <= 0 {
			m.advanceToEngaged()
			return
		}
		m.engaging.RoundsUntil--
		if m.engaging.RoundsUntil == 0 {
			m.advanceToEngaged()
		}
	case Engaged:
		// Combat resolution is driven by the round driver, not here.
	case Disengaging:
		// Flee resolution is driven externally via ResolveFlee.
	}
}

// advanceToEngaged transitions Engaging → Engaged, carrying
// EngagingData into EngagedData.
func (m *Machine) advanceToEngaged() {
	prevEngaging := m.engaging
	if prevEngaging == nil {
		return
	}
	if err := m.inner.TransitionTo(Engaged, state.TransitionReason{
		Trigger: TriggerEngagementReady,
		Target:  prevEngaging.Target,
	}); err != nil {
		return // shouldn't happen — invariant violation
	}
	m.engaged = &EngagedData{Target: prevEngaging.Target}
	m.engaging = nil
}

// ResolveFlee finalizes a Disengaging state. Success → Idle.
// Failure → back to Engaged.
func (m *Machine) ResolveFlee(success bool) {
	if m.State() != Disengaging {
		return
	}
	if success {
		m.ForceIdle(state.TransitionReason{Trigger: TriggerFleeSuccess})
		return
	}
	// Failure: restore Engaged with the last target.
	target := state.ActorRef{}
	if m.disengaging != nil {
		target = m.disengaging.LastTarget
	}
	if err := m.inner.TransitionTo(Engaged, state.TransitionReason{
		Trigger: TriggerFleeFailure,
	}); err != nil {
		return
	}
	m.engaged = &EngagedData{Target: target}
	m.disengaging = nil
}

// NotifyTargetDied is invoked by the dying target's Machine
// during its own ForceIdle/death cascade. If my current target
// matches, I transition to Idle.
func (m *Machine) NotifyTargetDied(target state.ActorRef) {
	if m.CurrentTarget() != target {
		return
	}
	m.ForceIdle(state.TransitionReason{
		Trigger: TriggerTargetDied,
		Target:  target,
	})
}

// NotifySelfDied is invoked when this character dies. Clears
// own outbound combat state, clears all inbound attackers
// (notifying them that their target is gone).
func (m *Machine) NotifySelfDied() {
	// Capture inbound attackers before clearing.
	attackers := append([]state.ActorRef{}, m.attackers...)

	// Force self to Idle (also clears state data and notifies
	// my own outbound target if any).
	if m.State() != Idle {
		m.ForceIdle(state.TransitionReason{Trigger: TriggerSelfDied})
	}
	// Explicitly clear inbound list (ForceIdle handles outbound).
	m.attackers = nil
	m.notifyAttackersChange()

	// Notify each inbound attacker that their target died.
	for _, a := range attackers {
		if am := lookupMachine(a); am != nil {
			am.ForceIdle(state.TransitionReason{Trigger: TriggerTargetDied})
		}
	}
}

// ForceIdle transitions to Idle from any state, clearing all
// state-data and removing self from target's inbound list.
// Used for death cascade, Combatant-toggle, target-died, etc.
func (m *Machine) ForceIdle(r state.TransitionReason) {
	switch m.State() {
	case Idle:
		return
	}
	// Capture target before we clear data so we can remove
	// our entry from target's inbound list.
	target := m.CurrentTarget()

	if err := m.inner.TransitionTo(Idle, r); err != nil {
		return
	}
	m.engaging = nil
	m.engaged = nil
	m.disengaging = nil
	// EndAggro used to nil the whole Aggro struct, so the round budget and the
	// ambush opening died with the engagement. Idle preserves that exactly.
	m.roundsWaiting = 0
	m.openingUnspent = false

	// Remove self from target's inbound list. Use r.Actor if set,
	// otherwise fall back to the machine's own registered identity.
	selfRef := r.Actor
	if selfRef.IsZero() {
		selfRef = m.self
	}
	if t := lookupMachine(target); t != nil && !selfRef.IsZero() {
		t.RemoveInboundAttacker(selfRef)
	}
}

// RecordInboundAttacker appends to the inbound attacker list.
// Called via the framework when another character transitions
// to Engaging with us as target. Idempotent — duplicate inserts
// are skipped.
func (m *Machine) RecordInboundAttacker(a state.ActorRef) {
	if a.IsZero() {
		return
	}
	for _, existing := range m.attackers {
		if existing == a {
			return
		}
	}
	m.attackers = append(m.attackers, a)
	m.notifyAttackersChange()
}

// RemoveInboundAttacker drops an attacker from the inbound list.
// Idempotent — removing a non-existent entry is a no-op.
func (m *Machine) RemoveInboundAttacker(a state.ActorRef) {
	for i, existing := range m.attackers {
		if existing == a {
			m.attackers = append(m.attackers[:i], m.attackers[i+1:]...)
			m.notifyAttackersChange()
			return
		}
	}
}

// notifyAttackersChange fires registered SubscribeAttackersChange
// callbacks (the callbacks themselves are registered via a stub
// today; Task 8 implements registration).
func (m *Machine) notifyAttackersChange() {
	for _, fn := range m.attackersChangeListeners {
		fn(m.Attackers())
	}
}

// SubscribeAttackersChange registers a callback that fires whenever the
// inbound Attackers list changes (add or remove).
func (m *Machine) SubscribeAttackersChange(fn func([]state.ActorRef)) {
	m.attackersChangeListeners = append(m.attackersChangeListeners, fn)
}

// RegisterCombatantVeto adds a veto that blocks Engaging when the attacker
// is a NonCombatant. check() returns true when combat IS allowed.
func (m *Machine) RegisterCombatantVeto(check func() bool) { m.vetoes.combatantSelf = check }

// RegisterActivityCheck adds a veto that blocks Engaging when the character
// is busy with an activity. check() returns true when free.
func (m *Machine) RegisterActivityCheck(check func() bool) { m.vetoes.activitySelf = check }

// RegisterLifeCheck adds a veto that blocks Engaging when the attacker
// is dead. check() returns true when alive.
func (m *Machine) RegisterLifeCheck(check func() bool) { m.vetoes.lifeSelf = check }

// RegisterPositionCheck adds a veto that blocks Disengaging when the
// character is grappled. check() returns true when movement is possible.
func (m *Machine) RegisterPositionCheck(check func() bool) { m.vetoes.positionSelf = check }

// RegisterTargetCombatantCheck adds a veto that blocks Engaging when the
// target is a NonCombatant. check(target) returns true when target can be attacked.
func (m *Machine) RegisterTargetCombatantCheck(c func(state.ActorRef) bool) {
	m.vetoes.targetCombatant = c
}

// RegisterTargetLifeCheck adds a veto that blocks Engaging when the target
// is dead. check(target) returns true when alive.
func (m *Machine) RegisterTargetLifeCheck(c func(state.ActorRef) bool) {
	m.vetoes.targetLife = c
}

// RegisterTargetPresenceCheck adds a veto that blocks Engaging when the
// target is AFK or disconnected. check(target) returns true when present.
func (m *Machine) RegisterTargetPresenceCheck(c func(state.ActorRef) bool) {
	m.vetoes.targetPresence = c
}

// OnTickEvent registers a callback that fires from DispatchTickEvent
// with the btree event name corresponding to the current state.
// Currently: "mob_combat_round" when Engaged, "mob_idle" when Idle.
// Engaging and Disengaging dispatch no events (silent states).
//
// The round driver (Task 15) wires this up to fire btree events
// for every character once per round.
func (m *Machine) OnTickEvent(fn func(name string, r state.TransitionReason)) {
	m.tickEventListeners = append(m.tickEventListeners, fn)
}

// DispatchTickEvent fires the appropriate per-state tick event
// to all registered listeners. Called by the round driver at the
// start of each round per character.
//
// Engaging and Disengaging are silent — those states are pre-
// engagement and mid-disengagement; their action is driven by
// internal counters (OnRoundTick) and resolution methods
// (ResolveFlee), not by btree events.
func (m *Machine) DispatchTickEvent() {
	var name string
	switch m.State() {
	case Engaged:
		name = "mob_combat_round"
	case Idle:
		name = "mob_idle"
	default:
		return // Engaging and Disengaging are silent
	}
	r := state.TransitionReason{Trigger: "tick"}
	for _, fn := range m.tickEventListeners {
		fn(name, r)
	}
}

// LookupMachineForTest exposes the unexported registry lookup so packages
// outside combatphase can assert registration lifecycle. Production code must
// never call this -- it exists so internal/characters can prove that
// syncMachineRegistry admits no zero ref and leaks no stale binding.
func LookupMachineForTest(ref state.ActorRef) *Machine {
	return lookupMachine(ref)
}
