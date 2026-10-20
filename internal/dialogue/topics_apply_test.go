package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApply_InfoRecordsHeardAndGrants(t *testing.T) {
	df := loadFixture(t, `mobid: 56
zone: testzone
topics:
  - key: clue
    kind: info
    label: "Any clues?"
    text: "Look under the bridge."
    grantsQuest: 4-clue
`)
	f := newFake()
	require.True(t, df.Apply(df.FindTopic("clue"), f.ps()))
	require.True(t, f.tokens["4-clue"], "grant landed")
	require.True(t, f.heard["56.clue"], "heard recorded")
}

func TestApply_OfferIsNotRecordedAsHeard(t *testing.T) {
	df, f := loadFixture(t, menuFixtureYAML), newFake()
	f.gold = 5
	require.True(t, df.Apply(df.FindTopic("hunt"), f.ps()))
	require.True(t, f.tokens["7-start"])
	require.False(t, f.heard["50.hunt"], "quest state lives in tokens")
}

func TestApply_TakesRequiredItem(t *testing.T) {
	df := loadFixture(t, `mobid: 52
zone: testzone
topics:
  - key: purse
    kind: handin
    quest: 9
    label: "The Lost Purse"
    playerLine: "I found your purse."
    text: "Thank you."
    requiresItem: 77
    questExcluded: [9-end]
    grantsQuest: 9-end
`)
	f := newFake()
	f.items[77] = true
	require.True(t, df.Apply(df.FindTopic("purse"), f.ps()))
	require.False(t, f.items[77])
	require.True(t, f.tokens["9-end"])
	require.False(t, f.heard["52.purse"], "only info topics are recorded as heard")
}

func TestApply_FailedChargeChangesNothing(t *testing.T) {
	df := loadFixture(t, `mobid: 53
zone: testzone
topics:
  - key: paid
    kind: info
    label: "Tell me the secret."
    text: "For a price."
    chargesGold: 10
    goldRequired: 10
    grantsQuest: 3-told
    questExcluded: [3-told, 3-end]
`)
	f := newFake()
	f.gold = 4
	require.False(t, df.Apply(df.FindTopic("paid"), f.ps()))
	require.Equal(t, 4, f.gold)
	require.False(t, f.tokens["3-told"])
	require.False(t, f.heard["53.paid"])
}

func TestApply_FailedDeliveryRefundsAndIsNotHeard(t *testing.T) {
	df := loadFixture(t, `mobid: 54
zone: testzone
topics:
  - key: gift
    kind: info
    label: "Can you spare a lantern?"
    text: "Take this."
    chargesGold: 2
    goldRequired: 2
    givesItem: 900
`)
	f := newFake()
	f.gold = 2
	ps := f.ps()
	ps.GiveItem = func(int) bool { return false }
	require.False(t, df.Apply(df.FindTopic("gift"), ps))
	require.Equal(t, 2, f.gold, "charge refunded")
	require.False(t, f.heard["54.gift"])
}
