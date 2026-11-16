package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/util"
	"github.com/stretchr/testify/require"
)

var mutationsWrapAnsiTag = regexp.MustCompile(`</?ansi[^>]*>`)

// The 2026-10-17 pre-deploy playtest measured the Tremorsense description at
// 220 characters on one line: `mutations` sends CategorySystem, which the
// pipeline never wraps. The command now wraps its prose to the reader's
// width, keeping each line's indent on the continuation lines.
func TestMutations_DescriptionWrapsToTheReadersWidth(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	const description = "Nerve clusters bloom in your feet and palms, reading vibrations " +
		"through stone and soil. You feel footsteps, heartbeats, the shift of weight " +
		"before a strike. But the constant noise overwhelms your fine motor control."
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"testwrap": {MutationId: "testwrap", Name: "Test Wrap", Description: description},
	}))

	user, room := getTestUserAndRoom(t)
	prev := user.Character.Mutations
	user.Character.Mutations = map[string]int{"testwrap": 1}
	t.Cleanup(func() { user.Character.Mutations = prev })
	events.DrainQueuedMessagesForTest(user.UserId)

	_, err := Mutations("", user, room, 0)
	require.NoError(t, err)

	var prose []string
	for _, msg := range events.DrainQueuedMessagesForTest(user.UserId) {
		plain := mutationsWrapAnsiTag.ReplaceAllString(msg, "")
		for line := range strings.SplitSeq(strings.ReplaceAll(plain, "\r\n", "\n"), "\n") {
			require.LessOrEqual(t, util.VisibleWidth(line), user.GetLineWidth(),
				"mutations line wider than the reader's width: %q", line)
			if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") {
				prose = append(prose, strings.TrimSpace(line))
			}
		}
	}
	require.Greater(t, len(prose), 1, "the description must wrap onto more than one line")
	require.Equal(t, description, strings.Join(prose, " "),
		"wrapping must keep every word of the description, each line indented")
}

// #687: shipped descriptions are YAML block scalars, hard-wrapped and ending
// in a newline. `mutations` printed a line holding only "." under some of
// them (Dense Muscles, Titan Growth). Every line must carry words.
func TestMutations_BlockScalarDescriptionHasNoStrayLines(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()

	// Dense Muscles' description as the YAML loader hands it over.
	const description = "Your muscles condense into cables of extraordinary power. Raw strength\n" +
		"surges and you can shoulder loads that would buckle a normal frame -- but\n" +
		"the dense tissue burns through your endurance faster.\n"
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"testblock": {MutationId: "testblock", Name: "Test Block", Description: description},
	}))

	user, room := getTestUserAndRoom(t)
	prev := user.Character.Mutations
	user.Character.Mutations = map[string]int{"testblock": 1}
	t.Cleanup(func() { user.Character.Mutations = prev })
	events.DrainQueuedMessagesForTest(user.UserId)

	_, err := Mutations("", user, room, 0)
	require.NoError(t, err)

	var words []string
	for _, msg := range events.DrainQueuedMessagesForTest(user.UserId) {
		plain := mutationsWrapAnsiTag.ReplaceAllString(msg, "")
		for line := range strings.SplitSeq(strings.ReplaceAll(plain, "\r\n", "\n"), "\n") {
			trimmed := strings.TrimSpace(line)
			require.NotEqual(t, ".", trimmed, "a line holding only a full stop")
			if strings.HasPrefix(line, "    ") && !strings.HasPrefix(line, "      ") {
				words = append(words, strings.Fields(trimmed)...)
			}
		}
	}
	require.Equal(t, strings.Fields(description), words,
		"the description reads whole, once, with no stray lines")
}
