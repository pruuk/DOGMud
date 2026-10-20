package behaviortree

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v2"
)

// condTopicIs succeeds when a player_topic event carries this branch's
// topic key (quest arc #375 R12: talk topics replace typed ask words).
func condTopicIs(params map[string]any, ctx *EvalContext) Result {
	if ctx.Event.EventType != "player_topic" {
		return Failure
	}
	if key := getStringParam(params, "topic"); key != "" && key == ctx.Event.Text {
		return Success
	}
	return Failure
}

// TopicKeysInFile lists the topic_is keys in a behavior file, sorted and
// unique, for the dialogue validator.
func TopicKeysInFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	seen := map[string]bool{}
	var walk func(n NodeDef)
	walk = func(n NodeDef) {
		if n.Check == "topic_is" {
			if k := getStringParam(n.Params, "topic"); k != "" {
				seen[k] = true
			}
		}
		for _, c := range n.Children {
			walk(c)
		}
		if n.Child != nil {
			walk(*n.Child)
		}
	}
	walk(def.Tree)
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out, nil
}
