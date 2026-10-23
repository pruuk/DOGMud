package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// doorDataDir points DataFiles at a temp dir holding files (path relative
// to the data dir, contents) for the rest of the test.
func doorDataDir(t *testing.T, files map[string]string) {
	t.Helper()
	root := t.TempDir()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := configs.GetConfig()
	cfg.FilePaths.DataFiles = configs.ConfigString(root)
	configs.SetConfigForTest(t, cfg)
}

func doorTemplate(id int, zone, name, archetype string) *mobs.Mob {
	return &mobs.Mob{MobId: mobs.MobId(id), Zone: zone, BehaviorArchetype: archetype,
		Character: characters.Character{Name: name}}
}

func problemText(probs []dialogue.Problem) string {
	var out []string
	for _, p := range probs {
		out = append(out, p.String())
	}
	return strings.Join(out, "\n")
}

const topicTree = "tree:\n  type: sequence\n  event: player_topic\n  children:\n    - type: condition\n      check: topic_is\n      topic: %s\n"

func treeWithTopic(key string) string { return strings.Replace(topicTree, "%s", key, 1) }

// #375 review M5: a dialogue file the runtime never reads (another zone
// folder, or an id no template has) is a problem; the right folder is not.
func TestDialogueFolderProblems(t *testing.T) {
	doorDataDir(t, map[string]string{
		"dialogue/right_zone/50.yaml": "mobid: 50\n",
		"dialogue/wrong_zone/51.yaml": "mobid: 51\n",
		"dialogue/right_zone/52.yaml": "mobid: 52\n",
	})
	tmpls := []*mobs.Mob{doorTemplate(50, "Right Zone", "Guide", ""), doorTemplate(51, "Right Zone", "Clerk", "")}
	got := problemText(dialogueFolderProblems(tmpls))
	if strings.Contains(got, "mob 50 ") {
		t.Errorf("a file in its mob's zone folder was refused:\n%s", got)
	}
	if !strings.Contains(got, `mob 51 `) || !strings.Contains(got, `reads dialogue/right_zone/51.yaml, so this NPC is mute`) {
		t.Errorf("a file in another zone folder passed:\n%s", got)
	}
	if !strings.Contains(got, "no mob template has id 52") {
		t.Errorf("a file for no mob passed:\n%s", got)
	}
}

// The behavior index reads the tree at the path the runtime loads, reports a
// tree that does not parse in place of dropping it, folds in the archetype's
// keys, and refuses topic_is keys in a file no mob reads (a stale suffix).
func TestBehaviorTopicIndex(t *testing.T) {
	doorDataDir(t, map[string]string{
		"behaviors/right_zone/50-ferry_guide.yaml": treeWithTopic("ferry"),
		"behaviors/right_zone/50-old_name.yaml":    treeWithTopic("stale"),
		"behaviors/right_zone/51-clerk.yaml":       "tree: [\n",
		"behaviors/archetypes/talker.yaml":         treeWithTopic("chat"),
		"behaviors/right_zone/77-nobody.yaml":      treeWithTopic("orphan"),
	})
	tmpls := []*mobs.Mob{
		doorTemplate(50, "Right Zone", "Ferry Guide", "talker"),
		doorTemplate(51, "Right Zone", "Clerk", ""),
	}
	perMob, archetype, probs := behaviorTopicIndex(tmpls)
	if strings.Join(perMob[50], ",") != "ferry" {
		t.Errorf("per-mob keys for 50: %v", perMob[50])
	}
	if strings.Join(archetype[50], ",") != "chat" {
		t.Errorf("archetype keys for 50: %v", archetype[50])
	}
	got := problemText(probs)
	if !strings.Contains(got, "mob 51 ") || !strings.Contains(got, "behavior file does not parse") {
		t.Errorf("a tree that does not parse was dropped:\n%s", got)
	}
	if !strings.Contains(got, "50-old_name.yaml: holds topic_is stale, but mob 50's tree is read from") {
		t.Errorf("a stale-suffix file with topic_is passed:\n%s", got)
	}
	if !strings.Contains(got, "77-nobody.yaml: holds topic_is orphan, but no mob template has id 77") {
		t.Errorf("a topic_is file for no mob passed:\n%s", got)
	}
	for _, p := range probs {
		if strings.HasSuffix(p.Where, "50-ferry_guide.yaml") {
			t.Errorf("the tree at the runtime's path was refused: %s", p.String())
		}
	}
}

// #375 review M4: at boot a dialogue problem stops the server; on a reload
// it is returned, so the reload reports the failure and never says "Done.".
func TestDialogueDoor_BootPanicsReloadReports(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)
	probs := []dialogue.Problem{{MobId: 1, Where: "topic \"x\"", Msg: "broken"}}
	if err := dialogueDoor(nil, false); err != nil {
		t.Fatalf("no problems: %v", err)
	}
	func() {
		defer func() {
			if r, _ := recover().(string); !strings.Contains(r, "1 dialogue problems") {
				t.Errorf("boot did not stop on a dialogue problem: %v", r)
			}
		}()
		_ = dialogueDoor(probs, false)
	}()
	err := dialogueDoor(probs, true)
	if err == nil || !strings.Contains(err.Error(), "1 dialogue problems") {
		t.Fatalf("reload error: %v", err)
	}
	if got := reloadResultText(err); strings.Contains(got, "Done.") || !strings.Contains(got, "Reload failed: 1 dialogue problems") {
		t.Errorf("reload broadcast: %q", got)
	}
	if got := reloadResultText(nil); got != "Done." {
		t.Errorf("clean reload broadcast: %q", got)
	}
	if got := reloadResultText(errors.New("rooms panicked")); !strings.Contains(got, "rooms panicked") {
		t.Errorf("a recovered loader panic is not named: %q", got)
	}
}
