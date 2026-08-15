package behaviortree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/buffs"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mobs"
)

// File: permob_archetype_composition_test.go
//
// Tests for the TryMobBehavior composition rule: a per-mob tree
// SPECIALIZES its declared archetype, it does not replace it.
//
// Before 2026-08-15 the archetype was consulted only when NO per-mob file
// existed, so the mere presence of a per-mob overlay silently disabled the
// mob's declared brain. The bandit camp (mobs 283-286) is the canonical
// victim: per-mob party-coordination overlays handling only mob_idle left
// every declared combat archetype dead. The per-mob fixture below mirrors
// that exact shape (top-level sequence, always-Success first action, then
// an idle-only selector).

// perMobIdleOnlyYAML mirrors the bandit-camp overlay shape: a top-level
// sequence whose first child always succeeds (party_ensure_npc_party in
// production; set_state here), followed by a selector whose branches are
// all tagged event: mob_idle. On any other event the selector fails, so
// the whole tree returns Failure — which must fall through to the
// archetype, not swallow the event.
const perMobIdleOnlyYAML = `
tree:
  type: sequence
  children:
    - type: action
      do: set_state
      key: overlay_ran
      value: 1
    - type: selector
      children:
        - type: sequence
          event: mob_idle
          children:
            - type: action
              do: command
              cmd: say overlay idle
`

// overlayTestArchetypeYAML is a minimal archetype with a distinguishable
// action on both mob_combat_round and mob_idle, so tests can tell exactly
// which tree acted.
const overlayTestArchetypeYAML = `
tree:
  type: selector
  children:
    - type: sequence
      event: mob_combat_round
      children:
        - type: action
          do: command
          cmd: say archetype combat
    - type: sequence
      event: mob_idle
      children:
        - type: action
          do: command
          cmd: say archetype idle
`

// installPerMobIdleOnlyTree compiles perMobIdleOnlyYAML and installs it in
// the engine's per-mob cache for the given mob template id.
func installPerMobIdleOnlyTree(t *testing.T, mobId int) {
	t.Helper()
	node, err := LoadTreeFromBytes([]byte(perMobIdleOnlyYAML))
	if err != nil {
		t.Fatalf("LoadTreeFromBytes(perMobIdleOnlyYAML): %v", err)
	}
	t.Cleanup(GetEngine().SetMobTreeForTest(mobId, node))
}

// loadOverlayTestArchetype writes overlayTestArchetypeYAML to a temp file
// and installs it in the engine under the given name (LoadArchetype is
// file-based; there is no bytes loader for archetypes).
func loadOverlayTestArchetype(t *testing.T, name string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), name+".yaml")
	if err := os.WriteFile(path, []byte(overlayTestArchetypeYAML), 0644); err != nil {
		t.Fatalf("writing archetype fixture: %v", err)
	}
	LoadArchetypeForTest(t, name, path)
}

// seedOverlayMob seeds a minimal mob with the given BehaviorArchetype.
func seedOverlayMob(t *testing.T, instanceId int, archetype string) (*mobs.Mob, func()) {
	t.Helper()
	m := &mobs.Mob{
		MobId:             mobs.MobId(400 + instanceId),
		InstanceId:        instanceId,
		BehaviorArchetype: archetype,
	}
	m.Character.Name = "testbandit"
	m.Character.Health = 100
	m.Character.HealthMax.Value = 100
	m.Character.Buffs = buffs.New()
	cleanup := mobs.SeedMobsForTest(
		map[int]*mobs.Mob{400 + instanceId: m},
		map[int]*mobs.Mob{instanceId: m},
	)
	return m, cleanup
}

// TestComposition_IdleOnlyOverlay_FallsThroughToArchetypeCombat is the bug
// reproduction: a mob with a bandit-shaped idle-only per-mob overlay AND a
// declared archetype whose mob_combat_round branch casts. Firing
// mob_combat_round must reach the archetype and queue its cast.
//
// Before the fix this returned false and queued nothing: the per-mob
// file's existence disabled the archetype entirely.
func TestComposition_IdleOnlyOverlay_FallsThroughToArchetypeCombat(t *testing.T) {
	defer seedCasterSpells(t)()
	LoadArchetypeForTest(t, "pure_caster", pureCasterYAML)

	// seedCasterMob declares BehaviorArchetype: pure_caster.
	mob, cleanup := seedCasterMob(t, 92001, map[string]int{
		"conviction-ward": 4,
		"iron-will":       3,
		"heal":            3,
		"mind-spike":      5,
		"sparks":          3,
	})
	defer cleanup()
	defer events.DrainQueuedInputsForTest(mob.InstanceId)

	installPerMobIdleOnlyTree(t, int(mob.MobId))

	ok := TryMobBehavior(mob.InstanceId, EventContext{EventType: "mob_combat_round"})
	if !ok {
		t.Fatalf("TryMobBehavior: expected archetype fall-through Success, got false (per-mob overlay swallowed the event)")
	}
	DrainAllDelayedActionsForTest(t)

	// Full-HP fresh caster → self_defense top pick = iron-will (6×45=270).
	cmd := events.InspectQueuedInputForTest(mob.InstanceId, "cast ")
	if !strings.HasPrefix(cmd, "cast iron-will") {
		t.Fatalf("archetype combat branch should have cast iron-will, got %q", cmd)
	}
}

// TestComposition_OverlaySuccessSuppressesArchetype verifies the ownership
// half of the rule: when the per-mob overlay handles the event (Success),
// the archetype is NOT evaluated — exactly one action queued, the
// overlay's, even though the archetype has its own mob_idle branch.
func TestComposition_OverlaySuccessSuppressesArchetype(t *testing.T) {
	loadOverlayTestArchetype(t, "c1_overlay_archetype")

	mob, cleanup := seedOverlayMob(t, 92002, "c1_overlay_archetype")
	defer cleanup()
	defer events.DrainQueuedInputsForTest(mob.InstanceId)

	installPerMobIdleOnlyTree(t, int(mob.MobId))

	ok := TryMobBehavior(mob.InstanceId, EventContext{EventType: "mob_idle"})
	if !ok {
		t.Fatalf("TryMobBehavior: expected per-mob overlay Success on mob_idle, got false")
	}
	DrainAllDelayedActionsForTest(t)

	inputs := events.DrainQueuedInputsForTest(mob.InstanceId)
	if len(inputs) != 1 || inputs[0] != "say overlay idle" {
		t.Fatalf("expected exactly the overlay's action queued, got %v", inputs)
	}
}

// TestComposition_NoArchetype_FailureFallsToLegacy verifies the unchanged
// legacy contract: a per-mob tree that fails the event on a mob with no
// declared archetype returns false so the caller runs the legacy path.
func TestComposition_NoArchetype_FailureFallsToLegacy(t *testing.T) {
	mob, cleanup := seedOverlayMob(t, 92003, "")
	defer cleanup()
	defer events.DrainQueuedInputsForTest(mob.InstanceId)

	installPerMobIdleOnlyTree(t, int(mob.MobId))

	ok := TryMobBehavior(mob.InstanceId, EventContext{EventType: "mob_combat_round"})
	if ok {
		t.Fatalf("TryMobBehavior: expected false (no archetype to fall through to), got true")
	}
	if inputs := events.DrainQueuedInputsForTest(mob.InstanceId); len(inputs) != 0 {
		t.Fatalf("expected no actions queued, got %v", inputs)
	}
}

// TestComposition_MissingArchetypeFile_WarnOnceNoPanic verifies the lazy
// archetype load's negative-cache semantics survive the restructure: a mob
// declaring a nonexistent archetype (plus a failing per-mob overlay)
// returns false without panicking, sets the HasNoArchetype negative cache
// on first miss, and stays false on repeat calls.
func TestComposition_MissingArchetypeFile_WarnOnceNoPanic(t *testing.T) {
	const badName = "c1_no_such_archetype"
	t.Cleanup(func() {
		e := GetEngine()
		e.mu.Lock()
		delete(e.noArchetype, badName)
		e.mu.Unlock()
	})

	mob, cleanup := seedOverlayMob(t, 92004, badName)
	defer cleanup()
	defer events.DrainQueuedInputsForTest(mob.InstanceId)

	installPerMobIdleOnlyTree(t, int(mob.MobId))

	if ok := TryMobBehavior(mob.InstanceId, EventContext{EventType: "mob_combat_round"}); ok {
		t.Fatalf("TryMobBehavior: expected false with missing archetype file, got true")
	}
	if !GetEngine().HasNoArchetype(badName) {
		t.Fatalf("missing archetype file should be negative-cached via SetNoArchetype")
	}
	// Second call must hit the negative cache (no re-stat, no panic).
	if ok := TryMobBehavior(mob.InstanceId, EventContext{EventType: "mob_combat_round"}); ok {
		t.Fatalf("TryMobBehavior: expected false on cached-miss repeat call, got true")
	}
}
