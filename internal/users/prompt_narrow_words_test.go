package users

import (
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/connections"
	"github.com/GoMudEngine/GoMud/internal/term"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

// Owner ruling, 2026-11-26 (Malia's Galaxy S21 check): a prompt whose
// health, stamina and conviction bars would make it wider than the player's
// screen is drawn with the words a screen reader account hears instead
// (vitalWord), below 80 columns only. An 80 column (desktop) screen keeps
// the bars even when the prompt wraps. A go-ahead client and an AI port
// connection keep the prompt whole and unchanged, bars and all. A screen
// reader account hears words at any width.

// barGlyphs are the three glyphs renderVitalBar draws.
var barGlyphs = []string{`█`, `░`, `▒`}

func requireBars(t *testing.T, prompt string) {
	t.Helper()
	require.Contains(t, promptVisible(prompt), `█`, "the bars show: %q", promptVisible(prompt))
}

func requireWords(t *testing.T, prompt string, words ...string) {
	t.Helper()
	shown := promptVisible(prompt)
	for _, g := range barGlyphs {
		require.NotContains(t, shown, g, "no bar glyph in %q", shown)
	}
	for _, w := range words {
		require.Contains(t, shown, w, "the word form shows %q: %q", w, shown)
	}
}

// wordForm is tmpl rendered as a screen reader account hears it: the word
// form a narrow prompt must show.
func wordForm(u *UserRecord, tmpl string) string {
	was := u.ScreenReader
	u.ScreenReader = true
	defer func() { u.ScreenReader = was }()
	return u.ProcessPromptString(tmpl)
}

// The shipped default prompt keeps its bars at 80 columns and shows words
// at the narrow form widths (42, 30) and at Malia's S21 (38).
func TestCommandPromptBody_DefaultPromptWordsWhenTheBarsDoNotFit(t *testing.T) {
	yamlPrompt, _ := shippedTextFormats(t)
	tmpl := util.ConvertColorShortTags(yamlPrompt)
	u := NewTestUser(7121, "narrow", "Narrowa", 0)
	u.SetConfigOption(`prompt-compiled`, tmpl)

	require.Equal(t, 80, u.WrapWidth())
	at80 := u.CommandPromptBody()
	requireBars(t, at80)
	require.Equal(t, u.ProcessPromptString(tmpl), at80, "at 80 columns the prompt is unchanged")

	for _, width := range []int{NarrowFormWidth(79), 38, NarrowFormWidth(41)} {
		u.LineWidth = width
		require.Equal(t, width, u.WrapWidth())
		got := u.CommandPromptBody()
		t.Logf("at %d: %q", width, promptVisible(got))
		requireWords(t, got, `unhurt`, `fresh`, `strong`)
		require.Equal(t, strings.Fields(promptVisible(wordForm(u, tmpl))), strings.Fields(promptVisible(got)),
			"at %d the prompt is the word form", width)
	}
}

// The shipped fight prompt: bars at 80 when it fits, words at 38, the
// tank's bar included. TestCommandPromptBody_WrapsTheFightPromptBetweenWords
// checks the same prompt breaks between words.
func TestCommandPromptBody_FightPromptWordsWhenTheBarsDoNotFit(t *testing.T) {
	defer SetPromptSightCheck(nil)
	_, fight := shippedTextFormats(t)
	tmpl := util.ConvertColorShortTags(fight)
	fx := newTankFixture(t)
	fx.player.SetConfigOption(`fprompt-compiled`, tmpl)
	require.True(t, fx.player.Character.IsInCombat())

	// With the wolf on the player there is no tank block, and the fight
	// prompt fits 80 columns with its bars.
	fx.mob.Character.SetAggro(fx.player.UserId, 0, characters.DefaultAttack)
	requireBars(t, fx.player.CommandPromptBody())
	require.NotContains(t, fx.player.CommandPromptBody(), "\n")

	// With someone else tanking, the tank block makes the bar form wider
	// than 80. An 80 column screen is a desktop: it keeps the bars and the
	// prompt wraps (owner, 2026-11-26). One column narrower gets the words.
	fx.mob.Character.SetAggro(fx.tank.UserId, 0, characters.DefaultAttack)
	require.Greater(t, util.WidestLine(fx.player.ProcessPromptString(tmpl)), 80)
	require.Equal(t, 80, fx.player.WrapWidth())
	at80 := fx.player.CommandPromptBody()
	requireBars(t, at80)
	require.Contains(t, at80, "\n", "the bar form wraps at 80")
	require.Contains(t, promptVisible(at80), `Thp:`, "the tank block shows with its bar")

	// At 79 the word form itself is a column too wide, so it wraps too.
	fx.player.LineWidth = 79
	requireWords(t, fx.player.CommandPromptBody(), `unhurt`, `Thp:badly`)

	fx.player.LineWidth = 38
	got := fx.player.CommandPromptBody()
	t.Logf("at 38: %q", promptVisible(got))
	requireWords(t, got, `unhurt`, `fresh`, `strong`, `Thp:badly hurt`)
}

// A screen reader account hears words at any width, 80 included.
func TestCommandPromptBody_ScreenReaderWordsAtAnyWidth(t *testing.T) {
	yamlPrompt, _ := shippedTextFormats(t)
	u := NewTestUser(7122, "reader", "Readia", 0)
	u.SetConfigOption(`prompt-compiled`, util.ConvertColorShortTags(yamlPrompt))
	u.ScreenReader = true
	for _, width := range []int{80, 42, 30} {
		u.LineWidth = width
		requireWords(t, u.CommandPromptBody(), `unhurt`, `fresh`, `strong`)
	}
}

// A go-ahead client and an AI port connection get the bar form whole at a
// narrow width, as before: a MUD client's gauges and an agent read the bars.
func TestCommandPromptBody_GoAheadAndAIPortKeepTheBarsWhole(t *testing.T) {
	yamlPrompt, _ := shippedTextFormats(t)
	tmpl := util.ConvertColorShortTags(yamlPrompt)

	const goAheadConn = 18851
	connections.RegisterTestConnection(goAheadConn)
	t.Cleanup(func() { connections.UnregisterTestConnection(goAheadConn) })
	connections.UpdateClientSettings(goAheadConn, func(cs *connections.ClientSettings) { cs.SendTelnetGoAhead = true })
	ga := NewTestUser(7123, "mudlet2", "Mudlettb", goAheadConn)
	ga.SetConfigOption(`prompt-compiled`, tmpl)
	ga.LineWidth = 30
	body := ga.CommandPromptBody()
	require.Equal(t, ga.ProcessPromptString(tmpl)+term.TelnetGoAhead.String(), body,
		"a go-ahead client gets the bar form whole")
	requireBars(t, body)

	const aiConn = 18852
	connections.RegisterTestConnection(aiConn)
	t.Cleanup(func() { connections.UnregisterTestConnection(aiConn) })
	connections.Get(aiConn).SetConnType(connections.ConnAI)
	ai := NewTestUser(7124, "agent3", "Agentib", aiConn)
	ai.SetConfigOption(`prompt-compiled`, tmpl)
	ai.LineWidth = 30
	require.Equal(t, ai.ProcessPromptString(tmpl), ai.CommandPromptBody(),
		"an AI port connection gets the bar form whole")

	// The same human connection without the go-ahead gets words, so the
	// test can fail.
	connections.UpdateClientSettings(goAheadConn, func(cs *connections.ClientSettings) { cs.SendTelnetGoAhead = false })
	requireWords(t, ga.CommandPromptBody(), `unhurt`)
}

// When even the word form does not fit (30 columns, the longest words), it
// still wraps between words.
func TestCommandPromptBody_WordFormWrapsBetweenWords(t *testing.T) {
	yamlPrompt, _ := shippedTextFormats(t)
	tmpl := util.ConvertColorShortTags(yamlPrompt)
	u := NewTestUser(7125, "worst", "Worsta", 0)
	u.SetConfigOption(`prompt-compiled`, tmpl)
	u.Character.Health = 5
	u.Character.Stamina = 0
	u.Character.Conviction = 0
	u.LineWidth = 30
	got := u.CommandPromptBody()
	requireWords(t, got, `near death`, `exhausted`, `broken`)
	requireWrappedBetweenWords(t, got, wordForm(u, tmpl), 30)
}
