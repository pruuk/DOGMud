package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v2"
)

const topicFixtureYAML = `mobid: 94
zone: thornwall_city
defaultMood: neutral
opener:
  text: "Watch business only."
  hints: "talk to the warden"
  variants:
    - questRequired: [10-start]
      text: "Any word on the racket?"
topics:
  - key: protection_report
    kind: info
    label: "I have proof of the protection racket."
    text: "Show me."
    marked: true
    heardRequired: [where_to_look]
    questRequired: [10-start]
    questExcluded: [10-report, 10-end]
    requiresItem: 30
    grantsQuest: 10-report
    moodChange: friendly
  - key: drowning_debt
    kind: offer
    quest: 10
    label: "The Drowning Post's Debt"
    playerLine: "Do you need help with the tavern's trouble?"
    text: "The keeper is paying someone. Find out who."
    breadcrumb: "Come back when the watch trusts you."
    questExcluded: [10-start, 10-end]
    grantsQuest: 10-start
`

func TestTopicFile_YAMLRoundTripKeepsInlineGatesAndEffects(t *testing.T) {
	var df DialogueFile
	require.NoError(t, yaml.UnmarshalStrict([]byte(topicFixtureYAML), &df))

	require.NotNil(t, df.Opener)
	require.Equal(t, "Watch business only.", df.Opener.Text)
	require.Len(t, df.Opener.Variants, 1)
	require.Equal(t, []string{"10-start"}, df.Opener.Variants[0].QuestRequired)

	require.Len(t, df.Topics, 2)
	info := df.Topics[0]
	require.Equal(t, KindInfo, info.Kind)
	require.Equal(t, 30, info.RequiresItem)
	require.Equal(t, "10-report", info.GrantsQuest)
	require.Equal(t, "friendly", info.MoodChange)
	require.Equal(t, []string{"where_to_look"}, info.HeardRequired)
	require.True(t, info.Marked)
	offer := df.Topics[1]
	require.Equal(t, KindOffer, offer.Kind)
	require.Equal(t, 10, offer.Quest)
	require.Equal(t, "Come back when the watch trusts you.", offer.Breadcrumb)

	out, err := yaml.Marshal(&df)
	require.NoError(t, err)
	var again DialogueFile
	require.NoError(t, yaml.UnmarshalStrict(out, &again))
	require.Equal(t, df.Topics, again.Topics)
	require.Equal(t, df.Opener, again.Opener)
}

func TestHeardKey(t *testing.T) {
	require.Equal(t, "94.where_to_look", HeardKey(94, "where_to_look"))
}
