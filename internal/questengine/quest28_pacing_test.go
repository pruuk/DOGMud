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

// The ask step's reply and its next-step line are one line, from Dewey's
// dialogue: the reply first, the way north last.
func TestQuest28_AskReplyComesBeforeTheNextPrompt(t *testing.T) {
	q := loadQuest28(t)
	for _, tr := range q.Triggers {
		if tr.Event != "dialogue" {
			continue
		}
		for _, a := range tr.Actions {
			require.Nil(t, a.NpcSay, "a dialogue trigger's npc_say line is said before the reply to the question")
		}
	}

	var df dialogue.DialogueFile
	require.NoError(t, yamlv2.Unmarshal(repoFile(t, "_datafiles", "world", "dogmud", "dialogue", "newcomer_antechamber", "9491.yaml"), &df),
		"9491.yaml must parse or Dewey goes mute")

	holding := func(tokens ...string) *dialogue.PlayerState {
		set := map[string]bool{}
		for _, tok := range tokens {
			set[tok] = true
		}
		return &dialogue.PlayerState{HasQuest: func(tok string) bool { return set[tok] }}
	}
	const prompt = "The way north is open."

	// The trigger grants 28-asked before the reply is chosen.
	for _, topic := range []string{"world", "gaius", "lanterns"} {
		reply, _, ok, _ := dialogue.MatchWithFallbackInfo(&df, 1, topic, holding("28-said", "28-asked"))
		require.True(t, ok)
		require.True(t, strings.HasSuffix(strings.TrimSpace(reply), prompt),
			"ask dewey %s on the ask step must end with the way north; got %q", topic, reply)
	}

	// Once the player has walked on, the way north is not repeated.
	reply, _, ok, _ := dialogue.MatchWithFallbackInfo(&df, 1, "world", holding("28-said", "28-asked", "28-proving_prompt"))
	require.True(t, ok)
	require.NotContains(t, reply, prompt)
}
