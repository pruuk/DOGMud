package behaviortree

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v2"
)

// knownFields lists the structural YAML keys that should be stripped
// from Params before passing to conditions/actions.
var knownFields = map[string]struct{}{
	"type":     {},
	"event":    {},
	"children": {},
	"check":    {},
	"do":       {},
	"mod":      {},
	"note":     {},
	"child":    {},
}

// LoadTreeFromFile reads a YAML file and compiles it into a Node tree.
func LoadTreeFromFile(path string) (Node, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return LoadTreeFromBytes(data)
}

// LoadArchetypeYAMLFromFile reads an archetype YAML file and returns
// the compiled tree Node, the chunk-4.2 goal_weights map, AND any
// chunk-4.3 default_goals list.
func LoadArchetypeYAMLFromFile(path string) (Node, map[string]float64, []GoalDefault, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, nil, err
	}
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, nil, nil, fmt.Errorf("parse error: %w", err)
	}
	if err := refuseItemVoice(def); err != nil {
		return nil, nil, nil, err
	}
	// Archetype trees compile under the root label "arch" (per-mob and
	// room trees use "root"). Cooldown/delay decorator state keys are
	// positional (path + "_cooldown" / "_delay"), and a composed mob
	// evaluates BOTH its per-mob tree and its archetype against the same
	// per-instance BehaviorState — with a shared root label, a decorator
	// at position N in one tree would silently share state with position
	// N in the other. Distinct labels disjoint the keyspaces. Note: this
	// one-time key rename CLEARS any in-flight archetype decorator state,
	// so a renamed cooldown can fire once immediately before re-arming —
	// a one-time, harmless hiccup, not a permanent behavior change.
	tree, err := compileNode(def.Tree, "arch")
	if err != nil {
		return nil, nil, nil, err
	}
	return tree, def.GoalWeights, def.DefaultGoals, nil
}

// LoadTreeFromBytes parses YAML bytes and compiles into a Node tree.
func LoadTreeFromBytes(data []byte) (Node, error) {
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}
	if err := refuseItemVoice(def); err != nil {
		return nil, err
	}
	return compileNode(def.Tree, "root")
}

// itemRootLabel is the root label item trees compile under (lighting 5e,
// Rule 1), so their decorator state keys can never meet a mob's ("root")
// or an archetype's ("arch"), and so the compiler knows which subject a
// node is being compiled for.
const itemRootLabel = "item"

// LoadItemTreeFromFile reads an item tree YAML file and compiles it,
// returning its voice too (nil when it has no speech).
func LoadItemTreeFromFile(path string) (Node, *ItemVoice, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}
	return loadItemTreeDef(data)
}

// LoadItemTreeFromBytes parses an item tree and compiles it under the item
// root label, which holds every node to the item-safe allowlist (Rule 8).
func LoadItemTreeFromBytes(data []byte) (Node, error) {
	node, _, err := loadItemTreeDef(data)
	return node, err
}

// loadItemTreeDef parses an item tree, checks its voice (item behaviour
// slice 2) and compiles its tree.
func loadItemTreeDef(data []byte) (Node, *ItemVoice, error) {
	var def TreeDef
	if err := yaml.Unmarshal(data, &def); err != nil {
		return nil, nil, fmt.Errorf("parse error: %w", err)
	}
	voice, err := itemVoiceFrom(def)
	if err != nil {
		return nil, nil, err
	}
	if err := checkSpeakNodes(def.Tree, voice, itemRootLabel); err != nil {
		return nil, nil, err
	}
	node, err := compileNode(def.Tree, itemRootLabel)
	if err != nil {
		return nil, nil, err
	}
	return node, voice, nil
}

// isItemTreePath reports whether a compile path belongs to an item tree.
func isItemTreePath(path string) bool {
	return path == itemRootLabel || strings.HasPrefix(path, itemRootLabel+".")
}

// Rule 8: the nodes an item tree may name. An allowlist, not a blocklist
// (X11): the 87 mob and room actions and 66 conditions carry no subject
// metadata, and a node not read for an item subject is refused rather than
// trusted. Decorators and composites are always allowed.
var (
	itemSafeConditions = map[string]bool{
		"time_of_day":        true,
		"round_mod":          true,
		"random_chance":      true,
		"state_equals":       true,
		"state_greater_than": true,
		"holder_asleep":      true,
		"worn":               true,
		"in_combat":          true,
		"chatter_ready":      true,
		"hunger_overdue":     true,
	}
	itemSafeActions = map[string]bool{
		"set_state":       true,
		"increment_state": true,
		"decrement_state": true,
		"set_light":       true,
		"pulse_light":     true,
		"speak":           true,
		"taunt_pull":      true,
	}
	// itemOnlyNodes need an item subject, so a mob or room tree may not
	// name them.
	itemOnlyNodes = map[string]bool{
		"holder_asleep":  true,
		"worn":           true,
		"in_combat":      true,
		"set_light":      true,
		"pulse_light":    true,
		"chatter_ready":  true,
		"hunger_overdue": true,
		"speak":          true,
		"taunt_pull":     true,
	}
)

// checkNodeSubject refuses a node the tree's subject may not name.
func checkNodeSubject(path, kind, name string) error {
	if isItemTreePath(path) {
		safe := itemSafeConditions
		if kind == "action" {
			safe = itemSafeActions
		}
		if !safe[name] {
			return fmt.Errorf("%s: %s %q is not allowed in an item tree (item-safe nodes only)", path, kind, name)
		}
		return nil
	}
	if itemOnlyNodes[name] {
		return fmt.Errorf("%s: %s %q needs an item subject and is allowed only in an item tree", path, kind, name)
	}
	return nil
}

// compileNode recursively converts a NodeDef into a concrete Node.
func compileNode(def NodeDef, path string) (Node, error) {
	var node Node
	var err error

	switch def.Type {
	case "selector":
		node, err = compileComposite(def, path, "selector")
	case "sequence":
		node, err = compileComposite(def, path, "sequence")
	case "condition":
		node, err = compileCondition(def, path)
	case "action":
		node, err = compileAction(def, path)
	case "decorator":
		node, err = compileDecorator(def, path)
	default:
		return nil, fmt.Errorf("%s: unknown node type %q", path, def.Type)
	}

	if err != nil {
		return nil, err
	}

	// Wrap in EventFilterNode if event is specified
	if def.Event != "" {
		node = &EventFilterNode{
			EventType: def.Event,
			Child:     node,
		}
	}

	return node, nil
}

func compileComposite(def NodeDef, path, kind string) (Node, error) {
	children := make([]Node, 0, len(def.Children))
	for i, childDef := range def.Children {
		childPath := fmt.Sprintf("%s.%s[%d]", path, kind, i)
		child, err := compileNode(childDef, childPath)
		if err != nil {
			return nil, err
		}
		children = append(children, child)
	}

	switch kind {
	case "selector":
		return &SelectorNode{Children: children}, nil
	default:
		return &SequenceNode{Children: children}, nil
	}
}

func compileCondition(def NodeDef, path string) (Node, error) {
	if def.Check == "" {
		return nil, fmt.Errorf("%s: condition node missing 'check' field", path)
	}
	fn := LookupCondition(def.Check)
	if fn == nil {
		return nil, fmt.Errorf("%s: unknown condition %q", path, def.Check)
	}
	if err := checkNodeSubject(path, "condition", def.Check); err != nil {
		return nil, err
	}
	return &ConditionNode{
		Name:   def.Check,
		Params: cleanParams(def),
		Fn:     fn,
	}, nil
}

func compileAction(def NodeDef, path string) (Node, error) {
	if def.Do == "" {
		return nil, fmt.Errorf("%s: action node missing 'do' field", path)
	}
	fn := LookupAction(def.Do)
	if fn == nil {
		return nil, fmt.Errorf("%s: unknown action %q", path, def.Do)
	}
	if err := checkNodeSubject(path, "action", def.Do); err != nil {
		return nil, err
	}
	return &ActionNode{
		Name:   def.Do,
		Params: cleanParams(def),
		Fn:     fn,
	}, nil
}

func compileDecorator(def NodeDef, path string) (Node, error) {
	if def.Child == nil {
		return nil, fmt.Errorf("%s: decorator node missing 'child' field", path)
	}

	childPath := fmt.Sprintf("%s.decorator[0]", path)
	child, err := compileNode(*def.Child, childPath)
	if err != nil {
		return nil, err
	}

	params := cleanParams(def)

	switch def.Mod {
	case "cooldown":
		return &CooldownDecorator{
			Rounds:   getIntParam(params, "rounds"),
			StateKey: path + "_cooldown",
			Child:    child,
		}, nil
	case "repeat":
		return &RepeatDecorator{
			Times: getIntParam(params, "times"),
			Child: child,
		}, nil
	case "invert":
		return &InvertDecorator{
			Child: child,
		}, nil
	case "random":
		return &RandomDecorator{
			Percent: getIntParam(params, "percent"),
			Child:   child,
		}, nil
	case "delay":
		return &DelayDecorator{
			Rounds:   getIntParam(params, "rounds"),
			StateKey: path + "_delay",
			Child:    child,
		}, nil
	default:
		return nil, fmt.Errorf("%s: unknown decorator mod %q", path, def.Mod)
	}
}

// cleanParams returns a copy of the inline Params map with structural
// YAML keys removed, leaving only user-defined parameters.
func cleanParams(def NodeDef) map[string]any {
	result := make(map[string]any, len(def.Params))
	for k, v := range def.Params {
		if _, known := knownFields[k]; !known {
			result[k] = v
		}
	}
	return result
}
