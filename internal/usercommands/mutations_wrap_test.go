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
		for _, line := range strings.Split(strings.ReplaceAll(plain, "\r\n", "\n"), "\n") {
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
