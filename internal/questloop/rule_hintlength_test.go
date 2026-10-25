package questloop

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestHintLength_OverTheCapIsAFinding(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Steps[1].Hint = strings.Repeat("a", 201)
	fs := Validate(w)
	require.Contains(t, rules(fs), "hint-length step firstkill")
	for _, f := range fs {
		if f.Rule == "hint-length" {
			require.Contains(t, f.Msg, "201")
			require.Contains(t, f.Msg, "200")
		}
	}
}

func TestHintLength_ExactlyTheCapPasses(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Steps[1].Hint = strings.Repeat("a", 200)
	require.Empty(t, Validate(w))
}

func TestHintLength_CountsRunesNotBytes(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Steps[1].Hint = strings.Repeat("é", 200) // 400 bytes, 200 runes
	require.Empty(t, Validate(w))
}

func TestHintLength_WhitespaceRunsCollapseBeforeCounting(t *testing.T) {
	w := goodWorld()
	// 100 words of "ab" joined by long whitespace runs: 299 runes once
	// collapsed to single spaces, far more raw.
	words := make([]string, 100)
	for i := range words {
		words[i] = "ab"
	}
	collapsedOver := strings.Join(words, " \n\t  ")
	w.Quests[0].Steps[1].Hint = collapsedOver
	require.Contains(t, rules(Validate(w)), "hint-length step firstkill")

	// 50 words: 149 runes collapsed, 299 raw, plus padding at both ends.
	w.Quests[0].Steps[1].Hint = "   \n" + strings.Join(words[:50], " \n\t  ") + "\n   "
	require.Empty(t, Validate(w), "raw length is over the cap but the collapsed hint is not")
}

func TestHintLength_NoHintIsLeftToTheHintRule(t *testing.T) {
	w := goodWorld()
	w.Quests[0].Steps[1].Hint = ""
	require.Equal(t, []string{"hint step firstkill"}, rules(Validate(w)))
}
