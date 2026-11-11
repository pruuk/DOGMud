package guards

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/characters"
	"github.com/GoMudEngine/GoMud/internal/conditions"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/skills"
	"github.com/GoMudEngine/GoMud/internal/species"
	"github.com/GoMudEngine/GoMud/internal/templates"
	"github.com/GoMudEngine/GoMud/internal/usercommands"
	"github.com/GoMudEngine/GoMud/internal/users"
)

// The character screens with narrow forms (plan 3b): fixtures that make each
// as wide as it gets in play, built from the data types their commands pass,
// and the goldens that prove a desktop sees at 80 columns what it saw before
// the forms. conditions, quests and title pass types private to
// internal/usercommands, so they are held there (narrow_forms_test.go).

var updateCharacterScreens = flag.Bool("update-character-screens", false,
	"rewrite internal/guards/testdata/character_screens_80/*.golden from the current templates")

// seedCharacterScreenRegistries gives the fixtures the species and the
// broken-limb condition they read, restored when the test ends.
func seedCharacterScreenRegistries(t *testing.T) {
	t.Helper()
	t.Cleanup(species.SeedSpeciesForTest(map[int]*species.Species{
		1: {SpeciesId: 1, Name: "human", Selectable: true},
	}))
	t.Cleanup(conditions.SeedConditionsForTest(map[int]*conditions.ConditionSpec{
		83: {ConditionId: 83, Name: "Broken Limb", TriggerRate: "1 round", RoundInterval: 1, TriggerCount: 12},
	}))
}

// screenItem is an item whose spec is its own, so no item registry is read.
func screenItem(id int, name string, typ items.ItemType, bonus int, enchanted bool) items.Item {
	itm := items.Item{ItemId: id, Spec: &items.ItemSpec{
		ItemId: id, Name: name, Type: typ, Hands: 1,
		Damage: items.Damage{BonusDamage: bonus},
	}}
	if enchanted {
		itm.Enchantments = 1
	}
	return itm
}

// statusScreenFixture is the status sheet of a character whose every value
// runs long: a long name and title, a stat at its longest word, a pool at a
// long word, gold and bank in the millions, a broken limb, two mutations with
// the longest ids, and the longest surrender policy.
func statusScreenFixture() any {
	return statusScreenFixtureNamed("Maximilianovich")
}

// statusScreenLongName is the status sheet of a character whose 26-letter
// name, the longest that fits beside " .: " at 30 columns, pushes "the" onto
// the title's line there.
func statusScreenLongName() any {
	return statusScreenFixtureNamed("Bartholomewsworthingtonian")
}

func statusScreenFixtureNamed(name string) any {
	u := users.NewUserRecord(88150, 0)
	c := u.Character
	c.Name = name
	c.SpeciesId = 1
	c.Stats.Strength.Base = 250
	c.Stats.Dexterity.Base = 140
	c.Stats.Perception.Base = 70
	c.Stats.Vitality.Base = 240
	c.Stats.Willpower.Base = 100
	c.Stats.Charisma.Base = 50
	c.RecalculateStats()
	c.Health = 1
	c.Gold = 1234567
	c.Bank = 987654321
	c.Mutations = map[string]int{"reflect-skin-frostbite": 4, "sustaining-presence": 2}
	c.SurrenderPolicy = characters.SurrenderPolicy{Mode: characters.SurrenderAutoTap, HpPctThreshold: 100}
	c.Conditions.Validate(true)
	c.Conditions.AddCondition(83, false)
	return u
}

// inventoryScreenFixture is the inventory data internal/usercommands'
// Inventory builds, for a character in long-named, enchanted gear carrying a
// long pack, components and a full bandolier.
func inventoryScreenFixture(searching bool) func() any {
	return func() any {
		u := users.NewUserRecord(88151, 0)
		c := u.Character
		c.Equipment.Weapon = screenItem(99201, "Pit Fighter's Serrated Basalt-Iron Blade", items.Weapon, 3, true)
		c.Equipment.Back = screenItem(99202, "Wayfarer's Bottomless Pack", items.Back, 0, false)
		c.Equipment.Neck = screenItem(99203, "A Salt-Stiff Page of a Sailor's Telling", items.Neck, 0, true)
		c.Equipment.Light = screenItem(99204, "Hooded Lantern", items.Light, 0, false)
		names := []string{"A Salt-Stiff Page of a Sailor's Telling (x12)", "Leather Backpack",
			"Reinforced Travel Pack", "Strongbox Key", "Pit Fighter's Sword", "Basalt-Iron Blade",
			"Spirit Fetish (turned)", "Healing Draught (3)", "A Stained Page with a Deathbed Word"}
		formatted := make([]string, len(names))
		for i, n := range names {
			formatted[i] = `<ansi fg="itemname">` + n + `</ansi>`
		}
		comps := []string{"Iron Ore (x24)", "Rough-Cut Moonstone Shard (x3)", "Tanned Hide"}
		compsFormatted := []string{`<ansi fg="itemname">Iron Ore</ansi> <ansi fg="uses-left">(x24)</ansi>`,
			`<ansi fg="itemname">Rough-Cut Moonstone Shard</ansi> <ansi fg="uses-left">(x3)</ansi>`,
			`<ansi fg="itemname">Tanned Hide</ansi>`}
		pots := []string{"Minor Potion of Healing (x4)", "Conviction Draught", "Stamina Draught (turned)"}
		potsFormatted := []string{`<ansi fg="itemname">Minor Potion of Healing</ansi> <ansi fg="uses-left">(x4)</ansi>`,
			`<ansi fg="itemname">Conviction Draught</ansi>`, `<ansi fg="8">Stamina Draught (turned)</ansi>`}
		return map[string]any{
			`Character`:               c,
			`Equipment`:               &c.Equipment,
			`ItemNames`:               names,
			`ItemNamesFormatted`:      formatted,
			`ComponentNames`:          comps,
			`ComponentNamesFormatted`: compsFormatted,
			`PotionNames`:             pots,
			`PotionNamesFormatted`:    potsFormatted,
			`Searching`:               searching,
			`EncumbranceLabel`:        "overburdened",
			`EncumbranceColor`:        "red-bold",
		}
	}
}

// skillsScreenFixture is every skill at its longest rank word, two of them
// recovering, as usercommands.Skills builds it.
func skillsScreenFixture() any {
	list, cooldowns, blurbs := map[string]int{}, map[string]int{}, map[string]string{}
	width := 12
	for _, tag := range skills.GetAllSkillNames() {
		name := string(tag)
		list[name] = 100
		cooldowns[name] = 0
		blurbs[name] = skills.SkillBlurb(name)
		width = max(width, len(name))
	}
	list["alchemy"] = 0
	cooldowns["unarmed-combat"] = 3
	cooldowns["skullduggery"] = 2
	return usercommands.SkillsOptions{SkillList: list, SkillCooldowns: cooldowns, SkillBlurbs: blurbs, NameWidth: width}
}

// cooldownsScreenFixture is the cooldown map at its longest duration word.
func cooldownsScreenFixture() any {
	return map[string]int{"special moves": 50, "unarmed-combat": 5, "skullduggery": 2}
}

// storageScreenFixture is the slot list Storage builds: NameComplex, with a
// stack count, for long, enchanted, bonus-damage items, past ten slots.
func storageScreenFixture() any {
	slots := []string{}
	for i := 0; i < 11; i++ {
		itm := screenItem(99300+i, "Basalt-Iron Blade", items.Weapon, 0, false)
		if i == 4 {
			itm = screenItem(99300+i, "A Salt-Stiff Page of a Sailor's Telling", items.Weapon, 12, true)
		}
		name := itm.NameComplex()
		if i == 10 {
			name += ` <ansi fg="uses-left">(x14)</ansi>`
		}
		slots = append(slots, name)
	}
	return slots
}

// statusLongestName is the longest name a character can hold: 32 letters,
// the maximum (NameSizeMax) until plan 3b-3 lowered it to 28 for new names
// (ruling 8), and kept by the characters who already have one.
const statusLongestName = "Bartholomewsworthingtonianfellow"

// A name is never split (owner, 2026-11-10: a single unbreakable word is an
// accepted exception, like a long URL). With a 32-letter name the status
// sheet still fits at 42 and 80; at 30 the one line that runs past is the
// name's own, " .: " and the name, 36 wide.
func TestStatusWithTheLongestNameRunsPastOnlyOnItsName(t *testing.T) {
	templateRenderSetup(t)
	seedCharacterScreenRegistries(t)
	if len(statusLongestName) != 32 {
		t.Fatalf("the name is %d letters, not the 32 a legacy name may have", len(statusLongestName))
	}
	for _, width := range templateWidths {
		out, err := templates.ProcessAtWidth("character/status", statusScreenFixtureNamed(statusLongestName), width)
		if err != nil {
			t.Fatal(err)
		}
		t.Logf("at %d the widest line is %d:\n%s", width, templates.WidestLine(out), out)
		for _, line := range strings.Split(strings.ReplaceAll(out, "\r\n", "\n"), "\n") {
			got := templates.WidestLine(line)
			if got <= width {
				continue
			}
			if width != 30 || !strings.Contains(line, statusLongestName) || got != 4+len(statusLongestName) {
				t.Errorf("at %d a line other than the bare name line is %d wide: %q", width, got, line)
			}
		}
	}
}

// characterScreenGoldens are the screens whose 80-column output is pinned:
// each fixture's base fits 80, so the golden is the base as master drew it.
var characterScreenGoldens = map[string]templateFixture{
	"character/status":    {setup: seedCharacterScreenRegistries, data: statusScreenFixture},
	"character/inventory": {data: inventoryScreenFixture(false)},
	"character/skills":    {data: skillsScreenFixture},
	"character/cooldowns": {data: cooldownsScreenFixture},
	"character/storage":   {data: storageScreenFixture},
}

func characterScreenGoldenPath(name string) string {
	return testdataPath(filepath.Join("character_screens_80", strings.ReplaceAll(name, "/", "_")+".golden"))
}

// A desktop sees what it always did: at 80 columns each character screen
// renders exactly the bytes master drew, line ends aside (a Windows checkout
// reads the templates with CRLF).
func TestCharacterScreensKeepTheir80ColumnOutput(t *testing.T) {
	templateRenderSetup(t)
	for name, fixture := range characterScreenGoldens {
		t.Run(name, func(t *testing.T) {
			if fixture.setup != nil {
				fixture.setup(t)
			}
			out, err := templates.ProcessAtWidth(name, fixture.data(), 80)
			if err != nil {
				t.Fatal(err)
			}
			got := strings.ReplaceAll(out, "\r\n", "\n")
			path := characterScreenGoldenPath(name)
			if *updateCharacterScreens {
				if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden: %v (record it from master with -update-character-screens)", err)
			}
			if w := templates.WidestLine(string(want)); w > 80 {
				t.Fatalf("the golden is %d wide: its fixture no longer fits 80, so it pins a form, not the base", w)
			}
			if got != string(want) {
				t.Errorf("%s at 80 changed:\n%s", name, lineDiff(string(want), got))
			}
		})
	}
}

// lineDiff names the first lines that differ.
func lineDiff(want, got string) string {
	w, g := strings.Split(want, "\n"), strings.Split(got, "\n")
	var b strings.Builder
	shown := 0
	for i := 0; i < max(len(w), len(g)) && shown < 6; i++ {
		var wl, gl string
		if i < len(w) {
			wl = w[i]
		}
		if i < len(g) {
			gl = g[i]
		}
		if wl != gl {
			fmt.Fprintf(&b, "line %d\n want %q\n got  %q\n", i+1, wl, gl)
			shown++
		}
	}
	return b.String()
}
