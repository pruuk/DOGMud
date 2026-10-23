// Command dialoguecheck checks a dialogue folder before it lands (quest arc
// #375 phase 1, written for plan Task 15's converted output). It is a dev
// tool for authors; it outlived the one-time converter cmd/dialogueconvert,
// deleted in Task 24.
//
//	go run ./cmd/dialoguecheck -data _datafiles/world/dogmud -dialogue <folder> [-zones a,b]
//
// It does two things:
//
//  1. Task 15 step 3: it loads the dialogue folder through
//     dialogue.LoadAllFiles and runs dialogue.ValidateWorld with the real
//     quest and item registries (dialogue.RegistryWorld). Every problem is
//     a VALIDATION line.
//  2. A gate walk from a fresh character (no tokens, nothing heard, no
//     items): a fixpoint over every topic, quest trigger, quest reward and
//     behavior-tree grant, then DEAD END, RE-OFFER, UNSTARTABLE QUEST,
//     ORPHAN STEP, UNREACHABLE TOPIC and ASSUMED lines. See walk.go for what
//     is modeled.
//
// -retired 29,30 names quests that are deliberately not startable: they print
// an informational RETIRED line instead of UNSTARTABLE QUEST, and the walk
// seeds a holder at their start step so DEAD END still checks that holders can
// finish.
//
// -zones filters the report; reachability is always computed over the whole
// world, because quests cross zones. The exit status is 1 when any error
// class (VALIDATION, DEAD END, RE-OFFER, UNSTARTABLE QUEST) is shown.
//
// When the dialogue folder sits in an output folder of the retired
// converter, the sibling report.md is applied (-report auto): the quest
// triggers it deletes (Task 16) are left out, and the player_ask branches it
// rewrites (Task 17) are read as player_topic branches. A folder with no
// report.md (the shipped dialogue) applies nothing. -report none applies
// nothing either.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
	"github.com/GoMudEngine/GoMud/internal/quests"
)

type options struct {
	Data     string
	Dialogue string
	Zones    string
	Report   string
	Retired  string
}

func main() {
	var o options
	flag.StringVar(&o.Data, "data", "_datafiles/world/dogmud", "data root (quests/, items/, behaviors/, mobs/, rooms/)")
	flag.StringVar(&o.Dialogue, "dialogue", "", "dialogue folder to check, named dialogue (zone folders of <mobid>.yaml); required")
	flag.StringVar(&o.Zones, "zones", "", "comma-separated zone folders to report on (default: all)")
	flag.StringVar(&o.Report, "report", "auto", "converter report.md to apply; auto uses <dialogue>/../report.md if present; none applies nothing")
	flag.StringVar(&o.Retired, "retired", "", "comma-separated quest ids retired by design (not startable; holders must still finish)")
	flag.Parse()

	code, err := run(os.Stdout, o)
	if err != nil {
		fmt.Fprintln(os.Stderr, "dialoguecheck:", err)
		os.Exit(2)
	}
	os.Exit(code)
}

// setDataFiles points FilePaths.DataFiles at dir in memory only (no
// config-overrides.yaml write, unlike configs.SetVal) and checks it took.
func setDataFiles(dir string) error {
	if err := configs.AddOverlayOverrides(map[string]any{"FilePaths.DataFiles": dir}); err != nil {
		return err
	}
	if got := string(configs.GetFilePathsConfig().DataFiles); got != dir {
		return fmt.Errorf("could not point FilePaths.DataFiles at %s (it reads %s; does config-overrides.yaml set it?)", dir, got)
	}
	return nil
}

func loadRegistries() (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("loading quests and items panicked: %v", r)
		}
	}()
	items.LoadDataFiles()
	quests.LoadDataFiles()
	if len(quests.GetAllQuests()) == 0 {
		return errors.New("no quests loaded; check -data")
	}
	return nil
}

var questTriggerWhere = regexp.MustCompile(`^quest (\d+) trigger (\d+)$`)

func run(out io.Writer, o options) (int, error) {
	if o.Dialogue == "" {
		return 0, errors.New("-dialogue is required")
	}
	data, err := filepath.Abs(o.Data)
	if err != nil {
		return 0, err
	}
	dlg, err := filepath.Abs(o.Dialogue)
	if err != nil {
		return 0, err
	}
	if filepath.Base(dlg) != "dialogue" {
		return 0, fmt.Errorf("-dialogue %s: the folder must be named dialogue (the loader reads <root>/dialogue)", dlg)
	}

	mudlog.SetupLogger(nil, "LOW", "", false)
	if _, err := os.Stat(filepath.Join("_datafiles", "config.yaml")); err == nil {
		if err := configs.ReloadConfig(); err != nil {
			return 0, err
		}
	}
	if err := setDataFiles(filepath.ToSlash(data)); err != nil {
		return 0, err
	}
	if err := loadRegistries(); err != nil {
		return 0, err
	}

	var plan *convPlan
	reportPath := o.Report
	if reportPath == "auto" {
		reportPath = filepath.Join(filepath.Dir(dlg), "report.md")
		if _, err := os.Stat(reportPath); err != nil {
			reportPath = ""
		}
	}
	if reportPath != "" && reportPath != "none" {
		b, err := os.ReadFile(reportPath)
		if err != nil {
			return 0, err
		}
		if plan, err = parseReport(string(b)); err != nil {
			return 0, err
		}
	}

	trees, err := readTrees(data)
	if err != nil {
		return 0, err
	}
	behaviorTopics := map[int][]string{}
	for _, tf := range trees {
		if tf.MobId == 0 {
			continue
		}
		keys, err := behaviortree.TopicKeysInFile(filepath.Join(data, filepath.FromSlash(tf.Path)))
		if err != nil {
			return 0, err
		}
		behaviorTopics[tf.MobId] = append(behaviorTopics[tf.MobId], keys...)
		if plan != nil {
			for _, bp := range plan.Branches[tf.Path] {
				behaviorTopics[tf.MobId] = append(behaviorTopics[tf.MobId], bp.Topics...)
			}
		}
	}
	for id, keys := range behaviorTopics {
		if len(keys) == 0 {
			delete(behaviorTopics, id)
			continue
		}
		sort.Strings(keys)
		behaviorTopics[id] = uniq(keys)
	}

	// The real loader, pointed at the folder under test.
	if err := setDataFiles(filepath.ToSlash(filepath.Dir(dlg))); err != nil {
		return 0, err
	}
	world, probs := dialogue.RegistryWorld(behaviorTopics)
	if err := setDataFiles(filepath.ToSlash(data)); err != nil {
		return 0, err
	}
	if plan != nil {
		var kept []string
		for _, g := range world.ItemGiveGrants {
			if m := questTriggerWhere.FindStringSubmatch(g); m != nil {
				q, _ := strconv.Atoi(m[1])
				n, _ := strconv.Atoi(m[2])
				if plan.DeleteTriggers[q][n] {
					continue
				}
			}
			kept = append(kept, g)
		}
		world.ItemGiveGrants = kept
	}
	probs = append(probs, dialogue.ValidateWorld(world)...)

	folders, err := dialogueFolders(dlg)
	if err != nil {
		return 0, err
	}
	in := Input{Quests: quests.GetAllQuests(), Trees: trees, Plan: plan}
	if in.Retired, err = parseRetired(o.Retired); err != nil {
		return 0, err
	}
	for _, f := range world.Files {
		in.Files = append(in.Files, dlgFile{Folder: folders[f.MobId], File: f})
	}
	if in.WorldItems, err = readWorldItems(data); err != nil {
		return 0, err
	}
	if in.RoomZones, err = readRoomZones(data); err != nil {
		return 0, err
	}
	in.ItemTokens = map[int]string{}
	for _, spec := range items.GetAllItemSpecs() {
		if spec.QuestToken != "" {
			in.ItemTokens[spec.ItemId] = spec.QuestToken
		}
	}
	if repo := repoRoot(data); repo != "" {
		if in.CodeGrants, err = readCodeGrants(repo); err != nil {
			return 0, err
		}
	}

	w := newWalker(in)
	w.fixpoint()
	w.check()
	findings := w.findings
	for _, p := range probs {
		var zones []string
		if z := folders[p.MobId]; p.MobId != 0 && z != "" {
			zones = []string{z}
		} else if m := questTriggerWhere.FindStringSubmatch(p.Where); m != nil {
			q, _ := strconv.Atoi(m[1])
			if qm := w.qs[q]; qm != nil {
				zones = sortedKeys(qm.zones)
			}
		}
		findings = append(findings, Finding{Class: ClassValidation, Zones: zones, Text: p.String()})
	}

	fmt.Fprintf(out, "# dialoguecheck: data %s, dialogue %s\n", filepath.ToSlash(data), filepath.ToSlash(dlg))
	if plan != nil {
		nd, nb := 0, 0
		for _, m := range plan.DeleteTriggers {
			nd += len(m)
		}
		for _, m := range plan.Branches {
			nb += len(m)
		}
		fmt.Fprintf(out, "# converter report applied: %s (%d quest triggers left out, %d player_ask branches read as player_topic)\n", filepath.ToSlash(reportPath), nd, nb)
	} else {
		fmt.Fprintln(out, "# no converter report applied")
	}
	fmt.Fprintf(out, "# %d dialogue files, %d quests, %d behavior trees, %d sources walked\n", len(in.Files), len(in.Quests), len(trees), len(w.srcs))
	// A file still carrying old patterns or a tree no longer decodes: the
	// strict LoadAllFiles reports it as a VALIDATION line (#375 Task 21).
	return report(out, findings, o.Zones), nil
}

// parseRetired reads the -retired list of quest ids.
func parseRetired(list string) (map[int]bool, error) {
	out := map[int]bool{}
	for _, f := range strings.Split(list, ",") {
		if f = strings.TrimSpace(f); f == "" {
			continue
		}
		id, err := strconv.Atoi(f)
		if err != nil {
			return nil, fmt.Errorf("-retired %q: %q is not a quest id", list, f)
		}
		out[id] = true
	}
	return out, nil
}

func uniq(s []string) []string {
	var out []string
	for i, v := range s {
		if i == 0 || v != s[i-1] {
			out = append(out, v)
		}
	}
	return out
}

// report prints the findings in the zone filter, sorted by class and text,
// then a count per class, and returns the exit status.
func report(out io.Writer, findings []Finding, zoneList string) int {
	filter := map[string]bool{}
	for _, z := range strings.Split(zoneList, ",") {
		if z = strings.TrimSpace(z); z != "" {
			filter[z] = true
		}
	}
	shown := func(f Finding) bool {
		if len(filter) == 0 || len(f.Zones) == 0 {
			return true
		}
		for _, z := range f.Zones {
			if filter[z] {
				return true
			}
		}
		return false
	}
	rank := map[string]int{}
	for i, c := range classOrder {
		rank[c] = i
	}
	var keep []Finding
	hidden := 0
	for _, f := range findings {
		if shown(f) {
			keep = append(keep, f)
		} else {
			hidden++
		}
	}
	sort.SliceStable(keep, func(i, j int) bool {
		if rank[keep[i].Class] != rank[keep[j].Class] {
			return rank[keep[i].Class] < rank[keep[j].Class]
		}
		return keep[i].Text < keep[j].Text
	})
	counts := map[string]int{}
	code := 0
	for _, f := range keep {
		fmt.Fprintln(out, f.String())
		counts[f.Class]++
		if isError(f.Class) {
			code = 1
		}
	}
	var parts []string
	for _, c := range classOrder {
		parts = append(parts, fmt.Sprintf("%s %d", c, counts[c]))
	}
	scope := "whole world"
	if len(filter) > 0 {
		scope = "zones " + strings.Join(sortedKeys(filter), ",") + fmt.Sprintf(" (%d findings outside the filter not shown)", hidden)
	}
	fmt.Fprintf(out, "summary (%s): %s\n", scope, strings.Join(parts, ", "))
	return code
}
