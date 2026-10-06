package behaviortree

import (
	"fmt"
	"sort"
	"strings"
)

// An item tree's voice (item behaviour slice 2, Rule 16): the line pools
// its `speak` nodes draw from and the chatter level that paces its ambient
// lines. The pools live in the tree file, beside the tree that speaks them,
// so the two cannot drift apart the way an item and a separate voice file
// could.

// The chatter levels. Each names a cooldown and a chance in config.yaml
// (ItemChatter*); a tree that names none is normal.
const (
	ChatterQuiet  = "quiet"
	ChatterNormal = "normal"
	ChatterChatty = "chatty"
)

// ItemVoice is an item tree's voice.
type ItemVoice struct {
	Speech  map[string][]string // pool name to its lines
	Chatter string              // ChatterQuiet, ChatterNormal or ChatterChatty
}

// itemVoiceFrom checks a tree's speech and chatter and returns its voice:
// nil for a tree with no speech. A chatter level with no speech, an unknown
// level, an empty pool or a blank line is an error.
func itemVoiceFrom(def TreeDef) (*ItemVoice, error) {
	if len(def.Speech) == 0 {
		if def.Chatter != `` {
			return nil, fmt.Errorf("chatter %q: a tree with no speech has nothing to pace", def.Chatter)
		}
		return nil, nil
	}
	chatter := def.Chatter
	switch chatter {
	case ``:
		chatter = ChatterNormal
	case ChatterQuiet, ChatterNormal, ChatterChatty:
	default:
		return nil, fmt.Errorf("chatter %q: want %s, %s or %s", def.Chatter, ChatterQuiet, ChatterNormal, ChatterChatty)
	}
	pools := make([]string, 0, len(def.Speech))
	for name := range def.Speech {
		pools = append(pools, name)
	}
	sort.Strings(pools)
	for _, name := range pools {
		lines := def.Speech[name]
		if len(lines) == 0 {
			return nil, fmt.Errorf("speech pool %q has no lines", name)
		}
		for i, line := range lines {
			if strings.TrimSpace(line) == `` {
				return nil, fmt.Errorf("speech pool %q line %d is blank", name, i+1)
			}
		}
	}
	return &ItemVoice{Speech: def.Speech, Chatter: chatter}, nil
}

// refuseItemVoice is the mob, room and archetype loaders' check: only an
// item tree has a voice.
func refuseItemVoice(def TreeDef) error {
	if len(def.Speech) > 0 || def.Chatter != `` {
		return fmt.Errorf("speech and chatter belong only in an item tree (behaviors/items/)")
	}
	return nil
}
