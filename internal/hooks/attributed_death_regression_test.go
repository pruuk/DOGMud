package hooks

import (
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/state/life"
)

// The happy path, and the one behaviour this whole slice exists to deliver: a
// queued death resolves through Die with the killer intact, so DeadData carries
// a real reference instead of the empty one every pre-U5c path supplied.
//
// The two live consumers of DeadData.Killer both gate on it being non-empty:
// Death_PlayerCorpse's gold transfer (!d.Killer.IsZero()) and
// attributeBountyKill's guard-kill branch (killer.IsMob() with a non-zero
// instance id). Before U5c neither ever fired on a combat or DoT death.
func TestAttributedDeath_KillerReachesDeadData(t *testing.T) {
	mob := newRouteDeathTestMob(t, -5)
	mob.Character.Life = life.NewMachine()
	mob.Character.DeathQueued = true

	RouteAttributedDeath(events.CharacterDied{
		MobInstanceId:       mob.InstanceId,
		KillerMobInstanceId: 4242,
		Trigger:             life.TriggerHealthZero,
	})

	d, ok := mob.Character.Life.DeadData()
	if !ok {
		t.Fatal("victim never entered the Dead state")
	}
	if d.Killer.IsZero() {
		t.Fatal("killer ref is empty; the gold transfer and the guard-kill bounty branch both no-op")
	}
	if d.Killer.MobInstanceId != 4242 {
		t.Errorf("killer = %+v, want mob 4242", d.Killer)
	}
	if !d.Killer.IsMob() {
		t.Error("killer must satisfy attributeBountyKill's IsMob branch")
	}
	if mob.Character.DeathQueued {
		t.Error("DeathQueued not cleared after the death resolved")
	}
}

// A player killer must arrive as a player, not be flattened into a mob ref.
func TestAttributedDeath_PlayerKillerIsCarried(t *testing.T) {
	mob := newRouteDeathTestMob(t, -5)
	mob.Character.Life = life.NewMachine()
	mob.Character.DeathQueued = true

	RouteAttributedDeath(events.CharacterDied{
		MobInstanceId: mob.InstanceId,
		KillerUserId:  7,
		Trigger:       life.TriggerHealthZero,
	})

	d, ok := mob.Character.Life.DeadData()
	if !ok {
		t.Fatal("victim never entered the Dead state")
	}
	if d.Killer.UserId != 7 {
		t.Errorf("killer UserId = %d, want 7", d.Killer.UserId)
	}
	if d.Killer.MobInstanceId != 0 {
		t.Errorf("killer MobInstanceId = %d, want 0 for a player killer",
			d.Killer.MobInstanceId)
	}
}

// The lethal blow is credited, not the next swing. Before U5c the death fired
// from whichever attack ran the death check next, so credit went to whoever
// swung after the kill rather than whoever landed it.
func TestAttributedDeath_LethalBlowIsCreditedNotTheNextSwing(t *testing.T) {
	mob := newRouteDeathTestMob(t, -5)
	mob.Character.Life = life.NewMachine()
	mob.Character.DeathQueued = true

	RouteAttributedDeath(events.CharacterDied{
		MobInstanceId:       mob.InstanceId,
		KillerMobInstanceId: 111, // landed the lethal blow
		Trigger:             life.TriggerHealthZero,
	})
	// A later swing's event for the same victim must not re-attribute. The
	// victim is Dead by now, so the listener takes its !IsAlive() branch.
	RouteAttributedDeath(events.CharacterDied{
		MobInstanceId:       mob.InstanceId,
		KillerMobInstanceId: 222, // swung afterwards
		Trigger:             life.TriggerHealthZero,
	})

	d, _ := mob.Character.Life.DeadData()
	if d.Killer.MobInstanceId != 111 {
		t.Errorf("killer = %d, want 111 — the lethal blow, not the next swing",
			d.Killer.MobInstanceId)
	}
}

// No test asserts the PVP config setting here, deliberately.
//
// _datafiles/config.yaml carries skip-worktree, so its contents are per-machine
// and a test reading it would fail for reasons unrelated to the code under
// test. Worse, it would give false comfort: the Go fallback for an unrecognised
// PVP value is PVPEnabled (config.gameplay.go), so PvP being off depends
// entirely on that file saying `disabled`, not on a safe default.
//
// What actually prevents player-sourced harm reaching another player is
// structural, and the compiler enforces it:
//
//   - usercommands.Throw's AoE loop and engageAfterThrow both take []*mobs.Mob.
//     A player cannot be passed to either without a type error.
//   - The player HarmArea branch in hooks/spell_resolution.go populates
//     TargetMobInstanceIds from room.GetMobs; TargetUserIds is only populated
//     for HelpArea, which heals.
//
// If either of those changes shape, this comment is the place to re-audit from.
