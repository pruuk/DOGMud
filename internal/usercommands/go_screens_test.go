package usercommands

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
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
	require.NoError(t, guilds.SetRankTitle("WOMG", guilds.RankOfficer, "Keepers of the Gates"))
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

// guildFitsAt fails any line of out wider than width, but for a legacy
// 32-letter name alone on its line at its indent (ruling 8: such a name may
// overrun a 30 screen on its own line only).
func guildFitsAt(t *testing.T, out string, width int) {
	t.Helper()
	for _, line := range plainLines(out) {
		if templates.WidestLine(line) > width && strings.TrimSpace(line) != goScreenLegacyName {
			t.Errorf("at %d a line is %d wide: %q in:\n%s", width, templates.WidestLine(line), line, strings.Join(plainLines(out), "\n"))
		}
	}
}

// The roster as drawn: the 3b mock's guild at 42 and 30 (its heading's tag
// and count drop to a line of their own), and the 3b-3 mock's 40-letter
// guild (at 30 its name splits at a space and keeps the indent, ruling 3).
// The guild MOTD rides CategorySystem, which the pipeline wraps.
func TestGuildRosterIsTheDrawing(t *testing.T) {
	captured := guildCaptured(t)
	wide := guildWide(t)
	for _, tc := range []struct {
		user  *users.UserRecord
		width int
		want  string
	}{
		{captured[0], 42, `Screen Testers Of Thornwall
  [SCRN]: 1 member(s)
"Meet at the bank before the bell. Bring
keys, rope, and a light for the tunnels
under Market Square."
  Screenly             leader`},
		{captured[0], 30, `Screen Testers Of Thornwall
  [SCRN]: 1 member(s)
"Meet at the bank before the
bell. Bring keys, rope, and a
light for the tunnels under
Market Square."
  Screenly             leader`},
		{wide[0], 42, `Wardens of the Old Thornwall Market Gate
  [WOMG]: 3 member(s)
"Muster at the Market Gate at dusk. Bring
a lantern and your own rope."
  ` + goScreenLongestName + ` leader
  Screenly Keepers of the Gates
  ` + goScreenLegacyName + ` member`},
		{wide[0], 30, `Wardens of the Old Thornwall
  Market Gate
  [WOMG]: 3 member(s)
"Muster at the Market Gate at
dusk. Bring a lantern and your
own rope."
  ` + goScreenLongestName + `
    leader
  Screenly
    Keepers of the Gates
  ` + goScreenLegacyName + `
    member`},
	} {
		got := strings.Join(plainLines(guildCommand(t, tc.user, tc.width, "")), "\n")
		if got != tc.want {
			t.Errorf("roster at %d is not the drawing:\n%s", tc.width, lineDiffForTest(tc.want, got))
		}
	}
}

// At 40 and 41 columns the roster takes the 42 form where it fits, as
// templates.Process picks a template's forms by measuring: the 40-letter
// name whole on its line (review of plan 3b-3: a band gave 30 there and
// split it).
func TestGuildRosterMeasuresItsForm(t *testing.T) {
	wide := guildWide(t)
	for _, width := range []int{40, 41} {
		got := strings.Join(plainLines(guildCommand(t, wide[0], width, "")), "\n")
		require.True(t, strings.HasPrefix(got, goScreenLongestGuild+"\n  [WOMG]: 3 member(s)\n"), "at %d:\n%s", width, got)
		guildFitsAt(t, got, width)
	}
}

// The roster fits at every width with the widest names, and at 60 (wide
// enough for every 80-column line) it is the 80-column screen.
func TestGuildRosterFitsEveryWidth(t *testing.T) {
	wide := guildWide(t)
	for _, width := range goScreenWidths {
		out := guildCommand(t, wide[0], width, "")
		require.Contains(t, out, "Keepers of the Gates", "at %d", width)
		guildFitsAt(t, out, width)
	}
	at60 := strings.Join(plainLines(guildCommand(t, wide[0], 60, "")), "\n")
	require.Contains(t, at60, goScreenLongestGuild+" [WOMG]: 3 member(s)\n", "at 60 the heading fits on one line")
	require.Contains(t, at60, "\n  Screenly             Keepers of the Gates\n", "at 60 the member line fits as it is")
}

// `guild list` as drawn (guild_list in the 3b-3 mock): at 80 the drawn
// redesign, the leader under the line when it does not fit; at 42 and 30
// the 40-letter name, wider than the room at its indent, splits at a space
// and keeps the indent (ruling 3, where the drawing pulled it left); the
// legacy 32-letter leader runs past 30 on its own line (ruling 8).
func TestGuildListIsTheDrawing(t *testing.T) {
	scrn := guildFixture(t, 97131, "SCRN", "Screen Testers", "", "Screenly")
	guildFixture(t, 97141, "WOMG", goScreenLongestGuild, "", goScreenLegacyName)
	for _, tc := range []struct {
		width int
		want  string
	}{
		{80, `Guilds:
  [SCRN] Screen Testers: 1 member(s), led by Screenly
  [WOMG] Wardens of the Old Thornwall Market Gate: 1 member(s), led by
    ` + goScreenLegacyName},
		{42, `Guilds:
  [SCRN] Screen Testers: 1 member(s),
    led by Screenly
  [WOMG]
    Wardens of the Old Thornwall Market
    Gate: 1 member(s), led by
    ` + goScreenLegacyName},
		{30, `Guilds:
  [SCRN] Screen Testers:
    1 member(s), led by
    Screenly
  [WOMG]
    Wardens of the Old
    Thornwall Market Gate:
    1 member(s), led by
    ` + goScreenLegacyName},
	} {
		got := strings.Join(plainLines(guildCommand(t, scrn[0], tc.width, "list")), "\n")
		if got != tc.want {
			t.Errorf("guild list at %d is not the drawing:\n%s", tc.width, lineDiffForTest(tc.want, got))
		}
	}
}

// `guild list` fits at every width with a 40-letter guild led by a name at
// the 28-letter maximum, and keeps its units whole where they fit.
func TestGuildListFitsEveryWidth(t *testing.T) {
	us := guildFixture(t, 97151, "WOMG", goScreenLongestGuild, "", goScreenLongestName)
	guildFixture(t, 97161, "SCRN", "Screen Testers Of Thornwall", "", "Screenly")
	for _, width := range goScreenWidths {
		out := guildCommand(t, us[0], width, "list")
		guildFitsAt(t, out, width)
		plain := strings.Join(plainLines(out), "\n")
		for _, unit := range []string{goScreenLongestName, "1 member(s),", "led by", "[WOMG]"} {
			require.Contains(t, plain, unit, "at %d", width)
		}
		// A name splits only when wider than the room at its indent, 4.
		if width >= 42 {
			require.Contains(t, plain, "Screen Testers Of Thornwall:", "at %d", width)
		}
		if width >= 80 {
			require.Contains(t, plain, goScreenLongestGuild, "at %d the name splits though it fits the line at its indent", width)
		}
		// The 28-letter leader fits a 30 line but not its indent of 4, and
		// has no space to split at, so it is pulled left to end at the edge.
		if width == 30 {
			require.Contains(t, plain, "\n  "+goScreenLongestName, "at %d", width)
		}
	}
}

// A guild name may be 40 letters with no space (validGuildName). Wider
// than a whole 30 line, it breaks at the edge in `guild list` and in the
// roster heading (as wrapbreak breaks signs and runes), where a character
// name would run past (ruling 8).
func TestASpacelessGuildNameBreaksAtTheEdge(t *testing.T) {
	const spaceless = "Wardens-of-the-Old-Thornwall-Market-Gate"
	us := guildFixture(t, 97171, "WOMG", spaceless, "", "Screenly")
	for _, rest := range []string{"list", ""} {
		for _, width := range goScreenWidths {
			out := guildCommand(t, us[0], width, rest)
			guildFitsAt(t, out, width)
			flat := strings.Join(strings.Fields(strings.Join(plainLines(out), "")), "")
			require.Contains(t, flat, spaceless, "`guild %s` at %d lost part of the name", rest, width)
		}
	}
}

// --- name limit ---

// The shipped name limit is the longest name these screens are tested
// with: NameSizeMax went from 32 to 28 in plan 3b-3 (ruling 8, owner
// 2026-11-11) so a new name fits a 30-column line. Read from
// _datafiles/config.yaml, since a test binary loads the Go default.
func TestShippedNameSizeMaxIsTheTestedLongestName(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "_datafiles", "config.yaml"))
	require.NoError(t, err)
	var cfg struct {
		Validation struct {
			NameSizeMax int `yaml:"NameSizeMax"`
		} `yaml:"Validation"`
	}
	require.NoError(t, yaml.Unmarshal(raw, &cfg))
	require.Equal(t, 28, cfg.Validation.NameSizeMax, "config.yaml's Validation.NameSizeMax")
	require.Len(t, goScreenLongestName, cfg.Validation.NameSizeMax)
}

// A character still carrying its placeholder name takes its username in
// `start`, checked with every name check but its own username (an online
// user is in the username list). A username longer than today's 28 (legal
// before plan 3b-3) is kept as the name when the length is its only fault;
// a username that fails any other check (a banned name) is not set, and the
// placeholder stays, as on master.
func TestStartKeepsALegacyLongUsernameAsTheName(t *testing.T) {
	cleanup := seedAllRegistries()
	t.Cleanup(cleanup)
	orig := int(configs.GetValidationConfig().NameSizeMax)
	_ = configs.AddOverlayOverrides(map[string]any{"Validation.NameSizeMax": 28})
	t.Cleanup(func() { _ = configs.AddOverlayOverrides(map[string]any{"Validation.NameSizeMax": orig}) })
	origBanned := []string(configs.GetValidationConfig().BannedNames)
	_ = configs.AddOverlayOverrides(map[string]any{"Validation.BannedNames": []string{"badword*"}})
	t.Cleanup(func() { _ = configs.AddOverlayOverrides(map[string]any{"Validation.BannedNames": origBanned}) })

	for i, tc := range []struct {
		username string
		kept     bool
	}{
		{goScreenLegacyName, true},               // too long only: kept
		{"screenly", true},                       // its own username (online, listed lower case) is no collision
		{"Badword_Ashgrove_Thornbramble", false}, // too long and banned
		{"Badwordly", false},                     // banned
	} {
		user := users.NewTestUser(97201+i, tc.username, "", uint64(99201+i))
		restore := users.SeedUsersForTest(map[int]*users.UserRecord{user.UserId: user})
		temp := user.TempName()
		user.Character.Name = temp
		user.Character.RoomId = -1
		_, _ = Start("", user, nil, 0)
		if tc.kept {
			require.Equal(t, tc.username, user.Character.Name, "%s was not kept", tc.username)
		} else {
			require.Equal(t, temp, user.Character.Name, "%s was set though it fails a name check", tc.username)
		}
		restore()
	}
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

// The shipped MOTD as drawn (login_motd_and_who in the 3b mock): rules only,
// no corners or side bars (ruling 1), every rule the form's width; the
// spaced title at 42 and the plain one at 30; the text at indent 2 and 1,
// and a link wider than the room at its indent starting at column 0. At 42
// "The novel:" keeps its short link on its line, which fits: the drawing
// broke it there only because it was made from the old, longer link.
func TestMotdIsTheDrawing(t *testing.T) {
	text := shippedMotd(t)
	r42, r30 := strings.Repeat("═", 42), strings.Repeat("═", 30)
	for _, tc := range []struct {
		width int
		want  string
	}{
		{42, r42 + `
  .:  M E S S A G E   O F   T H E   D A Y
` + r42 + `
  TALK TO PEOPLE. Type talk <name> for a
  numbered menu of what someone knows and
  what they need. A [!] before a name
  means they have work for you.
  EVERY STEP HAS A HINT. Your quest log
  says what to do next, and the map points
  the way.
  LOOK AND SEARCH. Type look <thing> to
  study anything you see, and type search
  to find what is hidden. Many secrets
  give way to one or the other, so bring a
  light.
  LISTEN TO THE GOSSIPS. Some of what they
  repeat is worth chasing.
  The novel: https://dogmud.org/novel
  Full patch notes:
  https://dogmud.org/patch-notes
` + r42},
		{30, r30 + `
  .:  MESSAGE OF THE DAY
` + r30 + `
 TALK TO PEOPLE. Type talk
 <name> for a numbered menu of
 what someone knows and what
 they need. A [!] before a
 name means they have work for
 you.
 EVERY STEP HAS A HINT. Your
 quest log says what to do
 next, and the map points the
 way.
 LOOK AND SEARCH. Type look
 <thing> to study anything you
 see, and type search to find
 what is hidden. Many secrets
 give way to one or the other,
 so bring a light.
 LISTEN TO THE GOSSIPS. Some
 of what they repeat is worth
 chasing.
 The novel:
 https://dogmud.org/novel
 Full patch notes:
https://dogmud.org/patch-notes
` + r30},
	} {
		got := strings.Join(plainLines(motdBox(text, tc.width)), "\n")
		if got != tc.want {
			t.Errorf("MOTD at %d is not the drawing:\n%s", tc.width, lineDiffForTest(tc.want, got))
		}
	}
}

// motdCorners are the box glyphs ruling 1 removes from every narrow form.
const motdCorners = "╔╗╚╝╠╣║"

// Below 80 columns the MOTD is the ruled form, never the box: the 42 form
// from 42 to 79 (as a template's 42 form is picked there) and the 30 form
// below, every rule exactly the form's width and no line wider. At 80 and
// wider it is the box at the screen's width, as master drew it.
func TestMotdFitsEveryWidth(t *testing.T) {
	text := shippedMotd(t)
	for _, width := range []int{30, 35, 42, 44, 45, 60, 79, 80, 100} {
		out := motdBox(text, width)
		lines := plainLines(out)
		if width >= 80 {
			require.True(t, strings.HasPrefix(lines[0], " ╔"), "at %d the box is gone:\n%s", width, out)
			continue
		}
		formWidth := 42
		if width < 42 {
			formWidth = 30
		}
		require.False(t, strings.ContainsAny(out, motdCorners), "at %d a narrow form has box glyphs:\n%s", width, out)
		rules := 0
		for _, line := range lines {
			require.LessOrEqual(t, templates.WidestLine(line), formWidth, "at %d: %q", width, line)
			if strings.HasPrefix(line, "═") {
				rules++
				require.Equal(t, formWidth, templates.WidestLine(line), "at %d a rule is not the form's width", width)
			}
		}
		require.Equal(t, 3, rules, "at %d:\n%s", width, out)
	}
}
