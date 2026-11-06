package guards

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Gossip breadcrumbs (#667 disc line, #375 Veyra line). A fact tagged `hint`
// in facts.yaml is spoken by any NPC whose mob YAML lists it in
// `knows_facts`, on its own timer (Balance.GossipHintIntervalRounds). The
// loader only warns on an unknown id, so this guard holds the wiring:
//
//  1. Every `knows_facts` id names a fact in facts.yaml.
//  2. Every hint fact has at least one NPC who knows it, or it is never heard.
//  3. No NPC knows hints from two lines (tags ending "-line"): the timer picks
//     one of the NPC's hints at random, so a second line halves the first.
//  4. A hint's description has no dash (it is player copy).
//  5. An NPC that knows a hint is in the `gossiper` group: the idle hook
//     speaks hints only for gossipers (MobIdle_HandleIdleMobs.go), so a
//     hint on anyone else is silent. A playtest caught two smiths missing it.
type gossipFact struct {
	Id          string   `yaml:"id"`
	Description string   `yaml:"description"`
	Tags        []string `yaml:"tags"`
}

type gossipMob struct {
	file       string
	Groups     []string `yaml:"groups"`
	KnowsFacts []string `yaml:"knows_facts"`
}

var gossipHintDash = regexp.MustCompile(`[\x{2013}\x{2014}]|--| - `)

func loadGossipWorld(t *testing.T, world string) (map[string]gossipFact, []gossipMob) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(world, "facts.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	var reg struct {
		Facts []gossipFact `yaml:"facts"`
	}
	if err := yaml.Unmarshal(raw, &reg); err != nil {
		t.Fatal(err)
	}
	facts := make(map[string]gossipFact, len(reg.Facts))
	for _, f := range reg.Facts {
		facts[f.Id] = f
	}
	var mobs []gossipMob
	err = filepath.WalkDir(filepath.Join(world, "mobs"), func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return err
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var m gossipMob
		if err := yaml.Unmarshal(b, &m); err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		if len(m.KnowsFacts) > 0 {
			m.file = filepath.ToSlash(strings.TrimPrefix(path, world))
			mobs = append(mobs, m)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return facts, mobs
}

// gossipHintProblems is the guard's check, apart from the shipped world so the
// can-fail fixture runs the same code.
func gossipHintProblems(facts map[string]gossipFact, mobs []gossipMob) []string {
	var out []string
	known := map[string]bool{}
	for _, m := range mobs {
		lines := map[string]bool{}
		silent := false
		for _, id := range m.KnowsFacts {
			f, ok := facts[id]
			if !ok {
				out = append(out, fmt.Sprintf("%s knows %q, which is not in facts.yaml", m.file, id))
				continue
			}
			if !slices.Contains(f.Tags, "hint") {
				continue
			}
			if !slices.Contains(m.Groups, "gossiper") {
				silent = true
				continue
			}
			known[id] = true
			for _, tag := range f.Tags {
				if strings.HasSuffix(tag, "-line") {
					lines[tag] = true
				}
			}
		}
		if silent {
			out = append(out, fmt.Sprintf("%s knows a hint but is not in the gossiper group, so it never says it", m.file))
		}
		if len(lines) > 1 {
			names := make([]string, 0, len(lines))
			for l := range lines {
				names = append(names, l)
			}
			sort.Strings(names)
			out = append(out, fmt.Sprintf("%s knows hints from %s; one line per NPC", m.file, strings.Join(names, " and ")))
		}
	}
	for id, f := range facts {
		if !slices.Contains(f.Tags, "hint") {
			continue
		}
		if !known[id] {
			out = append(out, fmt.Sprintf("hint fact %q: no NPC knows it, so it is never heard", id))
		}
		if gossipHintDash.MatchString(f.Description) {
			out = append(out, fmt.Sprintf("hint fact %q uses a dash: %q", id, f.Description))
		}
	}
	sort.Strings(out)
	return out
}

func TestGossipHintFactsAreWired(t *testing.T) {
	world := filepath.Join(repoRoot(t), "_datafiles", "world", "dogmud")
	facts, mobs := loadGossipWorld(t, world)
	// Floors: 9 hint facts (6 disc line, 3 Veyra line) known by 13 NPCs
	// (8 disc, 5 Veyra), counted 2026-11-06. Fewer means the walk broke.
	hints, lines := 0, map[string]int{}
	for _, f := range facts {
		if slices.Contains(f.Tags, "hint") {
			hints++
		}
	}
	for _, m := range mobs {
		for _, id := range m.KnowsFacts {
			for _, tag := range facts[id].Tags {
				if strings.HasSuffix(tag, "-line") && slices.Contains(facts[id].Tags, "hint") {
					lines[tag+" "+m.file] = 1
				}
			}
		}
	}
	if hints < 9 || len(lines) < 13 {
		t.Fatalf("read %d hint facts and %d NPC line pairs; want at least 9 and 13", hints, len(lines))
	}
	for _, p := range gossipHintProblems(facts, mobs) {
		t.Error(p)
	}
}

// The check can fail: each rule fires on a fixture built to break it.
func TestGossipHintFactsGuard_CanFail(t *testing.T) {
	facts := map[string]gossipFact{
		"a":     {Id: "a", Description: "fine.", Tags: []string{"x-line", "hint"}},
		"b":     {Id: "b", Description: "two lines.", Tags: []string{"y-line", "hint"}},
		"lone":  {Id: "lone", Description: "nobody knows.", Tags: []string{"x-line", "hint"}},
		"dashy": {Id: "dashy", Description: "a dash — here.", Tags: []string{"x-line", "hint"}},
	}
	mobs := []gossipMob{
		{file: "/mobs/one.yaml", Groups: []string{"gossiper"}, KnowsFacts: []string{"a", "b", "dashy"}},
		{file: "/mobs/two.yaml", Groups: []string{"gossiper"}, KnowsFacts: []string{"missing"}},
		{file: "/mobs/mute.yaml", Groups: []string{"merchant"}, KnowsFacts: []string{"a"}},
	}
	got := strings.Join(gossipHintProblems(facts, mobs), "\n")
	for _, want := range []string{
		`/mobs/two.yaml knows "missing"`,
		"/mobs/one.yaml knows hints from x-line and y-line",
		`hint fact "lone": no NPC knows it`,
		`hint fact "dashy" uses a dash`,
		"/mobs/mute.yaml knows a hint but is not in the gossiper group",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("fixture did not report %q; got:\n%s", want, got)
		}
	}
}
