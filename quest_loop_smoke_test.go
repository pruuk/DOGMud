package main

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questengine"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/questworld"
)

// loopBacklogWriteEnv, when set to a path, makes the sweep write the
// current findings there as backlog text instead of failing. It is how the
// backlog is generated (into a scratch path, then copied into place by
// hand); it never writes the shipped file.
const loopBacklogWriteEnv = `DOGMUD_LOOP_BACKLOG_WRITE`

// loadShippedQuestWorld loads the shipped data the quest loop sweeps read,
// skipping unless DOGMUD_BOOT_SMOKE is set.
func loadShippedQuestWorld(t *testing.T) {
	t.Helper()
	if os.Getenv(bootSmokeEnvVar) == `` {
		t.Skipf("set %s=1 to run the quest loop sweep", bootSmokeEnvVar)
	}
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}
	func() {
		defer func() { _ = recover() }() // boot failures are the boot test's job
		loadAllDataFiles(false)
	}()
	if len(quests.GetAllQuests()) < 70 {
		t.Fatalf("only %d quests loaded; the sweep would pass vacuously", len(quests.GetAllQuests()))
	}
}

// TestSmoke_ShippedQuestsPassTheLoop runs the quest loop rules and the
// completability walkthrough over shipped content: a finding not in
// loop-backlog.txt fails, and so does a backlog line no finding matches (the
// list only shrinks).
//
// The backlog holds both kinds of line. Boot (main_quests.go) runs Validate
// only and ignores stale lines, so the walk lines in the backlog never
// affect boot; this sweep ratchets against Validate plus Walk, so in CI a
// walk line is never stale while its finding stands.
func TestSmoke_ShippedQuestsPassTheLoop(t *testing.T) {
	loadShippedQuestWorld(t)

	w := questworld.Registry()
	fs := append(questloop.Validate(w), questengine.Walk(w, questworld.Sources())...)

	if path := os.Getenv(loopBacklogWriteEnv); path != `` {
		if err := os.WriteFile(path, questloop.FormatBacklog(fs), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d findings to %s", len(fs), path)
		return
	}

	b, err := questworld.LoadBacklog()
	if err != nil {
		t.Fatal(err)
	}
	fresh, known, stale := questloop.Ratchet(fs, b)
	for _, f := range fresh {
		t.Errorf("not in loop-backlog.txt: %s", f.String())
	}
	for _, k := range stale {
		t.Errorf("loop-backlog.txt line %q matches no finding: delete it", k)
	}
	t.Logf("quest loop: %d known backlog findings, %d new, %d stale", len(known), len(fresh), len(stale))
}

// TestSmoke_WalkSourcesIndexShippedWorld checks the walkthrough's item and
// spawn index over shipped data, one known answer per source kind, and that
// an empty answer is possible.
func TestSmoke_WalkSourcesIndexShippedWorld(t *testing.T) {
	loadShippedQuestWorld(t)
	src := questworld.Sources()

	has := func(id int, prefix string) bool {
		for _, s := range src.ItemSources(id) {
			if strings.HasPrefix(s, prefix) {
				return true
			}
		}
		return false
	}

	// Caravan Guard Garve, spawned by rooms/pothole_coulee/5234.yaml.
	if !src.MobSpawned(9112) {
		t.Error("mob 9112 should be spawned")
	}
	// mobs/greenford/9502-maret_the_miller.yaml stocks 40153 (Maret is
	// spawned by rooms/greenford/6280.yaml).
	if !has(40153, "shop 9502") {
		t.Errorf("item 40153 should come from shop 9502: %v", src.ItemSources(40153))
	}
	// recipes/cooking/antidote-broth.yaml makes 30027.
	if !has(30027, "recipe antidote-broth") {
		t.Errorf("item 30027 should come from recipe antidote-broth: %v", src.ItemSources(30027))
	}
	// forager.ForageYields["forest"] lists 40067 (pine pitch).
	if !has(40067, "forage forest") {
		t.Errorf("item 40067 should come from forage forest: %v", src.ItemSources(40067))
	}
	if got := src.ItemSources(999999999); len(got) != 0 {
		t.Errorf("item 999999999 should have no source: %v", got)
	}
	// Ledger 21 lies in room 421 (rooms/watchers_crossing/421.yaml), so a
	// player can hold it outside quest 5. Tally 40100 comes only from quest
	// 63's own give_item actions (quests/63-dock_rat.yaml): it is 63's own,
	// and outside for any other quest.
	if !src.ItemOutside(21, 5) {
		t.Error("item 21 should have a source outside quest 5")
	}
	if src.ItemOutside(40100, 63) {
		t.Errorf("item 40100 should be quest 63's own: %v", src.ItemSources(40100))
	}
	if !src.ItemOutside(40100, 5) {
		t.Error("item 40100 should count as outside quest 5")
	}
	if src.MobSpawned(999999999) {
		t.Error("mob 999999999 should not be spawned")
	}
}

// TestQuestDoor_RefusesAFreshFinding: a fresh finding panics at boot and is
// an error on a reload; known findings alone pass; a backlog that cannot be
// read refuses the same way.
func TestQuestDoor_RefusesAFreshFinding(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	hint := questloop.Finding{Rule: "hint", QuestId: 1, Where: "step start", Msg: "no hint"}
	fresh := func() ([]questloop.Finding, []questloop.Finding, []string, error) {
		return []questloop.Finding{hint}, nil, nil, nil
	}
	knownOnly := func() ([]questloop.Finding, []questloop.Finding, []string, error) {
		return nil, []questloop.Finding{hint}, []string{"end 2 step end"}, nil
	}
	unreadable := func() ([]questloop.Finding, []questloop.Finding, []string, error) {
		return nil, nil, nil, errors.New("loop-backlog.txt line 3: repeated line")
	}
	for name, src := range map[string]func() ([]questloop.Finding, []questloop.Finding, []string, error){
		"fresh": fresh, "unreadable": unreadable,
	} {
		if err := questDoorFrom(true, src); err == nil {
			t.Errorf("%s: a reload must return an error", name)
		}
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: boot must panic", name)
				}
			}()
			_ = questDoorFrom(false, src)
		}()
	}
	if err := questDoorFrom(false, knownOnly); err != nil {
		t.Errorf("known findings and stale lines must not refuse boot: %v", err)
	}
}

// TestQuestLoop_GuardCanFail proves the ratchet refuses an unlisted finding
// and flags a stale line, without the shipped world.
func TestQuestLoop_GuardCanFail(t *testing.T) {
	fs := []questloop.Finding{{Rule: "hint", QuestId: 1, Where: "step start"}}
	fresh, _, stale := questloop.Ratchet(fs, questloop.Backlog{"end 2 step end": true})
	if len(fresh) != 1 || len(stale) != 1 {
		t.Fatalf("ratchet did not refuse: fresh %v stale %v", fresh, stale)
	}
}
