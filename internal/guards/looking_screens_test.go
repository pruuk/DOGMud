package guards

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/actions"
	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/colorpatterns"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/exit"
	"github.com/GoMudEngine/GoMud/internal/gamelock"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mutations"
	"github.com/GoMudEngine/GoMud/internal/mutators"
	"github.com/GoMudEngine/GoMud/internal/pets"
	"github.com/GoMudEngine/GoMud/internal/rooms"
	"github.com/GoMudEngine/GoMud/internal/statmods"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/users"
	"github.com/GoMudEngine/GoMud/internal/util"
	"gopkg.in/yaml.v3"
)

// The "looking at things" screens with narrow forms (plan 3b-2): looking at
// a person, a corpse, a pet and into a container, identify and appraise,
// track, biome, and the room view's templates. Each fixture is as wide as the
// screen gets in play, or is the screen as the 2026-11-10 capture showed it,
// built from the data types its callers pass. The goldens prove a desktop
// sees at 80 columns what master drew, apart from the changes the owner
// ruled (the room alerts at every width, and an interior biome whose
// lighting line ran past 80).

var updateLookingScreens = flag.Bool("update-looking-screens", false,
	"rewrite internal/guards/testdata/looking_screens_80/*.golden from the current templates")

// seedLookingScreenRegistries gives the fixtures the conditions and
// mutations they name, restored when the test ends.
func seedLookingScreenRegistries(t *testing.T) {
	t.Helper()
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		83:  {ConditionId: 83, Name: "Broken Limb", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 12},
		940: {ConditionId: 940, Name: "Chrysalis Regeneration", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 50},
		941: {ConditionId: 941, Name: "Death's Shadow", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 5},
	}))
	t.Cleanup(mutations.SeedMutationsForTest(map[string]*mutations.MutationSpec{
		"reflect-skin-frostbite": {MutationId: "reflect-skin-frostbite", Name: "Reflect Skin",
			Visual: "Their skin has the faint blue sheen of old ice, and frost gathers in the creases of their knuckles."},
		"tail": {MutationId: "tail", Name: "Tail", Visual: "A long, sinuous tail sways behind them."},
	}))
}

// lookedAt is a character as another sees it: name, description, health.
func lookedAt(name, desc string, health, healthMax int, muts map[string]int) *characters.Character {
	c := users.NewUserRecord(88160, 0).Character
	c.Name = name
	c.Description = desc
	c.Health = health
	c.HealthMax.Value = healthMax
	c.Mutations = muts
	return c
}

// descriptionCaptured is `look Screenly` as captured.
func descriptionCaptured() any {
	return lookedAt("Screenly", "They seem thoroughly uninteresting.", 100, 100, nil)
}

// descriptionWide is a looked-at character with a 26-letter name (the
// longest whose health line fits beside its indent at 30), a two-paragraph
// description, mutation visuals, and the longest health words.
func descriptionWide() any {
	return lookedAt("Bartholomewsworthingtonian",
		"A tall, broad-shouldered traveller in a salt-stained greatcoat, with a weathered face and a nose that has been broken more than once. Their boots are caked with the grey mud of the lower marsh.\n\nA brass compass hangs from a cord at their throat, its needle trembling.",
		5, 100, map[string]int{"reflect-skin-frostbite": 2, "tail": 1})
}

// The two longest shipped mob names (29 letters each), read from their YAML.
const (
	rissMobFile  = "mobs/the_confluence/9443-riss_the_cartographer_scholar.yaml"
	odellMobFile = "mobs/east_road_to_greenford/9492-mistress_odell_the_victualler.yaml"
)

// shippedMobLooked is the shipped mob in file (under the dogmud world) as
// look shows it: its own name and description, at full health.
func shippedMobLooked(file string) func() any {
	return func() any {
		raw, err := os.ReadFile(filepath.Join(repoRootPath, dogmudDataFilesRoot, file))
		if err != nil {
			panic(err)
		}
		var mob struct {
			Character struct {
				Name        string `yaml:"name"`
				Description string `yaml:"description"`
			} `yaml:"character"`
		}
		if err := yaml.Unmarshal(raw, &mob); err != nil || mob.Character.Name == "" {
			panic(fmt.Sprintf("%s: no character name (%v)", file, err))
		}
		return lookedAt(mob.Character.Name, mob.Character.Description, 100, 100, nil)
	}
}

// lookingLongestName is a name at the configured maximum (NameSizeMax, 32),
// for players and pets alike.
const lookingLongestName = "Bartholomewsworthingtonianfellow"

// corpseCaptured is `look corpse` at the marsh rat, as captured.
func corpseCaptured() any {
	return lookedAt("Marsh Rat", "A small grey-brown marsh rat, wet-furred and quick, with bright black eyes and a pink scaly tail. It freezes at your approach and weighs whether you are a problem. ", 0, 10, nil)
}

// corpseWide is a corpse with a long name and a long description.
func corpseWide() any {
	return lookedAt("Drillmaster Vorn the Unyielding", "A broad, scarred veteran of the city watch, his grey hair cropped close and his arms roped with old muscle. He carries himself like a man who has never once lost an argument he cared about, and his eyes follow every recruit in the yard.", 0, 10, nil)
}

// inventoryLookFixture is the data look builds for character/inventory-look:
// the looked-at character's equipment and carried item names.
func inventoryLookFixture(wide, hide bool) func() any {
	return func() any {
		c := users.NewUserRecord(88161, 0).Character
		names := []string{"Leather Backpack", "Strongbox Key", "Healing Draught", "Iron Ore", "Tanned Hide"}
		if wide {
			long := screenItem(99211, "Pit Fighter's Serrated Basalt-Iron Blade", items.Weapon, 3, true)
			long.Spec.NameSimple = "serrated basalt-iron blade"
			c.Equipment.Weapon = long
			back := screenItem(99212, "Wayfarer's Bottomless Pack", items.Back, 0, false)
			back.Spec.NameSimple = "wayfarer's bottomless pack"
			c.Equipment.Back = back
			names = append(names, "Spirit Fetish", "A Salt-Stiff Page")
		} else {
			sword := screenItem(99213, "Longsword", items.Weapon, 0, false)
			sword.Spec.NameSimple = "longsword"
			c.Equipment.Weapon = sword
			pack := screenItem(99214, "Pack", items.Back, 0, false)
			pack.Spec.NameSimple = "pack"
			c.Equipment.Back = pack
		}
		return map[string]any{
			`Character`:     c,
			`Equipment`:     &c.Equipment,
			`ItemNames`:     names,
			`HideEquipment`: hide,
		}
	}
}

// petFixture is the owner look passes to character/pet, with a pet whose
// name and load run long when wide.
func petFixture(wide bool) func() any {
	return petNamed(wide, "Screenly", "Whiskerfield")
}

// petNamed is petFixture with the owner's and the pet's names given.
func petNamed(wide bool, owner, pet string) func() any {
	return func() any {
		u := users.NewUserRecord(88162, 0)
		u.Character.Name = owner
		u.Character.Pet = pets.Pet{Name: pet, Type: "cat"}
		if wide {
			u.Character.Name = "Bartholomewsworthingtonian"
			u.Character.Pet = pets.Pet{Name: "Sir Whiskerfield Pounce", Type: "warhound", Food: 3,
				Items: []items.Item{
					screenItem(99221, "Pit Fighter's Serrated Basalt-Iron Blade", items.Weapon, 0, false),
					screenItem(99222, "Reinforced Travel Pack", items.Back, 0, false),
					screenItem(99223, "A Salt-Stiff Page of a Sailor's Telling", items.Neck, 0, false),
				}}
		}
		return u
	}
}

// petNamedAfterMob is a pet given the name of the shipped mob in file, so
// the pet's name line runs as long as the longest shipped names.
func petNamedAfterMob(file string) func() any {
	return func() any {
		name := shippedMobLooked(file)().(*characters.Character).Name
		return petNamed(false, "Screenly", name)()
	}
}

// biomeFixture is a biome as rooms.GetBiome returns it.
func biomeFixture(name, symbol string, sky float64, lamp, street bool, desc string) func() any {
	return func() any {
		b := &rooms.BiomeInfo{Name: name, Symbol: symbol, Description: desc, StreetLamp: street}
		b.SkyLight = &sky
		if lamp {
			l := 40
			b.Lamp = &l
		}
		return b
	}
}

var (
	biomeOutdoorCaptured = biomeFixture("Deep Water", "≈", 1, false, false,
		"Deep water is dangerous to traverse without the appropriate equipment. Very easy to lose your footing, or go under.")
	biomeInteriorCaptured = biomeFixture("Interior", "⌂", 0.4, true, false,
		"Inside a built structure, whether a house, a hall, a temple or the sealed corridors of something older. Lit by what its builders put there rather than by the sky.")
	biomeStreetLamp = biomeFixture("City Streets", "#", 1, true, true,
		"Cobbled streets between close-set buildings, busy by day and watched by night. Lamplighters make their rounds at dusk.")
)

// identifyFixture is the data identify and appraise pass: the item and its
// spec.
func identifyFixture(item items.Item, spec items.ItemSpec) func() any {
	return func() any {
		itm, sp := item, spec
		itm.Spec = &sp
		return struct {
			Item     *items.Item
			ItemSpec *items.ItemSpec
		}{&itm, &sp}
	}
}

// identifyCaptured is the hull-plate cuirass the capture identified
// (items/armor-20000/body/20091-hull_plate_cuirass.yaml).
var identifyCaptured = identifyFixture(items.Item{ItemId: 20091}, items.ItemSpec{
	ItemId: 20091, Name: "Hull-Plate Cuirass", NameSimple: "cuirass", Type: items.Body, Subtype: items.Wearable,
	Description:        "A breastplate cut and folded from a curved section of the grey material, shaped somehow to the wearer without a rivet or a seam in sight. It is lighter than plate that size has any right to be and colder against the body, and it turns a blow with a low ringing note that hangs in the air. Long scores mar its face where something once tested it and failed. It was lifted from the leavings the buried place kept of those who came before.",
	PhysicalMitigation: 16, MagicalMitigation: 8, ConvictionMitigation: 5,
	StatMods: statmods.StatMods{"vitality": 8, "strength": 4}, Value: 1200,
})

// identifyWide is a cursed caster weapon with every section full: a long
// name, an element, applied and crit conditions, and stat mods.
var identifyWide = identifyFixture(items.Item{ItemId: 99230}, func() items.ItemSpec {
	spec := items.ItemSpec{
		ItemId: 99230, Name: "Pit Fighter's Serrated Basalt-Iron Blade", Type: items.Weapon, Subtype: items.Slashing,
		Description: "A heavy blade of dark basalt-iron, its edge cut into teeth.", Cursed: true,
		DamageMultiplier: 1.4, SpellDamageMultiplier: 1.2, Element: items.Electricity,
		ConditionIds: []int{940}, StatMods: statmods.StatMods{"strength": 25, "perception": -12, "vitality": 3},
		Value: 1234567,
	}
	spec.Damage.CritConditionIds = []int{941}
	return spec
}())

// containerFixture is the data look passes for what is inside a container.
func containerFixture(names ...string) func() any {
	return func() any {
		formatted := make([]string, len(names))
		for i, n := range names {
			formatted[i] = `<ansi fg="itemname">` + n + `</ansi>`
		}
		return map[string]any{`ItemNames`: names, `ItemNamesFormatted`: formatted}
	}
}

var (
	containerCaptured = containerFixture("Pit Fighter's Sword", "Reinforced Travel Pack")
	containerWide     = containerFixture("1,234,567 gold", "A Salt-Stiff Page of a Sailor's Telling",
		"Pit Fighter's Serrated Basalt-Iron Blade", "Reinforced Travel Pack", "Strongbox Key", "Iron Ore")
	containerEmpty = containerFixture()
)

// trackCaptured is `track` in Market Square as captured; trackWide has long
// names and the trails' exits.
func trackCaptured() any {
	return []actions.TrackingInfo{{Name: "City Guard", Type: "mob", Strength: "Warm"}}
}

func trackWide() any {
	return []actions.TrackingInfo{
		{Name: "Drillmaster Vorn the Unyielding", Type: "mob", Strength: "Fresh", ExitName: "northwest"},
		{Name: "Bartholomewsworthingtonian", Type: "user", Strength: "Faint", ExitName: "shimmering portal"},
		{Name: "City Guard", Type: "mob", Strength: "Warm"},
	}
}

// roomViewFixture is the room description block as look passes it: prose
// already wrapped (rooms.GetDetails wraps it to the reader, so it is short
// here), the alerts GetDetails lists, and a tracking line. Wide adds the
// longest shipped alerts: the character room, the start room, and the
// wildfire and blizzard mutators' texts.
func roomViewFixture(wide bool) func() any {
	return func() any {
		d := rooms.RoomTemplateDetails{
			Description: "A solid stone building\nwith iron-banded doors.",
			RoomAlerts:  []string{rooms.AlertBank, rooms.AlertStorage},
		}
		if wide {
			d.RoomAlerts = append(d.RoomAlerts, rooms.AlertCharacterRoom, rooms.AlertStart,
				"!!!                A wildfire is burning here!                !!!",
				"A blizzard howls through here.")
			d.TrackingString = `Tracking <ansi fg="mobname">Drillmaster Vorn the Unyielding</ansi>... They went <ansi fg="exit">northwest</ansi>`
			d.IsNight = true
		}
		return d
	}
}

// roomTitleFixture is the title line of a room as look passes it.
func roomTitleFixture(title, zone string, pvp bool) func() any {
	return func() any {
		return rooms.RoomTemplateDetails{Title: title, Zone: zone, RoomSymbol: "⌂", RoomLegend: "Bank", ShowPvp: pvp}
	}
}

var (
	roomTitleCaptured = roomTitleFixture("Thornwall Bank", "Thornwall City", false)
	roomTitleWide     = roomTitleFixture("The Drowned Lighthouse's Lower Gallery", "Saltmarsh Coast Ruins", true)
)

// exitsFixture is the exits line of a room: plain, locked, unlocked and
// secret exits, and a temporary one.
func exitsFixture(wide bool) func() any {
	return func() any {
		d := rooms.RoomTemplateDetails{
			VisibleExits:   map[string]exit.RoomExit{"north": {RoomId: 1}},
			TemporaryExits: map[string]exit.TemporaryRoomExit{},
		}
		if wide {
			d.VisibleExits = map[string]exit.RoomExit{
				"north": {RoomId: 1}, "south": {RoomId: 2}, "east": {RoomId: 3}, "west": {RoomId: 4},
				"northeast": {RoomId: 5, Lock: gamelock.Lock{Difficulty: 5}},
				"southwest": {RoomId: 6, Lock: gamelock.Lock{Difficulty: 5, UnlockedRound: 1}},
				"up":        {RoomId: 7, Secret: true}, "down": {RoomId: 8},
			}
			d.TemporaryExits = map[string]exit.TemporaryRoomExit{"portal": {RoomId: 9, Title: "a shimmering violet portal"}}
		}
		return d
	}
}

// exitsMixed is an exits line that fits 80 with every kind of exit: plain,
// locked, unlocked (opened this round), secret and temporary.
func exitsMixed() any {
	return rooms.RoomTemplateDetails{
		VisibleExits: map[string]exit.RoomExit{
			"down":  {RoomId: 1},
			"north": {RoomId: 2, Lock: gamelock.Lock{Difficulty: 5}},
			"east":  {RoomId: 3, Lock: gamelock.Lock{Difficulty: 5, UnlockedRound: util.GetRoundCount() + 1}},
			"up":    {RoomId: 4, Secret: true},
		},
		TemporaryExits: map[string]exit.TemporaryRoomExit{"ladder": {RoomId: 5, Title: "a rope ladder"}},
	}
}

// fixturesFixture is the room's fixture lines as look passes them.
func fixturesFixture(names ...string) func() any {
	return func() any {
		type line struct {
			Name     string
			Lit      bool
			Darkness bool
		}
		lines := []line{}
		for i, n := range names {
			lines = append(lines, line{Name: n, Lit: i%2 == 0, Darkness: i == 2})
		}
		return map[string]any{`Fixtures`: lines, `IsDark`: false, `IsNight`: true}
	}
}

var (
	fixturesShort = fixturesFixture("brazier", "lamp", "shadowstone")
	fixturesWide  = fixturesFixture("great iron chandelier of the Thornwall Temple", "lamp", "obsidian shard of the deep dark")
)

// signFixture is a sign or a rune as the room holds it.
func signFixture(text string) func() any {
	return func() any { return rooms.Sign{DisplayText: text} }
}

var (
	signShort = signFixture("Welcome")
	signWide  = signFixture("Beware the marsh at night. The lights you see out there are not lanterns, and they do not want you home.")
	// A player-written sign with one word longer than a whole line at 42.
	signLongWord = signFixture("Keep out: " + strings.Repeat("AAAAAAAAAA", 5) + " by order of the Watch.")
)

// lookingScreenGoldens are the screens whose 80-column output is pinned.
// Each is keyed by the template and a short name for its data.
var lookingScreenGoldens = map[string]struct {
	name    string
	fixture templateFixture
}{
	"description_captured":        {"character/description", templateFixture{setup: seedLookingScreenRegistries, data: descriptionCaptured}},
	"description_wide":            {"character/description", templateFixture{setup: seedLookingScreenRegistries, data: descriptionWide}},
	"description-corpse_captured": {"character/description-corpse", templateFixture{data: corpseCaptured}},
	"inventory-look_captured":     {"character/inventory-look", templateFixture{data: inventoryLookFixture(false, false)}},
	"inventory-look_hidden":       {"character/inventory-look", templateFixture{data: inventoryLookFixture(false, true)}},
	"pet_captured":                {"character/pet", templateFixture{data: petFixture(false)}},
	"identify_captured":           {"descriptions/identify", templateFixture{setup: seedLookingScreenRegistries, data: identifyCaptured}},
	"identify_wide":               {"descriptions/identify", templateFixture{setup: seedLookingScreenRegistries, data: identifyWide}},
	"insidecontainer_captured":    {"descriptions/insidecontainer", templateFixture{data: containerCaptured}},
	"insidecontainer_wide":        {"descriptions/insidecontainer", templateFixture{data: containerWide}},
	"insidecontainer_empty":       {"descriptions/insidecontainer", templateFixture{data: containerEmpty}},
	"track_captured":              {"descriptions/track", templateFixture{data: trackCaptured}},
	"room-title_captured":         {"descriptions/room-title", templateFixture{data: roomTitleCaptured}},
	"exits_captured":              {"descriptions/exits", templateFixture{data: exitsFixture(false)}},
	"exits_mixed":                 {"descriptions/exits", templateFixture{data: exitsMixed}},
	"room_tracking":               {"descriptions/room", templateFixture{data: trackingAfterMob(rissMobFile)}},
	"fixtures_short":              {"descriptions/fixtures", templateFixture{data: fixturesShort}},
	"fixtures_wide":               {"descriptions/fixtures", templateFixture{data: fixturesWide}},
	"rune_short":                  {"descriptions/rune", templateFixture{data: signShort}},
	"sign_short":                  {"descriptions/sign", templateFixture{data: signShort}},
	// Recorded on the branch: owner-approved 80-column changes (plan 3b-2):
	// the alerts between full-width rules, and the biome box drawn as two
	// 80-wide rules with its lighting wrapped under its value.
	"room_alerts":    {"descriptions/room", templateFixture{data: roomViewFixture(false)}},
	"biome_outdoor":  {"descriptions/biome", templateFixture{data: biomeOutdoorCaptured}},
	"biome_interior": {"descriptions/biome", templateFixture{data: biomeInteriorCaptured}},
	"biome_street":   {"descriptions/biome", templateFixture{data: biomeStreetLamp}},
}

// A name is never split (owner, 2026-11-10: a single unbreakable word is an
// accepted exception, as on the status sheet). With a 32-letter name, the
// configured maximum for players and pets, looking at a person, a corpse or
// a pet fits at 42 and 80; at 30 a line runs past only where the name is
// alone on it, beside at most its ".: " heading or its indent and one stop.
func TestLookingWithTheLongestNameRunsPastOnlyOnItsName(t *testing.T) {
	templateRenderSetup(t)
	seedLookingScreenRegistries(t)
	if len(lookingLongestName) != 32 {
		t.Fatalf("the name is %d letters, not the 32 maximum", len(lookingLongestName))
	}
	tags := regexp.MustCompile(`</?ansi[^>]*>`)
	alone := regexp.MustCompile(`^(\.: | *)` + lookingLongestName + `[.,]?$`)
	screens := map[string]any{
		"character/description":        lookedAt(lookingLongestName, "They seem thoroughly uninteresting.", 100, 100, nil),
		"character/description-corpse": lookedAt(lookingLongestName, "A body.", 0, 10, nil),
		"character/pet":                petNamed(false, lookingLongestName, lookingLongestName)(),
	}
	for name, data := range screens {
		for _, width := range templateWidths {
			out, err := templates.ProcessAtWidth(name, data, width)
			if err != nil {
				t.Fatal(err)
			}
			for _, line := range strings.Split(tags.ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), ""), "\n") {
				got := templates.WidestLine(line)
				if got <= width {
					continue
				}
				if width != 30 || !alone.MatchString(line) {
					t.Errorf("%s at %d: a line other than the name alone is %d wide: %q\n%s", name, width, got, line, out)
				} else {
					t.Logf("%s at 30: the name's own line is %d wide: %q", name, got, line)
				}
			}
		}
	}
}

// The shipped wildfire mutator's alert, with its per-letter flame colours,
// reads as its author spaced it at every width: "!!!", the sentence, "!!!",
// on one line when it fits, else one to a line, each centred.
func TestShippedWildfireAlertReadsWhole(t *testing.T) {
	templateRenderSetup(t)
	root := filepath.Join(repoRootPath, dogmudDataFilesRoot)
	var spec mutators.MutatorSpec
	raw, err := os.ReadFile(filepath.Join(root, "mutators", "wildfire.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &spec); err != nil || spec.AlertModifier == nil {
		t.Fatalf("wildfire.yaml has no alert (%v)", err)
	}
	var patterns map[string][]int
	raw, err = os.ReadFile(filepath.Join(root, "color-patterns.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(raw, &patterns); err != nil || len(patterns[spec.AlertModifier.ColorPattern]) == 0 {
		t.Fatalf("color-patterns.yaml has no %q pattern (%v)", spec.AlertModifier.ColorPattern, err)
	}
	t.Cleanup(colorpatterns.SeedPatternsForTest(patterns))
	alert := colorpatterns.ApplyColorPattern(spec.AlertModifier.Text, spec.AlertModifier.ColorPattern)
	if !strings.Contains(alert, `<ansi fg=`) {
		t.Fatal("the flame pattern was not applied; the test proves nothing")
	}

	pad := func(width int, s string) string { return strings.Repeat(" ", (width-len(s))/2) + s }
	want := map[int][]string{
		80: {pad(80, spec.AlertModifier.Text)},
		42: {pad(42, "!!!"), pad(42, "A wildfire is burning here!"), pad(42, "!!!")},
		30: {pad(30, "!!!"), pad(30, "A wildfire is burning here!"), pad(30, "!!!")},
	}
	tags := regexp.MustCompile(`</?ansi[^>]*>`)
	for width, lines := range want {
		out, err := templates.ProcessAtWidth("descriptions/room", rooms.RoomTemplateDetails{RoomAlerts: []string{alert}}, width)
		if err != nil {
			t.Fatal(err)
		}
		plain := strings.Split(strings.Trim(tags.ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), ""), "\n"), "\n")
		// The rules are the first and last lines; the alert is between.
		if len(plain) < 3 || strings.Join(plain[1:len(plain)-1], "|") != strings.Join(lines, "|") {
			t.Errorf("at %d the wildfire alert reads\n%q\nwant\n%q", width, plain, lines)
		}
	}
}

// An exit and its tag, and the room title's PvP tag, are one unit each: never
// split across lines while the unit fits on a line by itself (owner,
// 2026-11-10), at every narrow width.
//
// Every line fits too, but for one choice-9 line: a 32-letter single-word
// name, which cannot fit at 30, alone on its line with what is glued to it.
func TestRoomViewUnitsStayWhole(t *testing.T) {
	templateRenderSetup(t)
	tags := regexp.MustCompile(`</?ansi[^>]*>`)
	lone := regexp.MustCompile(`^ *` + lookingLongestName + `\S*$`)
	riss := shippedMobLooked(rissMobFile)().(*characters.Character).Name
	for _, tc := range []struct {
		name   string
		data   func() any
		widths []int
		units  []string
	}{
		{"descriptions/exits", exitsFixture(true), []int{30, 42}, []string{"northeast (locked)", "southwest (locked)", "(up)"}},
		{"descriptions/exits", func() any {
			return rooms.RoomTemplateDetails{
				VisibleExits: map[string]exit.RoomExit{"north": {RoomId: 1},
					"northeast": {RoomId: 5, Lock: gamelock.Lock{Difficulty: 5}}},
				TemporaryExits: map[string]exit.TemporaryRoomExit{},
			}
		}, []int{30, 42}, []string{"northeast (locked)"}},
		{"descriptions/room-title", roomTitleWide, []int{30, 42}, []string{"☠ PK Area ☠"}},
		// The tracking line as rooms.GetDetails builds it. At 30 the
		// 29-letter name with its "..." (32) is wider than a line, so it may
		// wrap; at 42 it stays whole.
		{"descriptions/room", trackingRoom(riss, "a shimmering violet portal"), []int{42}, []string{riss, "a shimmering violet portal"}},
		{"descriptions/room", trackingRoom(riss, "a shimmering violet portal"), []int{30}, []string{"a shimmering violet portal"}},
		{"descriptions/room", trackingRoom(lookingLongestName, "north"), []int{30, 42}, []string{lookingLongestName}},
	} {
		for _, width := range []int{30, 42} {
			out, err := templates.ProcessAtWidth(tc.name, tc.data(), width)
			if err != nil {
				t.Fatal(err)
			}
			plain := tags.ReplaceAllString(strings.ReplaceAll(out, "\r\n", "\n"), "")
			for _, line := range strings.Split(plain, "\n") {
				if w := templates.WidestLine(line); w > width && !(width == 30 && lone.MatchString(line)) {
					t.Errorf("%s at %d has a line %d wide: %q\n%s", tc.name, width, w, line, plain)
				}
			}
			if !slices.Contains(tc.widths, width) {
				continue
			}
			for _, unit := range tc.units {
				if !strings.Contains(plain, unit) {
					t.Errorf("%s at %d splits %q:\n%s", tc.name, width, unit, plain)
				}
			}
		}
	}
}

// trackingRoom is a room whose tracking line follows name out by exitName,
// as rooms.GetDetails builds it.
func trackingRoom(name, exitName string) func() any {
	return func() any {
		return rooms.RoomTemplateDetails{Description: "A quiet room.",
			TrackingString: `Tracking <ansi fg="mobname">` + name + `</ansi>... They went <ansi fg="exit">` + exitName + `</ansi>`}
	}
}

// trackingAfterMob is trackingRoom for the shipped mob in file.
func trackingAfterMob(file string) func() any {
	return func() any {
		return trackingRoom(shippedMobLooked(file)().(*characters.Character).Name, "a shimmering violet portal")()
	}
}

func lookingScreenGoldenPath(key string) string {
	return testdataPath(filepath.Join("looking_screens_80", key+".golden"))
}

// A desktop sees what it always did: at 80 columns each screen renders
// exactly the bytes in its golden, line ends aside.
func TestLookingScreensKeepTheir80ColumnOutput(t *testing.T) {
	templateRenderSetup(t)
	for key, g := range lookingScreenGoldens {
		t.Run(key, func(t *testing.T) {
			if g.fixture.setup != nil {
				g.fixture.setup(t)
			}
			out, err := templates.ProcessAtWidth(g.name, g.fixture.data(), 80)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.ReplaceAll(out, "\r\n", "\n")
			path := lookingScreenGoldenPath(key)
			if *updateLookingScreens {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v (record it from master with -update-looking-screens)", err)
			}
			if w := templates.WidestLine(string(want)); w > 80 {
				t.Fatalf("the golden is %d wide: its fixture no longer fits 80, so it pins a form, not the base", w)
			}
			if got != string(want) {
				t.Errorf("%s at 80 changed:\n%s", key, lineDiff(string(want), got))
			}
		})
	}
}
