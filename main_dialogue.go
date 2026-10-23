package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/GoMudEngine/GoMud/internal/behaviortree"
	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// validateShippedDialogue is the boot door of the dialogue validators: the
// shipped files against the loaded quests, items, mob templates and behavior
// trees. Load mobs, quests and items first. It reports ValidateWorld
// problems plus the file-placement rules only the loaded mobs can judge; the
// dialoguecheck walk findings (orphan steps, unreachable cases) are a
// separate, advisory tool.
func validateShippedDialogue() []dialogue.Problem {
	tmpls := mobs.AllMobTemplates()
	perMob, archetype, probs := behaviorTopicIndex(tmpls)
	w, loadProbs := dialogue.RegistryWorld(perMob)
	w.ArchetypeTopics = archetype
	probs = append(probs, loadProbs...)
	probs = append(probs, dialogueFolderProblems(tmpls)...)
	return append(probs, dialogue.ValidateWorld(w)...)
}

// dialogueDoor reports the boot door's problems. At boot a problem stops the
// server (panic). On a reload it costs only the reload: the problems are
// logged and returned as one error, so the admin is told the reload failed
// and the running world keeps serving (#375 review M4).
func dialogueDoor(probs []dialogue.Problem, isReload bool) error {
	if len(probs) == 0 {
		return nil
	}
	for _, p := range probs {
		mudlog.Error("dialogue", "problem", p.String())
	}
	msg := fmt.Sprintf("%d dialogue problems; see the log above", len(probs))
	if !isReload {
		panic(msg)
	}
	return errors.New(msg)
}

// reloadResultText is the reload command's closing broadcast: "Done." only
// when every loader succeeded, else the failure, so a reload that hit a bad
// file never reads as clean.
func reloadResultText(err error) string {
	if err == nil {
		return `Done.`
	}
	return fmt.Sprintf(`Reload failed: %s.`, err)
}

// sortedTemplates is tmpls ordered by mob id, so problems come out in a
// stable order.
func sortedTemplates(tmpls []*mobs.Mob) []*mobs.Mob {
	out := append([]*mobs.Mob(nil), tmpls...)
	sort.Slice(out, func(i, j int) bool { return out[i].MobId < out[j].MobId })
	return out
}

// dialogueFolderProblems checks where each dialogue file sits. The runtime
// reads dialogue/<zone folder>/<id>.yaml with the zone folder taken from the
// mob's template (dialogue.Load), so a file in another folder, or one for an
// id no template has, is never read and its NPC is mute.
func dialogueFolderProblems(tmpls []*mobs.Mob) []dialogue.Problem {
	byId := map[int]*mobs.Mob{}
	for _, m := range tmpls {
		byId[int(m.MobId)] = m
	}
	base := filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "dialogue")
	paths, _ := filepath.Glob(filepath.Join(base, "*", "*.yaml"))
	sort.Strings(paths)
	var probs []dialogue.Problem
	for _, p := range paths {
		id, err := strconv.Atoi(strings.TrimSuffix(filepath.Base(p), ".yaml"))
		if err != nil {
			continue // LoadAllFiles reports a name that is not a mob id
		}
		where := filepath.ToSlash(p)
		tmpl := byId[id]
		if tmpl == nil {
			probs = append(probs, dialogue.Problem{MobId: id, Where: where,
				Msg: fmt.Sprintf("no mob template has id %d, so no NPC reads this file", id)})
			continue
		}
		folder, want := filepath.Base(filepath.Dir(p)), mobs.ZoneNameSanitize(tmpl.Zone)
		if folder != want {
			probs = append(probs, dialogue.Problem{MobId: id, Where: where,
				Msg: fmt.Sprintf("is in folder %q, but mob %d's zone %q reads dialogue/%s/%d.yaml, so this NPC is mute", folder, id, tmpl.Zone, want, id)})
		}
	}
	return probs
}

// behaviorTopicIndex reads the topic_is keys the runtime can serve for each
// mob: perMob from the tree at the path TryMobBehavior loads
// (behaviortree.GetBehaviorPath from the template's zone and name), and
// archetype from the archetype the template declares, which TryMobBehavior
// runs when the per-mob tree fails. A tree that does not parse is a problem
// (its keys would otherwise read as missing, a misleading cause). A behavior
// file holding topic_is keys at any other path (a stale name suffix, another
// zone folder, no such mob) serves nothing and is a problem too.
func behaviorTopicIndex(tmpls []*mobs.Mob) (perMob, archetype map[int][]string, probs []dialogue.Problem) {
	perMob, archetype = map[int][]string{}, map[int][]string{}
	expected := map[string]int{}
	archKeys := map[string][]string{}
	for _, tmpl := range sortedTemplates(tmpls) {
		id := int(tmpl.MobId)
		path := behaviortree.GetBehaviorPath(id, tmpl.Zone, tmpl.Character.Name)
		expected[filepath.Clean(path)] = id
		if _, err := os.Stat(path); err == nil {
			keys, err := behaviortree.TopicKeysInFile(path)
			if err != nil {
				probs = append(probs, behaviorParseProblem(id, path, err))
			} else if len(keys) > 0 {
				perMob[id] = keys
			}
		}
		name := tmpl.BehaviorArchetype
		if name == "" {
			continue
		}
		keys, seen := archKeys[name]
		if !seen {
			apath := behaviortree.GetArchetypePath(name)
			if _, err := os.Stat(apath); err == nil {
				if keys, err = behaviortree.TopicKeysInFile(apath); err != nil {
					probs = append(probs, behaviorParseProblem(0, apath, err))
				}
			}
			archKeys[name] = keys
		}
		if len(keys) > 0 {
			archetype[id] = keys
		}
	}

	root := filepath.Join(string(configs.GetFilePathsConfig().DataFiles), "behaviors")
	paths, _ := filepath.Glob(filepath.Join(root, "*", "*.yaml"))
	sort.Strings(paths)
	for _, p := range paths {
		if _, ok := expected[filepath.Clean(p)]; ok {
			continue
		}
		idText, _, _ := strings.Cut(filepath.Base(p), "-")
		id, err := strconv.Atoi(idText)
		if err != nil {
			continue // archetypes and items carry no mob id
		}
		keys, err := behaviortree.TopicKeysInFile(p)
		if err != nil {
			probs = append(probs, behaviorParseProblem(id, p, err))
			continue
		}
		if len(keys) == 0 {
			continue
		}
		msg := fmt.Sprintf("holds topic_is %s, but no mob template has id %d, so no tree serves them", strings.Join(keys, ", "), id)
		for path, mobId := range expected {
			if mobId == id {
				msg = fmt.Sprintf("holds topic_is %s, but mob %d's tree is read from %s, so these keys serve nothing", strings.Join(keys, ", "), id, filepath.ToSlash(path))
			}
		}
		probs = append(probs, dialogue.Problem{MobId: id, Where: filepath.ToSlash(p), Msg: msg})
	}
	return perMob, archetype, probs
}

func behaviorParseProblem(mobId int, path string, err error) dialogue.Problem {
	return dialogue.Problem{MobId: mobId, Where: filepath.ToSlash(path),
		Msg: "behavior file does not parse, so its topic_is keys cannot be checked: " + strings.Join(strings.Fields(err.Error()), " ")}
}
