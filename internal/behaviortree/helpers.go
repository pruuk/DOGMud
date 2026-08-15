package behaviortree

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/util"
)

// calcReactionDelay computes a perception-scaled delay for behavior tree actions.
// Formula: delay = base - (perception / scale), clamped to [min, max].
// High-perception mobs react faster; low-perception mobs react slower.
// Returns 0 if the mob instance doesn't exist.
func calcReactionDelay(mobInstanceId int) time.Duration {
	mob := mobs.GetInstance(mobInstanceId)
	if mob == nil {
		return 0
	}
	cfg := configs.GetBalanceConfig()
	base := float64(cfg.MobBTreeReactionBase)
	scale := float64(cfg.MobBTreeReactionPerceptionScale)
	minDelay := float64(cfg.MobReactionDelayMin)
	maxDelay := float64(cfg.MobReactionDelayMax)

	if scale <= 0 {
		scale = 100
	}

	perception := float64(mob.Character.Stats.Perception.ValueAdj)
	delay := base - (perception / scale)

	if delay < minDelay {
		delay = minDelay
	}
	if delay > maxDelay {
		delay = maxDelay
	}
	return time.Duration(delay * float64(time.Second))
}

// GetBehaviorPath constructs the filesystem path to a mob's behavior tree YAML.
// Path: {dataFiles}/../behaviors/{zone}/{mobId}-{convertedName}.yaml
// Behavior trees live in a top-level behaviors/ directory parallel to mobs/,
// NOT inside mobs/ (the mob loader panics on unknown YAML in its tree).
func GetBehaviorPath(mobId int, zone string, name string) string {
	dataFiles := configs.GetFilePathsConfig().DataFiles.String()
	zoneSafe := mobs.ZoneNameSanitize(zone)
	nameSafe := util.ConvertForFilename(name)
	return util.FilePath(dataFiles, `/`, `behaviors`, `/`, zoneSafe, `/`,
		strconv.Itoa(mobId)+`-`+nameSafe+`.yaml`)
}

// EnsureBTreeState lazily initializes the BehaviorState on a mob instance.
func EnsureBTreeState(mob *mobs.Mob) *BehaviorState {
	if mob.BTreeState == nil {
		mob.BTreeState = NewBehaviorState()
	}
	state, ok := mob.BTreeState.(*BehaviorState)
	if !ok {
		state = NewBehaviorState()
		mob.BTreeState = state
	}
	return state
}

// GetRoomBehaviorPath constructs the filesystem path to a room's behavior tree
// YAML. Path: {dataFiles}/behaviors/rooms/{zoneSafe}/{roomId}.yaml
func GetRoomBehaviorPath(roomId int, zone string) string {
	dataFiles := configs.GetFilePathsConfig().DataFiles.String()
	zoneSafe := rooms.ZoneNameSanitize(zone)
	return util.FilePath(dataFiles, `/`, `behaviors`, `/`, `rooms`, `/`, zoneSafe, `/`,
		strconv.Itoa(roomId)+`.yaml`)
}

// GetArchetypePath constructs the filesystem path to an archetype btree YAML.
// Path: {dataFiles}/behaviors/archetypes/{name}.yaml
// Archetype names are treated as filesystem-safe tokens; callers are
// responsible for sanitization (we do not apply ConvertForFilename
// because archetype names are authored, not user-derived).
func GetArchetypePath(name string) string {
	dataFiles := configs.GetFilePathsConfig().DataFiles.String()
	return util.FilePath(dataFiles, `/`, `behaviors`, `/`, `archetypes`, `/`,
		name+`.yaml`)
}

// TryRoomBehavior is the main entry point for room behavior tree event dispatch.
// For room_command events it returns ctx.Intercepted; for all others it returns
// true when the tree evaluates to Success.
func TryRoomBehavior(roomId int, event EventContext) bool {
	room := rooms.LoadRoom(roomId)
	if room == nil {
		return false
	}

	// Ephemeral room instances inherit their template's behavior file (which
	// is named by the template id). Resolve the tree by the original id; keep
	// per-instance STATE keyed by the instance roomId so each player's copy
	// gates independently.
	behaviorRoomId := roomId
	if orig, ok := rooms.OriginalRoomId(roomId); ok {
		behaviorRoomId = orig
	}

	// Lazy-load tree if not cached.
	tree := GetEngine().GetRoomTree(behaviorRoomId)
	if tree == nil {
		if GetEngine().HasNoRoomTree(behaviorRoomId) {
			return false
		}
		path := GetRoomBehaviorPath(behaviorRoomId, room.Zone)
		if _, err := os.Stat(path); err != nil {
			GetEngine().SetNoRoomTree(behaviorRoomId)
			return false
		}
		if err := GetEngine().LoadRoomTree(behaviorRoomId, path); err != nil {
			mudlog.Error("TryRoomBehavior", "error", fmt.Sprintf("failed to load behavior tree for room %d: %v", behaviorRoomId, err))
			return false
		}
		tree = GetEngine().GetRoomTree(behaviorRoomId)
		if tree == nil {
			return false
		}
	}

	state := EnsureRoomBTreeState(roomId)
	event.RoomId = roomId

	ctx := &EvalContext{
		Event:    event,
		MobState: state,
		RoomId:   roomId,
	}
	result := tree.Evaluate(ctx)

	if event.EventType == "room_command" {
		return ctx.Intercepted
	}
	return result == Success
}

// resolvePerMobTree returns the mob's per-mob behavior tree, lazily
// loading it from behaviors/<zone>/<mobId>-<name>.yaml on first request.
// A missing or unloadable file is negative-cached via SetNoTree so the
// filesystem is not re-probed on every event. Returns nil when the mob
// has no per-mob tree.
func resolvePerMobTree(mob *mobs.Mob) Node {
	mobId := int(mob.MobId)
	engine := GetEngine()

	tree := engine.GetTree(mobId)
	if tree == nil && !engine.HasNoTree(mobId) {
		path := GetBehaviorPath(mobId, mob.Zone, mob.Character.Name)
		if _, err := os.Stat(path); err != nil {
			engine.SetNoTree(mobId)
		} else if err := engine.LoadTree(mobId, path); err != nil {
			mudlog.Error("TryMobBehavior", "error", fmt.Sprintf("failed to load behavior tree for mob %d (%s): %v", mobId, path, err))
			engine.SetNoTree(mobId)
		} else {
			tree = engine.GetTree(mobId)
		}
	}
	return tree
}

// resolveArchetypeTree returns the named archetype's tree, lazily loading
// it from behaviors/archetypes/<name>.yaml on first request. A missing
// file warns once and is negative-cached via SetNoArchetype; a file that
// fails to parse logs an error and is negative-cached the same way.
// Returns nil when the archetype cannot be resolved.
func resolveArchetypeTree(name string) Node {
	engine := GetEngine()

	tree := engine.GetArchetype(name)
	if tree == nil && !engine.HasNoArchetype(name) {
		path := GetArchetypePath(name)
		if _, err := os.Stat(path); err != nil {
			engine.SetNoArchetype(name)
			mudlog.Warn("TryMobBehavior", "archetype", name, "warning", fmt.Sprintf("archetype file not found: %s", path))
		} else if err := engine.LoadArchetype(name, path); err != nil {
			mudlog.Error("TryMobBehavior", "error", fmt.Sprintf("failed to load archetype %s (%s): %v", name, path, err))
			engine.SetNoArchetype(name)
		} else {
			tree = engine.GetArchetype(name)
		}
	}
	return tree
}

// TryMobBehavior is the main entry point for event dispatch.
// Returns true if a behavior tree handled the event (Success).
//
// Composition rule: a per-mob tree SPECIALIZES its declared archetype, it
// does not replace it.
//
//  1. The per-mob btree (behaviors/<zone>/<mobId>-<name>.yaml), when
//     present, is evaluated first. If it returns Success it owns the
//     event and the archetype is NOT evaluated — no double actions.
//     Running owns the event too: the tree claimed it and is mid-progress
//     (a delay decorator counting down, a goal plan whose step already
//     queued a command), and a second brain acting now would violate the
//     same no-double-actions rule Success carries. Running returns false
//     — legacy path, no archetype — exactly as it did before the
//     composition change.
//  2. Only if the per-mob tree is absent or returned Failure (which
//     includes every event-tagged branch mismatching the event) is the
//     mob's declared archetype (mob.BehaviorArchetype) resolved and
//     evaluated, with a fresh EvalContext. The BehaviorState is per mob
//     instance and intentionally shared between the two evaluations.
//  3. If neither succeeded, returns false and the caller runs the legacy
//     path (combatcommands / default attack / idle handlers).
//
// Why composition and not the old either/or: until 2026-08-15 step 2 ran
// ONLY when no per-mob file existed at all, so the mere presence of a
// per-mob file silently disabled the mob's declared archetype. 34
// production mobs had both. The entire bandit camp (mobs 283-286) carried
// per-mob party-coordination overlays that handle only mob_idle, which
// left every declared combat archetype (defensive_caster, generic_fighter,
// lookout, boss_soren) dead — a caster that never cast, a boss whose boss
// brain never ran — and no diagnostic fired anywhere. Per-mob files are
// overlays that specialize a brain; when they decline an event, the event
// must fall through to the archetype they specialize.
func TryMobBehavior(mobInstanceId int, event EventContext) bool {
	mob := mobs.GetInstance(mobInstanceId)
	if mob == nil {
		return false
	}

	mobId := int(mob.MobId)

	perMobTree := resolvePerMobTree(mob)
	if perMobTree == nil && mob.BehaviorArchetype == "" {
		// No tree at all — caller runs legacy path.
		return false
	}

	event.RoomId = mob.Character.RoomId

	// newCtx builds a fresh EvalContext per evaluation. EnsureBTreeState is
	// idempotent, so both evaluations share the same per-instance
	// BehaviorState; deferring it here also preserves the old behavior of
	// not allocating state for mobs whose trees never resolve.
	newCtx := func() *EvalContext {
		return &EvalContext{
			Event:      event,
			MobState:   EnsureBTreeState(mob),
			MobId:      mobId,
			InstanceId: mobInstanceId,
			RoomId:     mob.Character.RoomId,
			MobName:    mob.Character.Name,
		}
	}

	// 1. Per-mob tree first. Success owns the event outright, and so does
	// Running: the tree claimed the event and is mid-progress (e.g. mob
	// 254's negotiation delay countdown, or a goal plan that already
	// queued a command before reporting StatusRunning). Letting a second
	// brain act on the same round would break the no-double-actions rule,
	// so Running returns false — legacy path, no archetype — which is
	// byte-for-byte the pre-composition behavior. Only Failure falls
	// through to the archetype.
	if perMobTree != nil {
		switch perMobTree.Evaluate(newCtx()) {
		case Success:
			return true
		case Running:
			return false
		}
	}

	// 2. Fall through to the declared archetype on Failure only.
	if mob.BehaviorArchetype == "" {
		return false
	}
	archetypeTree := resolveArchetypeTree(mob.BehaviorArchetype)
	if archetypeTree == nil {
		return false
	}
	return archetypeTree.Evaluate(newCtx()) == Success
}
