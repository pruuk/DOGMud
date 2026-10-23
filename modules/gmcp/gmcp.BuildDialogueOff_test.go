package gmcp

import (
	"os"
	"strings"
	"testing"
)

// The old dialogue editor drops `topics` and `opener` on save, so every
// Build.Dialogue.* message is refused until the topic builder ships (#375
// phase 6). A refused message never reaches a file.
func TestRefuseDialogueBuild_RefusesEveryDialogueMessage(t *testing.T) {
	for _, cmd := range []string{
		`Build.Dialogue.Get`, `Build.Dialogue.Update`, `Build.Dialogue.Create`, `Build.Dialogue.Delete`,
	} {
		res, refused := refuseDialogueBuild(cmd)
		if !refused {
			t.Errorf("%s: not refused", cmd)
			continue
		}
		if res.Ok {
			t.Errorf("%s: Ok must be false", cmd)
		}
		if !strings.Contains(res.Error, "phase 6") {
			t.Errorf("%s: error %q must name phase 6", cmd, res.Error)
		}
	}
}

func TestRefuseDialogueBuild_LeavesOtherBuildMessagesAlone(t *testing.T) {
	for _, cmd := range []string{`Build.Quest.Update`, `Build.Mob.Get`, `Build.Room.Get`, `Build.Dialogue`} {
		if _, refused := refuseDialogueBuild(cmd); refused {
			t.Errorf("%s: must not be refused", cmd)
		}
	}
}

// handleBuildOp must consult the refusal before any other dispatch, so no
// dialogue message can reach a handler that writes a file.
func TestHandleBuildOp_ConsultsDialogueRefusalFirst(t *testing.T) {
	raw, err := os.ReadFile("gmcp.Build.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	if !strings.Contains(src, "refuseDialogueBuild(evt.Command)") {
		t.Fatal("handleBuildOp does not call refuseDialogueBuild(evt.Command)")
	}
	if strings.Contains(src, "buildDialogueUpdate") || strings.Contains(src, "SaveDialogueFile") {
		t.Fatal("gmcp.Build.go still reaches a dialogue writer")
	}
}
