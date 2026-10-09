package messaging

import (
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/state/perception"
)

// RoomVisibility is the minimal interface CanSeeClearly / CanSeeShapes
// need from a room. *rooms.Room satisfies this implicitly. Decoupled
// so messaging/ does not import rooms/ — rooms/ imports messaging/,
// and an interface here keeps the dependency arrow one-way.
//
// Graded lighting arc, plan 1 task 4: this interface's method was renamed
// from the old three-value visibility accessor's name to LightLevel() int.
// Room.LightLevel() (internal/rooms/lighting.go) reports the room's light
// on the graded -100..100 scale (Task 3). Task 5 deleted that old accessor
// and migrated its remaining callers, so LightLevel is this interface's
// only implementation obligation.
type RoomVisibility interface {
	LightLevel() int
}

// ParticipantSight is THE optics primitive. It answers what an observer can
// make out, and nothing else: blindness, room light, NightVision,
// InfraredVision.
//
// It does NOT consult sleep. Sleep is an attention property, not an optical
// one -- a sleeping character's eyes work, they are simply not reading -- and
// the policies below compose it where it belongs. Conflating the two is what
// left three predicates each carrying a comment explaining the split.
//
// WHO READS IT DIRECTLY, and why sleep's absence is load-bearing for them:
// messaging.SendTrio hides a name from its reader by this verdict, and
// actions.InitiateCast refuses a cast at something the caster cannot see. Both
// judge a PARTY to an event, and a sleeper struck in a lit room must still be
// told what hit them. Observers who are not a party go through CanSeeClearly
// and CanSeeShapes instead, which do compose attention, so a sleeper still
// receives no room lines.
//
// Full when the room's light or NightVision allow clear sight; shapes for an
// unblinded observer in a dim room, or with infrared in the dark; none
// otherwise. A nil observer sees fully, matching the policies below.
//
// GRADED LIGHTING PLAN 2. An ability does not grant sight outright; it moves
// where the observer's usable band sits on the light scale, and that band
// still has a floor. NightVisionStrength shifts the blind and dim edges down
// by that many points (SightThroughWindow, internal/messaging/window.go), so
// night sight is bought with bright-light comfort rather than being free: the
// same shift that lets a holder read a dim room by candlelight does nothing
// in a pitch dark one, because the shifted window is still blind below its
// floor. InfraReach is the separate number that reads past that floor, which
// is why an infrared holder and a nightvision-only holder diverge in a truly
// dark room even though both looked identical under the old flag shortcuts.
func ParticipantSight(observer *characters.Character, room RoomVisibility) SightDecision {
	if observer == nil {
		return SightFull
	}
	if observer.Perception != nil && observer.Perception.State() == perception.Blinded {
		return SightNone
	}
	if room == nil {
		// Reflection-free nil-interface guard: a typed-nil *rooms.Room
		// would panic on LightLevel; callers must pass nil interface,
		// not a typed-nil. The room/Room.SendText path always has a real
		// receiver, so this is safe in practice. (This was roomIsLit's
		// job before it was folded into this function; it had exactly
		// one caller, this one.)
		return SightFull
	}
	// Fetched once into a local rather than called from each case below.
	// GetBalanceConfig takes configDataLock (twice: once inside its own
	// ensureConfigValidated call, once itself) and returns Balance BY
	// VALUE -- a struct of well over 400 fields (424 counted directly off
	// internal/configs/config.balance.go at time of writing) -- so a
	// tagless switch that called it from both case expressions would pay
	// that cost twice on the dark path, where the first case is false and
	// the second is evaluated.
	balance := configs.GetBalanceConfig()
	return SightThroughWindow(
		room.LightLevel(),
		observer.NightVisionStrength(),
		observer.InfraReach(),
		int(balance.LightBlindBelow),
		int(balance.LightDimBelow),
	)
}

// SeesThroughExit reports whether the observer can see THROUGH an exit from
// this room into the next: it must make out something here at all
// (ParticipantSight is not SightNone), and the room's light must clear
// LightExitsAbove shifted down by its night-vision strength
// (ExitThroughWindow). Optics only, like ParticipantSight: it does not consult
// sleep. A nil observer or a nil room sees through, matching ParticipantSight.
func SeesThroughExit(observer *characters.Character, room RoomVisibility) bool {
	if observer == nil || room == nil {
		return true
	}
	if ParticipantSight(observer, room) == SightNone {
		return false
	}
	return ExitThroughWindow(room.LightLevel(), observer.NightVisionStrength(), configs.GetLightingConfig().ExitsAbove)
}

// SensesHeatThroughExit reports whether an observer whose light test through
// an exit failed (SeesThroughExit false) still makes out the next room's
// occupants as shapes by their heat (lighting plan 6, owner ruling O6). It
// needs infra reach, some sight here (ParticipantSight not SightNone, so a
// blinded observer senses nothing), and the next room's light at or above
// minus the reach, the same depth own-room infravision reads to
// (SightThroughWindow).
//
// Heat shows bodies, never names, a room's description or its items, and it
// never upgrades a view: a caller asks it only after SeesThroughExit has
// refused. A nil observer or room senses nothing.
func SensesHeatThroughExit(observer *characters.Character, here, next RoomVisibility) bool {
	if observer == nil || here == nil || next == nil {
		return false
	}
	reach := observer.InfraReach()
	if reach <= 0 {
		return false
	}
	if ParticipantSight(observer, here) == SightNone {
		return false
	}
	return next.LightLevel() >= -reach
}

// FixedLight is a RoomVisibility at one light value. A caller judging many
// observers in one room reads room.LightLevel() once and passes FixedLight,
// rather than recomposing the room's light per observer.
type FixedLight int

// LightLevel satisfies RoomVisibility.
func (l FixedLight) LightLevel() int { return int(l) }

// awake reports attention. Kept separate from optics on purpose; see
// ParticipantSight.
func awake(observer *characters.Character) bool {
	return observer == nil || !observer.HasConditionFlag(conditions.Sleeping)
}

// CanSeeClearly returns true if the observer can read normal-text
// visual broadcasts in this room. Composes Perception state, room
// lighting, and the NightVision condition flag.
//
// Blinded observers (any source) return false unconditionally.
// A nil observer defaults to true (defensive — pre-init characters
// during boot must not be silently dropped).
//
// Sleep is a perception state, even though it is carried as a condition flag
// rather than by the Perception machine. This pipeline had no concept of it
// at all until 2026-08-31, so a sleeping player kept receiving every visual
// broadcast in the room: NPC dialogue, ambient flavour, arrivals.
//
// AUDIO IS DELIBERATELY UNAFFECTED. Room.SendText bypasses this gate, so a
// shout still reaches a sleeper and still wakes them (shout.go owns that
// wake trigger). Gating audio here would make sleep unwakeable by sound.
func CanSeeClearly(observer *characters.Character, room RoomVisibility) bool {
	return awake(observer) && ParticipantSight(observer, room) == SightFull
}

// CanSeeSightImpairedOnly is CanSeeClearly WITHOUT the sleep gate: it reports
// whether sight is impaired by ROOM DARKNESS or BLINDNESS alone.
//
// WHY THIS EXISTS. CanSeeClearly is not a messaging-only predicate.
// internal/combat/combat.go feeds it into combatContext.sourceCanSee and
// targetCanSee, which drive Balance.DarknessCombatPenalty onto the attack score
// and onto every candidate defence score. When the sleep gate was added to
// CanSeeClearly on 2026-08-31, that silently applied a DARKNESS penalty to a
// sleeping defender standing in a LIT room.
//
// The final outcome was masked, because a sleeping victim is already auto-crit
// through AttackSide.ForceCrit and the contest result is overridden anyway. But
// the contest still ran, and its margins and z-scores are what
// combat-analytics.jsonl records and what tools/balance reads. Corrupting that
// telemetry with a phantom darkness term is not acceptable, and doubling a
// sleeper's disadvantage was never asked for.
//
// So combat keeps the pre-sleep semantics and messaging gets the sleep gate.
//
// It feeds Balance.DarknessCombatPenalty, so widening it would hand every
// infrared character a silent balance change; it is the optics question with
// NO attention test, and it is SightFull specifically -- infrared does not
// satisfy it.
//
// M4d closed the seam this comment used to describe: ParticipantSight is now
// the shared optics primitive both this function and CanSeeClearly are built
// on.
func CanSeeSightImpairedOnly(observer *characters.Character, room RoomVisibility) bool {
	return ParticipantSight(observer, room) == SightFull
}

// CanSeeShapes returns true if the observer can detect SOMETHING is
// happening — either full clarity (subsumes CanSeeClearly) OR
// infrared in the dark. Blindness gates this too — broken eyes don't
// see infrared. So does sleep: closed eyes see no shapes.
//
// A nil observer defaults to true (matches CanSeeClearly's defensive
// behavior).
//
// "Full sight OR shapes", written as two equalities on purpose: the
// SightDecision constants run BEST-TO-WORST (SightFull = 0, SightShapes = 1,
// SightNone = 2), so an ordered comparison such as `<= SightShapes` would
// read backwards and a `>=` would also match SightNone.
func CanSeeShapes(observer *characters.Character, room RoomVisibility) bool {
	if !awake(observer) {
		return false
	}
	d := ParticipantSight(observer, room)
	return d == SightFull || d == SightShapes
}

// ReaderSight is the sight decision a reader's visual line is judged at:
// SightFull when CanSeeClearly, SightShapes when CanSeeShapes, else
// SightNone. Unlike ParticipantSight it carries the sleep gate, as every
// visual sender does (rooms.visualDecision is this function). The fight
// prompt's {target} and GMCP Char.Enemies name an unseen foe with
// UnseenNoun of it, so they say what the combat lines say (#455), and the
// combat personal lines' name gate is this function too
// (combat.newCombatContext), so a sleeper hit in a lit room reads
// "something".
func ReaderSight(observer *characters.Character, room RoomVisibility) SightDecision {
	switch {
	case CanSeeClearly(observer, room):
		return SightFull
	case CanSeeShapes(observer, room):
		return SightShapes
	}
	return SightNone
}
