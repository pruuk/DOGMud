package usercommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/guilds"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// The Go-built screens with narrow forms (plan 3b-3): the craft list and the
// bare `craft` view, the guild roster and `guild list`, and the MOTD. Each is
// run through its real command at 30, 42, a width in between and 80, with
// data as wide as it gets in play (the longest shipped recipe and ingredient
// names read from the YAML, a 40-letter guild name, a 28-letter character
// name), against the drawings in _archive/notes/reports/narrow-screens-3b-mock
// and narrow-screens-3b3-mock. Where 80 columns must not change, a golden
// recorded from master pins it.

// goScreenWidths are the widths each screen is run at: the narrowest
// screen, an upright phone, a width in between, and a desktop.
var goScreenWidths = []int{30, 42, 60, 80}

// goScreenLongestName is a character name at the configured maximum
// (NameSizeMax, 28 from plan 3b-3).
const goScreenLongestName = "Bartholomewsworthingtonianus"

// goScreenLegacyName is a 32-letter name, legal before plan 3b-3 lowered
// NameSizeMax to 28 and kept by the characters who have it.
const goScreenLegacyName = "Wilhelmina_Ashgrove_Thornbramble"

// goScreenLongestGuild is a guild name at the 40-letter maximum
// (guilds.validGuildName), as the 3b-3 mock drew it.
const goScreenLongestGuild = "Wardens of the Old Thornwall Market Gate"

// plainLines is out (as queued: colour tags, CRLF or LF) as lines with the
// colour tags removed.
func plainLines(out string) []string {
	plain := motdAnsiTagRE.ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), "")
	return strings.Split(strings.TrimSuffix(plain, "\n"), "\n")
}

// --- guild ---

// guildFixture founds a guild led by a fresh test user and returns its
// members, leader first; the guild and the users go when the test ends.
func guildFixture(t *testing.T, userId int, tag, name, motd string, members ...string) []*users.UserRecord {
	t.Helper()
	t.Cleanup(guilds.SetDataDirForTest(t.TempDir()))
	recs := map[int]*users.UserRecord{}
	out := []*users.UserRecord{}
	for i, member := range members {
		u := users.NewTestUser(userId+i, strings.ToLower(member), member, uint64(90000+userId+i))
		recs[u.UserId] = u
		out = append(out, u)
	}
	t.Cleanup(users.SeedUsersForTest(recs))
	_, err := guilds.Create(tag, name, out[0].UserId, members[0])
	require.NoError(t, err)
	t.Cleanup(func() { guilds.Delete(tag) })
	for _, u := range out[1:] {
		require.NoError(t, guilds.AddMember(tag, u.UserId, u.Character.Name))
	}
	if motd != "" {
		require.NoError(t, guilds.SetMotd(tag, motd))
	}
	return out
}

// guildCommand runs `guild <rest>` for user at width and returns what was
// queued.
func guildCommand(t *testing.T, user *users.UserRecord, width int, rest string) string {
	t.Helper()
	user.LineWidth = width
	events.DrainQueuedMessagesForTest(user.UserId)
	_, err := Guild(rest, user, nil, 0)
	require.NoError(t, err)
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "")
}

const guildCapturedMotd = "Meet at the bank before the bell. Bring keys, rope, and a light for the tunnels under Market Square."

// guildCaptured is the roster the 3b mock captured; guildWide the 3b-3
// mock's, with a 40-letter name, a leader at the 28-letter maximum, a
// 32-letter legacy member and an officer with a 20-letter rank title.
func guildCaptured(t *testing.T) []*users.UserRecord {
	return guildFixture(t, 97101, "SCRN", "Screen Testers Of Thornwall", guildCapturedMotd, "Screenly")
}

func guildWide(t *testing.T) []*users.UserRecord {
	us := guildFixture(t, 97111, "WOMG", goScreenLongestGuild,
		"Muster at the Market Gate at dusk. Bring a lantern and your own rope.",
		goScreenLongestName, goScreenLegacyName, "Screenly")
	require.NoError(t, guilds.SetRank("WOMG", us[2].UserId, guilds.RankOfficer))
	require.NoError(t, guilds.SetRankTitle("WOMG", guilds.RankOfficer, "Keeper of the Gates"))
	return us
}

func TestGuildRosterKeepsIts80ColumnOutput(t *testing.T) {
	captured := guildCaptured(t)
	wide := guildWide(t)
	checkGolden(t, "guild_roster", guildCommand(t, captured[0], 80, "")+guildCommand(t, wide[0], 80, ""))
}

// The guild list at 80, with guilds whose lines fit, is what master drew.
func TestGuildListKeepsIts80ColumnOutput(t *testing.T) {
	captured := guildCaptured(t)
	guildFixture(t, 97121, "QC", "Quiet Company", "", goScreenLongestName)
	checkGolden(t, "guild_list", guildCommand(t, captured[0], 80, "list"))
}

// --- motd ---

// shippedMotd is the MOTD the game ships, read from _datafiles/config.yaml
// (a test binary loads the Go default, not the shipped config) and rendered
// as the motd command renders it.
func shippedMotd(t *testing.T) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	require.NoError(t, err)
	var cfg struct {
		Server struct {
			Motd string `yaml:"Motd"`
		} `yaml:"Server"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &cfg))
	require.Contains(t, cfg.Server.Motd, "https://dogmud.org/patch-notes", "config.yaml's MOTD did not load")
	text, err := templates.ProcessText(cfg.Server.Motd, nil)
	require.NoError(t, err)
	require.Contains(t, text, "TALK TO PEOPLE.")
	return text
}

// At 80 the MOTD box is what master drew (a wider telnet screen keeps the
// box too: TestMotdBoxIsExactlyTheLineWidth).
func TestMotdKeepsIts80ColumnOutput(t *testing.T) {
	checkGolden(t, "motd", motdBox(shippedMotd(t), 80))
}
