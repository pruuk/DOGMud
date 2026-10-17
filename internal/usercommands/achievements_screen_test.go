package usercommands

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/achievements"
	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

var achievementsTagRE = regexp.MustCompile(`</?ansi[^>]*>`)

// TestAchievementsHeadingsStandAlone lists the shipped achievements and checks
// each category heading reads as drawn: a blank line above it and no stop
// after it. The 2026-10-17 playtest saw "Combat." straight under the summary
// line: the heading rode CategorySystem, whose end-punctuation stage added
// the stop, and the empty SendText sent before it delivers nothing.
func TestAchievementsHeadingsStandAlone(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	t.Cleanup(achievements.LoadDataFiles) // runs after the config below is restored
	useDogmudTemplates(t)
	achievements.LoadDataFiles()
	require.NotEmpty(t, achievements.All(), "the shipped achievements must load")

	user, room := getTestUserAndRoom(t)
	events.DrainQueuedMessagesForTest(user.UserId)
	handled, err := Achievements("", user, room, 0)
	require.True(t, handled)
	require.NoError(t, err)
	msgs := events.DrainQueuedMessagesForTest(user.UserId)

	// A message may carry several lines (a heading brings its own blank line
	// above it), so split each one into lines.
	var lines []string
	for _, m := range msgs {
		plain := strings.ReplaceAll(achievementsTagRE.ReplaceAllString(m, ""), "\r", "")
		lines = append(lines, strings.Split(strings.TrimRight(plain, "\n"), "\n")...)
	}
	headings := 0
	for i, l := range lines {
		name := strings.TrimSuffix(l, ".")
		for _, cat := range achievementCategories {
			if strings.EqualFold(name, cat) {
				headings++
				require.Equal(t, strings.ToUpper(cat[:1])+cat[1:], l, "a heading must print bare, with no stop")
				require.Greater(t, i, 0)
				require.Equal(t, "", strings.TrimSpace(lines[i-1]), "a heading must have a blank line above it, got %q", lines[i-1])
			}
		}
	}
	require.Greater(t, headings, 0, "no heading found, so this check proves nothing: %q", lines)
}

// TestAchievementsNotTrackedLine checks the screen tells an excluded account
// (admin or AI-flagged) that achievements are not tracked for it, and tells a
// normal account nothing of the kind (#612). The award poll skips the same
// accounts through users.UserRecord.AchievementsExcluded.
func TestAchievementsNotTrackedLine(t *testing.T) {
	cleanup := seedAllRegistries()
	defer cleanup()
	t.Cleanup(achievements.LoadDataFiles)
	useDogmudTemplates(t)
	achievements.LoadDataFiles()
	require.NotEmpty(t, achievements.All())

	const line = "Achievements are not tracked on this account."
	user, room := getTestUserAndRoom(t)
	origRole, origAI := user.Role, user.IsAI
	t.Cleanup(func() { user.Role, user.IsAI = origRole, origAI })

	cases := []struct {
		name string
		role string
		ai   bool
		want bool
	}{
		{"normal user", users.RoleUser, false, false},
		{"admin", users.RoleAdmin, false, true},
		{"ai account", users.RoleUser, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			user.Role, user.IsAI = tc.role, tc.ai
			events.DrainQueuedMessagesForTest(user.UserId)
			handled, err := Achievements("", user, room, 0)
			require.True(t, handled)
			require.NoError(t, err)
			var all []string
			for _, m := range events.DrainQueuedMessagesForTest(user.UserId) {
				all = append(all, achievementsTagRE.ReplaceAllString(m, ""))
			}
			text := strings.Join(all, "\n")
			require.Equal(t, tc.want, strings.Contains(text, line), "screen text: %q", text)
			require.Contains(t, text, "Combat", "the full list must still print")
		})
	}
}
