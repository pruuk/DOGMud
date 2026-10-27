package questengine

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/dialogue"
	"github.com/stretchr/testify/require"
	yamlv2 "gopkg.in/yaml.v2"
)

// Dewey's timed lines lagged the player (2026-10-17 newbie-duo playtest):
// "Next, your skills... Type skills." arrived after the player had already
// typed skills, and his answer to `ask dewey world` arrived after his
// next-step line. Two causes:
//
//   - npc_say line delays add up. Each line goes through mobs.Mob.Command,
//     which schedules from the mob's last queued command, so delays of 0, 3,
//     8 and 13 put the last line 24 seconds out, not 13.
//   - On `ask`, the quest engine's dialogue trigger runs before the dialogue
//     reply is queued (usercommands askNpcChain), so a trigger's npc_say line
//     always lands ahead of the reply, whatever its delay.

// Every step's lines, the next-step prompt last among them, are all said
// within a few seconds of the step finishing. The two cooldown steps are the
// exception and have their own test below.
func TestQuest28_NpcLinesFinishWithinAFewSeconds(t *testing.T) {
	const maxSeconds = 8
	q := loadQuest28(t)
	seen := 0
	for _, tr := range q.Triggers {
		if quest28CooldownStep(tr) {
			continue
		}
		for _, a := range tr.Actions {
			if a.NpcSay == nil {
				continue
			}
			seen++
			total := npcSaySeconds(a.NpcSay)
			require.LessOrEqual(t, total, maxSeconds,
				"the %s %s%s trigger's lines take %d seconds (delays add up line to line); the player is ahead of them by then",
				tr.Event, tr.Command, tr.Noun, total)
		}
	}
	require.Greater(t, seen, 10, "the walk must reach quest 28's npc_say lines")
}

// Warcry, cast and trip share the special-move cooldown (#621, owner's prod
// spot check 2026-10-17). The step after warcry prompts cast spike and the
// step after the cast prompts trip, so each of those prompts waits out one
// cooldown, plus a second so the prompt never lands on the round the cooldown
// clears, and not much longer.
//
// The cooldown is read from the shipped config.yaml: a test binary's own
// config holds the Go defaults, not the shipped values.
func TestQuest28_CooldownStepsWaitOutTheSharedCooldown(t *testing.T) {
	var shipped struct {
		Timing struct {
			RoundSeconds int `yaml:"RoundSeconds"`
		} `yaml:"Timing"`
		Balance struct {
			SpecialMoveCooldown int `yaml:"SpecialMoveCooldown"`
		} `yaml:"Balance"`
	}
	require.NoError(t, yamlv2.Unmarshal(repoFile(t, "_datafiles", "config.yaml"), &shipped))
	cooldown := shipped.Timing.RoundSeconds * shipped.Balance.SpecialMoveCooldown
	require.Positive(t, cooldown, "config.yaml must ship RoundSeconds and SpecialMoveCooldown")
	least, most := cooldown+1, cooldown+4

	q := loadQuest28(t)
	seen := 0
	for _, tr := range q.Triggers {
		if !quest28CooldownStep(tr) {
			continue
		}
		for _, a := range tr.Actions {
			if a.NpcSay == nil {
				continue
			}
			seen++
			total := npcSaySeconds(a.NpcSay)
			require.GreaterOrEqual(t, total, least,
				"the %s %s%s prompt comes %d seconds after the move before it, while that move's cooldown (%d seconds) still rests",
				tr.Event, tr.Command, tr.Noun, total, cooldown)
			require.LessOrEqual(t, total, most,
				"the %s %s%s prompt waits %d seconds, well past the %d second cooldown",
				tr.Event, tr.Command, tr.Noun, total, cooldown)
		}
	}
	require.Equal(t, 2, seen, "the attack and cast steps must both prompt the next big move")
}

// quest28CooldownStep reports the two triggers whose prompt names a big move
// right after another big move: attack (after warcry) and cast (before trip).
func quest28CooldownStep(tr TriggerDef) bool {
	return (tr.Command == "attack" && tr.Noun == "effigy") ||
		(tr.Command == "cast" && tr.Noun == "conviction-spike")
}

func npcSaySeconds(n *NpcSayDef) int {
	total := 0
	for _, l := range n.Lines {
		total += l.Delay
	}
	return total
}

// The 28-asked step is Dewey's talk lesson topic (#375 R20, Task 19): only
// hearing that topic grants it (the quest's one topic_heard trigger on it,
// #375 phase 3b), the reply ends on the way north, and his opener then says
// the way is open.
func TestQuest28_TalkLessonGrantsTheStepWithItsReply(t *testing.T) {
	q := loadQuest28(t)
	lessonTriggers := 0
	for i, tr := range q.Triggers {
		require.NotEqual(t, "dialogue", tr.Event,
			"trigger %d: the ask chain that fired dialogue triggers is gone (#375 Task 19); the lesson is a talk topic", i)
		for _, a := range tr.Actions {
			if a.Grant != "28-asked" {
				continue
			}
			require.True(t, tr.Event == "topic_heard" && tr.Mob == 9491 && tr.Topic == "asked_28",
				"trigger %d grants 28-asked: only hearing Dewey's talk topic may, or the step skips the talk lesson", i)
			lessonTriggers++
		}
	}
	require.Equal(t, 1, lessonTriggers, "one topic_heard trigger on Dewey's asked_28 grants 28-asked")

	// #375 (R20): the ask step is Dewey's lesson topic. Choosing it plays the
	// reply, and hearing it fires the quest's topic_heard trigger, which grants
	// 28-asked; the reply itself ends on the way north.
	df := loadShippedDialogue(t, "newcomer_antechamber", "9491.yaml")
	set := map[string]bool{"28-start": true, "28-said": true}
	ps := &dialogue.PlayerState{
		QuestOpen: func(int) bool { return true },
		HasQuest:  func(tok string) bool { return set[tok] },
		GiveQuest: func(tok string) { set[tok] = true },
		HasHeard:  func(string) bool { return false },
		MarkHeard: func(string) {},
	}
	lesson := df.FindTopic("asked_28")
	require.NotNil(t, lesson, "Dewey's talk lesson topic")
	require.True(t, df.Eligible(lesson, ps), "the lesson is on the menu on the ask step")
	require.Empty(t, lesson.GrantsQuest, "the topic grants nothing itself; the quest's topic_heard trigger does")
	require.True(t, strings.HasSuffix(strings.TrimSpace(squash(lesson.Text)), "before you go north."),
		"the lesson's reply must end with the way north; got %q", lesson.Text)
	require.True(t, df.Apply(lesson, ps))
	require.False(t, set["28-asked"], "Apply alone grants nothing")

	e := NewEngine()
	e.RegisterQuest(&q)
	player := newFullMockPlayer(0)
	for tok := range set {
		player.quests[tok] = true
	}
	e.Notify("topic_heard", EventDetails{UserId: 1, MobId: 9491, Topic: "asked_28"}, player, newFullMockActionContext(1, player))
	require.True(t, player.quests["28-asked"], "hearing the lesson grants 28-asked")
	set["28-asked"] = true

	// After the lesson Dewey's opener says the way north is open; once the
	// player has walked on, it is not repeated.
	text, _ := df.OpenerFor(ps)
	require.Contains(t, squash(text), "the way north is open", "Dewey's opener after the lesson")
	set["28-proving_prompt"] = true
	text, _ = df.OpenerFor(ps)
	require.NotContains(t, squash(text), "the way north is open", "the way north is not repeated after the walk on")
}
