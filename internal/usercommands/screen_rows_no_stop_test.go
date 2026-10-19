package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

var (
	screenRowTagRE = regexp.MustCompile(`</?ansi[^>]*>`)
	screenRowEscRE = regexp.MustCompile("\x1b\\[[0-9;]*[A-Za-z]")
	// A stop the normalizer added after a label's colon or a percentage.
	screenRowStopAfterLabelRE = regexp.MustCompile(`[:%]\.$`)
)

// TestScreenRowsEndWithoutAStop runs a representative set of the laid-out
// screens #614 moved onto CategoryScreen and checks their rows print as
// drawn: no stop after a colon, a value word, a percentage or a command name,
// and player-typed text neither recased nor stopped. On CategorySystem the
// normalizer stopped the last row of a send ("See also: help online.",
// "trade: on.", "Your macros:.") and capitalized its first letter.
func TestScreenRowsEndWithoutAStop(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	useDogmudTemplates(t)
	defer mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"fixture-gills": {MutationId: "fixture-gills", Name: "Fixture Gills", Description: "Slits open along the neck."},
	})()

	user, room := getTestUserAndRoom(t)
	origMacros, origAliases := user.Macros, user.Aliases
	origMutations, origDesc := user.Character.Mutations, user.Character.Description
	t.Cleanup(func() {
		user.Macros, user.Aliases = origMacros, origAliases
		user.Character.Mutations, user.Character.Description = origMutations, origDesc
	})
	user.Macros = map[string]string{"=1": "look;say hi"}
	user.Aliases = map[string]string{}
	user.Character.Mutations = map[string]int{"fixture-gills": 1}
	// Player-typed: lower case and no stop, both of which must survive.
	user.Character.Description = "a lanky figure in a patched coat"

	type command func(string, *users.UserRecord, *rooms.Room, events.EventFlag) (bool, error)
	cases := []struct {
		name string
		cmd  command
		rest string
		// Rows that must appear exactly, after tags are stripped and
		// trailing spaces trimmed.
		want []string
		// The last non-blank line, exactly; empty skips the check.
		last string
	}{
		{name: "help afk ends in See also", cmd: Help, rest: "afk", last: "See also: help online"},
		{name: "status ends in a mutation row", cmd: Status, last: "   fixture-gills             minor"},
		{name: "channels", cmd: Channels,
			want: []string{"Channels (channels <name> to toggle):", "  chat: on"}, last: "  trade: on"},
		{name: "macros", cmd: Macros, want: []string{"Your macros:", "  =1:", "      a) look", "      b) say hi"}},
		{name: "killstats", cmd: Killstats, last: "Also try: killstats pvp, killstats area"},
		{name: "mutations", cmd: Mutations, want: []string{" .:. Your Mutations .:.", "  Fixture Gills (minor)"}},
		{name: "alias add", cmd: Alias, rest: "zz=north", last: "Custom Alias Added: zz=north"},
		{name: "setdesc shows the player's own text", cmd: SetDesc,
			want: []string{"Your current description:", "a lanky figure in a patched coat"},
			last: "To clear: setdesc clear"},
	}

	for _, c := range cases {
		events.DrainQueuedMessagesForTest(user.UserId)
		handled, err := c.cmd(c.rest, user, room, 0)
		require.True(t, handled, c.name)
		require.NoError(t, err, c.name)
		msgs := events.DrainQueuedMessagesForTest(user.UserId)
		require.NotEmpty(t, msgs, "%s sent nothing, so this check proves nothing", c.name)

		var lines []string
		for _, m := range msgs {
			plain := screenRowEscRE.ReplaceAllString(screenRowTagRE.ReplaceAllString(m, ""), "")
			plain = strings.NewReplacer("\r", "", "&lt;", "<", "&gt;", ">").Replace(plain)
			for _, l := range strings.Split(plain, "\n") {
				lines = append(lines, strings.TrimRight(l, " "))
			}
		}

		last := ""
		for _, l := range lines {
			if strings.TrimSpace(l) != "" {
				last = l
			}
			// ".:Mutations:." and " .:. Your Mutations .:." are drawn
			// heading decorations, not a stop after a label.
			if screenRowStopAfterLabelRE.MatchString(l) && !strings.Contains(l, ".:") {
				t.Errorf("%s: a stop was added after a label or a percentage: %q", c.name, l)
			}
		}
		if c.last != "" && last != c.last {
			t.Errorf("%s: last line = %q, want %q", c.name, last, c.last)
		}
		for _, w := range c.want {
			found := false
			for _, l := range lines {
				if l == w {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("%s: row %q missing (a stop or a recased letter?); got %q", c.name, w, lines)
			}
		}
		// Rows whose last word is a value or a command name: on/off,
		// "N member(s)" style counts are covered above; here the channel
		// state and the killstats suggestion list.
		for _, l := range lines {
			if strings.HasSuffix(l, ": on.") || strings.HasSuffix(l, ": off.") {
				t.Errorf("%s: a stop was added after a channel state: %q", c.name, l)
			}
			if strings.HasPrefix(l, "Also try: ") && strings.HasSuffix(l, ".") {
				t.Errorf("%s: a stop was added after a command name: %q", c.name, l)
			}
		}
	}
}
