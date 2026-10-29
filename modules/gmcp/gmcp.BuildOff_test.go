package gmcp

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// The whole admin web builder is off until several big content arcs land
// (owner 2026-10-28: #375 phase 6 is postponed, since every arc reshapes the
// data the builder writes). Every Build.* message is refused before dispatch,
// so none reaches a handler that reads or writes a file.

// routedBuildCommands is every Build.* command gmcp.go routes to handleBuildOp,
// read from the source so a new command cannot slip past the refusal.
func routedBuildCommands(t *testing.T) []string {
	t.Helper()
	raw, err := os.ReadFile("gmcp.go")
	if err != nil {
		t.Fatal(err)
	}
	cmds := regexp.MustCompile("`(Build\\.[A-Za-z.]+)`").FindAllStringSubmatch(string(raw), -1)
	if len(cmds) < 20 {
		t.Fatalf("found only %d routed Build commands; the probe is broken", len(cmds))
	}
	var out []string
	for _, m := range cmds {
		out = append(out, m[1])
	}
	return out
}

func TestRefuseWebBuilder_RefusesEveryRoutedBuildMessage(t *testing.T) {
	for _, cmd := range routedBuildCommands(t) {
		res, refused := refuseWebBuilder(cmd)
		if !refused {
			t.Errorf("%s: not refused", cmd)
			continue
		}
		if res.Ok || !strings.Contains(res.Error, "web builder is off") {
			t.Errorf("%s: want a refusal naming the web builder, got %+v", cmd, res)
		}
	}
}

func TestRefuseWebBuilder_LeavesNonBuildMessagesAlone(t *testing.T) {
	for _, cmd := range []string{`Char.Quests.Focus`, `Zone.Map`, `Build`} {
		if _, refused := refuseWebBuilder(cmd); refused {
			t.Errorf("%s: must not be refused", cmd)
		}
	}
}

// handleBuildOp must consult the refusal before any dispatch, and the file
// keeps no route to a dialogue writer (the old editor dropped topics on save).
func TestHandleBuildOp_ConsultsWebBuilderRefusalFirst(t *testing.T) {
	raw, err := os.ReadFile("gmcp.Build.go")
	if err != nil {
		t.Fatal(err)
	}
	src := string(raw)
	refuse := strings.Index(src, "refuseWebBuilder(evt.Command)")
	dispatch := strings.Index(src, "switch evt.Command {")
	if refuse < 0 || dispatch < 0 || refuse > dispatch {
		t.Fatal("handleBuildOp must call refuseWebBuilder(evt.Command) before its dispatch switch")
	}
	if strings.Contains(src, "buildDialogueUpdate") || strings.Contains(src, "SaveDialogueFile") {
		t.Fatal("gmcp.Build.go still reaches a dialogue writer")
	}
}
