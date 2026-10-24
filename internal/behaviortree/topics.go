package behaviortree

import (
	"fmt"
	"os"
	"sort"

	"gopkg.in/yaml.v2"
)

// condTopicIs succeeds when a player_topic event carries one of this
// branch's topic keys (quest arc #375 R12: talk topics replace typed ask
// words). The keys are `topic: <key>`, `topics: [<key>, ...]`, or both.
func condTopicIs(params map[string]any, ctx *EvalContext) Result {
	if ctx.Event.EventType != "player_topic" {
		return Failure
	}
	for _, key := range TopicIsKeys(params) {
		if key == ctx.Event.Text {
			return Success
		}
	}
	return Failure
}

// TopicIsKeys are the topic keys a topic_is check answers: its `topic`
// param, then each of its `topics`. The dialogue validator and the
// dialoguecheck walk read them through here too.
func TopicIsKeys(params map[string]any) []string {
	var keys []string
	if k := getStringParam(params, "topic"); k != "" {
		keys = append(keys, k)
	}
	for _, k := range getStringListParam(params, "topics") {
		if k != "" {
			keys = append(keys, k)
		}
	}
	return keys
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
			for _, k := range TopicIsKeys(n.Params) {
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
