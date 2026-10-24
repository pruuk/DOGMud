package dialogue

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/require"
)

// CollectFlagReferences feeds questengine.ValidateAllFlags, whose undeclared
// flag check panics at boot. It must see every flag a topic or an opener
// variant names, with a source that says where (#375 Task 21 rewrote it over
// topics and opener variants).
func TestCollectFlagReferences_TopicsAndOpenerVariants(t *testing.T) {
	df := &DialogueFile{
		MobId: 9,
		Opener: &Opener{Text: "Hello.", Variants: []OpenerVariant{
			{Text: "Plain."},
			{Gates: Gates{QuestFlagRequired: map[string]string{"10-door": "open"}}, Text: "Open."},
		}},
		Topics: []Topic{
			{Key: "door", Kind: KindInfo,
				Gates:   Gates{QuestFlagExcluded: map[string]string{"10-door": "shut"}},
				Effects: Effects{SetsQuestFlag: &QuestFlagSet{Key: "10-door", Value: "open"}}},
			{Key: "plain", Kind: KindInfo},
		},
	}

	refs, sets := CollectFlagReferences(df)
	got := []string{}
	for _, r := range refs {
		got = append(got, r.Source+" "+r.Key+"="+r.Value)
	}
	sort.Strings(got)
	require.Equal(t, []string{"opener variant 2 10-door=open", "topic door 10-door=shut"}, got)
	require.Equal(t, []FlagRef{{Key: "10-door", Value: "open", Source: "topic door"}}, sets)

	refs, sets = CollectFlagReferences(&DialogueFile{MobId: 9})
	require.Empty(t, refs, "no opener and no topics: nothing to collect")
	require.Empty(t, sets)

	refs, sets = CollectFlagReferences(nil)
	require.Empty(t, refs)
	require.Empty(t, sets)
}
