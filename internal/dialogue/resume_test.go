package dialogue

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type resumeProbe struct {
	charged, given, granted, paid int
	flags                         map[string]string
	carries                       bool
}

func (p *resumeProbe) state(shelved bool) *PlayerState {
	return &PlayerState{
		Shelved:     func(int) bool { return shelved },
		CarriesItem: func(int) bool { return p.carries },
		ChargeGold:  func(n int) bool { p.charged += n; return true },
		GiveGold:    func(n int) { p.paid += n },
		GiveItem:    func(int) bool { p.given++; return true },
		GiveQuest:   func(string) { p.granted++ },
		SetQuestFlag: func(k, v string) {
			if p.flags == nil {
				p.flags = map[string]string{}
			}
			p.flags[k] = v
		},
	}
}

// Owner ruling 2: a resume hands over only an item worth nothing that the
// player lacks, and charges nothing; the grant and the flag still apply.
func TestApply_ResumeEffects(t *testing.T) {
	t.Cleanup(SetItemValueForTest(map[int]int{30: 0, 10038: 40}))
	df := &DialogueFile{MobId: 1}
	notice := &Topic{Key: "o", Kind: KindOffer, Effects: Effects{GrantsQuest: "10-start", GivesItem: 30,
		SetsQuestFlag: &QuestFlagSet{Key: "10-x", Value: "y"}, ChargesGold: 5}, Gates: Gates{GoldRequired: 5}}
	sling := &Topic{Key: "s", Kind: KindOffer, Effects: Effects{GrantsQuest: "50-start", GivesItem: 10038}}

	p := &resumeProbe{}
	require.True(t, df.Apply(notice, p.state(true)))
	require.Equal(t, 0, p.charged)
	require.Equal(t, 1, p.given, "a worthless item the player lacks comes back")
	require.Equal(t, 1, p.granted)
	require.Equal(t, "y", p.flags["10-x"])

	p = &resumeProbe{carries: true}
	require.True(t, df.Apply(notice, p.state(true)))
	require.Equal(t, 0, p.given, "already carried")

	p = &resumeProbe{}
	require.True(t, df.Apply(sling, p.state(true)))
	require.Equal(t, 0, p.given, "starter gear does not come back")
	require.Equal(t, 1, p.granted)

	p = &resumeProbe{}
	require.True(t, df.Apply(sling, p.state(false)))
	require.Equal(t, 1, p.given, "a first accept is unchanged")

	p = &resumeProbe{}
	require.True(t, df.Apply(notice, p.state(false)))
	require.Equal(t, 5, p.charged, "a first accept still pays")
}

// A resume pays out no givesGold either (ruling 2: no handout with resale
// value on a resume); a first accept still pays it.
func TestApply_ResumePaysNoGold(t *testing.T) {
	df := &DialogueFile{MobId: 1}
	purse := &Topic{Key: "p", Kind: KindOffer, Effects: Effects{GrantsQuest: "10-start", GivesGold: 7}}

	p := &resumeProbe{}
	require.True(t, df.Apply(purse, p.state(true)))
	require.Equal(t, 0, p.paid, "a resume pays nothing")
	require.Equal(t, 1, p.granted)

	p = &resumeProbe{}
	require.True(t, df.Apply(purse, p.state(false)))
	require.Equal(t, 7, p.paid, "a first accept pays")
}
