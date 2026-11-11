package usercommands

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/GoMudEngine/GoMud/internal/events"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/parties"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/spells"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/stretchr/testify/require"
)

// The tables (tables/generic, tables/shoplist) through their real commands
// (plan 3b-3, rulings 6 and 7): history, who / online, party, keyring,
// spells, killstats and the shop list, at 30, 42 and 80 columns, with data
// as wide as it gets in play (28-letter names, the longest shipped room
// title and zone, every shipped spell, a difficulty-30 lock). Where a table
// fits 80 its bordered form is pinned to what master drew; a table that
// names no lead column is pinned at every width.

// tableWidths are the widths each table is run at.
var tableWidths = []int{30, 42, 80}

// tableRoomId is the room the table fixtures stand in.
const tableRoomId = 60557

// longestShippedLine returns the longest first submatch of re over every
// shipped dogmud file matching glob (relative to the world folder).
func longestShippedLine(t *testing.T, glob string, re *regexp.Regexp) string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "_datafiles", "world", "dogmud", glob))
	require.NoError(t, err)
	require.NotEmpty(t, files, "no shipped files match %s", glob)
	longest := ""
	for _, f := range files {
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		for _, m := range re.FindAllStringSubmatch(string(b), -1) {
			v := strings.Trim(strings.TrimSpace(m[1]), `"'`)
			if len(v) > len(longest) {
				longest = v
			}
		}
	}
	require.NotEmpty(t, longest, "nothing in %s matched", glob)
	return longest
}

// longestShippedRoomTitle is the longest title of any shipped room.
func longestShippedRoomTitle(t *testing.T) string {
	return longestShippedLine(t, "rooms/*/*.yaml", regexp.MustCompile(`(?m)^title:(.*)$`))
}

// tableRoom seeds one room, titled title, with a north exit locked at
// difficulty and a lockbox, and returns it.
func tableRoom(t *testing.T, title string, difficulty uint8) *rooms.Room {
	t.Helper()
	room := &rooms.Room{
		RoomId: tableRoomId, Zone: "TestZone", Title: title, Description: "A room.",
		Exits: map[string]exit.RoomExit{
			"north": {RoomId: tableRoomId, Lock: gamelock.Lock{Difficulty: difficulty}},
		},
		Containers: map[string]rooms.Container{
			"lockbox": {Lock: gamelock.Lock{Difficulty: 4}},
		},
	}
	t.Cleanup(rooms.SeedRoomsForTest(map[int]*rooms.Room{tableRoomId: room},
		map[string]*rooms.ZoneConfig{"TestZone": {Name: "TestZone", RoomId: tableRoomId, RoomIds: map[int]struct{}{tableRoomId: {}}}}))
	return room
}

// tableUsers seeds a user per name (ids from firstId), each in the table
// room, and returns them in order.
func tableUsers(t *testing.T, firstId int, names ...string) []*users.UserRecord {
	t.Helper()
	recs := map[int]*users.UserRecord{}
	out := []*users.UserRecord{}
	for i, name := range names {
		u := users.NewTestUser(firstId+i, strings.ToLower(name), name, uint64(firstId+i))
		u.Character.RoomId = tableRoomId
		u.Character.Zone = "TestZone"
		recs[u.UserId] = u
		out = append(out, u)
	}
	t.Cleanup(users.SeedUsersForTest(recs))
	return out
}

// tableRun runs a command for user at width and returns what it queued.
func tableRun(t *testing.T, user *users.UserRecord, width int, run func()) string {
	t.Helper()
	user.LineWidth = width
	events.DrainQueuedMessagesForTest(user.UserId)
	run()
	return strings.Join(events.DrainQueuedMessagesForTest(user.UserId), "")
}

// historyAt is `history` for a user whose log holds entries, at width.
func historyAt(t *testing.T, width int, entries ...users.UserLogEntry) string {
	u := tableUsers(t, 98101, "Screenly")[0]
	u.EventLog = append(users.UserLog{}, entries...)
	return tableRun(t, u, width, func() { _, _ = History("", u, nil, 0) })
}

// tableLogTime is when every logged entry happened.
var tableLogTime = time.Date(2026, 11, 11, 15, 4, 0, 0, time.UTC)

// purchaseLog is the log line internal/actions/buy.go writes for a purchase
// from a shopkeeper, for item (with its article) from mob for price.
func purchaseLog(article, item, mob, price string) string {
	return article + `<ansi fg="itemname">` + item + `</ansi> from <ansi fg="mobname">` + mob + `</ansi> for <ansi fg="gold">` + price + `</ansi>`
}

func historyFits() []users.UserLogEntry {
	return []users.UserLogEntry{
		{Category: "shop", WhenTime: tableLogTime, What: "Purchased " + purchaseLog("a ", "lantern", "Keeper", "40 gold")},
		{Category: "combat", WhenTime: tableLogTime, What: "You defeated a rat."},
	}
}

// historyWide holds a purchase whose bordered table is 97 columns.
func historyWide() []users.UserLogEntry {
	return []users.UserLogEntry{
		{Category: "shop", WhenTime: tableLogTime, What: "Purchased " + purchaseLog("a ", "lantern", "Mistress Odell the Victualler", "40 gold")},
		{Category: "shop", WhenTime: tableLogTime, What: "Purchased " + purchaseLog("a ", "coil of rope", "Keeper", "2 gold")},
	}
}

// onlineAt is `online` for the first of the named users, an admin when
// admin is set, at width.
func onlineAt(t *testing.T, width int, admin bool, zone string, names ...string) string {
	us := tableUsers(t, 98201, names...)
	for _, u := range us {
		u.Character.Zone = zone
	}
	if admin {
		us[0].Role = users.RoleAdmin
	}
	return tableRun(t, us[0], width, func() { _, _ = Online("", us[0], nil, 0) })
}

// partyAt is `party list` for a party of the named users, the first
// leading, in a room titled title, at width.
func partyAt(t *testing.T, width int, title string, names ...string) string {
	tableRoom(t, title, 4)
	us := tableUsers(t, 98301, names...)
	party := parties.New(us[0].UserId)
	require.NotNil(t, party)
	t.Cleanup(party.Disband)
	for _, u := range us[1:] {
		require.True(t, party.InvitePlayer(u.UserId))
		require.True(t, party.AcceptInvite(u.UserId))
	}
	return tableRun(t, us[0], width, func() { cmdPartyList(us[0], party) })
}

// keyringAt is `keyring` for a user who holds the lockbox's key and has
// learned the first two steps of the north exit's lock, set at difficulty,
// in a room titled title, at width.
func keyringAt(t *testing.T, width int, title string, difficulty uint8) string {
	tableRoom(t, title, difficulty)
	u := tableUsers(t, 98401, "Screenly")[0]
	u.Character.KeyRing = map[string]string{
		"key-60557-lockbox": "",
		"60557-north":       "UD",
	}
	return tableRun(t, u, width, func() { _, _ = KeyRing("", u, nil, 0) })
}

// spellsAt is `spells` for a user who knows every spell in book, at width.
func spellsAt(t *testing.T, width int, book map[string]*spells.SpellData) string {
	t.Cleanup(spells.SeedSpellsForTest(book))
	u := tableUsers(t, 98501, "Screenly")[0]
	u.Character.SpellBook = map[string]int{}
	for id := range book {
		u.Character.SpellBook[id] = 1
	}
	return tableRun(t, u, width, func() { _, _ = Spells("", u, nil, 0) })
}

// spellsFit are three spells whose bordered table fits 80.
func spellsFit() map[string]*spells.SpellData {
	return map[string]*spells.SpellData{
		"glow":   {SpellId: "glow", Name: "Glow", Cost: 5, Targeting: "self", Difficulty: 1},
		"sparks": {SpellId: "sparks", Name: "Sparks", Cost: 20, Targeting: "single", Difficulty: 2, EffectType: "damage"},
		"mend":   {SpellId: "mend", Name: "Mend", Cost: 40, Targeting: "single", Difficulty: 3, EffectType: "heal"},
	}
}

// killstatsAt is `killstats` for a user who has killed three of mob 1.
func killstatsAt(t *testing.T, width int) string {
	u := tableUsers(t, 98601, "Screenly")[0]
	u.Character.KD.Kills = map[int]int{1: 3}
	return tableRun(t, u, width, func() { _, _ = Killstats("", u, nil, 0) })
}

// shopAt is a shop's item list (renderShopTable, as `list` sends it), its
// help line after it.
func shopAt(t *testing.T, width int, rows [][]string) string {
	return shopWithHelpAt(t, width, `Type buy to buy.`, rows)
}

func shopWithHelpAt(t *testing.T, width int, help string, rows [][]string) string {
	u := tableUsers(t, 98701, "Screenly")[0]
	return tableRun(t, u, width, func() {
		renderShopTable(u, `Items available`, `cyan`, `Keeper`, `mobname`, []string{"Qty", "Name", "Type", "Price"}, rows, help)
	})
}

func shopFits() [][]string {
	return [][]string{{"3", "a flask of clear river water", "drinkable", "12"}, {"N/A", "a lantern", "light", "40"}}
}

// genericAt is tables/generic for a table that names no lead column (as
// alias, killstats, leaderboards, the alt list and the admin tables send
// it), at width.
func genericAt(t *testing.T, width int) string {
	t.Helper()
	out, err := templates.ProcessAtWidth("tables/generic", templates.GetTable("Lost and found",
		[]string{"Item", "Where", "Since"},
		[][]string{
			{"the Ancient Lantern of the Drowned Lighthouse", "the Thornwall Temple District", "two days"},
			{"a rope", "Main Street", "today"},
		}), width)
	require.NoError(t, err)
	return out
}

// genericFitsAt is genericAt with data whose bordered form fits 80, so
// its plain-column form (tables/generic.w40.template) is picked one column
// narrower; genericAt's rows are too wide for that form at any width.
func genericFitsAt(t *testing.T, width int) string {
	t.Helper()
	out, err := templates.ProcessAtWidth("tables/generic", templates.GetTable("Lost and found",
		[]string{"Item", "Where", "Since"},
		[][]string{
			{"a lantern", "the Thornwall Temple District", "two days"},
			{"a rope", "Main Street", "today"},
		}), width)
	require.NoError(t, err)
	return out
}

// tableScreen is a table run at a width, with data whose bordered form
// fits 80.
type tableScreen func(t *testing.T, width int) string

// leadTables are the tables ruling 6 gives a lead column (or that already
// lead with it), each with data whose bordered form fits 80.
var leadTables = map[string]tableScreen{
	"history": func(t *testing.T, width int) string { return historyAt(t, width, historyFits()...) },
	"online_player": func(t *testing.T, width int) string {
		return onlineAt(t, width, false, "Frostfang", goScreenLongestName)
	},
	"online_admin": func(t *testing.T, width int) string {
		return onlineAt(t, width, true, "Frostfang", "Screenly")
	},
	"party":   func(t *testing.T, width int) string { return partyAt(t, width, "Town Square", "Screenly", "Halix") },
	"keyring": func(t *testing.T, width int) string { return keyringAt(t, width, "Town Square", 4) },
	"spells":  func(t *testing.T, width int) string { return spellsAt(t, width, spellsFit()) },
	"shop":    func(t *testing.T, width int) string { return shopAt(t, width, shopFits()) },
}

// noLeadFitTables are tables that name no lead column, with data whose
// bordered form fits 80, pinned at 80 and in their plain-column form.
var noLeadFitTables = map[string]tableScreen{
	"generic_fits": genericFitsAt,
}

// noLeadTables are tables that name no lead column.
var noLeadTables = map[string]tableScreen{
	"killstats": killstatsAt,
	"generic":   genericAt,
}

// tableRender runs screen at width in a fresh world.
func tableRender(t *testing.T, screen tableScreen, width int) string {
	t.Helper()
	var out string
	t.Run("w"+strconv.Itoa(width), func(t *testing.T) {
		t.Cleanup(seedAllRegistries())
		useDogmudTemplates(t)
		out = screen(t, width)
	})
	return out
}

// The tables keep what master drew where ruling 6 leaves them alone: a lead
// table's bordered form at 80 and its plain-column form (at one column
// narrower than its bordered form, where that form is picked), and a table
// that names no lead column at every width.
func TestTablesKeepTheirMasterOutput(t *testing.T) {
	for name, screen := range leadTables {
		t.Run(name, func(t *testing.T) {
			at80 := tableRender(t, screen, 80)
			checkGolden(t, "tables_"+name+"_w80", at80)
			bordered := templates.WidestLine(at80)
			require.Contains(t, at80, "╒", "%s at 80 is not bordered", name)
			plain := tableRender(t, screen, bordered-1)
			require.NotContains(t, plain, "╒", "%s at %d is bordered", name, bordered-1)
			checkGolden(t, "tables_"+name+"_w40form", plain)
		})
	}
	// A table with no lead keeps master's bordered form at 80 and its
	// plain-column form one column narrower (tables/generic.w40.template,
	// which plan 3b-3 left as master has it).
	for name, screen := range noLeadFitTables {
		t.Run(name, func(t *testing.T) {
			at80 := tableRender(t, screen, 80)
			require.Contains(t, at80, "╒", "%s at 80 is not bordered", name)
			checkGolden(t, "tables_"+name+"_w80", at80)
			width := templates.WidestLine(at80) - 1
			plain := tableRender(t, screen, width)
			require.Regexp(t, `(?m)^Item +Where +Since`, regexp.MustCompile(`</?ansi[^>]*>`).ReplaceAllString(plain, ""),
				"%s at %d is not the plain-column form", name, width)
			checkGolden(t, "tables_"+name+"_w40form", plain)
		})
	}
	for name, screen := range noLeadTables {
		t.Run(name, func(t *testing.T) {
			for _, width := range tableWidths {
				checkGolden(t, "tables_"+name+"_w"+strconv.Itoa(width), tableRender(t, screen, width))
			}
		})
	}
}

// shippedRoomTitles is the title of every shipped room, by room id.
func shippedRoomTitles(t *testing.T) map[int]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join("..", "..", "_datafiles", "world", "dogmud", "rooms", "*", "*.yaml"))
	require.NoError(t, err)
	idRE := regexp.MustCompile(`(?m)^roomid:\s*(\d+)\s*$`)
	titleRE := regexp.MustCompile(`(?m)^title:(.*)$`)
	titles := map[int]string{}
	for _, f := range files {
		b, err := os.ReadFile(f)
		require.NoError(t, err)
		id, title := idRE.FindStringSubmatch(string(b)), titleRE.FindStringSubmatch(string(b))
		if id == nil || title == nil {
			continue // a zone-config.yaml
		}
		roomId, err := strconv.Atoi(id[1])
		require.NoError(t, err)
		titles[roomId] = strings.Trim(strings.TrimSpace(title[1]), `"'`)
	}
	require.Greater(t, len(titles), 1000, "the shipped rooms did not load")
	require.Equal(t, "The Coachman's Rest, Common Room", titles[5406])
	return titles
}

// No line of keyring's two-line form starts with the " ·" between pairs,
// for any shipped room's Location at 30 to 39 columns (review of plan
// 3b-3: "#5406 The Coachman's Rest, Common Room" left a bare "·" at 30 and
// 31). The dot stays on its pair's last word.
func TestKeyringDetailNeverStartsWithADot(t *testing.T) {
	t.Cleanup(seedAllRegistries())
	useDogmudTemplates(t)
	u := tableUsers(t, 98451, "Screenly")[0]
	checked := 0
	for roomId, title := range shippedRoomTitles(t) {
		room := &rooms.Room{
			RoomId: roomId, Zone: "TestZone", Title: title, Description: "A room.",
			Exits: map[string]exit.RoomExit{
				"north": {RoomId: roomId, Lock: gamelock.Lock{Difficulty: 4}},
			},
			Containers: map[string]rooms.Container{"lockbox": {Lock: gamelock.Lock{Difficulty: 4}}},
		}
		restore := rooms.SeedRoomsForTest(map[int]*rooms.Room{roomId: room},
			map[string]*rooms.ZoneConfig{"TestZone": {Name: "TestZone", RoomId: roomId, RoomIds: map[int]struct{}{roomId: {}}}})
		u.Character.RoomId = roomId
		id := strconv.Itoa(roomId)
		u.Character.KeyRing = map[string]string{"key-" + id + "-lockbox": "", id + "-north": "UD"}
		for width := 30; width <= 39; width++ {
			out := tableRun(t, u, width, func() { _, _ = KeyRing("", u, nil, 0) })
			for _, line := range plainLines(out) {
				if strings.HasPrefix(strings.TrimSpace(line), "·") {
					t.Errorf("room %d at %d: a line starts with a dot: %q in\n%s", roomId, width, line, strings.Join(plainLines(out), "\n"))
				}
			}
			checked++
		}
		restore()
	}
	require.Greater(t, checked, 10000)
}

// shippedSpells is every shipped spell, read from the YAML.
func shippedSpells(t *testing.T) map[string]*spells.SpellData {
	t.Helper()
	restore := spells.SeedSpellsForTest(nil)
	defer restore()
	spells.LoadSpellFiles()
	book := spells.GetAllSpells()
	require.Greater(t, len(book), 10, "the shipped spells did not load")
	return book
}

// tableItem is one item of a two-line table form: its lead (the lines at
// indent 0, joined) and its detail (the lines under it, joined).
type tableItem struct{ lead, detail string }

// twoLineItems reads the items of a two-line table form (plain lines, the
// title first), or reports false when out is a bordered or plain-column
// form.
func twoLineItems(lines []string) ([]tableItem, bool) {
	items := []tableItem{}
	for _, line := range lines[1:] {
		if strings.ContainsAny(line, "╒│") {
			return nil, false
		}
		switch {
		case line == "":
		case strings.HasPrefix(line, "  "):
			if len(items) == 0 {
				return nil, false
			}
			last := &items[len(items)-1]
			last.detail = strings.TrimSpace(last.detail + " " + strings.TrimSpace(line))
		case len(items) > 0 && items[len(items)-1].detail == "":
			last := &items[len(items)-1]
			last.lead += " " + line
		default:
			items = append(items, tableItem{lead: line})
		}
	}
	return items, true
}

// tableCase is a wide table: its render, the lead each item must have (in
// order when ordered), what each item's detail must list, and the widths at
// which its bordered and plain-column forms are too wide, so the two-line
// form must be used.
type tableCase struct {
	screen     tableScreen
	leads      func(t *testing.T) []string
	ordered    bool
	detail     []string
	twoLineAll bool // the two-line form at 80 too
}

var spellLeadNames = func(t *testing.T) []string {
	names := []string{}
	for _, sp := range shippedSpells(t) {
		name := sp.Name
		if len(sp.Aliases) > 0 && sp.Aliases[0] != "" {
			name += " (" + sp.Aliases[0] + ")"
		}
		names = append(names, name)
	}
	return names
}

// wideTables are the lead tables with data as wide as play makes them.
func wideTables() map[string]tableCase {
	return map[string]tableCase{
		"history": {
			screen: func(t *testing.T, width int) string { return historyAt(t, width, historyWide()...) },
			leads: func(t *testing.T) []string {
				return []string{"Purchased a lantern from Mistress Odell the Victualler for 40 gold", "Purchased a coil of rope from Keeper for 2 gold"}
			},
			ordered: true, detail: []string{"Type: shop", "Time: Nov 11 '26 3:04PM"}, twoLineAll: true,
		},
		"online_admin": {
			screen: func(t *testing.T, width int) string {
				return onlineAt(t, width, true, longestShippedZoneName(t), goScreenLongestName, goScreenLegacyName)
			},
			leads:  func(t *testing.T) []string { return []string{goScreenLongestName, goScreenLegacyName} },
			detail: []string{"UserId: ", "Online: ", "Role: ", "Zone: ", "RoomId: 60557"}, twoLineAll: true,
		},
		"online_player": {
			screen: func(t *testing.T, width int) string {
				return onlineAt(t, width, false, "Frostfang", goScreenLongestName, goScreenLegacyName)
			},
			leads:  func(t *testing.T) []string { return []string{goScreenLongestName, goScreenLegacyName} },
			detail: []string{"Title: ", "Online: ", "Role: user"},
		},
		"party": {
			screen: func(t *testing.T, width int) string {
				return partyAt(t, width, longestShippedRoomTitle(t), goScreenLongestName, goScreenLegacyName)
			},
			leads:   func(t *testing.T) []string { return []string{goScreenLongestName, goScreenLegacyName} },
			ordered: true, detail: []string{"Status: ", "Health: 100%", "Location: The Crossing, Great Hall Over the Waters"},
			twoLineAll: true,
		},
		"keyring": {
			screen:  func(t *testing.T, width int) string { return keyringAt(t, width, longestShippedRoomTitle(t), 30) },
			leads:   func(t *testing.T) []string { return []string{"lockbox", "north"} },
			ordered: true, detail: []string{"Type: ", "Location: #60557 The Crossing, Great Hall Over the Waters", "Sequence: "},
			twoLineAll: true,
		},
		"spells": {
			screen: func(t *testing.T, width int) string { return spellsAt(t, width, shippedSpells(t)) },
			leads:  spellLeadNames, detail: []string{"SpellId: ", "Target: ", "Cost: "}, twoLineAll: true,
		},
		"shop": {
			screen: func(t *testing.T, width int) string {
				// No help line, so every line is the table's.
				return shopWithHelpAt(t, width, "", [][]string{
					{"3", "A Salt-Stiff Page of a Sailor's Telling", "readable", "1240"},
					{"N/A", "a lantern", "light", "40"},
				})
			},
			leads:   func(t *testing.T) []string { return []string{"A Salt-Stiff Page of a Sailor's Telling", "a lantern"} },
			ordered: true, detail: []string{"Qty: ", "Type: ", "Price: "},
		},
	}
}

// tableFitsAt fails any line of lines wider than width, but for a legacy
// 32-letter name alone on its line (ruling 8).
func tableFitsAt(t *testing.T, name string, lines []string, width int) {
	t.Helper()
	for _, line := range lines {
		if templates.WidestLine(line) > width && strings.TrimSpace(line) != goScreenLegacyName {
			t.Errorf("%s at %d has a line %d wide: %q in:\n%s", name, width, templates.WidestLine(line), line, strings.Join(lines, "\n"))
		}
	}
}

// Every table fits at 30, 42 and 80 with data as wide as play makes it;
// where it takes its two-line form, each item is headed by its lead column
// (history: Log; who / online: Name; party: Name; keyring: Where; spells:
// Name; the shop list: what is for sale), and the detail under it lists the
// other columns, the first included (ruling 6).
func TestTablesLeadWithTheNamedColumn(t *testing.T) {
	for name, tc := range wideTables() {
		t.Run(name, func(t *testing.T) {
			for _, width := range tableWidths {
				lines := plainLines(tableRender(t, tc.screen, width))
				tableFitsAt(t, name, lines, width)
				items, twoLine := twoLineItems(lines)
				if width == 30 || (width == 80 && tc.twoLineAll) {
					require.True(t, twoLine, "%s at %d is not the two-line form:\n%s", name, width, strings.Join(lines, "\n"))
				}
				if !twoLine {
					continue
				}
				var leads []string
				for _, item := range items {
					leads = append(leads, item.lead)
					for _, want := range tc.detail {
						require.Contains(t, item.detail, want, "%s at %d: item %q", name, width, item.lead)
					}
				}
				var expected []string
				t.Run("leads", func(t *testing.T) {
					t.Cleanup(seedAllRegistries())
					useDogmudTemplates(t)
					expected = tc.leads(t)
				})
				if tc.ordered {
					require.Equal(t, expected, leads, "%s at %d", name, width)
				} else {
					require.ElementsMatch(t, expected, leads, "%s at %d", name, width)
				}
			}
		})
	}
}

// `online` shows every name and zone whole, in the admin view too (ruling
// 7): a list too wide for the reader takes the table's narrow form, at 80
// as on a phone, where it once cut the zone and then the name to "...".
// Login runs the same command (OnLoginCommands in config.yaml).
func TestOnlineShowsEveryNameAndZoneWhole(t *testing.T) {
	for _, admin := range []bool{true, false} {
		for _, width := range tableWidths {
			var zone, out string
			t.Run("render", func(t *testing.T) {
				t.Cleanup(seedAllRegistries())
				useDogmudTemplates(t)
				zone = longestShippedZoneName(t)
				out = strings.Join(plainLines(onlineAt(t, width, admin, zone, goScreenLongestName, goScreenLegacyName)), "\n")
			})
			require.NotContains(t, out, "...", "admin %v at %d:\n%s", admin, width, out)
			for _, whole := range []string{goScreenLongestName, goScreenLegacyName} {
				require.Contains(t, out, whole, "admin %v at %d", admin, width)
			}
			if admin {
				require.Contains(t, strings.Join(strings.Fields(out), " "), zone, "admin at %d", width)
			}
		}
	}
}

// A history purchase line whose bordered table is 97 columns takes the
// two-line form at 80, led by the Log; the 80-column sample for the plan.
func TestHistoryPurchaseLineAt97Columns(t *testing.T) {
	headers := []string{"Type", "Time", "Log"}
	rows := [][]string{}
	for _, e := range historyWide() {
		rows = append(rows, []string{e.Category, e.WhenTime.Format(`Jan 2 '06 3:04PM`), e.What})
	}
	table := templates.GetTable("History", headers, rows)
	bordered := len(headers) + 1
	for _, w := range table.ColumnWidths {
		bordered += w + 2
	}
	require.Equal(t, 97, bordered, "the purchase line's bordered table")

	at80 := plainLines(tableRender(t, func(t *testing.T, width int) string { return historyAt(t, width, historyWide()...) }, 80))
	require.Equal(t, []string{
		".: History",
		"Purchased a lantern from Mistress Odell the Victualler for 40 gold",
		"  Type: shop · Time: Nov 11 '26 3:04PM",
		"Purchased a coil of rope from Keeper for 2 gold",
		"  Type: shop · Time: Nov 11 '26 3:04PM",
	}, at80)

	// At 30 the shopkeeper's name, a unit that fits the line, stays whole.
	at30 := plainLines(tableRender(t, func(t *testing.T, width int) string { return historyAt(t, width, historyWide()...) }, 30))
	require.Equal(t, []string{
		".: History",
		"Purchased a lantern from",
		"Mistress Odell the Victualler",
		"for 40 gold",
		"  Type: shop ·",
		"  Time: Nov 11 '26 3:04PM",
		"Purchased a coil of rope from",
		"Keeper for 2 gold",
		"  Type: shop ·",
		"  Time: Nov 11 '26 3:04PM",
	}, at30)
}
