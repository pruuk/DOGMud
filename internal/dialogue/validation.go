package dialogue

import "fmt"

// FlagRef is a reference to a quest flag found in a dialogue file.
type FlagRef struct {
	Key    string
	Value  string
	Source string // e.g. "topic report_back", "opener variant 2"
}

// CollectFlagReferences scans a DialogueFile for all quest flag references.
// Returns refs (from questFlagRequired/questFlagExcluded on topics and opener
// variants) and sets (from a topic's setsQuestFlag; opener variants carry no
// effects).
func CollectFlagReferences(df *DialogueFile) (refs []FlagRef, sets []FlagRef) {
	if df == nil {
		return
	}

	gateRefs := func(g Gates, source string) {
		for k, v := range g.QuestFlagRequired {
			refs = append(refs, FlagRef{Key: k, Value: v, Source: source})
		}
		for k, v := range g.QuestFlagExcluded {
			refs = append(refs, FlagRef{Key: k, Value: v, Source: source})
		}
	}

	if df.Opener != nil {
		for i, v := range df.Opener.Variants {
			gateRefs(v.Gates, fmt.Sprintf("opener variant %d", i+1))
		}
	}

	for _, t := range df.Topics {
		source := "topic " + t.Key
		gateRefs(t.Gates, source)
		if t.SetsQuestFlag != nil {
			sets = append(sets, FlagRef{Key: t.SetsQuestFlag.Key, Value: t.SetsQuestFlag.Value, Source: source})
		}
	}

	return
}
