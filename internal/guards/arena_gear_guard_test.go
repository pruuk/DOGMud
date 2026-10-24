package guards

import (
	"regexp"
	"strings"
	"testing"

	"github.com/GoMudEngine/GoMud/internal/configs"
	"github.com/GoMudEngine/GoMud/internal/fileloader"
	"github.com/GoMudEngine/GoMud/internal/items"
	"github.com/GoMudEngine/GoMud/internal/mobs"
	"github.com/GoMudEngine/GoMud/internal/mudlog"
)

// arenaZone is the zone whose mobs respawn every round. The owner ruled
// (2026-10-17) that the arena's quick respawn is an intended gold farm, so
// any gear an arena mob wears from its template must never drop, or every
// kill would also hand out a free item.
const arenaZone = "Instance Arena"

// armedText matches mob text that puts a weapon in the mob's hands.
var armedText = regexp.MustCompile(`(?i)\b(blades?|swords?|knife|knives|mace|axe|spear)\b`)

// shieldText matches mob text that puts a shield on the mob's arm.
var shieldText = regexp.MustCompile(`(?i)\bshields?\b`)

// TestArenaMobsCarryTheGearTheirTextNames is the content guard for two rules:
//
//  1. A mob whose description, idle or combat text names a weapon (or a
//     shield) gets one, either worn from its template or from its
//     loot_pool. An unarmed mob is narrated with its fists, which
//     contradicts a cutthroat "flipping a blade" (2026-10-17 playtest).
//     loot_pool counts because every arena entry is paid, and a paid spawn
//     generates and wears each loot_pool item (rooms.SpawnMob); that is how
//     the Champion gets its mace and tower shield.
//  2. Every item an arena mob wears from its template is NeverDrops, so the
//     intended gold farm does not turn into an item farm. Paid-run loot
//     comes from loot_pool, which keeps the mob's itemdropchance.
//
// It loads the real items and every mob template, and checks every mob in
// the arena zone, so a new arena mob is held to the same rules.
func TestArenaMobsCarryTheGearTheirTextNames(t *testing.T) {
	mudlog.SetupLogger(nil, `LOW`, ``, false)

	configs.SetConfigForTest(t, configs.GetConfig())
	if err := configs.ReloadConfig(); err != nil {
		t.Fatalf("ReloadConfig: %v", err)
	}

	t.Cleanup(items.SeedItemsForTest(nil))
	items.LoadDataFiles()

	dataPath := configs.GetFilePathsConfig().DataFiles.String() + `/mobs`
	templates, err := fileloader.LoadAllFlatFiles[int, *mobs.Mob](dataPath)
	if err != nil {
		t.Fatalf("loading mob templates from %s: %v", dataPath, err)
	}

	checked := 0
	for id, m := range templates {
		if m.Zone != arenaZone {
			continue
		}
		checked++

		text := strings.Join(append(append([]string{m.Character.Description},
			m.IdleCommands...), m.CombatCommands...), "\n")
		hasWeapon := m.Character.Equipment.Weapon.ItemId != 0
		hasShield := m.Character.Equipment.Offhand.ItemId != 0
		for _, poolId := range m.LootPool {
			spec := items.GetItemSpec(poolId)
			if spec == nil {
				t.Errorf("mob %d (%s): loot_pool item %d does not exist",
					id, m.Character.Name, poolId)
				continue
			}
			switch spec.Type {
			case items.Weapon:
				hasWeapon = true
			case items.Offhand:
				hasShield = true
			}
		}
		if armedText.MatchString(text) && !hasWeapon {
			t.Errorf("mob %d (%s): its text names a weapon (%q) but neither its template nor its loot_pool gives it one",
				id, m.Character.Name, armedText.FindString(text))
		}
		if shieldText.MatchString(text) && !hasShield {
			t.Errorf("mob %d (%s): its text names a shield but neither its template nor its loot_pool gives it one",
				id, m.Character.Name)
		}

		for _, worn := range m.Character.Equipment.GetAllItems() {
			spec := items.GetItemSpec(worn.ItemId)
			if spec == nil {
				t.Errorf("mob %d (%s): wears item %d, which does not exist",
					id, m.Character.Name, worn.ItemId)
				continue
			}
			if !spec.NeverDrops {
				t.Errorf("mob %d (%s): wears %d (%s), which can drop; arena template gear must be never_drops",
					id, m.Character.Name, worn.ItemId, spec.Name)
			}
		}
	}
	if checked == 0 {
		t.Fatalf("no mobs found in zone %q: the guard is not seeing the arena", arenaZone)
	}
}
