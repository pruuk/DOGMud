// Package awareness defines the Awareness state machine —
// the second consumer of internal/state, after combatphase.
// It replaces the condition-#9 "Hidden flag" as the canonical
// source of "is this character hidden?" Condition #9 stays as the
// side-effect carrier (stat mods, room broadcast text); the
// Awareness machine drives its addition and removal via
// cascade handlers.
package awareness

import (
	"sync"

	"github.com/GoMudEngine/GoMud/internal/state"
)

// State is the Awareness state enum.
type State int

const (
	Visible State = iota
	Concealing
	Hidden
	Revealing
)

// String for logging/debugging.
func (s State) String() string {
	switch s {
	case Visible:
		return "Visible"
	case Concealing:
		return "Concealing"
	case Hidden:
		return "Hidden"
	case Revealing:
		return "Revealing"
	}
	return "Unknown"
}

// VisibleData is empty — default state has no per-state data.
type VisibleData struct{}

// ConcealingData captures an in-flight sneak attempt.
// Today synchronous; chunk-1 sets and clears in one call.
// Future multi-round concealment could populate RoundsUntil.
type ConcealingData struct {
	RoundsUntil int
}

// HideSource says what keeps a Hidden character hidden. Empathic Shroud is a
// real hide (#444, owner 2026-10-09), and only one hide holds at a time: the
// stronger of the two.
type HideSource uint8

const (
	// HideSneak is the default: the sneak command, or a record 9 from a
	// mob's spawn-time conditionids or an admin. It hides at the holder's
	// Dexterity plus Skullduggery x SkillWeight.
	HideSneak HideSource = iota
	// HideShroud is Empathic Shroud (condition 31). It hides at Score: the
	// caster's spell stat plus Spellcasting x SkillWeight.
	HideShroud
)

// HiddenData carries hidden-state metadata: what is hiding the character,
// and for a shroud the score it hides at.
type HiddenData struct {
	Source HideSource
	// Score is a shroud hide's base score, in place of Dexterity plus
	// Skullduggery x SkillWeight. Unused for HideSneak.
	Score float64
}

// RevealingData captures the in-flight reveal cascade.
// Reason carries context for subscribers ("why is this character
// being revealed?"). Lifetime is one cascade cycle.
type RevealingData struct {
	Reason state.TransitionReason
}

// Machine wraps state.Machine[State] with awareness-specific
// API including per-state data storage.
type Machine struct {
	inner      *state.Machine[State]
	concealing *ConcealingData
	hidden     *HiddenData
	revealing  *RevealingData
	self       state.ActorRef
	vetoes     vetoChain
}

// NewMachine returns an Awareness machine in Visible.
func NewMachine() *Machine {
	return &Machine{
		inner: state.NewMachine(Visible, validTransitions),
	}
}

// State returns the current state.
func (m *Machine) State() State { return m.inner.State() }

// IsHidden returns true when state is Hidden.
func (m *Machine) IsHidden() bool { return m.State() == Hidden }

// ConcealingData returns the in-flight sneak data if Concealing.
func (m *Machine) ConcealingData() (ConcealingData, bool) {
	if m.State() != Concealing || m.concealing == nil {
		return ConcealingData{}, false
	}
	return *m.concealing, true
}

// RevealingData returns the cascade context if Revealing.
func (m *Machine) RevealingData() (RevealingData, bool) {
	if m.State() != Revealing || m.revealing == nil {
		return RevealingData{}, false
	}
	return *m.revealing, true
}

// Inner returns the underlying state.Machine — used by rules.go
// (Task 3+) and hooks (Task 5+). Not part of the stable API.
func (m *Machine) Inner() *state.Machine[State] { return m.inner }

// SetSelf binds the machine to its owning ActorRef. Called from
// the registry during character creation.
func (m *Machine) SetSelf(ref state.ActorRef) { m.self = ref }

// Self returns the bound ActorRef.
func (m *Machine) Self() state.ActorRef { return m.self }

// === Machine registry ===
// Cross-character lookups for observer detection, reveal cascades, etc.

var (
	registryMu      sync.Mutex
	machineRegistry = map[state.ActorRef]*Machine{}
)

// RegisterMachine binds an ActorRef to its Machine for cross-
// character notifications.
func RegisterMachine(ref state.ActorRef, m *Machine) {
	registryMu.Lock()
	defer registryMu.Unlock()
	machineRegistry[ref] = m
	m.self = ref
}

// UnregisterMachine removes a binding (player logout, mob despawn).
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

// === Transition methods ===

// TransitionToConcealing initiates a sneak attempt. Runs the
// Activity veto first (can't sneak while casting/crafting).
// Stores ConcealingData; caller is responsible for calling
// ResolveConcealment with the roll outcome.
func (m *Machine) TransitionToConcealing(d ConcealingData, r state.TransitionReason) error {
	if m.vetoes.activitySelf != nil && !m.vetoes.activitySelf() {
		return &state.VetoError{
			HandlerName: "activity_self",
			Reason:      "busy with activity",
		}
	}
	if err := m.inner.TransitionTo(Concealing, r); err != nil {
		return err
	}
	m.concealing = &d
	return nil
}

// ResolveConcealment finalizes the sneak attempt.
// success=true → Hidden; success=false → Visible.
// Idempotent: no-op if not currently Concealing.
func (m *Machine) ResolveConcealment(success bool, r state.TransitionReason) {
	if m.State() != Concealing {
		return
	}
	target := Visible
	if success {
		target = Hidden
	}
	_ = m.inner.TransitionTo(target, r)
	m.concealing = nil
	if success {
		m.hidden = &HiddenData{}
	} else {
		m.hidden = nil
	}
}

// ResolveConcealmentAs is ResolveConcealment(true, r) for a hide whose data
// matters. d is stored BEFORE the move to Hidden, so the Hidden cascade
// (internal/hooks/Awareness_Cascades.go) can read what is hiding the
// character. No-op if not currently Concealing.
func (m *Machine) ResolveConcealmentAs(d HiddenData, r state.TransitionReason) {
	if m.State() != Concealing {
		return
	}
	m.hidden = &d
	if err := m.inner.TransitionTo(Hidden, r); err != nil {
		m.hidden = nil
		return
	}
	m.concealing = nil
}

// HiddenData returns the hide's data while Hidden. ok is false in any other
// state, and inside ResolveConcealment's own Hidden cascade, which runs
// before that call stores its data; a caller reads a missing value as a
// sneak.
func (m *Machine) HiddenData() (HiddenData, bool) {
	if m.State() != Hidden || m.hidden == nil {
		return HiddenData{}, false
	}
	return *m.hidden, true
}

// SetHiddenData replaces the hide's data while Hidden, for one hide taking
// over from another without a reveal. A no-op in any other state.
func (m *Machine) SetHiddenData(d HiddenData) {
	if m.State() != Hidden {
		return
	}
	m.hidden = &d
}

// TransitionToRevealing transitions Hidden → Revealing → Visible
// same-tick. Cascade subscribers fire during Revealing; then
// Visible is set immediately.
//
// Idempotent if not currently Hidden — returns nil.
func (m *Machine) TransitionToRevealing(r state.TransitionReason) error {
	if m.State() != Hidden {
		return nil
	}
	if err := m.inner.TransitionTo(Revealing, r); err != nil {
		return err
	}
	m.hidden = nil
	m.revealing = &RevealingData{Reason: r}

	// Same-tick transition to Visible. Inner.TransitionTo fires
	// the framework's AfterTransition cascades for the
	// Revealing→Visible step; subscribers see both transitions
	// (Hidden→Revealing AND Revealing→Visible) within the same
	// stack frame.
	if err := m.inner.TransitionTo(Visible, state.TransitionReason{
		Trigger: r.Trigger,
		Actor:   r.Actor,
		Target:  r.Target,
	}); err != nil {
		return err
	}
	m.revealing = nil
	return nil
}

// NotifyRoomChanged is called when the actor changes room.
// Caller (typically internal/hooks/go.go) has already run per-
// observer detection rolls; this method receives the outcome.
//
// detected=true means at least one observer's perception beat
// the sneaker's score → transition Hidden → Revealing.
// detected=false means stealth held — no-op.
func (m *Machine) NotifyRoomChanged(detected bool, r state.TransitionReason) {
	if detected && m.State() == Hidden {
		_ = m.TransitionToRevealing(r)
	}
}

// ForceVisible drops to Visible from any state. Used by logout
// safety valve, death cascade, charm changes, etc. If currently
// Hidden, routes through Revealing (cascade subscribers fire).
// If currently Concealing, transitions directly to Visible.
// If already Visible or in flight (Revealing), no-op.
func (m *Machine) ForceVisible(r state.TransitionReason) {
	switch m.State() {
	case Hidden:
		_ = m.TransitionToRevealing(r)
	case Concealing:
		_ = m.inner.TransitionTo(Visible, r)
		m.concealing = nil
	}
}
