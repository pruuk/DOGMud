package mobs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
)

// Skill and stat progression must not survive a mob's death.
//
// U10b-0 Phase C raises the mob skill ceiling from 3 to 25, which makes this
// invariant load-bearing: today's cap of 3 is reachable in roughly 180 rounds,
// so a long-lived mob simply stopped. With the higher ceiling, anything that let
// progression outlive death would let a creature creep upward indefinitely over
// a server's lifetime.
//
// The death pipeline (hooks.scheduleMobDespawnFromLife) deletes the instance
// file before destroying the in-memory record, so a respawn finds nothing to
// load. This pins the persistence half of that contract: after the delete, a
// fresh spawn reads from template. The hook that calls it lives in
// internal/hooks and cannot be imported here without a cycle, so this asserts
// the invariant the hook depends on rather than driving the hook itself.
func TestDeathDeletesInstance_RespawnReadsFromTemplate(t *testing.T) {
	// Write instance files under a scratch data root so the real world is
	// untouched. DataFiles is absolute, so no chdir is needed -- and chdir-ing
	// INTO t.TempDir() makes Windows refuse to remove it at cleanup.
	dir := t.TempDir()
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(dir)
	// A test binary never loads config.yaml, so Balance is the zero value and
	// MobProgressionEnabled is FALSE -- SaveMobInstance would early-return and
	// the whole fixture would prove nothing.
	cfg.Balance.MobProgressionEnabled = true
	configs.SetConfigForTest(t, cfg)

	cleanup := seedRegistry()
	defer cleanup()

	const id = MobId(1)
	const homeRoom = 4242

	// Baseline from a CONTROL spawn, not from the raw template field: Validate
	// runs ensureAllSkills, which floors every skill at 1, so a spawned mob and
	// its template do not agree on an unauthored skill.
	control := NewMobById(id, homeRoom)
	if control == nil {
		t.Fatal("control spawn returned nil")
	}
	tmplSkill := control.Character.Skills["weapon-combat"]
	DestroyInstance(control.InstanceId)

	mob := NewMobById(id, homeRoom)
	if mob == nil {
		t.Fatal("NewMobById returned nil")
	}

	// Progress it well past the template, and past the old cap of 3.
	mob.Character.Skills["weapon-combat"] = tmplSkill + 20
	mob.Character.Stats.Strength.Training = 15
	if err := SaveMobInstance(mob); err != nil {
		t.Fatalf("SaveMobInstance: %v", err)
	}

	savePath := filepath.Join(dir, "mobs.instances")
	if !anyFileUnder(savePath) {
		t.Fatalf("no instance file was written under %s; the fixture proves nothing", savePath)
	}

	// Confirm the progression really does come back while the file exists,
	// or the delete half below would pass vacuously.
	revived := NewMobById(id, homeRoom)
	if revived == nil {
		t.Fatal("respawn returned nil")
	}
	if got := revived.Character.Skills["weapon-combat"]; got != tmplSkill+20 {
		t.Fatalf("saved progression did not restore (%d, want %d); the delete test below would be vacuous",
			got, tmplSkill+20)
	}
	DestroyInstance(revived.InstanceId)

	// This is what the death pipeline does.
	DeleteMobInstance(id, mob.Zone, mob.Character.Name, homeRoom)

	fresh := NewMobById(id, homeRoom)
	if fresh == nil {
		t.Fatal("post-death respawn returned nil")
	}
	if got := fresh.Character.Skills["weapon-combat"]; got != tmplSkill {
		t.Errorf("skill progression survived death: weapon-combat %d, want a fresh spawn's %d",
			got, tmplSkill)
	}
	if got := fresh.Character.Stats.Strength.Training; got != 0 {
		t.Errorf("stat gains survived death: Strength.Training %d, want 0", got)
	}
}

func anyFileUnder(root string) bool {
	found := false
	_ = filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err == nil && info != nil && !info.IsDir() {
			found = true
		}
		return nil
	})
	return found
}
