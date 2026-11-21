package users

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/pager"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// requireWrappedBetweenWords checks that prompt (a CommandPromptBody) shows
// as more than one line, none wider than width, and that every word of
// unwrapped (the same prompt drawn on one line) is kept whole: the breaks
// fall only between words.
func requireWrappedBetweenWords(t *testing.T, prompt, unwrapped string, width int) {
	t.Helper()
	shown := promptVisible(prompt)
	t.Logf("at %d columns: %q", width, shown)
	lines := strings.Split(shown, "\n")
	require.Greater(t, len(lines), 1, "at %d columns the prompt wraps: %q", width, shown)
	for _, line := range lines {
		require.LessOrEqual(t, util.VisibleWidth(line), width, "line %q of %q", line, shown)
	}
	require.Equal(t, strings.Fields(promptVisible(unwrapped)), strings.Fields(shown),
		"a word was broken: %q", shown)
}

// The shipped default prompt on a 38-column phone (Malia's S21) breaks
// between its bars, never inside one.
func TestCommandPromptBody_WrapsTheDefaultPromptBetweenWords(t *testing.T) {
	yamlPrompt, _ := shippedTextFormats(t)
	u := NewTestUser(7111, "phone", "Phonia", 0)
	u.SetConfigOption(`prompt-compiled`, util.ConvertColorShortTags(yamlPrompt))

	wide := u.CommandPromptBody()
	require.NotContains(t, wide, "\n", "at 80 columns the default prompt fits one line")
	require.Equal(t, u.ProcessPromptString(util.ConvertColorShortTags(yamlPrompt)), wide,
		"a prompt that fits is unchanged byte for byte")

	u.LineWidth = 38
	require.Equal(t, 38, u.WrapWidth())
	requireWrappedBetweenWords(t, u.CommandPromptBody(), wide, 38)
}

// The shipped fight prompt, with a tank, at 38 columns.
func TestCommandPromptBody_WrapsTheFightPromptBetweenWords(t *testing.T) {
	defer SetPromptSightCheck(nil)
	_, fight := shippedTextFormats(t)
	fx := newTankFixture(t)
	fx.player.SetConfigOption(`fprompt-compiled`, util.ConvertColorShortTags(fight))
	require.True(t, fx.player.Character.IsInCombat())

	unwrapped := fx.player.ProcessPromptString(util.ConvertColorShortTags(fight))
	require.Contains(t, promptVisible(unwrapped), "T:Tanker", "the fixture's tank block shows")

	fx.player.LineWidth = 38
	requireWrappedBetweenWords(t, fx.player.CommandPromptBody(), unwrapped, 38)
}

// PromptCols measures the last line of a wrapped prompt: the lines above it
// are counted by the connection from the line breaks it writes.
func TestPromptCols_MeasuresTheLastLine(t *testing.T) {
	require.Equal(t, 6, PromptCols(`<ansi fg="red">HP:100</ansi>`))
	require.Equal(t, 10, PromptCols("Page 1 of 3: Enter,\n<ansi fg=\"red\">q to stop </ansi>"))
	require.Equal(t, 2, PromptCols("first line\r\nab"+term.TelnetGoAhead.String()))
}

// The pager's stop line at 20 columns breaks between words, and keeps its
// trailing space on its last line so the cursor sits after it.
func TestCommandPromptBody_WrapsTheStopLineBetweenWords(t *testing.T) {
	u := pagerUser(t, 10)
	for _, l := range numbered(25) {
		u.HoldReply(l)
	}
	u.EndReply()
	require.True(t, u.PagesWaiting())

	u.LineWidth = 20
	prompt := u.CommandPromptBody()
	requireWrappedBetweenWords(t, prompt, pager.StopLine(1, 3), 20)
	require.True(t, strings.HasSuffix(prompt, "stop "), "the cursor stays after the stop line: %q", prompt)
}
