package gmcp

import (
	"os"
	"strings"
	"testing"
)

// The quest editor is off until quest arc phase 6 (owner ruling P7): a create
// or edit there could save a quest that fails the loop rules and stops the next
// boot. Every file-touching Build.Quest.* message is refused.
func TestRefuseQuestBuild_RefusesEveryQuestFileMessage(t *testing.T) {
	for _, cmd := range []string{
		`Build.Quest.Get`, `Build.Quest.Update`, `Build.Quest.Create`, `Build.Quest.Delete`,
	} {
		res, refused := refuseQuestBuild(cmd)
		if !refused {
			t.Errorf("%s: not refused", cmd)
			continue
		}
		if res.Ok {
			t.Errorf("%s: Ok must be false", cmd)
		}
		want := "the quest editor is off until the quest builder ships (quest arc phase 6); edit quest files in the repo"
		if res.Error != want {
			t.Errorf("%s: error %q, want %q", cmd, res.Error, want)
		}
	}
}

// The list is read-only, so it keeps answering; nothing else is touched.
func TestRefuseQuestBuild_LeavesOtherBuildMessagesAlone(t *testing.T) {
	for _, cmd := range []string{`Build.Quest.List`, `Build.Mob.Get`, `Build.Room.Get`, `Build.Dialogue.Get`, `Build.Quest`} {
		if _, refused := refuseQuestBuild(cmd); refused {
			t.Errorf("%s: must not be refused", cmd)
		}
	}
}

// handleBuildOp must consult the refusal before the dispatch switch.
func TestHandleBuildOp_ConsultsQuestRefusalBeforeDispatch(t *testing.T) {
	raw, err := os.ReadFile("gmcp.Build.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	call := strings.Index(src, "refuseQuestBuild(evt.Command)")
	if call < 0 {
		t.Fatal("handleBuildOp does not call refuseQuestBuild(evt.Command)")
	}
	dispatch := strings.Index(src, "switch evt.Command {")
	if dispatch < 0 || call > dispatch {
		t.Fatal("refuseQuestBuild must run before the dispatch switch")
	}
}
