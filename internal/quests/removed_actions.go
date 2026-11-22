package quests

import "fmt"

// removedActionKeys are action keys that no longer exist. yaml.v2 drops an
// unknown key without a word, so a quest file that still carried one would
// load and silently lose that action. Naming them here makes an old file fail
// to parse instead.
var removedActionKeys = map[string]string{
	"lock_exits":   "removed 2026-11-22: it locked every exit of a room for everyone; block one player with a room_gate trigger",
	"unlock_exits": "removed 2026-11-22: it unlocked every exit of a room for everyone",
}

// UnmarshalYAML decodes an action as usual after refusing a removed key.
func (a *ActionDef) UnmarshalYAML(unmarshal func(any) error) error {
	var keys map[string]any
	if err := unmarshal(&keys); err == nil {
		for k, why := range removedActionKeys {
			if _, ok := keys[k]; ok {
				return fmt.Errorf("action key %q is no longer supported (%s)", k, why)
			}
		}
	}
	type plain ActionDef
	return unmarshal((*plain)(a))
}
