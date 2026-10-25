package main

import (
	"errors"
	"os"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/questloop"
	"github.com/GoMudEngine/GoMud/internal/quests"
	"github.com/GoMudEngine/GoMud/internal/questworld"
)

// loopBacklogWriteEnv, when set to a path, makes the sweep write the
// current findings there as backlog text instead of failing. It is how the
// backlog is generated (into a scratch path, then copied into place by
// hand); it never writes the shipped file.
const loopBacklogWriteEnv = `DOGMUD_LOOP_BACKLOG_WRITE`

// TestSmoke_ShippedQuestsPassTheLoop runs the boot door's quest loop rules
// over shipped content: a finding not in loop-backlog.txt fails, and so does
// a backlog line no finding matches (the list only shrinks).
func TestSmoke_ShippedQuestsPassTheLoop(t *testing.T) {
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

	if path := os.Getenv(loopBacklogWriteEnv); path != `` {
		fs := questloop.Validate(questworld.Registry())
		if err := os.WriteFile(path, questloop.FormatBacklog(fs), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %d findings to %s", len(fs), path)
		return
	}

	fresh, known, stale, err := validateShippedQuests()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range fresh {
		t.Errorf("not in loop-backlog.txt: %s", f.String())
	}
	for _, k := range stale {
		t.Errorf("loop-backlog.txt line %q matches no finding: delete it", k)
	}
	t.Logf("quest loop: %d known backlog findings, %d new, %d stale", len(known), len(fresh), len(stale))
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
